package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
)

// Embedding represents a row with columns id, content, and embedding.
type Embedding struct {
	// PK is the primary key column name
	PK interface{}
	// Content is what vector embeddings are created from
	Content string
	// Embedding is the vector embedding for the content
	Embedding pgvector.Vector
}

// CreateEmbedding inserts embeddings into the table based on the contentSelectQuery
// If no contentSelectQuery is supplied, it assumes the content column of the table is already populated, and use content column directly
// If an empty contentSelectQuery is provided, it queries the table rows and constructs the content column as col1_name:col1_value,col2_name:col2_value,...
// For a non-empty contentSelectQuery, it expects the query to return two columns: primary key and content.
func (c *Client) CreateEmbedding(ctx context.Context, contentSelectQuery ...string) error {
	var query string

	// Ensure the table exists and has the required columns
	if err := c.ensureTableConfig(ctx); err != nil {
		return err
	}

	// Check the query conditions
	if len(contentSelectQuery) == 0 {
		// Case 1: No query supplied, assume content is already populated
		query = fmt.Sprintf("SELECT %s, content FROM %s", c.Config.TablePrimaryKeyCol, c.Config.TableName)
	} else if contentSelectQuery[0] == "" {
		// Case 2: Empty string query, construct content column
		schema, tableName := splitSchemaTableName(c.Config.TableName)
		c.logger.Info("Query is empty, using table columns as content", zap.String("table", c.Config.TableName))
		columns, err := c.queryAndFilterColumnNames(ctx, schema, tableName, []string{"embedding", "content"})
		if err != nil {
			return err
		}
		query = fmt.Sprintf("SELECT %s FROM %s", strings.Join(columns, ", "), c.Config.TableName)
	} else {
		// Case 3: Non-empty query
		query = contentSelectQuery[0]
	}

	// queryAndProcessEmbeddingContents to process contents and ids
	rows, err := c.queryAndProcessEmbeddingContents(ctx, query)
	if err != nil {
		return err
	}
	c.logger.Debug("contents", zap.Any("contents", rows))

	contents := []string{}
	for _, embedding := range rows {
		contents = append(contents, embedding.Content)
	}

	embeddings, err := c.FetchEmbedding(ctx, contents)
	if err != nil {
		return fmt.Errorf("failed to fetch embeddings: %w", err)
	}
	c.logger.Debug("embeddings", zap.Int("embeddings", len(embeddings)))

	// Ensure the lengths match
	if len(contents) != len(embeddings) {
		return fmt.Errorf("mismatch between contents and embeddings length: %d vs %d", len(contents), len(embeddings))
	}

	// update the embeddings into the database
	for i := range contents { // Use index to access both contents and ids
		embedding := embeddings[i]
		pk := rows[i].PK

		c.logger.Debug("embedding", zap.Any("id", pk), zap.String("content", contents[i]), zap.Any("embedding[0]", embedding[0]))

		// Call the helper function to update the embedding
		if err := c.updateEmbedding(ctx, embedding, contents[i], pk); err != nil {
			return err
		}
	}

	return nil
}

// Retrieve retrieves the most similar rows to the input
func (c *Client) Retrieve(ctx context.Context, input string, limit int) ([]Embedding, error) {
	// Step 1: Fetch the embedding for the input text
	embedding, err := c.FetchEmbedding(ctx, []string{input})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch embedding for input: %w", err)
	}

	// Step 2: Query the database using the fetched embedding
	queryStr := fmt.Sprintf(
		"SELECT id, content, embedding FROM %s ORDER BY embedding <=> $1 LIMIT $2",
		c.Config.TableName,
	)

	// Execute the query with the embedding
	rows, err := c.conn.Query(ctx, queryStr, pgvector.NewVector(embedding[0]), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Collect the results
	var results []Embedding
	for rows.Next() {
		var embedding Embedding
		if err := rows.Scan(&embedding.PK, &embedding.Content, &embedding.Embedding); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, embedding)
	}

	return results, nil
}

// ensureTableConfig ensures the table exists and has the required columns
func (c *Client) ensureTableConfig(ctx context.Context) error {
	tableExists, err := c.tableExists(ctx, c.Config.TableName)
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if !tableExists {
		err = c.createTable(ctx, c.Config.TableName)
		if err != nil {
			// Check if the error is because the table already exists
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42P07" {
				// Table already exists, so we can proceed
				tableExists = true
			} else {
				return fmt.Errorf("failed to create table: %w", err)
			}
		}
	}

	if tableExists {
		err = c.ensureRequiredColumnsExist(ctx, c.Config.TableName)
		if err != nil {
			return fmt.Errorf("failed to ensure embedding column: %w", err)
		}
	}

	return nil
}

// ensureRequiredColumnsExist ensures the table has the required columns
func (c *Client) ensureRequiredColumnsExist(ctx context.Context, tableName string) error {
	// Check for 'embedding' column
	var embeddingColumnExists bool
	err := c.conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name=$1 AND column_name='embedding')", tableName).Scan(&embeddingColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check for embedding column: %w", err)
	}

	// Check for 'content' column
	var contentColumnExists bool
	err = c.conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name=$1 AND column_name='content')", tableName).Scan(&contentColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check for content column: %w", err)
	}

	// Check for primary key column
	var pkColumnExists bool
	err = c.conn.QueryRow(ctx, fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name=$1 AND column_name='%s')", c.Config.TablePrimaryKeyCol), tableName).Scan(&pkColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check for id column: %w", err)
	}

	// Add 'embedding' column if it doesn't exist
	if !embeddingColumnExists {
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN embedding VECTOR(%d)", tableName, c.Config.Dimensions)
		_, err = c.conn.Exec(ctx, query)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42701" {
			} else {
				return fmt.Errorf("failed to add embedding column: %w", err)
			}
		}
	}

	// Add 'content' column if it doesn't exist
	if !contentColumnExists {
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN content TEXT", tableName)
		_, err = c.conn.Exec(ctx, query)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42701" {
			} else {
				return fmt.Errorf("failed to add content column: %w", err)
			}
		}
	}

	// Add primary key column if it doesn't exist
	if !pkColumnExists {
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s BIGINT PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY", tableName, c.Config.TablePrimaryKeyCol)
		_, err = c.conn.Exec(ctx, query)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42701" {
			} else {
				return fmt.Errorf("failed to add content column: %w", err)
			}
		}
	}

	return nil
}

// tableExists checks if the table exists
func (c *Client) tableExists(ctx context.Context, tableName string) (bool, error) {
	var exists bool
	err := c.conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name=$1)", tableName).Scan(&exists)
	return exists, err
}

// createTable creates the table if it doesn't exist
func (c *Client) createTable(ctx context.Context, tableName string) error {
	query := fmt.Sprintf(`
		CREATE TABLE %s (
			id BIGINT PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,
			content TEXT,
			embedding VECTOR(%d)
		)`, tableName, c.Config.Dimensions)

	_, err := c.conn.Exec(ctx, query)
	return err
}

// formatRowValues formats the row values into a string
func formatRowValues(fields []pgconn.FieldDescription, values []interface{}) string {
	var pairs []string
	for i, field := range fields {
		pairs = append(pairs, fmt.Sprintf("%s:%v", field.Name, values[i]))
	}
	return strings.Join(pairs, ",")
}

func splitSchemaTableName(tableName string) (string, string) {
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "public", tableName
}

func (c *Client) updateEmbedding(ctx context.Context, embedding []float32, content string, id interface{}) error {
	// Prepare update query
	query := fmt.Sprintf(`
		UPDATE %s 
		SET embedding = $1, content = $2 
		WHERE %s = $3
	`, c.Config.TableName, c.Config.TablePrimaryKeyCol)

	// Execute the query
	_, err := c.conn.Exec(ctx, query, pgvector.NewVector(embedding), content, id)
	if err != nil {
		return fmt.Errorf("failed to update embedding: %w", err)
	}
	return nil
}

// queryAndProcessEmbeddingContents queries the database and processes the rows to populate contents and ids.
func (c *Client) queryAndProcessEmbeddingContents(ctx context.Context, selectQuery string) ([]Embedding, error) {
	var contents []Embedding

	rows, err := c.conn.Query(ctx, selectQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("failed to get row values: %w", err)
		}

		content := formatRowValues(rows.FieldDescriptions(), values)
		contents = append(contents, Embedding{
			PK:      values[0],
			Content: content,
			// Embedding: values[2].(pgvector.Vector),
		})
	}

	return contents, nil
}

func (c *Client) queryAndFilterColumnNames(ctx context.Context, schema, tableName string, toRemove []string) ([]string, error) {
	columnsQuery := fmt.Sprintf(`
		SELECT column_name 
		FROM information_schema.columns
		WHERE table_schema = '%s'
		AND table_name = '%s'
	`, schema, tableName)

	rows, err := c.conn.Query(ctx, columnsQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %w", err)
	}
	defer rows.Close()

	// Create a map for columns to remove
	removeMap := make(map[string]struct{})
	for _, col := range toRemove {
		removeMap[col] = struct{}{}
	}

	var filtered []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, fmt.Errorf("failed to scan column name: %w", err)
		}
		// Only append if the column is not in the remove map
		if _, found := removeMap[column]; !found {
			filtered = append(filtered, column)
		}
	}
	return filtered, nil
}

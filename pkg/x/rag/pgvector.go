package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
)

// Embedding represents a piece of content (e.g. a document, a sentence, base64-encoded image, etc.)
// with its embedding and a unique ID (primary key)
// In a PostgreSQL table, it represents a row with columns: primary key, content, and embedding.
type Embedding struct {
	// PK is the primary key column name
	// pk can be named anything (e.g. id, uuid, etc.) and of any type (e.g. int, string, etc.)
	// allowing usage of existing indexes on the primary key
	PK interface{}
	// Content is what vector embeddings are created from
	Content string
	// Embedding is the vector embedding for the content
	Embedding pgvector.Vector
}

// CreateEmbedding inserts embeddings into the table based on the (optional) contentSelectQuery argument
// If no contentSelectQuery is supplied, it assumes the content column of the table is already populated, and use content column directly
// If an empty(`""`) contentSelectQuery is provided, it queries the table rows and constructs the content column as col1_name:col1_value,col2_name:col2_value,...
// For a non-empty contentSelectQuery, it expects the query to return two columns: primary key and content.
//
// Exectuting contentSelectQuery returns output in below format:
// primary_key, content; pk can be named anything and of any type, allowing usage of existing indexes on the primary key
//
/*
postgres=# SELECT id, CONCAT('title:', title, ', summary:', summary) AS content FROM lms.courses;
     id       |                                                     content
--------------+-----------------------------------------------------------------------------------------------------------------
 15670e34129c | title:Data Science Fundamentals, summary:Explore statistical analysis, machine learning, and data visualization
 69c26a5ace74 | title:Web Development Bootcamp, summary:Learn HTML, CSS, JavaScript, and popular frameworks
 7a354de48450 | title:Introduction to Python, summary:A comprehensive course for beginners to start their Python journey
*/
func (c *Client) CreateEmbedding(ctx context.Context, contentSelectQuery ...string) error {
	// Ensure the table configuration before proceeding
	if err := c.ensureTableConfig(ctx); err != nil {
		return fmt.Errorf("failed to ensure table configuration: %w", err)
	}

	var query string

	switch {
	case len(contentSelectQuery) == 0:
		// Case 1: No query supplied, assume content is already populated
		query = fmt.Sprintf("SELECT %s, content FROM %s", c.Config.TablePrimaryKeyCol, c.Config.TableName)
	case contentSelectQuery[0] == "":
		// Case 2: Empty string query, construct content column
		schema, tableName := splitSchemaTableName(c.Config.TableName)
		c.logger.Info("Query is empty, using table columns as content", zap.String("table", c.Config.TableName))
		columns, err := c.queryAndFilterColumnNames(ctx, schema, tableName, []string{"embedding", "content"})
		if err != nil {
			return err
		}
		query = fmt.Sprintf("SELECT %s FROM %s", strings.Join(columns, ", "), c.Config.TableName)
	default:
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
	c.logger.Info("Ensuring table configuration", zap.String("table", c.Config.TableName))

	tableExists, err := c.tableExists(ctx, c.Config.TableName)
	if err != nil {
		c.logger.Error("Failed to check if table exists", zap.Error(err))
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if !tableExists {
		c.logger.Info("Table does not exist, creating it", zap.String("table", c.Config.TableName))
		err = c.createTable(ctx, c.Config.TableName)
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	c.logger.Info("Table exists, ensuring required columns", zap.String("table", c.Config.TableName))
	err = c.ensureRequiredColumnsExist(ctx, c.Config.TableName)
	if err != nil {
		c.logger.Error("Failed to ensure required columns", zap.Error(err))
		return fmt.Errorf("failed to ensure required columns: %w", err)
	}
	c.logger.Info("Required columns ensured", zap.String("table", c.Config.TableName))

	return nil
}

// createTable creates the table if it doesn't exist
func (c *Client) createTable(ctx context.Context, tableName string) error {
	c.logger.Info("Creating table", zap.String("table", tableName))
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			%s BIGINT PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,
			content TEXT,
			embedding vector(%d)
		)`, tableName, c.Config.TablePrimaryKeyCol, c.Config.Dimensions)

	_, err := c.conn.Exec(ctx, query)
	if err != nil {
		c.logger.Error("Failed to create table", zap.Error(err))
		return fmt.Errorf("failed to create table: %w", err)
	}
	c.logger.Info("Table created successfully", zap.String("table", tableName))

	return nil
}

// ensureRequiredColumnsExist ensures the table has the required columns
func (c *Client) ensureRequiredColumnsExist(ctx context.Context, tableName string) error {
	schema, table := splitSchemaTableName(tableName)

	c.logger.Info("Checking required columns", zap.String("table", tableName))

	// Check for primary key column
	var pkColumnExists bool
	err := c.conn.QueryRow(ctx, fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2 AND column_name='%s')", c.Config.TablePrimaryKeyCol), schema, table).Scan(&pkColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check for primary key column: %w", err)
	}
	c.logger.Info("Primary key column check", zap.String("column", c.Config.TablePrimaryKeyCol), zap.Bool("exists", pkColumnExists))

	// Check for 'content' column
	var contentColumnExists bool
	err = c.conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2 AND column_name='content')", schema, table).Scan(&contentColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check for content column: %w", err)
	}
	c.logger.Info("Content column check", zap.Bool("exists", contentColumnExists))

	// Check for 'embedding' column
	var embeddingColumnExists bool
	err = c.conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2 AND column_name='embedding')", schema, table).Scan(&embeddingColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check for embedding column: %w", err)
	}
	c.logger.Info("Embedding column check", zap.Bool("exists", embeddingColumnExists))

	// Add primary key column if it doesn't exist and is configured
	if !pkColumnExists && c.Config.TablePrimaryKeyCol != "" {
		c.logger.Info("Adding primary key column", zap.String("table", tableName), zap.String("column", c.Config.TablePrimaryKeyCol))
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s BIGINT PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY", tableName, c.Config.TablePrimaryKeyCol)
		_, err = c.conn.Exec(ctx, query)
		if err != nil {
			c.logger.Error("Failed to add primary key column", zap.Error(err))
			return fmt.Errorf("failed to add primary key column: %w", err)
		}
		c.logger.Info("Successfully added primary key column", zap.String("table", tableName), zap.String("column", c.Config.TablePrimaryKeyCol))
	}

	// Add 'content' column if it doesn't exist
	if !contentColumnExists {
		c.logger.Info("Adding content column", zap.String("table", tableName))
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN content TEXT", tableName)
		_, err = c.conn.Exec(ctx, query)
		if err != nil {
			c.logger.Error("Failed to add content column", zap.Error(err))
			return fmt.Errorf("failed to add content column: %w", err)
		}
		c.logger.Info("Successfully added content column", zap.String("table", tableName))
	}

	// Add 'embedding' column if it doesn't exist
	if !embeddingColumnExists {
		c.logger.Info("Adding embedding column", zap.String("table", tableName))
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN embedding VECTOR(%d)", tableName, c.Config.Dimensions)
		_, err = c.conn.Exec(ctx, query)
		if err != nil {
			c.logger.Error("Failed to add embedding column", zap.Error(err))
			return fmt.Errorf("failed to add embedding column: %w", err)
		}
		c.logger.Info("Successfully added embedding column", zap.String("table", tableName))
	}

	// Final verification
	c.logger.Info("Verifying all required columns")
	err = c.conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2 AND column_name='embedding')", schema, table).Scan(&embeddingColumnExists)
	if err != nil {
		return fmt.Errorf("failed to verify embedding column: %w", err)
	}
	if !embeddingColumnExists {
		return fmt.Errorf("embedding column still does not exist after attempting to add it")
	}
	c.logger.Info("All required columns verified", zap.String("table", tableName))

	return nil
}

// tableExists checks if the table exists
func (c *Client) tableExists(ctx context.Context, tableName string) (bool, error) {
	schema, table := splitSchemaTableName(tableName)

	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1
			AND table_name = $2
		)`

	err := c.conn.QueryRow(ctx, query, schema, table).Scan(&exists)
	return exists, err
}

// splitSchemaTableName splits a schema-qualified table name into schema and table parts
func splitSchemaTableName(tableName string) (string, string) {
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "public", tableName
}

// formatRowValues formats the row values into a string
func formatRowValues(fields []pgconn.FieldDescription, values []interface{}) string {
	var pairs []string
	for i, field := range fields {
		pairs = append(pairs, fmt.Sprintf("%s:%v", field.Name, values[i]))
	}
	return strings.Join(pairs, ",")
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

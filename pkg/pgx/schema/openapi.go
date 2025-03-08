package schema

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// OpenAPIInfo contains API metadata for the OpenAPI specification
type OpenAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Contact     struct {
		Name  string `json:"name,omitempty"`
		Email string `json:"email,omitempty"`
		URL   string `json:"url,omitempty"`
	} `json:"contact,omitempty"`
}

// OpenAPIGenerator generates OpenAPI specs from the schema cache
type OpenAPIGenerator struct {
	cache   *Cache
	baseURL string
	info    OpenAPIInfo
}

// NewOpenAPIGenerator creates a new OpenAPI generator
func NewOpenAPIGenerator(cache *Cache, baseURL string, info OpenAPIInfo) *OpenAPIGenerator {
	return &OpenAPIGenerator{
		cache:   cache,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		info:    info,
	}
}

// ServeHTTP implements http.Handler to serve the OpenAPI specification
func (g *OpenAPIGenerator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	spec := g.GenerateSpecification()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(spec)
}

// GenerateSpecification creates a complete OpenAPI specification
func (g *OpenAPIGenerator) GenerateSpecification() map[string]interface{} {
	tables := g.cache.Snapshot()
	paths := make(map[string]interface{})
	schemas := make(map[string]interface{})

	// Add paths for each table
	for tableKey, table := range tables {
		tablePath := fmt.Sprintf("/%s", table.Name)
		if table.Schema != "public" {
			tablePath = fmt.Sprintf("/%s/%s", table.Schema, table.Name)
		}

		// Build paths for table-level operations
		paths[tablePath] = g.buildTableOperations(table)

		// Build paths for record-level operations (if table has primary keys)
		if len(table.PrimaryKeys) > 0 {
			recordPath := g.buildRecordPath(table)
			paths[recordPath] = g.buildRecordOperations(table)
		}

		// Add schema definition for this table
		schemas[g.getSchemaRef(tableKey)] = g.buildTableSchema(table)
	}

	// Assemble complete specification
	return map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":       g.info.Title,
			"description": g.info.Description,
			"version":     g.info.Version,
			"contact": map[string]interface{}{
				"name":  g.info.Contact.Name,
				"email": g.info.Contact.Email,
				"url":   g.info.Contact.URL,
			},
		},
		"servers": []map[string]interface{}{
			{
				"url":         g.baseURL,
				"description": "API Server",
			},
		},
		"paths": paths,
		"components": map[string]interface{}{
			"schemas": schemas,
		},
	}
}

// buildTableOperations defines the operations available on a table resource
func (g *OpenAPIGenerator) buildTableOperations(table Table) map[string]interface{} {
	schemaRef := g.getSchemaRef(table.fullName())

	return map[string]interface{}{
		"get": map[string]interface{}{
			"summary":     fmt.Sprintf("List %s records", table.Name),
			"description": fmt.Sprintf("Retrieves records from %s.%s", table.Schema, table.Name),
			"parameters":  g.buildQueryParameters(table),
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "Success",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"type":  "array",
								"items": map[string]string{"$ref": fmt.Sprintf("#/components/schemas/%s", schemaRef)},
							},
						},
					},
				},
				"400": map[string]string{"description": "Bad Request"},
				"401": map[string]string{"description": "Unauthorized"},
				"404": map[string]string{"description": "Not Found"},
			},
			"tags": []string{table.Schema},
		},
		"post": map[string]interface{}{
			"summary":     fmt.Sprintf("Create %s record", table.Name),
			"description": fmt.Sprintf("Creates a new record in %s.%s", table.Schema, table.Name),
			"requestBody": map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]string{"$ref": fmt.Sprintf("#/components/schemas/%s", schemaRef)},
					},
				},
				"required": true,
			},
			"responses": map[string]interface{}{
				"201": map[string]interface{}{
					"description": "Created",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]string{"$ref": fmt.Sprintf("#/components/schemas/%s", schemaRef)},
						},
					},
				},
				"400": map[string]string{"description": "Bad Request"},
				"401": map[string]string{"description": "Unauthorized"},
				"409": map[string]string{"description": "Conflict"},
			},
			"tags": []string{table.Schema},
		},
	}
}

// buildRecordOperations defines operations available on a single record
func (g *OpenAPIGenerator) buildRecordOperations(table Table) map[string]interface{} {
	schemaRef := g.getSchemaRef(table.fullName())

	return map[string]interface{}{
		"get": map[string]interface{}{
			"summary":     fmt.Sprintf("Get %s record", table.Name),
			"description": fmt.Sprintf("Retrieves a single record from %s.%s by primary key", table.Schema, table.Name),
			"parameters":  g.buildPrimaryKeyParameters(table),
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "Success",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]string{"$ref": fmt.Sprintf("#/components/schemas/%s", schemaRef)},
						},
					},
				},
				"404": map[string]string{"description": "Not Found"},
			},
			"tags": []string{table.Schema},
		},
		"patch": map[string]interface{}{
			"summary":     fmt.Sprintf("Update %s record", table.Name),
			"description": fmt.Sprintf("Updates a record in %s.%s by primary key", table.Schema, table.Name),
			"parameters":  g.buildPrimaryKeyParameters(table),
			"requestBody": map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]string{"$ref": fmt.Sprintf("#/components/schemas/%s", schemaRef)},
					},
				},
				"required": true,
			},
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "Success",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]string{"$ref": fmt.Sprintf("#/components/schemas/%s", schemaRef)},
						},
					},
				},
				"400": map[string]string{"description": "Bad Request"},
				"404": map[string]string{"description": "Not Found"},
			},
			"tags": []string{table.Schema},
		},
		"delete": map[string]interface{}{
			"summary":     fmt.Sprintf("Delete %s record", table.Name),
			"description": fmt.Sprintf("Deletes a record from %s.%s by primary key", table.Schema, table.Name),
			"parameters":  g.buildPrimaryKeyParameters(table),
			"responses": map[string]interface{}{
				"200": map[string]string{"description": "Success"},
				"404": map[string]string{"description": "Not Found"},
			},
			"tags": []string{table.Schema},
		},
	}
}

// buildRecordPath generates the path for a single record with primary keys
func (g *OpenAPIGenerator) buildRecordPath(table Table) string {
	path := fmt.Sprintf("/%s", table.Name)
	if table.Schema != "public" {
		path = fmt.Sprintf("/%s/%s", table.Schema, table.Name)
	}

	for _, key := range table.PrimaryKeys {
		path += fmt.Sprintf("/{%s}", key)
	}

	return path
}

// buildQueryParameters generates common query parameters for table operations
func (g *OpenAPIGenerator) buildQueryParameters(table Table) []map[string]interface{} {
	params := []map[string]interface{}{
		{
			"name":        "limit",
			"in":          "query",
			"description": "Limit the number of returned records",
			"schema":      map[string]string{"type": "integer"},
		},
		{
			"name":        "offset",
			"in":          "query",
			"description": "Offset for pagination",
			"schema":      map[string]string{"type": "integer"},
		},
		{
			"name":        "order",
			"in":          "query",
			"description": "Order by column(s)",
			"schema":      map[string]string{"type": "string"},
		},
	}

	// Add column-specific filters
	for _, col := range table.Columns {
		params = append(params, map[string]interface{}{
			"name":        col.Name,
			"in":          "query",
			"description": fmt.Sprintf("Filter by %s", col.Name),
			"schema":      g.getParameterSchema(col),
		})
	}

	return params
}

// buildPrimaryKeyParameters generates path parameters for primary keys
func (g *OpenAPIGenerator) buildPrimaryKeyParameters(table Table) []map[string]interface{} {
	params := []map[string]interface{}{}

	for _, key := range table.PrimaryKeys {
		var col Column
		for _, c := range table.Columns {
			if c.Name == key {
				col = c
				break
			}
		}

		params = append(params, map[string]interface{}{
			"name":        key,
			"in":          "path",
			"required":    true,
			"description": fmt.Sprintf("Primary key %s", key),
			"schema":      g.getParameterSchema(col),
		})
	}

	return params
}

// buildTableSchema generates the schema definition for a table
func (g *OpenAPIGenerator) buildTableSchema(table Table) map[string]interface{} {
	properties := make(map[string]interface{})
	required := []string{}

	for _, col := range table.Columns {
		properties[col.Name] = g.getColumnSchema(col)

		if !col.IsNullable {
			required = append(required, col.Name)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// getColumnSchema maps PostgreSQL data types to OpenAPI schema types
func (g *OpenAPIGenerator) getColumnSchema(col Column) map[string]interface{} {
	schema := make(map[string]interface{})

	switch {
	case strings.Contains(col.DataType, "int"):
		schema["type"] = "integer"
		if strings.Contains(col.DataType, "smallint") {
			schema["format"] = "int16"
		} else if strings.Contains(col.DataType, "bigint") {
			schema["format"] = "int64"
		} else {
			schema["format"] = "int32"
		}
	case strings.Contains(col.DataType, "numeric"), strings.Contains(col.DataType, "decimal"),
		strings.Contains(col.DataType, "real"), strings.Contains(col.DataType, "double"):
		schema["type"] = "number"
		if strings.Contains(col.DataType, "double") {
			schema["format"] = "double"
		} else {
			schema["format"] = "float"
		}
	case strings.Contains(col.DataType, "bool"):
		schema["type"] = "boolean"
	case strings.Contains(col.DataType, "date"):
		schema["type"] = "string"
		schema["format"] = "date"
	case strings.Contains(col.DataType, "timestamp"):
		schema["type"] = "string"
		schema["format"] = "date-time"
	case strings.Contains(col.DataType, "time"):
		schema["type"] = "string"
		schema["format"] = "time"
	case strings.Contains(col.DataType, "uuid"):
		schema["type"] = "string"
		schema["format"] = "uuid"
	case strings.Contains(col.DataType, "json"), strings.Contains(col.DataType, "jsonb"):
		schema["type"] = "object"
		schema["additionalProperties"] = true
	case strings.Contains(col.DataType, "char"), strings.Contains(col.DataType, "text"):
		schema["type"] = "string"
	default:
		schema["type"] = "string"
	}

	return schema
}

// getParameterSchema gets schema for query parameters
func (g *OpenAPIGenerator) getParameterSchema(col Column) map[string]string {
	switch {
	case strings.Contains(col.DataType, "int"), strings.Contains(col.DataType, "numeric"),
		strings.Contains(col.DataType, "decimal"), strings.Contains(col.DataType, "real"),
		strings.Contains(col.DataType, "double"):
		return map[string]string{"type": "number"}
	case strings.Contains(col.DataType, "bool"):
		return map[string]string{"type": "boolean"}
	default:
		return map[string]string{"type": "string"}
	}
}

// getSchemaRef generates a reference name for a table schema
func (g *OpenAPIGenerator) getSchemaRef(tableKey string) string {
	return strings.Replace(tableKey, ".", "_", -1)
}

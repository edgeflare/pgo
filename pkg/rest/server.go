package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/edgeflare/pgo/pkg/httputil"
	"github.com/edgeflare/pgo/pkg/pgx/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	pool        *pgxpool.Pool
	mux         *http.ServeMux
	schemaCache *schema.Cache
	baseURL     string
}

func NewServer(connString string, baseURL string) (*Server, error) {
	schemaCache, err := schema.NewCache(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create schema cache: %w", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	mux := http.NewServeMux()

	server := &Server{
		pool:        pool,
		mux:         mux,
		schemaCache: schemaCache,
		baseURL:     baseURL,
	}

	server.registerHandlers()

	return server, nil
}

func (s *Server) registerHandlers() {
	s.mux.HandleFunc("/", s.handleRequest)
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Extract path components to determine schema and table
	path := strings.TrimPrefix(r.URL.Path, s.baseURL)
	if path == "" || path == "/" {
		// Root path - could return API documentation or a list of schemas
		// TODO: generate openAPI spec
		httputil.JSON(w, http.StatusOK, map[string]string{"message": "PGO API Server"})
		return
	}

	// Parse path to extract schema and table
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathParts) < 1 {
		httputil.Error(w, http.StatusBadRequest, "Invalid path format")
		return
	}

	// Determine if this is a schema request or a table request
	var schemaName, tableName string
	if len(pathParts) == 1 {
		// Default to public schema if only table is provided
		schemaName = "public"
		tableName = pathParts[0]
	} else {
		schemaName = pathParts[0]
		tableName = pathParts[1]
	}

	// Get table schema from cache
	tableKey := fmt.Sprintf("%s.%s", schemaName, tableName)
	tableSchema, exists := s.schemaCache.Snapshot()[tableKey]
	if !exists {
		httputil.Error(w, http.StatusNotFound, fmt.Sprintf("Table %s not found", tableKey))
		return
	}

	// Handle request based on method
	switch r.Method {
	case http.MethodGet:
		s.handleGet(w, r, tableSchema)
	case http.MethodPost:
		s.handlePost(w, r, tableSchema)
	case http.MethodPatch:
		s.handlePatch(w, r, tableSchema)
	case http.MethodDelete:
		s.handleDelete(w, r, tableSchema)
	default:
		httputil.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
	}

	// Log request
	log.Printf("%s %s %s %s", r.Method, r.URL.Path, r.RemoteAddr, time.Since(startTime))
}

// handleGet processes GET requests to fetch data
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, table schema.Table) {
	params := parseQueryParams(r)

	query, args, err := buildSelectQuery(table, params)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Database query error")
		log.Printf("Query error: %v", err)
		return
	}
	defer rows.Close()

	results, err := pgRowsToJSON(rows)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Error processing results")
		log.Printf("Results processing error: %v", err)
		return
	}

	httputil.JSON(w, http.StatusOK, results)
}

// handlePost processes POST requests to insert data
func (s *Server) handlePost(w http.ResponseWriter, r *http.Request, table schema.Table) {
	var data map[string]any
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	query, args, err := buildInsertQuery(table, data)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Database query error")
		log.Printf("Query error: %v", err)
		return
	}
	defer rows.Close()

	results, err := pgRowsToJSON(rows)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Error processing results")
		log.Printf("Results processing error: %v", err)
		return
	}

	httputil.JSON(w, http.StatusCreated, results)
}

// handlePatch processes PATCH requests to update data
func (s *Server) handlePatch(w http.ResponseWriter, r *http.Request, table schema.Table) {
	params := parseQueryParams(r)

	var data map[string]any
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	query, args, err := buildUpdateQuery(table, data, params)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Database query error")
		log.Printf("Query error: %v", err)
		return
	}
	defer rows.Close()

	results, err := pgRowsToJSON(rows)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Error processing results")
		log.Printf("Results processing error: %v", err)
		return
	}

	httputil.JSON(w, http.StatusOK, results)
}

// handleDelete processes DELETE requests
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, table schema.Table) {
	params := parseQueryParams(r)

	query, args, err := buildDeleteQuery(table, params)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Database query error")
		log.Printf("Query error: %v", err)
		return
	}
	defer rows.Close()

	results, err := pgRowsToJSON(rows)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "Error processing results")
		log.Printf("Results processing error: %v", err)
		return
	}

	httputil.JSON(w, http.StatusOK, results)
}

// Start server on the given address
func (s *Server) Start(addr string) error {
	if err := s.schemaCache.Init(context.Background()); err != nil {
		return fmt.Errorf("failed to initialize schema cache: %w", err)
	}

	go func() {
		for tables := range s.schemaCache.Watch() {
			log.Printf("Schema cache updated: %d tables", len(tables))
		}
	}()

	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// Gracefully shut down server
func (s *Server) Shutdown() {
	log.Println("Server shutting down")
	s.schemaCache.Close()
	s.pool.Close()
}

func pgRowsToJSON(rows pgx.Rows) ([]map[string]any, error) {
	fieldDescriptions := rows.FieldDescriptions()
	columnNames := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columnNames[i] = string(fd.Name)
	}

	var result []map[string]any

	for rows.Next() {
		values := make([]any, len(columnNames))
		valuePointers := make([]any, len(columnNames))
		for i := range values {
			valuePointers[i] = &values[i]
		}

		if err := rows.Scan(valuePointers...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]any)
		for i, name := range columnNames {
			rowMap[name] = values[i]
		}
		result = append(result, rowMap)
	}

	return result, nil
}

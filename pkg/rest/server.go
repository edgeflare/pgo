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
	mw "github.com/edgeflare/pgo/pkg/httputil/middleware"
	"github.com/edgeflare/pgo/pkg/pgx/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	pool        *pgxpool.Pool
	mux         *http.ServeMux
	schemaCache *schema.Cache
	baseURL     string
	middleware  []httputil.Middleware
	httpServer  *http.Server
}

func NewServer(connString, baseURL string) (*Server, error) {
	schemaCache, err := schema.NewCache(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create schema cache: %w", err)
	}

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	server := &Server{
		pool:        pool,
		mux:         http.NewServeMux(),
		schemaCache: schemaCache,
		baseURL:     baseURL,
	}

	server.registerHandlers()

	return server, nil
}

func (s *Server) SetMiddlewares(middleware []httputil.Middleware) {
	s.middleware = middleware
}

func (s *Server) registerHandlers() {
	s.mux.HandleFunc("/", s.wrapWithMiddleware(s.handleRequest))
}

func (s *Server) wrapWithMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mw.Chain(handler, s.middleware...).ServeHTTP(w, r)
	}
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		log.Printf("%s %s %s %s", r.Method, r.URL.Path, r.RemoteAddr, time.Since(startTime))
	}()

	path := strings.TrimPrefix(r.URL.Path, s.baseURL)
	if path == "" || path == "/" {
		httputil.JSON(w, http.StatusOK, map[string]string{"message": "PGO API Server"})
		return
	}

	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathParts) < 1 {
		httputil.Error(w, http.StatusBadRequest, "Invalid path format")
		return
	}

	schemaName := "public"
	tableName := pathParts[0]
	if len(pathParts) > 1 {
		schemaName = pathParts[0]
		tableName = pathParts[1]
	}

	tableKey := fmt.Sprintf("%s.%s", schemaName, tableName)
	tableSchema, exists := s.schemaCache.Snapshot()[tableKey]
	if !exists {
		httputil.Error(w, http.StatusNotFound, fmt.Sprintf("Table %s not found", tableKey))
		return
	}

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
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, table schema.Table) {
	params := parseQueryParams(r)

	query, args, err := buildSelectQuery(table, params)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	s.executeQuery(w, r, query, args)
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request, table schema.Table) {
	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	query, args, err := buildInsertQuery(table, data)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	s.executeQuery(w, r, query, args)
}

func (s *Server) handlePatch(w http.ResponseWriter, r *http.Request, table schema.Table) {
	params := parseQueryParams(r)

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	query, args, err := buildUpdateQuery(table, data, params)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	s.executeQuery(w, r, query, args)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, table schema.Table) {
	params := parseQueryParams(r)

	query, args, err := buildDeleteQuery(table, params)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	s.executeQuery(w, r, query, args)
}

func (s *Server) executeQuery(w http.ResponseWriter, r *http.Request, query string, args []any) {
	_, conn, err := httputil.ConnWithRole(r)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer conn.Release()

	rows, pgErr := conn.Query(r.Context(), query, args...)
	if pgErr != nil {
		log.Printf("Query error: %v", pgErr)
		httputil.Error(w, http.StatusInternalServerError, "Database query error")
		return
	}
	defer rows.Close()

	results, pgErr := pgx.CollectRows(rows, pgx.RowToMap)
	if pgErr != nil {
		httputil.Error(w, http.StatusInternalServerError, "Error collecting results")
		return
	}

	status := http.StatusOK
	if r.Method == http.MethodPost {
		status = http.StatusCreated
	}
	httputil.JSON(w, status, results)
}

func (s *Server) Start(addr string) error {
	if err := s.schemaCache.Init(context.Background()); err != nil {
		return fmt.Errorf("failed to initialize schema cache: %w", err)
	}

	go func() {
		for range s.schemaCache.Watch() {
			// Just consume the updates
		}
	}()

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}

	log.Printf("Server starting on %s", addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.schemaCache.Close()
	err := s.httpServer.Shutdown(ctx)
	s.pool.Close()
	return err
}

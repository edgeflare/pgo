package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

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
	server.addOpenAPIEndpoint()

	return server, nil
}

func (s *Server) AddMiddleware(middleware ...httputil.Middleware) {
	s.middleware = append(s.middleware, middleware...)
}

func (s *Server) registerHandlers() {
	s.mux.HandleFunc("/", s.wrapWithMiddleware(s.handleRequest))
}

func (s *Server) wrapWithMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mw.Add(handler, s.middleware...).ServeHTTP(w, r)
	}
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// startTime := time.Now()
	// defer func(rec *middleware.ResponseRecorder) {
	// 	pgRole, ok := r.Context().Value(httputil.OIDCRoleClaimCtxKey).(string)
	// 	if !ok {
	// 		pgRole = "unknown"
	// 	}
	// 	log.Printf("%s %s %s %s %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(startTime), pgRole, r.UserAgent(), rec.StatusCode)
	// }(middleware.NewResponseRecorder(w))

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
	_, conn, pgErr := httputil.ConnWithRole(r)
	if pgErr != nil {
		httputil.Error(w, http.StatusInternalServerError, pgErr.Error())
		return
	}
	defer conn.Release()

	pgRole, ok := r.Context().Value(httputil.OIDCRoleClaimCtxKey).(string)
	if !ok || pgRole == "" {
		log.Println("pgrole not found in OIDC claims")
		httputil.Error(w, http.StatusUnauthorized, "pgrole not found in OIDC claims")
		return
	}

	rows, err := conn.Query(r.Context(), query, args...)
	if err != nil {
		log.Printf("TODO - map pg-err to http status: query error: %+v", err)
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("%s pgrole: %s", err.Error(), pgRole)) // debug
		return
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		log.Printf("TODO - map pg-err to http status: parse error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("%s pgrole: %s", err.Error(), pgRole)) // debug
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
			log.Println("reloaded schema cache")
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

func (s *Server) addOpenAPIEndpoint() {
	info := schema.OpenAPIInfo{
		Title:       "PGO REST API",
		Description: "Auto-generated REST API for PostgreSQL",
		Version:     "1.0.0",
	}
	info.Contact.Name = "edgeflare.io"
	info.Contact.Email = "support@edgeflare.io"

	openAPIGenerator := schema.NewOpenAPIGenerator(s.schemaCache, s.baseURL, info)

	s.mux.Handle("/openapi.json", s.wrapWithMiddleware(openAPIGenerator.ServeHTTP))
}

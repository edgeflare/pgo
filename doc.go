// Package pgo simplifies the process of querying PostgreSQL database upon an HTTP request. It uses [jackc/pgx](https://pkg.go.dev/github.com/jackc/pgx/v5) as the PostgreSQL driver.
//
// Key Features:
//
// - HTTP Routing: A lightweight router with support for middleware, grouping, and customizable error handling.
//
// - PostgreSQL Integration: Streamlined interactions with PostgreSQL databases using pgx, a powerful PostgreSQL driver for Go.
//
//   - Convenience Functions: A collection of helper functions for common database operations (select, insert, update, etc.),
//     designed to work seamlessly within HTTP request contexts.
//
//   - Error Handling: Robust error handling mechanisms for both HTTP routing and database operations, ensuring predictable
//     and informative error responses.
//
// Example Usage:
//
//	router := pgo.NewRouter(pgo.WithTLS("cert.pem", "key.pem"))
//	router.Use(middleware.Logger)
//
//	// Define routes
//	router.Handle("GET /users", usersHandler)
//	router.Handle("POST /users", createUserHandler)
//
//	api := router.Group("/api")
//	api.Handle("GET /products", productsHandler)
//
//	// Start the server
//	router.ListenAndServe(":8080")
//
// Additional Information:
//
// - For detailed information on HTTP routing, see the documentation for the `Router` type and its associated functions.
//
// - For PostgreSQL-specific helpers and utilities, refer to the documentation for the `pgxutil` package.
package pgo

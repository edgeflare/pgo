// Package rest provides a PostgreSQL REST API server similar to PostgREST.
//
// The server automatically exposes database tables and views as REST endpoints.
// Each endpoint supports standard HTTP methods: GET, POST, PATCH, DELETE.
//
// Tables and views are exposed at /table_name or `/schema_name/table_or_view_name` path.
// If schema_name is skipped `schema_name=public` is used
//
// Query parameters control filtering, pagination, and ordering:
//
//	Parameter         | Description
//	------------------|------------------------------------------------
//	?select=col1,col2 | Select specific columns
//	?order=col.desc   | Order results (supports nullsfirst/nullslast)
//	?order=similarity(col, 'search string') | Order by similarity (requires pg_trgm extension)
//	?limit=100        | Limit number of results (default: 100)
//	?offset=0         | Pagination offset (default: 0)
//	?col=eq.val       | Filter by column equality
//	?col=gt.val       | Filter with greater than comparison
//	?col=lt.val       | Filter with less than comparison
//	?col=gte.val      | Filter with greater than or equal comparison
//	?col=lte.val      | Filter with less than or equal comparison
//	?col=like.val     | Filter with pattern matching
//	?col=in.(a,b,c)   | Filter with value lists
//	?col=is.null      | Filter for null values
//	?or=(a.eq.x,b.lt.y) | Combine filters with logical operators
//
// HTTP headers control response format for POST/PATCH/DELETE operations:
//
//	Header                         | Description
//	-------------------------------|----------------------------------------
//	Prefer: return=minimal         | Return status code only (default)
//	Prefer: return=representation  | Return modified rows in response body
//	Prefer: count=exact            | Add exact total count of rows for the query in Content-Range header
//	Prefer: return=headers-only    | Return headers with metadata only
//
// API is compatible with PostgREST. For more details, see:
// https://docs.postgrest.org/en/stable/references/api/tables_views.html
//
// Example usage:
//
//	server, err := rest.NewServer("postgres://user:pass@localhost/db", "")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer server.Shutdown()
//	log.Fatal(server.Start(":8080"))
package rest

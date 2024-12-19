package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"

	mw "github.com/edgeflare/pgo/middleware"
)

// Embed the static directory
//
//go:embed dist/*
var embeddedFS embed.FS

var (
	port        = flag.Int("port", 8080, "port to listen on")
	directory   = flag.String("dir", "dist", "directory to serve files from")
	spaFallback = flag.Bool("spa", false, "fallback to index.html for not-found files")
	useEmbedded = flag.Bool("embed", false, "use embedded static files")
)

func main() {
	flag.Parse()

	mux := http.NewServeMux()

	// other mux handlers
	//
	// static handler should be the last handler to catch all other requests
	if *useEmbedded {
		// Serve embedded static files
		mux.Handle("/", mw.ServeEmbedded(embeddedFS, *directory, *spaFallback))
	} else {
		// Serve static files from a directory
		mux.Handle("/", mw.ServeStatic(*directory, *spaFallback))
	}

	// // or mount on a `/different/path` other than root `/`
	// mux.Handle("/static/", http.StripPrefix("/static", mw.ServeStatic(*directory, *spaFallback)))

	// Start the server
	log.Printf("Starting server on :%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), mux))
}

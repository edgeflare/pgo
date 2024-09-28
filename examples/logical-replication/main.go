package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/edgeflare/pgo/pkg/x/logrepl"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Check if PGO_POSTGRES_LOGREPL_CONN_STRING is set
	if os.Getenv("PGO_POSTGRES_LOGREPL_CONN_STRING") == "" {
		return fmt.Errorf("PGO_POSTGRES_LOGREPL_CONN_STRING environment variable is not set")
	}

	// Optional: Set PGO_POSTGRES_LOGREPL_TABLES to specify tables for replication
	// Format: comma-separated list of table names (optionally schema-qualified)
	// Example: export PGO_POSTGRES_LOGREPL_TABLES="public.users,public.orders,custom_schema.products"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming CDC events
	eventsChan, err := logrepl.Run(ctx)
	if err != nil {
		return err
	}

	// Process events in a separate goroutine
	go func() {
		for event := range eventsChan {
			log.Printf("Received CDC event: %+v", event)
			// Handle the CDC event here
		}
	}()

	log.Println("Logical replication started. Press Ctrl+C to exit.")

	// Wait for termination signal
	<-sigChan
	log.Println("Received termination signal, shutting down gracefully...")

	// Trigger cancellation of the context
	cancel()

	log.Println("Shutdown complete")
	return nil
}

package main

import (
	"cmp"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	conn, err := pgconn.Connect(context.Background(), cmp.Or(os.Getenv("PGO_PGLOGREPL_CONN_STRING"), "postgres://postgres:secret@localhost:5432/testdb?replication=database"))
	if err != nil {
		log.Fatalln("failed to connect to PostgreSQL server:", err)
	}
	defer conn.Close(context.Background())

	// Start consuming CDC events
	eventsChan, err := pglogrepl.Main(ctx, conn, cmp.Or(os.Getenv("PGO_PGLOGREPL_CONN_STRING"), ""))
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

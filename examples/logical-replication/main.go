package main

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
)

func main() {
	conn, err := pgconn.Connect(context.Background(), cmp.Or(os.Getenv("PGO_PGLOGREPL_CONN_STRING"),
		"postgres://postgres:secret@localhost:5432/testdb?replication=database"))
	if err != nil {
		log.Fatal("connect failed:", err)
	}
	defer conn.Close(context.Background())

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		cancel()
	}()

	// optional replication params
	cfg := pglogrepl.Config{
		// tables to watch
		// PublicationTables: []string{"example_table","example_schema.table"},
		//
		// Publication:       "pgo_pub",
		// ReplicationSlot:   "pgo_slot",
		// Plugin:            "pgoutput",
		// StandbyPeriod:     10 * time.Second,
	}

	// Start streaming changes
	events, err := pglogrepl.Stream(ctx, conn, &cfg)
	if err != nil {
		log.Fatal("stream setup failed:", err)
	}

	// Process events
	for event := range events {
		switch event.Payload.Op {
		case "c":
			fmt.Printf("Insert: %v\n", event.Payload.After)
		case "u":
			fmt.Printf("Update: Before=%v After=%v\n", event.Payload.Before, event.Payload.After)
		case "d":
			fmt.Printf("Delete: %v\n", event.Payload.Before)
		}
	}
}

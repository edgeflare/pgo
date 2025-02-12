package main

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/jackc/pgx/v5/pgconn"

	// Register built-in connectors
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/clickhouse"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/debug"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/kafka"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/mqtt"
)

func pipelinesDemo() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming CDC events
	conn, err := pgconn.Connect(context.Background(), cmp.Or(os.Getenv("PGO_PGLOGREPL_CONN_STRING"), "postgres://postgres:secret@localhost:5432/testdb?replication=database"))
	if err != nil {
		log.Fatalln("failed to connect to PostgreSQL server:", err)
	}
	defer conn.Close(context.Background())

	eventsChan, err := pglogrepl.Main(ctx, conn, cmp.Or(os.Getenv("PGO_PGLOGREPL_CONN_STRING"), ""))
	if err != nil {
		return err
	}

	// Get the pipeline manager
	m := pipeline.NewManager()
	// TODO: here should also take config and any publication args
	_, err = m.AddPeer(pipeline.ConnectorMQTT, "mqtt-default")
	if err != nil {
		return err
	}

	_, err = m.AddPeer(pipeline.ConnectorDebug, "debug")
	if err != nil {
		return err
	}

	_, err = m.AddPeer(pipeline.ConnectorKafka, "kafka-default")
	if err != nil {
		return err
	}

	_, err = m.AddPeer(pipeline.ConnectorClickHouse, "clickhouse-default")
	if err != nil {
		return err
	}

	// Initialize all peers
	for _, p := range m.Peers() {
		// use config if not nil. then check env var. finally fall back to defaults
		err := p.Connector().Connect(nil)
		if err != nil {
			return fmt.Errorf("failed to initialize connector %s: %w", p.Name, err)
		}
	}

	// Create a slice to hold channels for each peer
	peerChannels := make([]chan pglogrepl.CDC, len(m.Peers()))
	for i := range peerChannels {
		peerChannels[i] = make(chan pglogrepl.CDC, 100) // Use a buffered channel
	}

	// Start a goroutine to broadcast events to all peer channels
	go func() {
		for event := range eventsChan {
			for _, ch := range peerChannels {
				ch <- event
			}
		}
		for _, ch := range peerChannels {
			close(ch)
		}
	}()

	// Process events in a separate goroutine for each peer
	for i, p := range m.Peers() {
		go func(peer pipeline.Peer, ch chan pglogrepl.CDC) {
			for event := range ch {
				err := peer.Connector().Pub(event)
				if err != nil {
					log.Printf("Error publishing to %s: %v", peer.Name, err)
				}
			}
		}(p, peerChannels[i])
	}

	log.Println("Logical replication started. Press Ctrl+C to exit.")

	// Wait for termination signal
	<-sigChan
	log.Println("Received termination signal, shutting down gracefully...")

	// Trigger cancellation of the context
	cancel()

	log.Println("Shutdown complete")
	return nil
}

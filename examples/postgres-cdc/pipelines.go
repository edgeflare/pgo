package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/edgeflare/pgo/pkg/pipeline/debug"
	_ "github.com/edgeflare/pgo/pkg/pipeline/kafka"
	_ "github.com/edgeflare/pgo/pkg/pipeline/mqtt"

	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/x/logrepl"
)

func pipelinesDemo() error {
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

	peers := []pipeline.Peer{}

	// Get the pipeline manager
	m := pipeline.Manager()
	// TODO: should also take config here
	mqttPeer, err := m.AddPeer(pipeline.ConnectorMQTT, "mqtt-default")
	if err != nil {
		return err
	}
	peers = append(peers, *mqttPeer)

	debugPeer, err := m.AddPeer(pipeline.ConnectorDebug, "debug")
	if err != nil {
		return err
	}
	peers = append(peers, *debugPeer)

	kafkaPeer, err := m.AddPeer(pipeline.ConnectorKafka, "kafka-default")
	if err != nil {
		return err
	}
	peers = append(peers, *kafkaPeer)

	// Initialize all peers
	for _, p := range peers {
		// use config it not nil. then check env var. finally fall back to defaults
		err := p.Connector().Init(nil)
		if err != nil {
			return fmt.Errorf("failed to initialize connector %s: %w", p.Name(), err)
		}
	}

	// Create a buffered channel for broadcasting events to all peers
	broadcastChan := make(chan logrepl.PostgresCDC, 100)

	// Start a goroutine to broadcast events to all peers
	go func() {
		for event := range eventsChan {
			for range m.Peers() {
				broadcastChan <- event
			}
		}
		close(broadcastChan)
	}()

	// Process events in a separate goroutine for each peer
	for _, p := range m.Peers() {
		go func(peer pipeline.Peer) {
			for event := range broadcastChan {
				err := peer.Connector().Publish(event)
				if err != nil {
					log.Printf("Error publishing to %s: %v", peer.Name(), err)
				}
			}
		}(p)
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

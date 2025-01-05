package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/spf13/cobra"

	// Register built-in connectors
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/clickhouse"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/debug"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/kafka"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/mqtt"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/pg"
)

var pipelineCmd = &cobra.Command{
	Use:     "pipeline",
	Aliases: []string{"p"},
	Short:   "Run the PGO pipeline",
	Long:    `Run the PGO pipeline to replicate data changes from PostgreSQL to various destinations.`,
	RunE:    runPipeline,
}

func runPipeline(cmd *cobra.Command, args []string) error {
	// Use the configuration instead of directly checking environment variables
	if cfg.Postgres.LogReplConnString == "" {
		return fmt.Errorf("postgres.logrepl_conn_string is not set")
	}

	fmt.Println(cfg.Postgres.LogReplConnString)
	fmt.Println(cfg.Postgres.Tables)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming CDC events
	// eventsChan, err := logrepl.Run(ctx, cfg.Postgres.LogReplConnString, cfg.Postgres.Tables)
	conn, err := pgconn.Connect(context.Background(), cmp.Or(os.Getenv("PGO_PGLOGREPL_CONN_STRING"), "postgres://postgres:secret@localhost:5432/testdb?replication=database"))
	if err != nil {
		log.Fatalln("failed to connect to PostgreSQL server:", err)
	}
	defer conn.Close(context.Background())

	eventsChan, err := pglogrepl.Main(ctx, conn, cmp.Or(os.Getenv("PGO_LOGREPL_TABLES"), ""))
	if err != nil {
		return err
	}

	// Get the pipeline manager
	m := pipeline.Manager()

	// Add peers based on configuration
	for _, peerConfig := range cfg.Peers {
		_, err := m.AddPeer(peerConfig.Connector, peerConfig.Name)
		if err != nil {
			return fmt.Errorf("failed to add peer %s: %w", peerConfig.Name, err)
		}
	}

	fmt.Println(m.Peers())

	// Initialize all peers
	for _, p := range m.Peers() {
		peerConfig, err := json.Marshal(cfg.GetPeerConfig(p.Name()))
		if err != nil {
			return fmt.Errorf("failed to marshal config for peer %s: %w", p.Name(), err)
		}
		err = p.Connector().Connect(json.RawMessage(peerConfig))
		if err != nil {
			return fmt.Errorf("failed to initialize connector %s: %w", p.Name(), err)
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
					log.Printf("Error publishing to %s: %v", peer.Name(), err)
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

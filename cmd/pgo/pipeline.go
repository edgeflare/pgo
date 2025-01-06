package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/edgeflare/pgo/pkg/config"
	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/pipeline/transform"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create an error channel to collect errors from goroutines
	errChan := make(chan error, 1)

	// Initialize pipeline manager
	m := pipeline.Manager()

	// Add all peers from configuration
	for _, peerConfig := range cfg.Peers {
		_, err := m.AddPeer(peerConfig.Connector, peerConfig.Name)
		if err != nil {
			return fmt.Errorf("failed to add peer %s: %w", peerConfig.Name, err)
		}
	}

	// Initialize all peers
	for _, p := range m.Peers() {
		peerConfig := cfg.GetPeer(p.Name())
		if peerConfig == nil {
			return fmt.Errorf("peer config not found for %s", p.Name())
		}

		configJSON, err := json.Marshal(peerConfig.Config)
		if err != nil {
			return fmt.Errorf("failed to marshal config for peer %s: %w", p.Name(), err)
		}

		err = p.Connector().Connect(json.RawMessage(configJSON))
		if err != nil {
			return fmt.Errorf("failed to initialize connector %s: %w", p.Name(), err)
		}
	}

	// Start source connections and set up pipelines
	var wg sync.WaitGroup
	for _, pl := range cfg.Pipelines {
		for _, source := range pl.Sources {
			sourcePeer := cfg.GetPeer(source.Name)
			if sourcePeer == nil {
				return fmt.Errorf("source peer %s not found", source.Name)
			}

			if sourcePeer.Connector != "postgres" {
				continue // Skip non-postgres sources for now
			}

			var cfg struct {
				ConnString      string   `json:"connString"`
				ReplicateTables []string `json:"replicateTables"`
			}

			jsonData, err := json.Marshal(sourcePeer.Config)
			if err != nil {
				return fmt.Errorf("error marshaling config: %w", err)
			}

			if err := json.Unmarshal(jsonData, &cfg); err != nil {
				return fmt.Errorf("error parsing config: %w", err)
			}

			conn, err := pgconn.Connect(ctx, cfg.ConnString)
			if err != nil {
				return fmt.Errorf("failed to connect to PostgreSQL server %s: %w", source.Name, err)
			}

			eventsChan, err := pglogrepl.Main(ctx, conn, cfg.ReplicateTables...)
			if err != nil {
				conn.Close(ctx)
				return fmt.Errorf("failed to start replication for %s: %w", source.Name, err)
			}

			sinkChannels := make(map[string]chan pglogrepl.CDC)
			for _, sink := range pl.Sinks {
				sinkChannels[sink.Name] = make(chan pglogrepl.CDC, 100)
			}

			wg.Add(1)

			// Source goroutine
			go func(pipelineCfg config.PipelineConfig, sourceCfg config.SourceConfig) {
				defer wg.Done()
				defer conn.Close(ctx)

				for {
					select {
					case event, ok := <-eventsChan:
						if !ok {
							for _, ch := range sinkChannels {
								close(ch)
							}
							return
						}

						// Apply source transformations
						transformedEvent, err := applyTransformations(&event, sourceCfg.Transformations)
						if err != nil {
							log.Printf("Error applying source transformations: %v", err)
							select {
							case errChan <- err:
							default:
							}
							continue
						}

						// Apply pipeline transformations
						transformedEvent, err = applyTransformations(transformedEvent, pipelineCfg.Transformations)
						if err != nil {
							log.Printf("Error applying pipeline transformations: %v", err)
							select {
							case errChan <- err:
							default:
							}
							continue
						}

						for _, sink := range pipelineCfg.Sinks {
							if ch, ok := sinkChannels[sink.Name]; ok {
								select {
								case ch <- *transformedEvent:
								case <-ctx.Done():
									return
								default:
									log.Printf("Warning: Channel full for sink %s", sink.Name)
								}
							}
						}
					case <-ctx.Done():
						for _, ch := range sinkChannels {
							close(ch)
						}
						return
					}
				}
			}(pl, source)

			// Start sink goroutines
			for _, sink := range pl.Sinks {
				sinkPeer, _ := m.GetPeer(sink.Name)
				if sinkPeer == nil {
					return fmt.Errorf("sink peer %s not found", sink.Name)
				}

				ch := sinkChannels[sink.Name]
				wg.Add(1)

				go func(sink config.SinkConfig, peer *pipeline.Peer, ch chan pglogrepl.CDC) {
					defer wg.Done()
					for {
						select {
						case event, ok := <-ch:
							if !ok {
								return
							}
							transformedEvent, err := applyTransformations(&event, sink.Transformations)
							if err != nil {
								log.Printf("Error applying sink transformations: %v", err)
								select {
								case errChan <- err:
								default:
								}
								continue
							}
							if err := peer.Connector().Pub(*transformedEvent); err != nil {
								log.Printf("Error publishing to %s: %v", peer.Name(), err)
								select {
								case errChan <- err:
								default:
								}
							}
						case <-ctx.Done():
							return
						}
					}
				}(sink, sinkPeer, ch)
			}
		}
	}

	log.Println("Pipelines started. Press Ctrl+C to exit.")

	// Wait for either termination signal or error
	select {
	case <-sigChan:
		log.Println("Received termination signal, shutting down gracefully...")
		cancel() // Cancel context to signal all goroutines
	case err := <-errChan:
		log.Printf("Error in pipeline: %v", err)
		cancel()
	}

	// Wait with timeout for all goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Shutdown complete")
	case <-time.After(10 * time.Second):
		log.Println("Shutdown timed out after 10 seconds")
	}

	return nil
}

func applyTransformations(event *pglogrepl.CDC, transformations []transform.TransformConfig) (*pglogrepl.CDC, error) {
	if len(transformations) == 0 {
		return event, nil
	}

	// Get the transform manager
	manager := transform.NewManager()
	manager.RegisterBuiltins()

	// Create the transformation pipeline
	chainTransformations, err := manager.Chain(transformations)
	if err != nil {
		return event, fmt.Errorf("error creating transformation pipeline: %w", err)
	}

	// Apply the transformations
	return chainTransformations(event)
}

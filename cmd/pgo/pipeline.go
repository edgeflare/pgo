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
	"github.com/spf13/cobra"

	// Register built-in connectors
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/clickhouse"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/debug"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/grpc"
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	doneChan := make(chan struct{})

	var wg sync.WaitGroup

	m := pipeline.Manager()

	if err := initializePeers(m); err != nil {
		return fmt.Errorf("failed to initialize peers: %w", err)
	}

	if err := startPipelineProcessing(ctx, m, &wg, errChan); err != nil {
		return fmt.Errorf("failed to start pipeline processing: %w", err)
	}

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Println("Received termination signal, shutting down gracefully...")
		cancel()
	case err := <-errChan:
		log.Printf("Pipeline error: %v", err)
		cancel()
	}

	// Wait for goroutines to complete
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	// Wait with timeout
	select {
	case <-doneChan:
		log.Println("Shutdown complete")
	case <-time.After(10 * time.Second):
		log.Println("Shutdown timed out after 10 seconds")
	}

	return nil
}

// initializePeers sets up all peers from configuration
func initializePeers(m *pipeline.Mngr) error {
	// Add peers to the manager
	for _, peerConfig := range cfg.Peers {
		_, err := m.AddPeer(peerConfig.Connector, peerConfig.Name)
		if err != nil {
			return fmt.Errorf("failed to add peer %s: %w", peerConfig.Name, err)
		}
	}

	// Initialize each peer
	for _, p := range m.Peers() {
		peerConfig := cfg.GetPeer(p.Name())
		if peerConfig == nil {
			return fmt.Errorf("peer config not found for %s", p.Name())
		}

		configJSON, err := json.Marshal(peerConfig.Config)
		if err != nil {
			return fmt.Errorf("failed to marshal config for peer %s: %w", p.Name(), err)
		}

		if err := p.Connector().Connect(json.RawMessage(configJSON)); err != nil {
			return fmt.Errorf("failed to initialize connector %s: %w", p.Name(), err)
		}
	}

	return nil
}

// startPipelineProcessing sets up source and sink processing for all pipelines
func startPipelineProcessing(
	ctx context.Context,
	m *pipeline.Mngr,
	wg *sync.WaitGroup,
	errChan chan<- error,
) error {
	// Process each pipeline
	for _, pl := range cfg.Pipelines {
		// Create sink channels for this pipeline
		sinkChannels := make(map[string]chan pglogrepl.CDC)
		for _, sink := range pl.Sinks {
			sinkChannels[sink.Name] = make(chan pglogrepl.CDC, 100)
		}

		// Process each source in the pipeline
		for _, source := range pl.Sources {
			sourcePeer := cfg.GetPeer(source.Name)
			if sourcePeer == nil {
				return fmt.Errorf("source peer %s not found", source.Name)
			}

			peer, _ := m.GetPeer(source.Name)
			var eventsChan <-chan pglogrepl.CDC

			// Determine source type and start subscription
			switch sourcePeer.Connector {
			case "postgres":
				var cfg struct {
					ConnString      string `json:"connString"`
					ReplicateTables []any  `json:"replicateTables"`
				}

				// Marshal and unmarshal source config
				jsonData, err := json.Marshal(sourcePeer.Config)
				if err != nil {
					return fmt.Errorf("error marshaling postgres config: %w", err)
				}

				if err := json.Unmarshal(jsonData, &cfg); err != nil {
					return fmt.Errorf("error parsing postgres config: %w", err)
				}

				// Start PostgreSQL replication
				eventsChan, err = peer.Connector().Sub(cfg.ReplicateTables...)
				if err != nil {
					return fmt.Errorf("failed to start postgres replication for %s: %w", source.Name, err)
				}

			case "mqtt":
				var cfg struct {
					TopicPrefix string `json:"topicPrefix"`
				}
				jsonData, err := json.Marshal(sourcePeer.Config)
				if err != nil {
					return fmt.Errorf("error marshaling mqtt config: %w", err)
				}

				if err := json.Unmarshal(jsonData, &cfg); err != nil {
					return fmt.Errorf("error parsing postgres config: %w", err)
				}

				if cfg.TopicPrefix == "" {
					cfg.TopicPrefix = "/pgo" // Default topic prefix
				}
				eventsChan, err = peer.Connector().Sub(cfg.TopicPrefix)
				if err != nil {
					return fmt.Errorf("failed to start MQTT subscription for %s: %w", source.Name, err)
				}

			case "grpc":
				var cfg struct {
					Address  string `json:"address"`
					IsServer bool   `json:"isServer"`
				}
				jsonData, err := json.Marshal(sourcePeer.Config)
				if err != nil {
					return fmt.Errorf("error marshaling grpc config: %w", err)
				}

				if err := json.Unmarshal(jsonData, &cfg); err != nil {
					return fmt.Errorf("error parsing grpc config: %w", err)
				}

				// Only allow subscription for non-server mode
				if cfg.IsServer {
					return fmt.Errorf("cannot subscribe to gRPC server peer %s", source.Name)
				}

				eventsChan, err = peer.Connector().Sub()
				if err != nil {
					return fmt.Errorf("failed to start gRPC subscription for %s: %w", source.Name, err)
				}

			default:
				log.Printf("Unsupported source connector: %s", sourcePeer.Connector)
				continue
			}

			// Start source event processing goroutine
			wg.Add(1)
			go func(pipelineCfg config.PipelineConfig, sourceCfg config.SourceConfig) {
				defer wg.Done()
				defer func() {
					// Close all sink channels when source processing is done
					for _, ch := range sinkChannels {
						close(ch)
					}
				}()

				for {
					select {
					case event, ok := <-eventsChan:
						if !ok {
							return // Source channel closed
						}

						// Apply source transformations
						transformedEvent, err := applyTransformations(&event, sourceCfg.Transformations)
						if err != nil {
							log.Printf("Source transformation error: %v", err)
							continue
						}
						if transformedEvent == nil {
							continue
						}

						// Apply pipeline transformations
						transformedEvent, err = applyTransformations(transformedEvent, pipelineCfg.Transformations)
						if err != nil {
							log.Printf("Pipeline transformation error: %v", err)
							continue
						}
						if transformedEvent == nil {
							continue
						}

						// Distribute to sink channels
						for _, sink := range pipelineCfg.Sinks {
							if ch, ok := sinkChannels[sink.Name]; ok {
								select {
								case ch <- *transformedEvent:
								default:
									log.Printf("Warning: Sink channel %s is full", sink.Name)
								}
							}
						}

					case <-ctx.Done():
						return
					}
				}
			}(pl, source)

			// Start sink processing goroutines
			for _, sink := range pl.Sinks {
				sinkPeer, _ := m.GetPeer(sink.Name)
				if sinkPeer == nil {
					return fmt.Errorf("sink peer %s not found", sink.Name)
				}

				ch := sinkChannels[sink.Name]
				wg.Add(1)

				go func(sink config.SinkConfig, peer *pipeline.Peer, ch <-chan pglogrepl.CDC) {
					defer wg.Done()

					for {
						select {
						case event, ok := <-ch:
							if !ok {
								return // Sink channel closed
							}

							// Apply sink-specific transformations
							transformedEvent, err := applyTransformations(&event, sink.Transformations)
							if err != nil {
								log.Printf("Sink transformation error: %v", err)
								continue
							}
							if transformedEvent == nil {
								continue
							}

							// Publish to sink
							if err := peer.Connector().Pub(*transformedEvent); err != nil {
								log.Printf("Publish error to %s: %v", peer.Name(), err)
							}

						case <-ctx.Done():
							return
						}
					}
				}(sink, sinkPeer, ch)
			}
		}
	}

	return nil
}

func applyTransformations(event *pglogrepl.CDC, transformations []transform.TransformConfig) (*pglogrepl.CDC, error) {
	if len(transformations) == 0 {
		return event, nil
	}

	if event == nil {
		return nil, fmt.Errorf("cannot transform nil event")
	}

	// Get the transform manager
	manager := transform.NewManager()
	manager.RegisterBuiltins()

	// Create the transformation pipeline
	chainTransformations, err := manager.Chain(transformations)
	if err != nil {
		return nil, fmt.Errorf("error creating transformation pipeline: %w", err)
	}

	result, err := chainTransformations(event)
	if result == nil && err == nil {
		// Transform indicated event should be filtered out
		return nil, nil
	}
	return result, err
}

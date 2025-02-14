package pgo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/edgeflare/pgo/pkg/metrics"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/pipeline/cdc"
	"github.com/edgeflare/pgo/pkg/pipeline/transform"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	// Register built-in connectors
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/clickhouse"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/debug"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/grpc"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/kafka"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/mqtt"
	_ "github.com/edgeflare/pgo/pkg/pipeline/peer/nats"
	"github.com/edgeflare/pgo/pkg/pipeline/peer/pg"
	// _ "github.com/edgeflare/pgo/pkg/pipeline/peer/pg"
)

var (
	prometheusEnabled bool
	prometheusAddr    string
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

	// Start Prometheus server if enabled
	startPrometheusServer(ctx, &wg)

	m := pipeline.NewManager()

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
func initializePeers(m *pipeline.Manager) error {
	// Add peers to the manager
	for _, peerConfig := range cfg.Peers {
		_, err := m.AddPeer(peerConfig.ConnectorName, peerConfig.Name)
		if err != nil {
			return fmt.Errorf("failed to add peer %s: %w", peerConfig.Name, err)
		}
	}

	// Initialize each peer
	for _, p := range m.Peers() {
		peerConfig := cfg.GetPeer(p.Name)
		if peerConfig == nil {
			return fmt.Errorf("peer config not found for %s", p.Name)
		}

		configJSON, err := json.Marshal(peerConfig.Config)
		if err != nil {
			return fmt.Errorf("failed to marshal config for peer %s: %w", p.Name, err)
		}

		if err := p.Connector().Connect(json.RawMessage(configJSON)); err != nil {
			return fmt.Errorf("failed to initialize connector %s: %w", p.Name, err)
		}
	}

	return nil
}

func startPipelineProcessing(
	ctx context.Context,
	m *pipeline.Manager,
	wg *sync.WaitGroup,
	errChan chan<- error,
) error {
	for _, pl := range cfg.Pipelines {
		if err := setupPipeline(ctx, m, wg, pl); err != nil {
			return fmt.Errorf("failed to setup pipeline %s: %w", pl.Name, err)
		}
	}
	return nil
}

// setupSource configures and starts a single source within a pipeline
func setupSource(
	ctx context.Context,
	m *pipeline.Manager,
	wg *sync.WaitGroup,
	pl pipeline.Pipeline,
	source pipeline.Source,
	sinkChannels map[string]chan cdc.Event,
) error {
	sourcePeer := cfg.GetPeer(source.Name)
	if sourcePeer == nil {
		return fmt.Errorf("source peer %s not found", source.Name)
	}

	peer, err := m.GetPeer(source.Name)
	if err != nil {
		return err
	}

	// Check if this is the first subscription before adding the new one
	isFirst := m.IsFirstSubscription(source.Name)

	// Add the subscription
	m.AddSubscription(source.Name, pl.Name, sinkChannels)

	// Only set up the source connection for the first subscription
	if isFirst {
		eventsChan, err := setupSourceConnection(sourcePeer, peer)
		if err != nil {
			return err
		}

		// Start source event processing with fan-out
		wg.Add(1)
		go processSourceEventsWithFanout(ctx, wg, m, source.Name, eventsChan)
	}

	// Setup sinks for this pipeline
	if err := setupSinks(ctx, m, wg, pl, sinkChannels); err != nil {
		return fmt.Errorf("failed to setup sinks: %w", err)
	}

	return nil
}

// setupSourceConnection establishes the connection for a source based on its type
func setupSourceConnection(sourcePeer *pipeline.Peer, peer *pipeline.Peer) (<-chan cdc.Event, error) {
	switch sourcePeer.ConnectorName {
	case "postgres":
		var cfg pg.Config
		if err := unmarshalConfig(sourcePeer.Config, &cfg); err != nil {
			return nil, fmt.Errorf("error parsing postgres config: %w", err)
		}
		return peer.Connector().Sub()
	case "mqtt":
		return setupMQTTConnection(sourcePeer, peer)
	case "grpc":
		return setupGRPCConnection(sourcePeer, peer)
	default:
		return nil, fmt.Errorf("unsupported source connector: %s", sourcePeer.ConnectorName)
	}
}

// setupMQTTConnection handles MQTT-specific connection setup
func setupMQTTConnection(sourcePeer *pipeline.Peer, peer *pipeline.Peer) (<-chan cdc.Event, error) {
	var cfg struct {
		TopicPrefix string `json:"topicPrefix"`
	}

	if err := unmarshalConfig(sourcePeer.Config, &cfg); err != nil {
		return nil, fmt.Errorf("error parsing mqtt config: %w", err)
	}

	if cfg.TopicPrefix == "" {
		cfg.TopicPrefix = "/pgo"
	}

	return peer.Connector().Sub(cfg.TopicPrefix)
}

// setupGRPCConnection handles gRPC-specific connection setup
func setupGRPCConnection(sourcePeer *pipeline.Peer, peer *pipeline.Peer) (<-chan cdc.Event, error) {
	var cfg struct {
		Address  string `json:"address"`
		IsServer bool   `json:"isServer"`
	}

	if err := unmarshalConfig(sourcePeer.Config, &cfg); err != nil {
		return nil, fmt.Errorf("error parsing grpc config: %w", err)
	}

	if cfg.IsServer {
		return nil, fmt.Errorf("cannot subscribe to gRPC server peer")
	}

	return peer.Connector().Sub()
}

// unmarshalConfig is a helper function to handle config unmarshaling
func unmarshalConfig(config interface{}, target interface{}) error {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	return nil
}

func processSourceEventsWithFanout(
	ctx context.Context,
	wg *sync.WaitGroup,
	m *pipeline.Manager,
	sourceName string,
	eventsChan <-chan cdc.Event,
) {
	defer wg.Done()

	for {
		select {
		case event, ok := <-eventsChan:
			if !ok {
				return
			}

			// Get all subscriptions for this source
			subs := m.GetSubscriptions(sourceName)

			// Fan out the event to all subscribed pipelines
			for _, sub := range subs {
				// Get pipeline config for this subscription
				pl := cfg.GetPipeline(sub.PipelineName)
				if pl == nil {
					log.Printf("Pipeline %s not found", sub.PipelineName)
					continue
				}

				// Find the matching source config for this event's source
				var matchingSource *pipeline.Source
				for _, src := range pl.Sources {
					if src.Name == sourceName {
						matchingSource = &src
						break
					}
				}

				if matchingSource == nil {
					log.Printf("Source %s not found in pipeline %s", sourceName, sub.PipelineName)
					continue
				}

				// Process the event with this pipeline's configuration using the matched source
				processEvent(*pl, *matchingSource, event, sub.SinkChannels)
			}

		case <-ctx.Done():
			return
		}
	}
}

// processEvent handles the processing of a single event
func processEvent(
	pl pipeline.Pipeline,
	source pipeline.Source,
	event cdc.Event,
	sinkChannels map[string]chan cdc.Event,
) {
	timer := prometheus.NewTimer(metrics.EventProcessingDuration.WithLabelValues(
		pl.Name,
		source.Name,
		"",
	))
	defer timer.ObserveDuration()

	// Process source transformations
	transformedEvent := applyEventTransformations(event, source, pl, sinkChannels)
	if transformedEvent == nil {
		return
	}

	// Distribute to sinks
	distributeToSinks(pl, source, *transformedEvent, sinkChannels)
}

// applyEventTransformations applies all transformations to an event
func applyEventTransformations(
	event cdc.Event,
	source pipeline.Source,
	pl pipeline.Pipeline,
	sinkChannels map[string]chan cdc.Event,
) *cdc.Event {
	// Source transformations
	transformed, err := applyTransformations(&event, source.Transformations)
	if err != nil {
		metrics.TransformationErrors.WithLabelValues(
			"source",
			pl.Name,
			source.Name,
			"",
		).Inc()
		log.Printf("Source transformation error: %v", err)
		return nil
	}
	if transformed == nil {
		return nil
	}

	// Pipeline transformations
	transformed, err = applyTransformations(transformed, pl.Transformations)
	if err != nil {
		metrics.TransformationErrors.WithLabelValues(
			"pipeline",
			pl.Name,
			source.Name,
			"",
		).Inc()
		log.Printf("Pipeline transformation error: %v", err)
		return nil
	}

	return transformed
}

// distributeToSinks sends the transformed event to all configured sinks
func distributeToSinks(
	pl pipeline.Pipeline,
	source pipeline.Source,
	event cdc.Event,
	sinkChannels map[string]chan cdc.Event,
) {
	for _, sink := range pl.Sinks {
		if ch, ok := sinkChannels[sink.Name]; ok {
			select {
			case ch <- event:
				metrics.ProcessedEvents.WithLabelValues(
					pl.Name,
					source.Name,
					sink.Name,
				).Inc()
			default:
				log.Printf("Warning: Sink channel %s is full", sink.Name)
			}
		}
	}
}

// setupPipeline handles the setup of a single pipeline
func setupPipeline(ctx context.Context, m *pipeline.Manager, wg *sync.WaitGroup, pl pipeline.Pipeline) error {
	// Create channels for each sink that will be shared across all sources
	sinkChannels := make(map[string]chan cdc.Event)
	for _, sink := range pl.Sinks {
		sinkChannels[sink.Name] = make(chan cdc.Event, 100)
	}

	// Setup each source independently
	for _, source := range pl.Sources {
		if err := setupSource(ctx, m, wg, pl, source, sinkChannels); err != nil {
			// Close all sink channels on error
			for _, ch := range sinkChannels {
				close(ch)
			}
			return fmt.Errorf("failed to setup source %s: %w", source.Name, err)
		}
	}

	// Setup sinks to process events from all sources
	return setupSinks(ctx, m, wg, pl, sinkChannels)
}

func setupSinks( // doesn't require a specific source
	ctx context.Context,
	m *pipeline.Manager,
	wg *sync.WaitGroup,
	pl pipeline.Pipeline,
	sinkChannels map[string]chan cdc.Event,
) error {
	for _, sink := range pl.Sinks {
		sinkPeer, err := m.GetPeer(sink.Name)
		if err != nil {
			return fmt.Errorf("sink peer %s not found: %w", sink.Name, err)
		}

		ch := sinkChannels[sink.Name]
		wg.Add(1)
		go processSinkEvents(ctx, wg, pl, sink, sinkPeer, ch)
	}
	return nil
}

// processSinkEvents handles events from multiple sources
func processSinkEvents(
	ctx context.Context,
	wg *sync.WaitGroup,
	pl pipeline.Pipeline,
	sink pipeline.Sink,
	peer *pipeline.Peer,
	ch <-chan cdc.Event,
) {
	defer wg.Done()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}

			// Apply sink-specific transformations
			transformedEvent, err := applyTransformations(&event, sink.Transformations)
			if err != nil {
				metrics.TransformationErrors.WithLabelValues(
					"sink",
					pl.Name,
					"multiple",
					sink.Name,
				).Inc()
				log.Printf("Sink transformation error: %v", err)
				continue
			}
			if transformedEvent == nil {
				continue
			}

			// Publish the transformed event
			if err := peer.Connector().Pub(*transformedEvent); err != nil {
				metrics.PublishErrors.WithLabelValues(sink.Name).Inc()
				log.Printf("Publish error to %s: %v", peer.Name, err)
			}

		case <-ctx.Done():
			return
		}
	}
}

func applyTransformations(event *cdc.Event, transformations []transform.Transformation) (*cdc.Event, error) {
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

func startPrometheusServer(ctx context.Context, wg *sync.WaitGroup) {
	if !prometheusEnabled {
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		server := &http.Server{
			Addr:    prometheusAddr,
			Handler: mux,
		}

		// Channel to signal server shutdown completion
		serverClosed := make(chan struct{})

		go func() {
			log.Printf("Starting Prometheus metrics server on %s", prometheusAddr)
			if err := server.ListenAndServe(); err != http.ErrServerClosed {
				log.Printf("Metrics server error: %v", err)
			}
			close(serverClosed)
		}()

		// Wait for context cancellation
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error shutting down metrics server: %v", err)
		}

		// Wait for server to close
		<-serverClosed
		log.Println("Metrics server shutdown complete")
	}()
}

func init() {
	pipelineCmd.Flags().BoolVar(&prometheusEnabled, "metrics", true, "Enable Prometheus metrics server")
	pipelineCmd.Flags().StringVar(&prometheusAddr, "metrics-addr", ":9100", "Prometheus metrics server address")

	err := viper.BindPFlag("metrics.enabled", pipelineCmd.Flags().Lookup("metrics"))
	if err != nil {
		log.Fatalf("Error binding flag 'metrics.enabled': %v", err)
	}

	err = viper.BindPFlag("metrics.addr", pipelineCmd.Flags().Lookup("metrics-addr"))
	if err != nil {
		log.Fatalf("Error binding flag 'metrics.addr': %v", err)
	}
}

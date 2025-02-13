package nats

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/pipeline/cdc"
	"github.com/nats-io/nats.go"
)

// PeerNATS implements the source and sink for NATS
type PeerNATS struct {
	nc          *nats.Conn
	js          nats.JetStreamContext
	stream      string
	subject     string
	topicPrefix string
}

// Config represents NATS-specific configuration
type Config struct {
	Servers     []string   `json:"servers"`
	Stream      string     `json:"stream"`
	Subject     string     `json:"subject"`
	TopicPrefix string     `json:"topicPrefix"`
	Username    string     `json:"username,omitempty"`
	Password    string     `json:"password,omitempty"`
	TLS         *TLSConfig `json:"tls,omitempty"`
}

// TLSConfig holds TLS-specific configuration
type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"certFile,omitempty"`
	KeyFile  string `json:"keyFile,omitempty"`
	CAFile   string `json:"caFile,omitempty"`
}

func (p *PeerNATS) Connect(config json.RawMessage, args ...any) error {
	var cfg Config
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal NATS config: %w", err)
	}

	// Set defaults if not provided
	if len(cfg.Servers) == 0 {
		cfg.Servers = []string{nats.DefaultURL}
	}
	if cfg.Stream == "" {
		cfg.Stream = "cdc-stream"
	}
	if cfg.Subject == "" {
		cfg.Subject = "cdc.>"
	}
	if cfg.TopicPrefix == "" {
		cfg.TopicPrefix = "cdc"
	}

	// Configure connection options
	opts := []nats.Option{
		nats.Timeout(5 * time.Second),
		nats.PingInterval(10 * time.Second),
		nats.MaxPingsOutstanding(3),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
	}

	// Add authentication if provided
	if cfg.Username != "" && cfg.Password != "" {
		opts = append(opts, nats.UserInfo(cfg.Username, cfg.Password))
	}

	// Add TLS configuration if enabled
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsOpt := nats.ClientCert(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if cfg.TLS.CAFile != "" {
			tlsOpt = nats.RootCAs(cfg.TLS.CAFile)
		}
		opts = append(opts, tlsOpt)
	}

	// Connect to first available server
	var err error
	for _, server := range cfg.Servers {
		p.nc, err = nats.Connect(server, opts...)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("failed to connect to any NATS server: %w", err)
	}

	// Create JetStream context
	p.js, err = p.nc.JetStream()
	if err != nil {
		p.nc.Close()
		return fmt.Errorf("failed to create JetStream context: %w", err)
	}

	// Check if stream exists and create or update it
	if err := p.ensureStream(cfg.Stream, cfg.Subject); err != nil {
		p.nc.Close()
		return fmt.Errorf("failed to ensure stream: %w", err)
	}

	p.stream = cfg.Stream
	p.subject = cfg.Subject
	p.topicPrefix = cfg.TopicPrefix

	return nil
}

func (p *PeerNATS) Pub(event cdc.Event, args ...any) error {
	if p.js == nil {
		return fmt.Errorf("NATS connection not initialized")
	}

	// Create subject based on schema, table and operation
	subject := fmt.Sprintf("%s.%s.%s.%s",
		p.topicPrefix,
		event.Payload.Source.Schema,
		event.Payload.Source.Table,
		event.Payload.Op)

	// Marshal the event
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal CDC event: %w", err)
	}

	// Publish with JetStream
	_, err = p.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func (p *PeerNATS) Sub(args ...any) (<-chan cdc.Event, error) {
	if p.js == nil {
		return nil, fmt.Errorf("NATS connection not initialized")
	}

	// Create buffered channel for events
	events := make(chan cdc.Event, 100)

	// Create durable consumer
	_, err := p.js.AddConsumer(p.stream, &nats.ConsumerConfig{
		Durable:       "cdc-consumer",
		AckPolicy:     nats.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       time.Minute,
		FilterSubject: p.subject,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	// Create pull subscription
	sub, err := p.js.PullSubscribe(p.subject, "cdc-consumer")
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	// Start message processing in background
	go func() {
		defer close(events)
		defer sub.Unsubscribe()

		for {
			msgs, err := sub.Fetch(10, nats.MaxWait(time.Second))
			if err != nil {
				if err != nats.ErrTimeout {
					log.Printf("Error fetching messages: %v", err)
				}
				continue
			}

			for _, msg := range msgs {
				var event cdc.Event
				if err := json.Unmarshal(msg.Data, &event); err != nil {
					log.Printf("Error unmarshaling message: %v", err)
					msg.Nak()
					continue
				}

				select {
				case events <- event:
					msg.Ack()
				default:
					msg.Nak() // Channel full, retry later
				}
			}
		}
	}()

	return events, nil
}

func (p *PeerNATS) Type() pipeline.ConnectorType {
	return pipeline.ConnectorTypePubSub
}

func (p *PeerNATS) Disconnect() error {
	if p.nc != nil {
		p.nc.Close()
	}
	return nil
}

// ensureStream creates a stream if it doesn't exist or updates it if it does
func (p *PeerNATS) ensureStream(streamName, subject string) error {
	streamConfig := &nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{subject},
		Storage:  nats.FileStorage,
		Replicas: 1,
	}

	stream, err := p.js.StreamInfo(streamName)
	if err != nil {
		if err == nats.ErrStreamNotFound {
			// Stream doesn't exist, create it
			_, err = p.js.AddStream(streamConfig)
			if err != nil {
				return fmt.Errorf("failed to create stream: %w", err)
			}
			log.Printf("Created new stream: %s", streamName)
			return nil
		}
		return fmt.Errorf("failed to get stream info: %w", err)
	}

	// Stream exists, update if necessary
	if !streamConfigEqual(stream.Config, *streamConfig) {
		_, err = p.js.UpdateStream(streamConfig)
		if err != nil {
			return fmt.Errorf("failed to update stream: %w", err)
		}
		log.Printf("Updated existing stream: %s", streamName)
	}

	return nil
}

func streamConfigEqual(a, b nats.StreamConfig) bool {
	return a.Name == b.Name &&
		sliceEqual(a.Subjects, b.Subjects) &&
		a.Storage == b.Storage &&
		a.Replicas == b.Replicas
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorNATS, &PeerNATS{})
}

package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/util"
	"github.com/edgeflare/pgo/pkg/x/logrepl"
	"go.uber.org/zap"
)

type PeerKafka struct {
	producer sarama.SyncProducer
	logger   *zap.Logger
	client   *Client
}

func NewPeerKafka(logger *zap.Logger) *PeerKafka {
	return &PeerKafka{
		logger: logger,
	}
}

func (p *PeerKafka) Publish(event logrepl.PostgresCDC) error {
	// Convert the event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	// Create a Kafka message
	msg := &sarama.ProducerMessage{
		Topic: p.client.config.ProducerTopic,
		Value: sarama.StringEncoder(eventJSON),
	}

	// Send the message to Kafka
	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to Kafka: %w", err)
	}

	p.logger.Info("Message published to Kafka",
		zap.String("topic", p.client.config.ProducerTopic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset))

	return nil
}

func (p *PeerKafka) Init(config json.RawMessage, args ...any) error {
	// Check if logger is nil and initialize with a default logger if needed
	if p.logger == nil {
		var err error
		p.logger, err = zap.NewProduction()
		if err != nil {
			return fmt.Errorf("failed to create default logger: %w", err)
		}
	}

	var kafkaConfig Config

	if config != nil {
		if err := json.Unmarshal(config, &kafkaConfig); err != nil {
			return fmt.Errorf("config parse error: %w", err)
		}
	} else {
		// If config is nil, initialize with an empty Config
		kafkaConfig = Config{}
	}

	// Set default values if not provided in the config
	if len(kafkaConfig.Brokers) == 0 {
		kafkaConfig.Brokers = []string{util.GetEnvOrDefault("PGO_KAFKA_BROKER", "localhost:9092")}
	}

	if kafkaConfig.ProducerTopic == "" {
		kafkaConfig.ProducerTopic = util.GetEnvOrDefault("PGO_KAFKA_TOPIC", "test")
	}

	// Set SASL configuration
	username := util.GetEnvOrDefault("PGO_KAFKA_USERNAME", "user1")
	password := util.GetEnvOrDefault("PGO_KAFKA_PASSWORD", "")

	// Only enable SASL if both username and password are provided
	if username != "" && password != "" {
		kafkaConfig.SASL.Enable = true
		kafkaConfig.SASL.Username = username
		kafkaConfig.SASL.Password = password
		kafkaConfig.SASL.Algorithm = util.GetEnvOrDefault("PGO_KAFKA_SASL_ALGORITHM", "sha256")
	} else {
		kafkaConfig.SASL.Enable = false
	}

	// Set a default Kafka version if not provided
	if kafkaConfig.Version == "" {
		kafkaConfig.Version = util.GetEnvOrDefault("PGO_KAFKA_VERSION", "2.1.1")
	}

	// Create Kafka client
	p.client = NewClient(&kafkaConfig, p.logger)

	// Create Kafka producer
	producer, err := p.client.CreateProducer()
	if err != nil {
		return fmt.Errorf("failed to create Kafka producer: %w", err)
	}
	p.producer = producer

	p.logger.Info("Kafka peer initialized",
		zap.Strings("brokers", kafkaConfig.Brokers),
		zap.String("topic", kafkaConfig.ProducerTopic),
		zap.Bool("sasl_enabled", kafkaConfig.SASL.Enable),
		zap.String("version", kafkaConfig.Version))

	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorKafka, NewPeerKafka(nil))
}

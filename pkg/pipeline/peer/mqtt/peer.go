package mqtt

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/pipeline/cdc"
	"go.uber.org/zap"
)

// PeerMQTT implements the source and sink functionality for MQTT
type PeerMQTT struct {
	*Client
	Config Config
}

type Config struct {
	Servers       []string `json:"servers"`
	TopicPrefix   string   `json:"topicPrefix"`
	ClientOptions `json:"clientOptions"`
}

func (p *PeerMQTT) Connect(config json.RawMessage, args ...any) error {
	var cfg Config

	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal MQTT config: %w", err)
	}

	opts := cfg.ClientOptions

	for _, server := range cfg.Servers {
		u, err := url.Parse(server)
		if err != nil {
			return fmt.Errorf("failed to parse server URL %s: %w", server, err)
		}
		opts.Servers = append(opts.Servers, u)
	}

	p.Config.TopicPrefix = cmp.Or(p.Config.TopicPrefix, "pgo")

	mqttOpts := convertToPahoOptions(&opts)

	setDefaultOptions(mqttOpts)

	p.Client = NewClient(mqttOpts)

	if err := p.Client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", err)
	}

	return nil
}

func (p *PeerMQTT) Pub(event cdc.Event, args ...any) error {
	topic := fmt.Sprintf("%s/%s/%s/%s", p.Config.TopicPrefix, event.Payload.Source.Schema, event.Payload.Source.Table, event.Payload.Op)
	data, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	return p.Client.Publish(topic, 0, false, data)
}

func (p *PeerMQTT) Sub(args ...any) (<-chan cdc.Event, error) {
	prefix := p.Config.TopicPrefix
	if len(args) > 0 {
		if prefixArg, ok := args[0].(string); ok {
			prefix = strings.Trim(prefixArg, "/")
		}
	}

	enableResponse := true
	if len(args) > 1 {
		if enabled, ok := args[1].(bool); ok {
			enableResponse = enabled
		}
	}

	filter := prefix + "/#"
	events := make(chan cdc.Event, 100)

	token := p.Client.client.Subscribe(filter, 0, func(_ mqtt.Client, msg mqtt.Message) {
		topic := msg.Topic()
		parts := strings.Split(strings.TrimPrefix(topic, prefix+"/"), "/")

		if len(parts) < 3 {
			p.logger.Warn("invalid topic format, expected schema/table/operation", zap.String("topic", topic))
			return
		}
		schema, table := parts[0], parts[1]

		operation := parts[2]
		var opCode string
		switch operation {
		case "c", "create":
			opCode = string(cdc.OpCreate)
		case "u", "update":
			opCode = string(cdc.OpUpdate)
		case "d", "delete":
			opCode = string(cdc.OpDelete)
		case "r", "read":
			opCode = string(cdc.OpRead)
		default:
			p.logger.Warn("unknown operation", zap.String("operation", operation))
			return
		}

		isBatch := false
		conditionsStartIdx := 3
		if len(parts) > 3 && parts[3] == "batch" {
			isBatch = true
			conditionsStartIdx = 4
		}

		conditions := make(map[string]string)
		if len(parts) > conditionsStartIdx {
			for i := conditionsStartIdx; i < len(parts); i += 2 {
				if i+1 < len(parts) {
					conditions[parts[i]] = parts[i+1]
				}
			}
		}

		var payloads []interface{}
		if len(msg.Payload()) > 0 {
			if isBatch {
				if err := json.Unmarshal(msg.Payload(), &payloads); err != nil {
					var singlePayload interface{}
					if err := json.Unmarshal(msg.Payload(), &singlePayload); err != nil {
						p.logger.Warn("invalid payload",
							zap.Error(err),
							zap.String("topic", topic))
						return
					}
					payloads = []interface{}{singlePayload}
				}
			} else {
				var singlePayload interface{}
				if err := json.Unmarshal(msg.Payload(), &singlePayload); err != nil {
					p.logger.Warn("invalid payload",
						zap.Error(err),
						zap.String("topic", topic))
					return
				}
				payloads = []interface{}{singlePayload}
			}
		}

		for _, payload := range payloads {
			event := createEvent(schema, table, opCode, payload)

			select {
			case events <- event:
			default:
				p.logger.Warn("event channel full, dropping message")
			}
		}

		if enableResponse {
			responseTopic := "/response" + msg.Topic()
			response := map[string]interface{}{
				"success":   true,
				"timestamp": time.Now().UnixMilli(),
				"count":     len(payloads),
			}

			responseData, err := json.Marshal(response)
			if err != nil {
				p.logger.Error("failed to marshal response", zap.Error(err))
				return
			}

			if err := p.Client.Publish(responseTopic, 0, false, responseData); err != nil {
				p.logger.Error("failed to publish response",
					zap.Error(err),
					zap.String("topic", responseTopic))
			}
		}
	})

	if err := token.Error(); err != nil {
		close(events)
		return nil, fmt.Errorf("mqtt subscribe failed: %w", err)
	}

	return events, nil
}

func (p *PeerMQTT) Type() pipeline.ConnectorType {
	return pipeline.ConnectorTypePubSub
}

func (p *PeerMQTT) Disconnect() error {
	p.client.Disconnect(500)
	return nil
}

// createEvent creates a new CDC event using the builder pattern
func createEvent(schema, table, opCode string, payload interface{}) cdc.Event {
	source := cdc.NewSourceBuilder("mqtt", "mqtt-source").
		WithDatabase("mqtt").
		WithSchema(schema).
		WithTable(table).
		WithTimestamp(time.Now().UnixMilli()).
		Build()

	builder := cdc.NewEventBuilder().
		WithSource(source).
		WithOperation(cdc.Operation(opCode)).
		WithTimestamp(time.Now().UnixMilli())

	if opCode == string(cdc.OpDelete) {
		builder.WithBefore(payload)
	} else {
		builder.WithAfter(payload)
	}

	return builder.Build()
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorMQTT, &PeerMQTT{})
}

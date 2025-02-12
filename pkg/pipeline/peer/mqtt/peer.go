package mqtt

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"go.uber.org/zap"
)

type PeerMQTT struct {
	*Client
}

func (p *PeerMQTT) Pub(event pglogrepl.CDC, args ...any) error {
	// Create the topic using the trimmed prefix
	topic := fmt.Sprintf("%s/%s", p.topicPrefix, event.Payload.Source.Table)
	data, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	return p.Client.Publish(topic, 0, false, data)
}

func (p *PeerMQTT) Connect(config json.RawMessage, args ...any) error {
	var opts ClientOptions

	// Unmarshal JSON into a temporary struct with servers as strings
	var tempOpts struct {
		ClientOptions
		Servers []string `json:"servers"`
	}

	if err := json.Unmarshal(config, &tempOpts); err != nil {
		return fmt.Errorf("failed to unmarshal MQTT config: %w", err)
	}

	// Copy the unmarshaled data to opts
	opts = tempOpts.ClientOptions

	// Convert string servers to url.URL
	for _, server := range tempOpts.Servers {
		u, err := url.Parse(server)
		if err != nil {
			return fmt.Errorf("failed to parse server URL %s: %w", server, err)
		}
		opts.Servers = append(opts.Servers, u) // Dereference the pointer
	}

	parsedArgs := parseArgs(args)
	topicPrefix, ok := parsedArgs[0].(string)
	if !ok {
		return errors.New("parseArgs did not return a string")
	}

	// Convert our ClientOptions to paho mqtt.ClientOptions
	mqttOpts := convertToPahoOptions(&opts)

	// Set default options
	setDefaultOptions(mqttOpts)

	// Create and embed MQTT client
	p.Client = NewClient(mqttOpts)

	// Connect to MQTT broker
	if err := p.Client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", err)
	}

	p.logger.Info("MQTT peer initialized",
		zap.Strings("brokers", getBrokerStrings(mqttOpts)),
		zap.String("client_id", mqttOpts.ClientID),
		zap.String("publish_topic_prefix", topicPrefix))

	// Store trimmed topicPrefix in the PeerMQTT struct
	p.topicPrefix = strings.TrimRight(topicPrefix, "/")

	return nil
}

// topic: /prefix/OPTIONAL_SCHEMA.TABLE/OPERATION/COL1/VAL1/COL2/VAL2/...
// payload: JSON (object / array)
//
// OPERATION
// c=create, u=update, d=delete, r=read
//
// Example:
// mosquitto_pub -t /example/prefix/devices/c -m '{"name":"kitchen-light"}' // defaults to public.table_name
// mosquitto_pub -t /example/prefix/iot.sensors/r/name/kitchen-light
// mosquitto_pub -t /example/prefix/iot.sensors/read/name/kitchen-light
// mosquitto_pub -t /example/prefix/iot.sensors/u/id/100 -m '{"name":"kitchen-light", "status": 0}'
// mosquitto_pub -t /example/prefix/iot.sensors/d/id/100

// In all cases, a response is published, unless disabled, to the /response/original/topic
// mosquitto_sub -t /response/example/prefix/iot.sensors/d/id/100
//
// With a trailing /batch in topic, it's possible to supply an array of json objects for supported operations
// mosquitto_pub -t /example/prefix/devices/c/batch -m '[{"name":"device1"}, {"name":"device2"}]'
func (p *PeerMQTT) Sub(args ...any) (<-chan pglogrepl.CDC, error) {
	if len(args) < 1 {
		return nil, errors.New("topic prefix required")
	}

	prefix, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("expected string prefix, got %T", args[0])
	}

	// Default enableResponse to true if not specified
	enableResponse := true
	if len(args) > 1 {
		if enabled, ok := args[1].(bool); ok {
			enableResponse = enabled
		}
	}

	prefix = strings.TrimRight(prefix, "/")
	filter := prefix + "/#"

	// Buffer size chosen to handle bursts while preventing excessive memory use
	events := make(chan pglogrepl.CDC, 100)

	token := p.Client.client.Subscribe(filter, 0, func(_ mqtt.Client, msg mqtt.Message) {
		// Parse topic parts: prefix/[schema.]table/operation/batch[/col1/val1,...]
		topic := msg.Topic()
		parts := strings.Split(strings.TrimPrefix(topic, prefix+"/"), "/")

		if len(parts) < 2 {
			p.logger.Warn("invalid topic format", zap.String("topic", topic))
			return
		}

		// Parse schema.table
		var schema, table string
		if schemaTable := strings.SplitN(parts[0], ".", 2); len(schemaTable) == 2 {
			schema = schemaTable[0]
			table = schemaTable[1]
		} else {
			schema = "public"
			table = schemaTable[0]
		}

		// Parse operation
		operation := parts[1]
		var opCode string
		switch operation {
		case "c", "create":
			opCode = "c"
		case "u", "update":
			opCode = "u"
		case "d", "delete":
			opCode = "d"
		case "r", "read":
			opCode = "r"
			// TODO: query database and publish the response at /response/original/topic
		default:
			p.logger.Warn("unknown operation", zap.String("operation", operation))
			return
		}

		// Check if this is a batch operation
		isBatch := false
		conditionsStartIdx := 2
		if len(parts) > 2 && parts[2] == "batch" {
			isBatch = true
			conditionsStartIdx = 3
		}

		// Parse conditions if present (col1/val1,col2/val2,...)
		conditions := make(map[string]string)
		if len(parts) > conditionsStartIdx {
			for i := conditionsStartIdx; i < len(parts); i += 2 {
				if i+1 < len(parts) {
					conditions[parts[i]] = parts[i+1]
				}
			}
		}

		// Parse payload
		var payloads []interface{}
		if len(msg.Payload()) > 0 {
			if isBatch {
				// For batch operations, expect an array
				if err := json.Unmarshal(msg.Payload(), &payloads); err != nil {
					// Try single object if array fails
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
				// For single operations, wrap in array
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

		// Create CDC events for each payload
		// Batch operations should somehow be communicated so that sinks etc to can process the data in batch
		for _, payload := range payloads {
			event := pglogrepl.CDC{
				Schema: struct {
					Type     string            `json:"type"`
					Optional bool              `json:"optional"`
					Name     string            `json:"name"`
					Fields   []pglogrepl.Field `json:"fields"`
				}{
					Type:     "struct",
					Optional: false,
					Name:     "io.debezium.connector.mqtt.Source",
					Fields:   pglogrepl.GetDefaultSchema().Fields,
				},
				Payload: struct {
					Before interface{} `json:"before"`
					After  interface{} `json:"after"`
					Source struct {
						Version   string `json:"version"`
						Connector string `json:"connector"`
						Name      string `json:"name"`
						TsMs      int64  `json:"ts_ms"`
						Snapshot  bool   `json:"snapshot"`
						Db        string `json:"db"`
						Sequence  string `json:"sequence"`
						Schema    string `json:"schema"`
						Table     string `json:"table"`
						TxID      int64  `json:"txId"`
						Lsn       int64  `json:"lsn"`
						Xmin      *int64 `json:"xmin,omitempty"`
					} `json:"source"`
					Op          string `json:"op"`
					TsMs        int64  `json:"ts_ms"`
					Transaction *struct {
						ID                  string `json:"id"`
						TotalOrder          int64  `json:"total_order"`
						DataCollectionOrder int64  `json:"data_collection_order"`
					} `json:"transaction,omitempty"`
				}{
					Before: nil,
					After:  payload,
					Source: struct {
						Version   string `json:"version"`
						Connector string `json:"connector"`
						Name      string `json:"name"`
						TsMs      int64  `json:"ts_ms"`
						Snapshot  bool   `json:"snapshot"`
						Db        string `json:"db"`
						Sequence  string `json:"sequence"`
						Schema    string `json:"schema"`
						Table     string `json:"table"`
						TxID      int64  `json:"txId"`
						Lsn       int64  `json:"lsn"`
						Xmin      *int64 `json:"xmin,omitempty"`
					}{
						Version:   "1.0",
						Connector: "mqtt",
						Name:      "mqtt-source",
						TsMs:      time.Now().UnixMilli(),
						Snapshot:  false,
						Db:        "mqtt",
						Sequence:  "[0,0]",
						Schema:    schema,
						Table:     table,
						TxID:      0,
						Lsn:       0,
					},
					Op:   opCode,
					TsMs: time.Now().UnixMilli(),
				},
			}

			// Send event to channel
			select {
			case events <- event:
			default:
				p.logger.Warn("event channel full, dropping message")
			}
		}

		// Send response if enabled
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
				// TODO: consider better error handling eg reporting prom metrics
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

	p.logger.Info("subscribed to mqtt topic", zap.String("filter", filter))
	return events, nil
}

func (p *PeerMQTT) Type() pipeline.ConnectorType {
	return pipeline.ConnectorTypePubSub
}

func (p *PeerMQTT) Disconnect() error {
	p.client.Disconnect(500) // 500ms. maybe make configurable
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorMQTT, &PeerMQTT{})
}

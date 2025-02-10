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

// topic: /prefix/OPTIONAL_SCHEMA.TABLE/OPERATION (insert, update, delete)
// payload: JSON
//
// Example:
// mosquitto_pub -t /pgo/iot.sensors/update -m '{"name":"kitchen-light", "status": 0}'
// mosquitto_pub -t /pgo/sensors/update -m '{"name":"kitchen-light", "status": 0}' // defaults to public.table_name
func (p *PeerMQTT) Sub(args ...any) (<-chan pglogrepl.CDC, error) {
	if len(args) == 0 {
		return nil, errors.New("topic prefix required")
	}

	prefix, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("expected string prefix, got %T", args[0])
	}

	prefix = strings.TrimRight(prefix, "/")
	filter := prefix + "/#"

	// Buffer size chosen to handle bursts while preventing excessive memory use
	// TODO: improve
	events := make(chan pglogrepl.CDC, 100)

	token := p.Client.client.Subscribe(filter, 0, func(_ mqtt.Client, msg mqtt.Message) {
		event, err := p.parseMessage(prefix, msg)
		if err != nil {
			p.logger.Warn("failed to parse message",
				zap.Error(err),
				zap.String("topic", msg.Topic()))
			return
		}

		select {
		case events <- event:
		default:
			p.logger.Warn("event channel full, dropping message")
		}
	})

	if err := token.Error(); err != nil {
		close(events)
		return nil, fmt.Errorf("mqtt subscribe failed: %w", err)
	}

	p.logger.Info("subscribed to mqtt topic", zap.String("filter", filter))
	return events, nil
}

func (p *PeerMQTT) parseMessage(prefix string, msg mqtt.Message) (pglogrepl.CDC, error) {
	topic := msg.Topic()
	topicParts := strings.Split(strings.TrimPrefix(topic, prefix+"/"), "/")
	if len(topicParts) != 2 {
		return pglogrepl.CDC{}, fmt.Errorf("invalid topic format: %s", topic)
	}

	var schema, table string
	if schemaTable := strings.SplitN(topicParts[0], ".", 2); len(schemaTable) == 2 {
		schema = schemaTable[0]
		table = schemaTable[1]
	} else if len(schemaTable) == 1 {
		schema = "public"
		table = schemaTable[0]
	}

	operation := topicParts[1]
	var opCode string
	switch operation {
	case "insert":
		opCode = "c"
	case "update":
		opCode = "u"
	case "delete":
		opCode = "d"
	default:
		p.logger.Warn("Unknown operation", zap.String("operation", operation))
		return pglogrepl.CDC{}, fmt.Errorf("invalid operation: %s", operation)
	}

	var payload map[string]any
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		return pglogrepl.CDC{}, fmt.Errorf("invalid json payload: %w", err)
	}

	return pglogrepl.CDC{
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
			Before: nil, // No previous state for MQTT messages
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
				Name:      "mqtt-source", // use host or some id
				TsMs:      time.Now().UnixMilli(),
				Snapshot:  false,
				Db:        "mqtt",
				Sequence:  "[0,0]", // No LSN for MQTT
				Schema:    schema,
				Table:     table,
				TxID:      0,
				Lsn:       0,
			},
			Op:   opCode,
			TsMs: time.Now().UnixMilli(), // maybe check if message has timestamp
		},
	}, nil
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

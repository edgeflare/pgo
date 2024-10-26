package pipeline

import (
	"encoding/json"

	"github.com/edgeflare/pgo/pkg/x/logrepl"
)

// A Connector represents a data publishing component.
type Connector interface {
	// Publish sends the given PostgresCDC event to the connector's destination.
	// It returns an error if the publish operation fails.
	Publish(event logrepl.PostgresCDC) error

	// Init initializes the connector with the provided configuration.
	// The config parameter is a raw JSON message containing connector-specific settings.
	// Additional arguments can be passed via the args parameter.
	// It returns an error if initialization fails.
	Init(config json.RawMessage, args ...any) error
}

// Predefined connector types.
const (
	ConnectorClickHouse = "clickhouse"
	ConnectorDebug      = "debug"
	ConnectorWebhook    = "webhook"
	ConnectorKafka      = "kafka"
	ConnectorMQTT       = "mqtt"
	ConnectorGRPC       = "grpc"
)

// RegisterConnector adds a new connector to the registry.
// The name parameter is used as a key to identify the connector type.
func RegisterConnector(name string, c Connector) {
	connectors[name] = c
}

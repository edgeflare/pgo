package pipeline

import (
	"encoding/json"
)

// Connector defines the interface for data publishing components.
type Connector interface {
	// Publish sends the given data to the connector's destination.
	// It returns an error if the publish operation fails.
	Publish(data interface{}) error

	// Init initializes the connector with the provided configuration.
	// The config parameter is a raw JSON message containing connector-specific settings.
	// It returns an error if initialization fails.
	Init(config json.RawMessage) error
}

// Predefined connector types.
const (
	ConnectorClickHouse = "clickhouse"
	ConnectorDebug      = "debug"
	ConnectorHTTP       = "http"
	ConnectorKafka      = "kafka"
	ConnectorMQTT       = "mqtt"
	ConnectorGRPC       = "grpc"
)

// RegisterConnector adds a new connector to the registry.
// The name parameter is used as a key to identify the connector type.
func RegisterConnector(name string, c Connector) {
	connectors[name] = c
}

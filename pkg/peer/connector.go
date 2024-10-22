package peer

type Connector interface {
	Publish(data interface{}) error
	Init() error
}

const (
	ConnectorClickHouse = "clickhouse"
	ConnectorDebug      = "debug"
	ConnectorHTTP       = "http"
	ConnectorKafka      = "kafka"
	ConnectorMQTT       = "mqtt"
	ConnectorGRPC       = "grpc"
)

func RegisterConnector(name string, c Connector) {
	connectors[name] = c
}

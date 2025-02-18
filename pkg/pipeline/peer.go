package pipeline

// Peer is a data source/destination with an associated connector (ie NATS, Kafka, MQTT, ClickHouse, etc).
type Peer struct {
	Name          string `mapstructure:"name"`
	ConnectorName string `mapstructure:"connector"`
	// Config contains the connection config of underlying library
	// eg github.com/IBM/sarama.Config, github.com/eclipse/paho.mqtt.golang.ClientOptions etc
	Config map[string]any `mapstructure:"config"`
	// Extra arguments for Connect, Pub, Sub methods
	Args []any
}

func (p *Peer) Connector() Connector {
	return connectors[p.ConnectorName]
}

// // need to check back for plugins / extensions
// func (p *Peer) Name() string {
// 	return p.name
// }

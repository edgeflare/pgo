package pipeline

// Peer is a data source/destination with an associated connector (ie ClickHouse, Kafka, MQTT, etc).
type Peer struct {
	Config        map[string]any `mapstructure:"config"`
	Name          string         `mapstructure:"name"`
	ConnectorName string         `mapstructure:"connector"`
}

func (p *Peer) Connector() Connector {
	return connectors[p.ConnectorName]
}

// // need to check back for plugins / extensions
// func (p *Peer) Name() string {
// 	return p.name
// }

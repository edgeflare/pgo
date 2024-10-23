package pipeline

// Peer is a data source/destination with an associated connector (ie ClickHouse, Kafka, MQTT, etc).
type Peer struct {
	connector string
	name      string
}

func (p *Peer) Connector() Connector {
	return connectors[p.connector]
}

func (p *Peer) Name() string {
	return p.name
}

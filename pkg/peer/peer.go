package peer

type Peer struct {
	connector string
}

var (
	connectors = make(map[string]Connector)
	peers      = make(map[string]Peer)
)

func New(config map[string]interface{}) (Peer, error) {
	return Peer{}, nil
}

func (p *Peer) Connector() Connector {
	return connectors[p.connector]
}

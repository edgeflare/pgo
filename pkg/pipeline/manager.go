package pipeline

import (
	"fmt"
	"plugin"
)

var (
	connectors = make(map[string]Connector)
	peers      = make(map[string]Peer)
)

// Mngr (Manager) handles connectors and peers for data pipeline operations.
// It supports dynamic loading of connector plugins and manages the lifecycle
// of data flows from PostgreSQL to various destinations.
type Mngr struct {
	connectors map[string]Connector
	peers      map[string]Peer
}

// Manager returns the singleton Manager instance
func Manager() *Mngr {
	return &Mngr{
		connectors: connectors,
		peers:      peers,
	}
}

// RegisterConnectorPlugin loads and registers a connector plugin from the specified path.
func (m *Mngr) RegisterConnectorPlugin(path string, name string) error {
	plug, err := plugin.Open(path)
	if err != nil {
		return err
	}

	symbol, err := plug.Lookup("Connector")
	if err != nil {
		return err
	}

	connector, ok := symbol.(*Connector)
	if !ok {
		return fmt.Errorf("invalid connector plugin")
	}

	RegisterConnector(name, *connector)
	return nil
}

// NewPeer creates a new Peer
func (m *Mngr) AddPeer(connector string, name string) (*Peer, error) {
	if _, exists := m.connectors[connector]; !exists {
		return nil, fmt.Errorf("connector %s not found", connector)
	}

	peer := Peer{connector: connector, name: name}
	m.peers[name] = peer
	return &peer, nil
}

func (m *Mngr) Peers() []Peer {
	peers := make([]Peer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, p)
	}
	return peers
}

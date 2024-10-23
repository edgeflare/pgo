package pipeline

import (
	"fmt"
	"plugin"
)

var (
	connectors = make(map[string]Connector)
	peers      = make(map[string]Peer)
)

// Manager handles connectors and peers for data pipeline operations.
// It supports dynamic loading of connector plugins and manages the lifecycle
// of data flows from PostgreSQL to various destinations.
type Manager struct {
	connectors map[string]Connector
	peers      map[string]Peer
}

// NewManager creates a new Manager instance.
func NewManager() *Manager {
	return &Manager{
		connectors: make(map[string]Connector),
		peers:      make(map[string]Peer),
	}
}

// RegisterConnectorPlugin loads and registers a connector plugin from the specified path.
func (m *Manager) RegisterConnectorPlugin(path string, name string) error {
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
func (m *Manager) NewPeer(connector string, name string) (*Peer, error) {
	if _, exists := m.connectors[connector]; !exists {
		return nil, fmt.Errorf("connector %s not found", connector)
	}

	peer := Peer{connector: connector, name: name}
	m.peers[name] = peer
	return &peer, nil
}

// Start initializes and runs all registered peers and their associated connectors.
func (m *Manager) Start() {
	// check connectors
	for _, c := range connectors {
		fmt.Println(c.Init(nil))
		fmt.Println(c.Publish("hello.."))
	}

	// check peers
	m.NewPeer(ConnectorMQTT, "mqtt-test")
	m.NewPeer(ConnectorHTTP, "kafka-test")
	for _, p := range peers {
		fmt.Println(p.Name(), p.Connector().Init(nil))
		fmt.Println(p.Name(), p.Connector().Publish("hello.."))
	}
}

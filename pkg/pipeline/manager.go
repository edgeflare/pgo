package pipeline

import (
	"fmt"
	"plugin"
	"sync"

	"github.com/edgeflare/pgo/pkg/pipeline/cdc"
)

var (
	connectors    = make(map[string]Connector)
	peers         = make(map[string]Peer)
	subscriptions = make(map[string][]SourceSubscription)
	mu            sync.RWMutex
)

type SourceSubscription struct {
	SinkChannels map[string]chan cdc.CDC
	PipelineName string
}

// Manager handles connectors and peers for data pipeline operations.
type Manager struct {
	connectors    map[string]Connector
	peers         map[string]Peer
	subscriptions map[string][]SourceSubscription
}

// NewManager returns a new Manager instance with the default connectors.
func NewManager() *Manager {
	return &Manager{
		connectors:    connectors,
		peers:         map[string]Peer{},
		subscriptions: map[string][]SourceSubscription{},
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
func (m *Manager) AddPeer(connector string, name string) (*Peer, error) {
	if _, exists := m.connectors[connector]; !exists {
		return nil, fmt.Errorf("connector %s not found", connector)
	}

	peer := Peer{ConnectorName: connector, Name: name}
	m.peers[name] = peer
	return &peer, nil
}

func (m *Manager) Peers() []Peer {
	peers := make([]Peer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, p)
	}
	return peers
}

func (m *Manager) GetPeer(name string) (*Peer, error) {
	if peer, exists := m.peers[name]; exists {
		return &peer, nil
	}
	return nil, fmt.Errorf("peer %s not found", name)
}

// AddSubscription adds a new subscription for a source
func (m *Manager) AddSubscription(sourceName, pipelineName string, sinkChannels map[string]chan cdc.CDC) {
	mu.Lock()
	m.subscriptions[sourceName] = append(m.subscriptions[sourceName], SourceSubscription{
		PipelineName: pipelineName,
		SinkChannels: sinkChannels,
	})
	mu.Unlock()
}

// GetSubscriptions returns all subscriptions for a source
func (m *Manager) GetSubscriptions(sourceName string) []SourceSubscription {
	mu.RLock()
	defer mu.RUnlock()
	return m.subscriptions[sourceName]
}

// IsFirstSubscription checks if this is the first subscription for a source
func (m *Manager) IsFirstSubscription(sourceName string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return len(m.subscriptions[sourceName]) == 0
}

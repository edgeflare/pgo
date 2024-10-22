package peer

import (
	"errors"
	"fmt"
	"plugin"
)

var (
	ErrInvalidConnectorPlugin = errors.New("invalid connector plugin")
)

type Manager struct {
	connectors map[string]Connector
	peers      map[string]Peer
}

func NewManager() *Manager {
	return &Manager{
		connectors: make(map[string]Connector),
		peers:      make(map[string]Peer),
	}
}

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
		return ErrInvalidConnectorPlugin
	}

	RegisterConnector(name, *connector)
	return nil
}

func (m *Manager) Start() {
	fmt.Println(peers)

	for _, c := range connectors {
		fmt.Println(c)
		fmt.Println(c.Init())
		fmt.Println(c.Publish("hello.."))
	}
}

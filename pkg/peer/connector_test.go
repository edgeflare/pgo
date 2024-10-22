package peer

import (
	"fmt"
	"testing"
)

func TestNewManager(t *testing.T) {
	// Create a new manager
	manager := NewManager()

	// Test starting the manager
	t.Run("Start Manager", func(t *testing.T) {
		manager.Start()
	})

	// Test connectors
	t.Run("Test Connectors", func(t *testing.T) {
		for _, c := range connectors {
			t.Run(fmt.Sprintf("Connector: %T", c), func(t *testing.T) {
				if err := c.Init(); err != nil {
					t.Errorf("Failed to initialize connector: %v", err)
				}

				msg := "hello..."
				if err := c.Publish(msg); err != nil {
					t.Errorf("Failed to publish message: %v", err)
				}
			})
		}
	})

	// Test plugin registration
	t.Run("Register Plugin", func(t *testing.T) {
		// go build -buildmode=plugin -o /tmp/example-plugin.so ./pkg/peer/plugin_example/...
		err := manager.RegisterConnectorPlugin("/tmp/example-plugin.so", "example")
		if err != nil {
			t.Fatalf("Failed to load plugin: %v", err)
		}
	})
}

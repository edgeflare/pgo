package pipeline

import (
	"fmt"
	"testing"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
)

func TestNewManager(t *testing.T) {
	// Create a new manager
	manager := Manager()

	// Test connectors
	t.Run("Test Connectors", func(t *testing.T) {
		for _, c := range connectors {
			t.Run(fmt.Sprintf("Connector: %T", c), func(t *testing.T) {
				if err := c.Connect(nil); err != nil {
					t.Errorf("Failed to initialize connector: %v", err)
				}

				msg := pglogrepl.CDC{
					// TODO PostgresCDC --> CDC
					// Table:     "test",
					// Data:      map[string]interface{}{"hello": "world"},
					// Operation: logrepl.OperationInsert,
				}
				if err := c.Pub(msg); err != nil {
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

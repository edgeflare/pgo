package main

// See [docs/pgcdc-mqtt.md](../../docs/pgcdc-mqtt.md) for more information.

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/util"
	"github.com/edgeflare/pgo/pkg/util/rand"

	// "github.com/edgeflare/pgo/pkg/x/logrepl"
	"github.com/jackc/pgx/v5/pgconn"
)

func main() {
	// if err := run(); err != nil {
	// 	log.Fatal(err)
	// }

	if err := pipelinesDemo(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming CDC events
	conn, err := pgconn.Connect(context.Background(), cmp.Or(os.Getenv("PGO_PGLOGREPL_CONN_STRING"), "postgres://postgres:secret@localhost:5432/testdb?replication=database"))
	if err != nil {
		log.Fatalln("failed to connect to PostgreSQL server:", err)
	}
	defer conn.Close(context.Background())

	eventsChan, err := pglogrepl.Main(ctx, conn, cmp.Or(os.Getenv("PGO_PGLOGREPL_CONN_STRING"), ""))
	if err != nil {
		return err
	}

	// Initialize MQTT client
	opts := mqtt.NewClientOptions()

	opts.AddBroker(util.GetEnvOrDefault("PGO_MQTT_BROKER", "tcp://127.0.0.1:1883"))
	// opts.SetUsername(util.GetEnvOrDefault("PGO_MQTT_USERNAME", ""))
	// opts.SetPassword(util.GetEnvOrDefault("PGO_MQTT_PASSWORD", ""))
	opts.SetClientID(fmt.Sprintf("pgo-logrepl-%s", rand.NewName()))

	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	// Process events in a separate goroutine
	go func() {
		for event := range eventsChan {
			log.Printf("Received CDC event: %+v", event)
			// Publish the changes "data" to MQTT
			jsonData, err := json.Marshal(event.Payload) // Marshal event.Data to JSON
			if err != nil {
				log.Printf("Error marshaling event data to JSON: %v", err)
				continue // Skip this event if there's an error
			}
			mqttClient.Publish(fmt.Sprintf("/pgcdc/%s", event.Payload.Source.Table), 0, false, jsonData) // Publish JSON data to /pgcdc/<tableName> topic
		}
	}()

	log.Println("Logical replication started. Press Ctrl+C to exit.")

	// Wait for termination signal
	<-sigChan
	log.Println("Received termination signal, shutting down gracefully...")

	// Trigger cancellation of the context
	cancel()

	// Disconnect the MQTT client before exiting
	mqttClient.Disconnect(250)

	log.Println("Shutdown complete")
	return nil
}

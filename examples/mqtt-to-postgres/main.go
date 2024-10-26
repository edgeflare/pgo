package main

import (
	"os"
	"os/signal"
	"syscall"

	mq "github.com/eclipse/paho.mqtt.golang"
	"github.com/edgeflare/pgo/pkg/pipeline/mqtt"

	"go.uber.org/zap"
)

// It persists MQTT messages to PostgreSQL.
// topic: /pgo/TABLE_NAME/OPERATION (insert, update, delete)
// payload: JSON
// mosquitto_pub -t /pgo/users/insert -m '{"name":"some1"}'
func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Create MQTT client
	opts := mq.NewClientOptions().
		AddBroker("tcp://localhost:1883").
		SetClientID("mqtt-demo-client")

	// Create a new MQTT client
	client := mqtt.NewClient(opts, logger)

	// Connect to MQTT broker
	if err := client.Connect(); err != nil {
		logger.Fatal("Failed to connect to MQTT broker", zap.Error(err))
	}
	defer client.Disconnect()

	// Subscribe to the PGO topic
	topicPrefix := "/pgo/#"
	if err := client.Subscribe(topicPrefix, 0, client.MessageToPostgres); err != nil {
		logger.Fatal("Failed to subscribe to topic", zap.Error(err), zap.String("topic", topicPrefix))
	}

	// Set up a channel to handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Keep the program running until an OS signal is received
	<-sigChan

	logger.Info("Shutting down...")
}

package mqtt

import (
	"fmt"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

// Client represents an MQTT client with PostgREST forwarding capabilities.
type Client struct {
	opts               *mqtt.ClientOptions
	client             mqtt.Client
	logger             *zap.Logger
	publishTopicPrefix string
}

// init ensures that the logger is not nil
func (c *Client) init() {
	if c.logger == nil {
		logger, err := zap.NewProduction()
		if err != nil {
			// If we can't create a production logger, fall back to a no-op logger
			fmt.Fprintf(os.Stderr, "Failed to create default logger: %v\n", err)
			c.logger = zap.NewNop()
		} else {
			c.logger = logger
		}
	}
}

// NewClient creates a new MQTT client with the given options and logger.
func NewClient(opts *mqtt.ClientOptions, logger ...*zap.Logger) *Client {
	client := &Client{
		opts: opts,
	}

	if len(logger) > 0 {
		client.logger = logger[0]
	}

	client.init()
	return client
}

// Connect establishes a connection to the MQTT broker.
func (c *Client) Connect() error {
	c.client = mqtt.NewClient(c.opts)
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("broker connection error: %w", token.Error())
	}

	c.logger.Info("Connected to MQTT broker")
	return nil
}

// Publish sends a message to the specified MQTT topic.
func (c *Client) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	token := c.client.Publish(topic, qos, retained, payload)
	token.Wait()
	if err := token.Error(); err != nil {
		c.logger.Error("Publish error", zap.Error(err))
		return err
	}
	c.logger.Debug("Message published", zap.String("topic", topic))
	return nil
}

// Disconnect closes the connection to the MQTT broker.
func (c *Client) Disconnect() {
	c.client.Disconnect(250)
	c.logger.Info("Disconnected from MQTT broker")
}

// Subscribe registers a callback for messages on the specified MQTT topic.
func (c *Client) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error {
	token := c.client.Subscribe(topic, qos, callback)
	token.Wait()
	if err := token.Error(); err != nil {
		c.logger.Error("Subscribe error", zap.Error(err), zap.String("topic", topic))
		return fmt.Errorf("subscribe error: %w", err)
	}
	c.logger.Debug("Subscribed to topic", zap.String("topic", topic))
	return nil
}

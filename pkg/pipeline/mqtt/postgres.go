package mqtt

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/edgeflare/pgo"
	"github.com/edgeflare/pgo/pkg/pg"
	"github.com/edgeflare/pgo/pkg/x/logrepl"
	"go.uber.org/zap"
)

// MessageToPostgres persists an MQTT message to PostgreSQL. It expects the topic to be in the following format:
// topic: /pgo/TABLE_NAME/OPERATION (insert, update, delete)
// payload: JSON
// mosquitto_pub -t /pgo/users/insert -m '{"name":"some1"}'
func (c *Client) MessageToPostgres(client mqtt.Client, msg mqtt.Message) {
	// TODO: IMPROVE THIS
	pool, poolErr := pgo.InitDefaultPool(os.Getenv("PGO_POSTGRES_CONN_STRING"))
	if poolErr != nil {
		c.logger.Error("Failed to initialize PostgreSQL connection pool", zap.Error(poolErr))
		return
	}

	conn, connErr := pool.Acquire(context.Background())
	if connErr != nil {
		c.logger.Error("Failed to acquire PostgreSQL connection", zap.Error(connErr))
		return
	}

	c.logger.Debug("Received message", zap.String("topic", msg.Topic()), zap.ByteString("payload", msg.Payload()))

	// Parse the topic
	topicParts := strings.Split(msg.Topic(), "/")

	if len(topicParts) != 4 || topicParts[1] != "pgo" {
		c.logger.Error("Invalid topic format", zap.String("topic", msg.Topic()))
		return
	}

	tableName := topicParts[2]
	operation := strings.ToUpper(topicParts[3])

	// Parse the payload
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		c.logger.Error("Failed to parse payload", zap.Error(err))
		return
	}

	// Execute the appropriate database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err error
	switch operation {
	case logrepl.OperationInsert:
		// err = c.insertRecord(ctx, conn, tableName, payload)
		err = pg.InsertRow(ctx, conn, tableName, msg.Payload())
		if err != nil {
			c.logger.Error("Failed to insert record", zap.Error(err))
			return
		}
	case logrepl.OperationUpdate:
		// TODO: Implement update
		// err = c.updateRecord(ctx, tableName, payload)
	case logrepl.OperationDelete:
		// TODO: Implement delete
		// err = c.deleteRecord(ctx, tableName, payload)
	default:
		c.logger.Error("Unsupported operation", zap.String("operation", operation))
		return
	}

	if err != nil {
		c.logger.Error("Failed to persist data to PostgreSQL", zap.Error(err))
		return
	}

	c.logger.Debug("Successfully persisted data to PostgreSQL",
		zap.String("table", tableName),
		zap.String("operation", operation))
}

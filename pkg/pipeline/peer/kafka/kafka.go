package kafka

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// CreateProducer creates a new SyncProducer
func (c *Client) CreateProducer() (sarama.SyncProducer, error) {
	conf, err := c.config.ToSaramaConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create sarama config: %w", err)
	}

	producer, err := sarama.NewSyncProducer(c.config.GetBrokers(), conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync producer: %w", err)
	}

	return producer, nil
}

// CreateConsumer creates a new Consumer
func (c *Client) CreateConsumer() (sarama.Consumer, error) {
	conf, err := c.config.ToSaramaConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create sarama config: %w", err)
	}

	consumer, err := sarama.NewConsumer(c.config.GetBrokers(), conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	return consumer, nil
}

// ConsumeMessages consumes messages from a topic
func (c *Client) ConsumeMessages(consumer sarama.Consumer, topic string, logMsg bool) {
	partitionConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		c.logger.Fatal("Failed to start consumer", zap.Error(err))
	}
	defer func() {
		if err := partitionConsumer.Close(); err != nil {
			c.logger.Fatal("Failed to close partition consumer", zap.Error(err))
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	consumed := 0
ConsumerLoop:
	for {
		select {
		case msg := <-partitionConsumer.Messages():
			c.logger.Info("Consumed message",
				zap.Int64("offset", msg.Offset),
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition))
			if logMsg {
				c.logger.Info("Message content",
					zap.ByteString("key", msg.Key),
					zap.ByteString("value", msg.Value))
			}
			consumed++
		case <-signals:
			break ConsumerLoop
		}
	}

	c.logger.Info("Consumption finished", zap.Int("consumed", consumed))
}

// ProduceMessage produces a message to a topic
func (c *Client) ProduceMessage(producer sarama.SyncProducer, topic string, message []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(message),
	}
	partition, offset, err := producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	c.logger.Info("Message produced",
		zap.String("topic", topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset))
	return nil
}

// ListTopics lists all topics
func (c *Client) ListTopics() (map[string]sarama.TopicDetail, error) {
	admin, err := c.newClusterAdmin()
	if err != nil {
		return nil, err
	}
	defer admin.Close()

	topics, err := admin.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	return topics, nil
}

// CreateTopic creates a new topic
func (c *Client) CreateTopic(topicName string, detail *sarama.TopicDetail) error {
	admin, err := c.newClusterAdmin()
	if err != nil {
		return err
	}
	defer admin.Close()

	err = admin.CreateTopic(topicName, detail, false)
	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	c.logger.Info("Topic created", zap.String("topic", topicName))
	return nil
}

package kafka

import (
	"log"
	"os"
	"os/signal"

	"github.com/IBM/sarama"
)

func CreateProducer(config KafkaConfig) (sarama.SyncProducer, error) {
	conf, brokers, err := InitConfig(config)
	if err != nil {
		return nil, err
	}

	producer, err := sarama.NewSyncProducer(brokers, conf)
	if err != nil {
		return nil, err
	}

	return producer, nil
}

func CreateConsumer(config KafkaConfig) (sarama.Consumer, error) {
	conf, brokers, err := InitConfig(config)
	if err != nil {
		return nil, err
	}

	consumer, err := sarama.NewConsumer(brokers, conf)
	if err != nil {
		return nil, err
	}

	return consumer, nil
}

func ConsumeMessages(consumer sarama.Consumer, config KafkaConfig) {
	partitionConsumer, err := consumer.ConsumePartition(config.Topic, 0, sarama.OffsetOldest)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := partitionConsumer.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	consumed := 0
ConsumerLoop:
	for {
		select {
		case msg := <-partitionConsumer.Messages():
			logger.Printf("Consumed message offset %d\n", msg.Offset)
			if config.LogMsg {
				log.Printf("KEY: %s VALUE: %s", msg.Key, msg.Value)
			}
			consumed++
		case <-signals:
			break ConsumerLoop
		}
	}

	logger.Printf("Consumed: %d\n", consumed)
}

func ProduceMessage(producer sarama.SyncProducer, topic string, message []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(message),
	}
	_, _, err := producer.SendMessage(msg)
	return err
}

func ListTopics(config KafkaConfig) (map[string]sarama.TopicDetail, error) {
	conf, brokers, err := InitConfig(config)
	if err != nil {
		return nil, err
	}

	admin, err := sarama.NewClusterAdmin(brokers, conf)
	if err != nil {
		return nil, err
	}
	defer admin.Close()

	topics, err := admin.ListTopics()
	if err != nil {
		return nil, err
	}

	return topics, nil
}

func CreateTopic(config KafkaConfig, topicName string, detail sarama.TopicDetail) error {
	conf, brokers, err := InitConfig(config)
	if err != nil {
		return err
	}

	admin, err := sarama.NewClusterAdmin(brokers, conf)
	if err != nil {
		return err
	}
	defer admin.Close()

	err = admin.CreateTopic(topicName, &detail, false)
	if err != nil {
		return err
	}

	return nil
}

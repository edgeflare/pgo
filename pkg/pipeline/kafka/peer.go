package kafka

import (
	"encoding/json"
	"log"

	"github.com/edgeflare/pgo/pkg/pipeline"
)

type PeerKafka struct{}

func (p *PeerKafka) Publish(data interface{}) error {
	// TODO: implement
	// send (possibly transformed) data to kafka topic
	log.Println(pipeline.ConnectorKafka, data)
	return nil
}

func (p *PeerKafka) Init(config json.RawMessage) error {
	// TODO: implement
	// parse config and set up kafka client
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorKafka, &PeerKafka{})
}

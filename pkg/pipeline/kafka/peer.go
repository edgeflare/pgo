package kafka

import (
	"encoding/json"
	"log"

	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/x/logrepl"
)

type PeerKafka struct{}

func (p *PeerKafka) Publish(event logrepl.PostgresCDC) error {
	// TODO: implement
	// send (possibly transformed) data to kafka topic
	log.Println(pipeline.ConnectorKafka, event)
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

package mqtt

import (
	"encoding/json"
	"log"

	"github.com/edgeflare/pgo/pkg/pipeline"
)

type PeerMQTT struct{}

func (p *PeerMQTT) Publish(data interface{}) error {
	// TODO: implement
	// send (possibly transformed) data to mqtt topic
	log.Println(pipeline.ConnectorMQTT, data)
	return nil
}

func (p *PeerMQTT) Init(config json.RawMessage) error {
	// TODO: implement
	// parse config and set up mqtt client
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorMQTT, &PeerMQTT{})
}

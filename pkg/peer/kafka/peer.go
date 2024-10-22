package kafka

import (
	"log"

	"github.com/edgeflare/pgo/pkg/peer"
)

type KafkaPeer struct{}

func (p *KafkaPeer) Publish(data interface{}) error {
	log.Println(peer.ConnectorKafka, data)
	return nil
}

func (p *KafkaPeer) Init() error {
	return nil
}

func init() {
	peer.RegisterConnector(peer.ConnectorKafka, &KafkaPeer{})
}

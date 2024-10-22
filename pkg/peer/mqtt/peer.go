package mqtt

import (
	"log"

	"github.com/edgeflare/pgo/pkg/peer"
)

type MqttPeer struct{}

func (p *MqttPeer) Publish(data interface{}) error {
	log.Println(peer.ConnectorMQTT, data)
	return nil
}

func (p *MqttPeer) Init() error {
	return nil
}

func init() {
	peer.RegisterConnector(peer.ConnectorMQTT, &MqttPeer{})
}

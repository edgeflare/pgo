package debug

import (
	"log"

	"github.com/edgeflare/pgo/pkg/peer"
)

type DebugPeer struct{}

func (p *DebugPeer) Publish(data interface{}) error {
	log.Println(peer.ConnectorDebug, data)
	return nil
}

func (p *DebugPeer) Init() error {
	return nil
}

func init() {
	peer.RegisterConnector(peer.ConnectorDebug, &DebugPeer{})
}

package debug

import (
	"encoding/json"
	"log"

	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/x/logrepl"
)

// PeerDebug is a debug peer that logs the data to the console
type PeerDebug struct{}

func (p *PeerDebug) Publish(event logrepl.PostgresCDC) error {
	log.Println(pipeline.ConnectorDebug, event)
	return nil
}

func (p *PeerDebug) Init(config json.RawMessage) error {
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorDebug, &PeerDebug{})
}

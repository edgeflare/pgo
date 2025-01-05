package debug

import (
	"encoding/json"
	"log"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pipeline"
)

// PeerDebug is a debug peer that logs the data to the console
type PeerDebug struct{}

func (p *PeerDebug) Pub(event pglogrepl.CDC, args ...any) error {
	log.Println(pipeline.ConnectorDebug, event)
	return nil
}

func (p *PeerDebug) Connect(config json.RawMessage, args ...any) error {
	return nil
}

func (p *PeerDebug) Sub(args ...any) (<-chan pglogrepl.CDC, error) {
	return nil, pipeline.ErrConnectorTypeMismatch
}

func (p *PeerDebug) Type() pipeline.ConnectorType {
	return pipeline.ConnectorTypePub
}

func (p *PeerDebug) Disconnect() error {
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorDebug, &PeerDebug{})
}

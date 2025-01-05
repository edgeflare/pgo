package main

import (
	"encoding/json"
	"log"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pipeline"
)

type PeerExample struct{}

func (p *PeerExample) Pub(event pglogrepl.CDC, args ...any) error {
	log.Println("example connector plugin publish", event)
	return nil
}

func (p *PeerExample) Connect(config json.RawMessage, args ...any) error {
	log.Println("example connector plugin init", config)
	return nil
}

func (p *PeerExample) Sub(args ...any) (<-chan pglogrepl.CDC, error) {
	// for pub-only peers (sinks), or implement for sub/pubsub peers
	return nil, pipeline.ErrConnectorTypeMismatch
}

func (p *PeerExample) Type() pipeline.ConnectorType {
	return pipeline.ConnectorTypePub
}

func (p *PeerExample) Disconnect() error {
	return nil
}

var Connector pipeline.Connector = &PeerExample{}

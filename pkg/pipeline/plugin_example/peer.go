package main

import (
	"encoding/json"
	"log"

	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/x/logrepl"
)

type PeerExample struct{}

func (p *PeerExample) Publish(event logrepl.PostgresCDC) error {
	log.Println("example connector plugin publish", event)
	return nil
}

func (p *PeerExample) Init(config json.RawMessage) error {
	log.Println("example connector plugin init", config)
	return nil
}

var Connector pipeline.Connector = &PeerExample{}

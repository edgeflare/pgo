package main

import (
	"encoding/json"
	"log"

	"github.com/edgeflare/pgo/pkg/pipeline"
)

type PeerExample struct{}

func (p *PeerExample) Publish(data interface{}) error {
	log.Println("example connector plugin publish", data)
	return nil
}

func (p *PeerExample) Init(config json.RawMessage) error {
	log.Println("example connector plugin init", config)
	return nil
}

var Connector pipeline.Connector = &PeerExample{}

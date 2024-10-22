package main

import (
	"log"

	"github.com/edgeflare/pgo/pkg/peer"
)

type ExamplePeer struct{}

func (p *ExamplePeer) Publish(data interface{}) error {
	log.Println("example connector plugin", data)
	return nil
}

func (p *ExamplePeer) Init() error {
	log.Println("example connector plugin init")
	return nil
}

var Connector peer.Connector = &ExamplePeer{}

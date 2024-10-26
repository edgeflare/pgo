package rand

import (
	mrand "math/rand"
)

var adjectives = []string{
	"agile", "brave", "calm", "daring", "eager",
	"fancy", "gentle", "happy", "intelligent", "jolly",
	"kind", "lively", "mighty", "noble", "optimistic",
	"playful", "quick", "radiant", "spirited", "trusty",
	"upbeat", "vibrant", "wise", "youthful", "zealous",
	"ambitious", "bright", "cheerful", "dynamic", "elegant",
	"fearless", "graceful", "hopeful", "inspired", "jovial",
	"keen", "loyal", "motivated", "nimble", "passionate",
	"resourceful", "sturdy", "tenacious", "uplifted", "vigorous",
	"warm", "xenial", "zesty",
}

var birds = []string{
	"albatross", "bluebird", "canary", "dove", "eagle",
	"falcon", "goldfinch", "hawk", "ibis", "jay",
	"kingfisher", "lark", "magpie", "nightingale", "oriole",
	"parrot", "quail", "robin", "sparrow", "toucan",
	"umbrella bird", "vulture", "woodpecker", "yellowhammer",
	"zebra finch", "avocet", "bunting", "crane", "duck",
	"egret", "flamingo", "goose", "heron", "indigo bunting",
	"junco", "kestrel", "loon", "mockingbird", "nuthatch",
	"owl", "pelican", "raven", "starling",
	"tern", "vireo", "wren", "xantus's hummingbird", "yellowthroat",
}

func NewName() string {
	adj := adjectives[mrand.Intn(len(adjectives))]
	bird := birds[mrand.Intn(len(birds))]
	return adj + "-" + bird
}

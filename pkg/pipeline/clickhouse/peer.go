package clickhouse

import (
	"encoding/json"
	"log"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/edgeflare/pgo/pkg/pipeline"
)

type ClickHousePeer struct {
	conn clickhouse.Conn
}

func (p *ClickHousePeer) Publish(data interface{}) error {
	// TODO: implement
	// send (possibly transformed) data to clickhouse
	log.Println(pipeline.ConnectorClickHouse, data)
	return nil
}

func (p *ClickHousePeer) Init(config json.RawMessage) error {
	// TODO: implement
	// parse config and set up clickhouse client
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorClickHouse, &ClickHousePeer{})
}

package clickhouse

import (
	"log"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/edgeflare/pgo/pkg/peer"
)

type ClickHousePeer struct {
	conn clickhouse.Conn
}

func (p *ClickHousePeer) Publish(data interface{}) error {
	log.Println(peer.ConnectorClickHouse, data)
	return nil
}

func (p *ClickHousePeer) Init() error {
	return nil
}

func init() {
	peer.RegisterConnector(peer.ConnectorClickHouse, &ClickHousePeer{})
}

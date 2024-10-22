package clickhouse

import (
	"crypto/tls"
	"fmt"
	"os"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func hello() {
	conn := clickhouse.OpenDB(&clickhouse.Options{
		Addr:     []string{os.Getenv("CLICKHOUSE_ADDR")},
		Protocol: clickhouse.Native,
		TLS:      &tls.Config{}, // enable secure TLS
		Auth: clickhouse.Auth{
			Username: os.Getenv("CLICKHOUSE_AUTH_USER"),
			Password: os.Getenv("CLICKHOUSE_AUTH_PASSWORD"),
		},
	})
	row := conn.QueryRow("SELECT 1")
	var col uint8
	if err := row.Scan(&col); err != nil {
		fmt.Printf("An error while reading the data: %s", err)
	} else {
		fmt.Printf("Result: %d", col)
	}
}

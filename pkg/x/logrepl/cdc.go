package logrepl

import (
	"context"
	"encoding/json"
)

type Peer[T Event] interface {
	Name() string
	Connect(ctx context.Context, config interface{}) error
	Disconnect(ctx context.Context) error
	Write(ctx context.Context, data T) error
	BatchWrite(ctx context.Context, data []T) error
	Read(ctx context.Context) (<-chan T, error)
	BatchRead(ctx context.Context, batchSize int) (<-chan []T, error)
	Errors() <-chan error
	Metadata() map[string]interface{}
	PauseStreaming(ctx context.Context) error
	ResumeStreaming(ctx context.Context) error
}

type Event interface {
	Type() string
	Encode() ([]byte, error)
	Decode([]byte) error
}

type PostgresEvent struct {
	Operation string                 `json:"operation"`
	Schema    string                 `json:"schema"`
	Table     string                 `json:"table"`
	Data      map[string]interface{} `json:"data,omitempty"`
	OldData   map[string]interface{} `json:"old_data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
	XID       uint32                 `json:"xid"`
}

func (e PostgresEvent) Type() string {
	return "PostgresEvent"
}

func (e PostgresEvent) Encode() ([]byte, error) {
	return json.Marshal(e)
}

func (e *PostgresEvent) Decode(data []byte) error {
	return json.Unmarshal(data, e)
}

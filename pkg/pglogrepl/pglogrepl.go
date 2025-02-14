// Package logrepl provides functionality for logical replication of PostgreSQL databases.
// It uses the github.com/jackc/pglogrepl library to connect to a PostgreSQL server and stream
// write-ahead log (WAL) changes in Debezium CDC format.
package pglogrepl

import (
	"fmt"
	"time"
)

// Op represents a type of database operation to be replicated.
type Op string

const (
	OpInsert   Op = "insert"
	OpUpdate   Op = "update"
	OpDelete   Op = "delete"
	OpTruncate Op = "truncate"

	defaultStandbyUpdateInterval = 10 * time.Second
	defaultBufferSize            = 1000
	defaultPublication           = "pgo_pub"
	defaultSlot                  = "pgo_slot"
	defaultPlugin                = "pgoutput"
)

// Config holds replication configuration.
type Config struct {
	Publication           string        `json:"publication"`
	Slot                  string        `json:"slot"`
	Plugin                string        `json:"plugin"`
	Tables                []string      `json:"tables"`
	AllTables             bool          `json:"allTables"`
	Schemas               []string      `json:"schemas"`
	Ops                   []Op          `json:"ops"`
	PartitionRoot         bool          `json:"partitionRoot"`
	StandbyUpdateInterval time.Duration `json:"standbyUpdateInterval"`
	// Manually execute for update eg ALTER TABLE schema_name.table_name REPLICA IDENTITY FULL;
	ReplicaIdentity map[string]Identity `json:"replicaIdentity"`
	BufferSize      int                 `json:"bufferSize"`
}

// Identity defines how UPDATE and DELETE operations identify rows.
type Identity string

const (
	IdentityDefault Identity = "d"
	IdentityNone    Identity = "n"
	IdentityFull    Identity = "f"
)

func DefaultConfig() *Config {
	return &Config{
		Publication:           defaultPublication,
		Slot:                  defaultSlot,
		Plugin:                defaultPlugin,
		StandbyUpdateInterval: defaultStandbyUpdateInterval,
		Ops:                   []Op{OpInsert, OpUpdate, OpDelete, OpTruncate},
		BufferSize:            defaultBufferSize,
	}
}

func validateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if cfg.AllTables && (len(cfg.Tables) > 0 || len(cfg.Schemas) > 0) {
		return fmt.Errorf("cannot specify AllTables with Tables or Schemas")
	}
	for _, op := range cfg.Ops {
		switch op {
		case OpInsert, OpUpdate, OpDelete, OpTruncate:
		default:
			return fmt.Errorf("invalid operation: %s", op)
		}
	}
	if cfg.StandbyUpdateInterval < time.Second {
		return fmt.Errorf("standby update interval must be at least 1 second")
	}
	return nil
}

func mergeWithDefaults(cfg *Config) *Config {
	def := DefaultConfig()
	if cfg == nil {
		return def
	}

	if cfg.Publication == "" {
		cfg.Publication = def.Publication
	}
	if cfg.Slot == "" {
		cfg.Slot = def.Slot
	}
	if cfg.Plugin == "" {
		cfg.Plugin = def.Plugin
	}
	if cfg.StandbyUpdateInterval == 0 {
		cfg.StandbyUpdateInterval = def.StandbyUpdateInterval
	}
	if len(cfg.Ops) == 0 {
		cfg.Ops = def.Ops
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = def.BufferSize
	}

	return cfg
}

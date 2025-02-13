package pglogrepl

import (
	"context"
	"fmt"
	"time"

	"github.com/edgeflare/pgo/pkg/pipeline/cdc"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	defaultPublication     = "pgo_pub"
	defaultReplicationSlot = "pgo_slot"
	defaultPlugin          = "pgoutput"
	defaultStandbyPeriod   = 10 * time.Second
)

// Config holds optional replication parameters
type Config struct {
	Publication       string        `json:"publication"`
	ReplicationSlot   string        `json:"replicationSlot"`
	Plugin            string        `json:"plugin"`
	StandbyPeriod     time.Duration `json:"standbyPeriod"`
	PublicationTables []string      `json:"publicationTables"`
}

// Stream starts logical replication and returns a channel of CDC events.
func Stream(ctx context.Context, conn *pgconn.PgConn, cfg *Config) (<-chan cdc.Event, error) {
	if conn == nil {
		return nil, fmt.Errorf("connection is required")
	}

	// If no config provided, use defaults
	if cfg == nil {
		cfg = &Config{}
	}

	// Apply defaults
	if cfg.Publication == "" {
		cfg.Publication = defaultPublication
	}
	if cfg.ReplicationSlot == "" {
		cfg.ReplicationSlot = defaultReplicationSlot
	}
	if cfg.Plugin == "" {
		cfg.Plugin = defaultPlugin
	}
	if cfg.StandbyPeriod == 0 {
		cfg.StandbyPeriod = defaultStandbyPeriod
	}

	if err := setupReplication(ctx, conn, cfg); err != nil {
		return nil, fmt.Errorf("setup replication: %w", err)
	}

	events := make(chan cdc.Event)
	go streamEvents(ctx, conn, cfg, events)
	return events, nil
}

func setupReplication(ctx context.Context, conn *pgconn.PgConn, cfg *Config) error {
	if err := ensurePublication(conn, cfg.Publication, cfg.PublicationTables); err != nil {
		return fmt.Errorf("publication: %w", err)
	}

	sysID, err := pglogrepl.IdentifySystem(ctx, conn)
	if err != nil {
		return fmt.Errorf("identify system: %w", err)
	}

	if err := ensureSlot(conn, cfg.ReplicationSlot, cfg.Plugin); err != nil {
		return fmt.Errorf("slot: %w", err)
	}

	opts := pglogrepl.StartReplicationOptions{
		PluginArgs: []string{
			"proto_version '2'",
			fmt.Sprintf("publication_names '%s'", cfg.Publication),
			"messages 'true'",
			"streaming 'true'",
		},
	}

	return pglogrepl.StartReplication(ctx, conn, cfg.ReplicationSlot, sysID.XLogPos, opts)
}
func streamEvents(ctx context.Context, conn *pgconn.PgConn, cfg *Config, events chan<- cdc.Event) {
	defer close(events)
	relations := make(map[uint32]*pglogrepl.RelationMessageV2)
	typeMap := pgtype.NewMap()
	nextStandby := time.Now().Add(cfg.StandbyPeriod)
	var walPos pglogrepl.LSN
	inStream := false

	for {
		if time.Now().After(nextStandby) {
			if err := sendStandby(ctx, conn, walPos); err != nil {
				return
			}
			nextStandby = time.Now().Add(cfg.StandbyPeriod)
		}

		msg, err := receiveMessage(ctx, conn, nextStandby)
		if err != nil {
			if !pgconn.Timeout(err) {
				return
			}
			continue
		}

		copyData, ok := msg.(*pgproto3.CopyData)
		if !ok {
			continue
		}

		walPos = handleMessage(copyData, walPos, relations, typeMap, &inStream, events,
			conn.Conn().RemoteAddr().String())
	}
}

func sendStandby(ctx context.Context, conn *pgconn.PgConn, pos pglogrepl.LSN) error {
	return pglogrepl.SendStandbyStatusUpdate(ctx, conn, pglogrepl.StandbyStatusUpdate{WALWritePosition: pos})
}

func receiveMessage(ctx context.Context, conn *pgconn.PgConn, deadline time.Time) (pgproto3.BackendMessage, error) {
	msgCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	return conn.ReceiveMessage(msgCtx)
}

func handleMessage(msg *pgproto3.CopyData, walPos pglogrepl.LSN,
	relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map,
	inStream *bool, events chan<- cdc.Event, addr string) pglogrepl.LSN {

	switch msg.Data[0] {
	case pglogrepl.PrimaryKeepaliveMessageByteID:
		pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
		if err != nil {
			return walPos
		}
		if pkm.ServerWALEnd > walPos {
			walPos = pkm.ServerWALEnd
		}

	case pglogrepl.XLogDataByteID:
		xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
		if err != nil {
			return walPos
		}
		if xld.WALStart > walPos {
			walPos = xld.WALStart
		}
		for _, event := range processV2(xld.WALData, relations, typeMap, inStream, "", addr) {
			events <- event
		}
	}
	return walPos
}

func ensurePublication(conn *pgconn.PgConn, name string, tables []string) error {
	exists, err := checkExists(conn, "pg_publication", "pubname", name)
	if err != nil {
		return err
	}

	if !exists {
		result := conn.Exec(context.Background(), fmt.Sprintf("CREATE PUBLICATION %s", name))
		_, err := result.ReadAll()
		if err != nil {
			return fmt.Errorf("create publication: %w", err)
		}
	}

	for _, table := range tables {
		result := conn.Exec(context.Background(),
			fmt.Sprintf("ALTER PUBLICATION %s ADD TABLE %s", name, table))
		_, err := result.ReadAll()
		if err != nil {
			// Check if table is already in publication
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.SQLState() == "42710" {
				continue
			}
			return fmt.Errorf("add table to publication: %w", err)
		}
	}
	return nil
}

func ensureSlot(conn *pgconn.PgConn, name, plugin string) error {
	exists, err := checkExists(conn, "pg_replication_slots", "slot_name", name)
	if err != nil {
		return err
	}

	if !exists {
		_, err = pglogrepl.CreateReplicationSlot(context.Background(), conn, name, plugin,
			pglogrepl.CreateReplicationSlotOptions{Temporary: false})
		return err
	}
	return nil
}

func checkExists(conn *pgconn.PgConn, table, column, value string) (bool, error) {
	if table != "pg_publication" && table != "pg_replication_slots" {
		return false, fmt.Errorf("invalid table name")
	}
	if column != "pubname" && column != "slot_name" {
		return false, fmt.Errorf("invalid column name")
	}

	result := conn.Exec(context.Background(),
		fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE %s = '%s')", table, column, value))
	rows, err := result.ReadAll()
	if err != nil {
		return false, fmt.Errorf("check exists: %w", err)
	}
	return len(rows) > 0 && len(rows[0].Rows) > 0 && string(rows[0].Rows[0][0]) == "t", nil
}

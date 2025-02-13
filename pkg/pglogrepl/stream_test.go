package pglogrepl

import (
	"context"
	"testing"
	"time"

	"github.com/edgeflare/pgo/internal/testutil/pgtest"
	"github.com/edgeflare/pgo/pkg/pipeline/cdc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestStream(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgtest.WithConn(t, func(conn *pgx.Conn) {
		_, err := conn.Exec(ctx, `
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_replication
			WHERE application_name LIKE 'test_slot%';
		`)
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		_, err = conn.Exec(ctx, `
			SELECT pg_drop_replication_slot(slot_name)
			FROM pg_replication_slots
			WHERE slot_name = 'test_slot';
		`)
		require.NoError(t, err)

		_, err = conn.Exec(ctx, `
			DROP PUBLICATION IF EXISTS test_pub;
			DROP TABLE IF EXISTS test_users;
		`)
		require.NoError(t, err)

		_, err = conn.Exec(ctx, `
			CREATE TABLE test_users (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL
			);
		`)
		require.NoError(t, err)

		_, err = conn.Exec(ctx, `CREATE PUBLICATION test_pub FOR TABLE test_users;`)
		require.NoError(t, err)

		replConn, err := pgconn.Connect(ctx, conn.Config().ConnString()+"?replication=database&application_name=test_slot_"+time.Now().Format("20060102150405"))
		require.NoError(t, err)
		defer func() {
			cancel()
			time.Sleep(100 * time.Millisecond)
			replConn.Close(context.Background())

			cleanupCtx := context.Background()
			cleanupConn, err := pgx.Connect(cleanupCtx, conn.Config().ConnString())
			if err == nil {
				defer cleanupConn.Close(cleanupCtx)
				cleanupConn.Exec(cleanupCtx, `
					SELECT pg_terminate_backend(pid)
					FROM pg_stat_replication
					WHERE application_name LIKE 'test_slot%';
					SELECT pg_drop_replication_slot(slot_name)
					FROM pg_replication_slots
					WHERE slot_name = 'test_slot';
					DROP PUBLICATION IF EXISTS test_pub;
					DROP TABLE IF EXISTS test_users;
				`)
			}
		}()

		cfg := Config{
			Publication:       "test_pub",
			ReplicationSlot:   "test_slot",
			Plugin:            "pgoutput",
			PublicationTables: []string{"public.test_users"},
			StandbyPeriod:     1 * time.Second,
		}

		events, err := Stream(ctx, replConn, &cfg)
		require.NoError(t, err)

		collected := make([]cdc.Event, 0)
		done := make(chan struct{})

		go func() {
			defer close(done)
			for event := range events {
				t.Logf("Received event: Op=%v, Before=%v, After=%v",
					event.Payload.Op,
					event.Payload.Before,
					event.Payload.After)
				collected = append(collected, event)
				if len(collected) >= 4 {
					return
				}
			}
		}()

		_, err = conn.Exec(ctx, `INSERT INTO test_users (name) VALUES ('alice');`)
		require.NoError(t, err)

		_, err = conn.Exec(ctx, `UPDATE test_users SET name = 'alice-updated' WHERE name = 'alice';`)
		require.NoError(t, err)

		_, err = conn.Exec(ctx, `UPDATE test_users SET name = 'alice-final' WHERE name = 'alice-updated';`)
		require.NoError(t, err)

		_, err = conn.Exec(ctx, `DELETE FROM test_users WHERE name = 'alice-final';`)
		require.NoError(t, err)

		select {
		case <-done:
			// Success
		case <-time.After(10 * time.Second):
			t.Fatal("timeout waiting for replication events")
		}

		// verify we got exactly 4 events
		require.Len(t, collected, 4, "expected exactly 4 events")

		// verify INSERT event
		insertEvent := collected[0]
		require.Equal(t, cdc.OpCreate, insertEvent.Payload.Op, "first event should be CREATE")
		require.Nil(t, insertEvent.Payload.Before, "INSERT event should have no Before value")
		afterInsert, ok := insertEvent.Payload.After.(map[string]interface{})
		require.True(t, ok, "After payload should be a map")
		require.Equal(t, "alice", afterInsert["name"], "INSERT event should have correct name")
		require.NotNil(t, afterInsert["id"], "INSERT event should have an ID")

		// verify first UPDATE event
		update1Event := collected[1]
		require.Equal(t, cdc.OpUpdate, update1Event.Payload.Op, "second event should be UPDATE")
		afterUpdate1, ok := update1Event.Payload.After.(map[string]interface{})
		require.True(t, ok, "After payload should be a map")
		require.Equal(t, "alice-updated", afterUpdate1["name"], "first UPDATE should have updated name")
		require.Equal(t, afterInsert["id"], afterUpdate1["id"], "UPDATE should preserve ID")

		// verify second UPDATE event
		update2Event := collected[2]
		require.Equal(t, cdc.OpUpdate, update2Event.Payload.Op, "third event should be UPDATE")
		afterUpdate2, ok := update2Event.Payload.After.(map[string]interface{})
		require.True(t, ok, "After payload should be a map")
		require.Equal(t, "alice-final", afterUpdate2["name"], "second UPDATE should have final name")
		require.Equal(t, afterInsert["id"], afterUpdate2["id"], "UPDATE should preserve ID")

		// verify DELETE event
		deleteEvent := collected[3]
		require.Equal(t, cdc.OpDelete, deleteEvent.Payload.Op, "fourth event should be DELETE")
		require.Nil(t, deleteEvent.Payload.After, "DELETE event should have no After value")
		beforeDelete, ok := deleteEvent.Payload.Before.(map[string]interface{})
		require.True(t, ok, "Before payload should be a map")
		require.Equal(t, afterInsert["id"], beforeDelete["id"], "DELETE should reference same ID")
	})
}

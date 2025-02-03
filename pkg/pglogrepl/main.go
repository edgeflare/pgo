// Package logrepl provides functionality for logical replication of PostgreSQL databases.
// It uses the pglogrepl library to connect to a PostgreSQL server and stream changes
// from the write-ahead log (WAL).
package pglogrepl

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	logger, _ = zap.NewProduction()
	zap.ReplaceGlobals(logger)
}

// Main starts the logical replication process and returns a channel of PostgresCDC events.
// It sets up the necessary publication and replication slot, and begins streaming changes from the WAL.
func Main(ctx context.Context, conn *pgconn.PgConn, publicationTables ...string) (<-chan CDC, error) {
	cdcEventsChan := make(chan CDC)
	dbHost := conn.Conn().RemoteAddr().String()

	publicationExists, err := checkPublicationExists(conn, publicationName)
	if err != nil {
		log.Println("checkPublicationExists failed:", err)
		return nil, err
	}
	if !publicationExists {
		err = createPublication(conn, publicationName)
		if err != nil {
			log.Fatalln("createPublication failed:", err)
			return nil, err
		}
		logger.Info("Created publication", zap.String("publicationName", publicationName))
	} else {
		logger.Info("Publication", zap.String("publicationName", publicationName), zap.Error(err))
	}

	// Add tables to the publication as needed
	// tableNames := strings.Split(os.Getenv("PGO_POSTGRES_LOGREPL_TABLES"), ",")
	for _, fullTableName := range publicationTables {
		fullTableName = strings.TrimSpace(fullTableName)
		if fullTableName == "" {
			continue
		}

		parts := strings.Split(fullTableName, ".")
		var schemaName, tableName string
		if len(parts) == 2 {
			schemaName, tableName = parts[0], parts[1]
		} else {
			schemaName, tableName = "public", fullTableName
		}

		err = addTableToPublication(conn, publicationName, schemaName, tableName)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.SQLState() == "42710" {
				// Table is already a member of the publication
				logger.Info("Table is already a member of publication",
					zap.String("publicationName", publicationName),
					zap.String("table", fullTableName))
			} else {
				// Other errors
				log.Println("Failed to add table to publication:", err)
				logger.Error("Failed to add table to publication", zap.Error(err))
			}
		} else {
			logger.Info("Added table to publication",
				zap.String("publicationName", publicationName),
				zap.String("table", fullTableName))
		}
	}

	var pluginArguments []string
	var v2 bool
	if outputPlugin == "pgoutput" {
		// streaming of large transactions is available since PG 14 (protocol version 2)
		// we also need to set 'streaming' to 'true'
		pluginArguments = []string{
			"proto_version '2'",
			fmt.Sprintf("publication_names '%s'", publicationName), // use the publicationName variable
			"messages 'true'",
			"streaming 'true'",
		}
		v2 = true
		// uncomment for v1
		// pluginArguments = []string{
		//	"proto_version '1'",
		//	fmt.Sprintf("publication_names '%s'", publicationName),
		//	"messages 'true'",
		// }
	} else if outputPlugin == "wal2json" {
		pluginArguments = []string{"\"pretty-print\" 'true'"}
	}

	sysident, err := pglogrepl.IdentifySystem(context.Background(), conn)
	if err != nil {
		logger.Error("IdentifySystem failed", zap.Error(err))
		return nil, err
	}
	// log.Println("SystemID:", sysident.SystemID, "Timeline:", sysident.Timeline, "XLogPos:", sysident.XLogPos, "DBName:", sysident.DBName)
	logger.Info("System identified",
		zap.String("SystemID", sysident.SystemID),
		zap.Int32("Timeline", sysident.Timeline),
		zap.String("XLogPos", sysident.XLogPos.String()),
		zap.String("DBName", sysident.DBName))

	slotExists, err := checkSlotExists(conn, slotName)
	if err != nil {
		log.Println("checkSlotExists failed:", err)
		conn.Close(context.Background())
		return nil, err
	}
	if !slotExists {
		err = createReplicationSlot(conn, slotName, outputPlugin)
		if err != nil {
			log.Fatalln("createReplicationSlot failed:", err)
			conn.Close(context.Background())
			return nil, err
		}
		// log.Println("Created replication slot", slotName)
		logger.Info("Created replication slot", zap.String("slotName", slotName))
	} else {
		// log.Println("Replication slot", slotName, "already exists")
		logger.Info("Replication slot already exists", zap.String("slotName", slotName))
	}

	err = pglogrepl.StartReplication(context.Background(), conn, slotName, sysident.XLogPos, pglogrepl.StartReplicationOptions{PluginArgs: pluginArguments})
	if err != nil {
		log.Fatalln("StartReplication failed:", err)
	}
	// log.Println("Logical replication started on slot", slotName)
	logger.Info("Logical replication started on slot", zap.String("slotName", slotName))

	clientXLogPos := sysident.XLogPos
	standbyMessageTimeout := time.Second * 10
	nextStandbyMessageDeadline := time.Now().Add(standbyMessageTimeout)
	relations := map[uint32]*pglogrepl.RelationMessage{}
	relationsV2 := map[uint32]*pglogrepl.RelationMessageV2{}
	typeMap := pgtype.NewMap()

	// whenever we get StreamStartMessage we set inStream to true and then pass it to DecodeV2 function
	// on StreamStopMessage we set it back to false
	inStream := false

	go func() {
		defer close(cdcEventsChan)
		for {
			if time.Now().After(nextStandbyMessageDeadline) {
				err = pglogrepl.SendStandbyStatusUpdate(context.Background(), conn, pglogrepl.StandbyStatusUpdate{WALWritePosition: clientXLogPos})
				if err != nil {
					log.Fatalln("SendStandbyStatusUpdate failed:", err)
				}
				// log.Printf("Sent Standby status message at %s\n", clientXLogPos.String())
				nextStandbyMessageDeadline = time.Now().Add(standbyMessageTimeout)
			}

			ctx, cancel := context.WithDeadline(context.Background(), nextStandbyMessageDeadline)
			rawMsg, err := conn.ReceiveMessage(ctx)
			cancel()
			if err != nil {
				if pgconn.Timeout(err) {
					continue
				}
				logger.Error("ReceiveMessage failed", zap.Error(err))
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					logger.Warn("Connection closed, attempting to reconnect")
					if err := reconnect(ctx, conn); err != nil {
						logger.Error("Failed to reconnect", zap.Error(err))
						return
					}
					continue
				}
				return
			}

			if errMsg, ok := rawMsg.(*pgproto3.ErrorResponse); ok {
				log.Fatalf("received Postgres WAL error: %+v", errMsg)
			}

			msg, ok := rawMsg.(*pgproto3.CopyData)
			if !ok {
				log.Printf("Received unexpected message: %T\n", rawMsg)
				continue
			}

			switch msg.Data[0] {
			case pglogrepl.PrimaryKeepaliveMessageByteID:
				pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
				if err != nil {
					log.Fatalln("ParsePrimaryKeepaliveMessage failed:", err)
				}
				// log.Println("Primary Keepalive Message =>", "ServerWALEnd:", pkm.ServerWALEnd, "ServerTime:", pkm.ServerTime, "ReplyRequested:", pkm.ReplyRequested)
				if pkm.ServerWALEnd > clientXLogPos {
					clientXLogPos = pkm.ServerWALEnd
				}
				if pkm.ReplyRequested {
					nextStandbyMessageDeadline = time.Time{}
				}

			case pglogrepl.XLogDataByteID:
				xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
				if err != nil {
					log.Fatalln("ParseXLogData failed:", err)
				}

				if outputPlugin == "wal2json" {
					log.Printf("wal2json data: %s\n", string(xld.WALData))
				} else {
					// log.Printf("XLogData => WALStart %s ServerWALEnd %s ServerTime %s WALData:\n", xld.WALStart, xld.ServerWALEnd, xld.ServerTime)
					if v2 {
						events := processV2(xld.WALData, relationsV2, typeMap, &inStream, sysident.DBName, dbHost)
						for _, event := range events {
							cdcEventsChan <- event
						}
					} else {
						events, err := processV1(xld.WALData, relations, typeMap)
						if err != nil {
							logger.Error("Error processing V1 WAL data", zap.Error(err))
							continue
						}
						for _, event := range events {
							cdcEventsChan <- event
						}
					}
				}

				if xld.WALStart > clientXLogPos {
					clientXLogPos = xld.WALStart
				}
			}
		}
	}()

	return cdcEventsChan, nil
}

// reconnect attempts to re-establish a connection to the PostgreSQL server.
// It will try to reconnect up to maxRetries times, with an increasing delay between attempts.
//
// If successful, it returns nil. If all attempts fail, it returns an error.
func reconnect(ctx context.Context, conn *pgconn.PgConn) error {
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		logger.Info("Attempting to reconnect", zap.Int("attempt", i+1))
		err := conn.Ping(ctx)
		if err == nil {
			logger.Info("Reconnection successful")
			return nil
		}
		logger.Warn("Reconnection failed", zap.Error(err))
		time.Sleep(time.Second * time.Duration(i+1))
	}
	return fmt.Errorf("failed to reconnect after %d attempts", maxRetries)
}

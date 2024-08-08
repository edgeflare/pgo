package logrepl

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

func Run(ctx context.Context, p Peer[Event]) {
	log, _ := zap.NewProduction()
	conn, sysident, err := SetupReplication()
	if err != nil {
		log.Fatal("SetupReplication failed", zap.Error(err))
	}
	defer conn.Close(context.Background())

	pluginArguments := []string{
		"proto_version '2'",
		"publication_names 'pglogrepl_demo'",
		"messages 'true'",
		"streaming 'true'",
	}

	err = StartReplication(conn, sysident, pluginArguments)
	if err != nil {
		log.Fatal("StartReplication failed", zap.Error(err))
	}

	clientXLogPos := sysident.XLogPos
	standbyMessageTimeout := standbyMessageTimer
	nextStandbyMessageDeadline := time.Now().Add(standbyMessageTimeout)
	relations := map[uint32]*pglogrepl.RelationMessage{}
	relationsV2 := map[uint32]*pglogrepl.RelationMessageV2{}
	typeMap := pgtype.NewMap()
	inStream := false

	go func() {
		for {
			select {
			case err := <-p.Errors():
				log.Error("Error", zap.Error(err))
			case <-ctx.Done():
				log.Info("Shutting down error listener")
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Info("Received shutdown signal, exiting replication loop")
			return
		default:
			if time.Now().After(nextStandbyMessageDeadline) {
				err = pglogrepl.SendStandbyStatusUpdate(context.Background(), conn, pglogrepl.StandbyStatusUpdate{WALWritePosition: clientXLogPos})
				if err != nil {
					log.Fatal("SendStandbyStatusUpdate failed", zap.Error(err))
				}
				log.Info("Sent Standby status message", zap.String("position", clientXLogPos.String()))
				nextStandbyMessageDeadline = time.Now().Add(standbyMessageTimeout)
			}

			ctx, cancel := context.WithDeadline(context.Background(), nextStandbyMessageDeadline)
			rawMsg, err := conn.ReceiveMessage(ctx)
			cancel()
			if err != nil {
				if pgconn.Timeout(err) {
					continue
				}
				log.Fatal("ReceiveMessage failed", zap.Error(err))
			}

			if errMsg, ok := rawMsg.(*pgproto3.ErrorResponse); ok {
				log.Fatal("received Postgres WAL error", zap.Any("error", errMsg))
			}

			msg, ok := rawMsg.(*pgproto3.CopyData)
			if !ok {
				log.Info("Received unexpected message", zap.Any("message", rawMsg))
				continue
			}

			switch msg.Data[0] {
			case pglogrepl.PrimaryKeepaliveMessageByteID:
				handlePrimaryKeepaliveMessage(msg.Data[1:], &clientXLogPos, &nextStandbyMessageDeadline, log)

			case pglogrepl.XLogDataByteID:
				xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
				if err != nil {
					log.Fatal("ParseXLogData failed", zap.Error(err))
				}

				if os.Getenv("PGO_POSTGRES_LOGREPL_OUTPUT_PLUGIN") == "wal2json" {
					log.Info("wal2json data", zap.String("data", string(xld.WALData)))
				} else {
					log.Info("XLogData",
						zap.String("WALStart", xld.WALStart.String()),
						zap.String("ServerWALEnd", xld.ServerWALEnd.String()),
						zap.Time("ServerTime", xld.ServerTime))

					var events []Event
					if len(relationsV2) > 0 {
						events = processV2(xld.WALData, relationsV2, typeMap, &inStream)
					} else {
						events = processV1(xld.WALData, relations, typeMap)
					}

					for _, event := range events {
						if err := p.Write(context.Background(), event); err != nil {
							log.Error("Error writing event", zap.Error(err))
						}
					}
				}

				if xld.WALStart > clientXLogPos {
					clientXLogPos = xld.WALStart
				}
			}
		}
	}
}

func handlePrimaryKeepaliveMessage(data []byte, clientXLogPos *pglogrepl.LSN, nextStandbyMessageDeadline *time.Time, log *zap.Logger) {
	pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(data)
	if err != nil {
		log.Fatal("ParsePrimaryKeepaliveMessage failed", zap.Error(err))
	}
	log.Info("Primary Keepalive Message",
		zap.String("ServerWALEnd", pkm.ServerWALEnd.String()),
		zap.Time("ServerTime", pkm.ServerTime),
		zap.Bool("ReplyRequested", pkm.ReplyRequested))
	if pkm.ServerWALEnd > *clientXLogPos {
		*clientXLogPos = pkm.ServerWALEnd
	}
	if pkm.ReplyRequested {
		*nextStandbyMessageDeadline = time.Time{}
	}
}

func StartReplication(conn *pgconn.PgConn, sysident pglogrepl.IdentifySystemResult, pluginArguments []string) error {
	err := pglogrepl.StartReplication(context.Background(), conn, slotName, sysident.XLogPos, pglogrepl.StartReplicationOptions{PluginArgs: pluginArguments})
	return err
}

package pglogrepl

import (
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

func processV2(walData []byte, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map, inStream *bool) []CDC {
	logicalMsg, err := pglogrepl.ParseV2(walData, *inStream)
	if err != nil {
		zap.L().Fatal("ParseV2 failed", zap.Error(err))
	}
	var cdcEvents []CDC
	switch logicalMsg := logicalMsg.(type) {
	case *pglogrepl.RelationMessageV2:
		relations[logicalMsg.RelationID] = logicalMsg
		// zap.L().Info("Relation message received", zap.Uint32("relationID", logicalMsg.RelationID))

	case *pglogrepl.BeginMessage:
		// zap.L().Info("Begin message", zap.Uint32("xid", logicalMsg.Xid))

	case *pglogrepl.CommitMessage:
		// zap.L().Info("Commit message", zap.Uint32("xid", uint32(logicalMsg.TransactionEndLSN)))

	case *pglogrepl.InsertMessageV2:
		cdcEvent := handleInsertMessageV2(logicalMsg, relations, typeMap, "serverName", "dbName", int64(logicalMsg.Xid))
		cdcEvents = append(cdcEvents, cdcEvent)
		// Remove the logging from here

	case *pglogrepl.UpdateMessageV2:
		cdcEvent := handleUpdateMessageV2(logicalMsg, relations, typeMap, "serverName", "dbName", int64(logicalMsg.Xid))
		cdcEvents = append(cdcEvents, cdcEvent)
		// Remove the logging from here

	case *pglogrepl.DeleteMessageV2:
		cdcEvent := handleDeleteMessageV2(logicalMsg, relations, typeMap, "serverName", "dbName", int64(logicalMsg.Xid))
		cdcEvents = append(cdcEvents, cdcEvent)
		// Remove the logging from here

	case *pglogrepl.TruncateMessageV2:
		cdcEvent := handleTruncateMessageV2(logicalMsg, relations, "serverName", "dbName", int64(logicalMsg.Xid))
		cdcEvents = append(cdcEvents, cdcEvent)
		// Remove the logging from here

	case *pglogrepl.TypeMessageV2:
		zap.L().Info("Type message received")
	case *pglogrepl.OriginMessage:
		zap.L().Info("Origin message received")
	case *pglogrepl.LogicalDecodingMessageV2:
		zap.L().Info("Logical decoding message", zap.String("prefix", logicalMsg.Prefix), zap.String("content", string(logicalMsg.Content)))
	case *pglogrepl.StreamStartMessageV2:
		*inStream = true
		zap.L().Info("Stream start message", zap.Uint32("xid", logicalMsg.Xid))
	case *pglogrepl.StreamStopMessageV2:
		*inStream = false
		zap.L().Info("Stream stop message")
	case *pglogrepl.StreamCommitMessageV2:
		zap.L().Info("Stream commit message", zap.Uint32("xid", logicalMsg.Xid))
	case *pglogrepl.StreamAbortMessageV2:
		zap.L().Info("Stream abort message", zap.Uint32("xid", logicalMsg.Xid))
	default:
		zap.L().Warn("Unknown message type in pgoutput stream", zap.Any("message", logicalMsg))
	}

	return cdcEvents
}

func handleInsertMessageV2(msg *pglogrepl.InsertMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map, serverName, dbName string, lsn int64) CDC {
	rel, ok := relations[msg.RelationID]
	if !ok {
		zap.L().Error("unknown relation ID", zap.Uint32("relationID", msg.RelationID))
		return CDC{}
	}

	values := make(map[string]interface{})
	for idx, col := range msg.Tuple.Columns {
		colName := rel.Columns[idx].Name
		values[colName] = decodeColumn(col, typeMap, rel.Columns[idx].DataType)
	}

	event := CDC{
		Schema: getDefaultSchema(),
	}
	event.Payload.Before = nil
	event.Payload.After = values
	event.Payload.Source = createSource(serverName, dbName, msg, rel, lsn)
	event.Payload.Op = "c"
	event.Payload.TsMs = time.Now().UnixMilli()

	return event
}

func handleUpdateMessageV2(msg *pglogrepl.UpdateMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map, serverName, dbName string, lsn int64) CDC {
	rel, ok := relations[msg.RelationID]
	if !ok {
		zap.L().Error("unknown relation ID", zap.Uint32("relationID", msg.RelationID))
		return CDC{}
	}

	var oldValues, newValues map[string]interface{}

	if msg.OldTuple != nil {
		oldValues = make(map[string]interface{})
		for idx, col := range msg.OldTuple.Columns {
			colName := rel.Columns[idx].Name
			oldValues[colName] = decodeColumn(col, typeMap, rel.Columns[idx].DataType)
		}
	}

	if msg.NewTuple != nil {
		newValues = make(map[string]interface{})
		for idx, col := range msg.NewTuple.Columns {
			colName := rel.Columns[idx].Name
			newValues[colName] = decodeColumn(col, typeMap, rel.Columns[idx].DataType)
		}
	}

	event := CDC{
		Schema: getDefaultSchema(),
	}
	event.Payload.Before = oldValues
	event.Payload.After = newValues
	event.Payload.Source = createSource(serverName, dbName, msg, rel, lsn)
	event.Payload.Op = "u"
	event.Payload.TsMs = time.Now().UnixMilli()

	return event
}

func handleDeleteMessageV2(msg *pglogrepl.DeleteMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map, serverName, dbName string, lsn int64) CDC {
	rel, ok := relations[msg.RelationID]
	if !ok {
		zap.L().Error("unknown relation ID", zap.Uint32("relationID", msg.RelationID))
		return CDC{}
	}

	oldValues := make(map[string]interface{})
	for idx, col := range msg.OldTuple.Columns {
		colName := rel.Columns[idx].Name
		oldValues[colName] = decodeColumn(col, typeMap, rel.Columns[idx].DataType)
	}

	event := CDC{
		Schema: getDefaultSchema(),
	}
	event.Payload.Before = oldValues
	event.Payload.After = nil
	event.Payload.Source = createSource(serverName, dbName, msg, rel, lsn)
	event.Payload.Op = "d"
	event.Payload.TsMs = time.Now().UnixMilli()

	return event
}

func handleTruncateMessageV2(msg *pglogrepl.TruncateMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2, serverName, dbName string, lsn int64) CDC {
	// Get the first relation for basic source info
	var rel *pglogrepl.RelationMessageV2
	for _, relation := range relations {
		rel = relation
		break
	}

	if rel == nil {
		zap.L().Error("no relations found for truncate message")
		return CDC{}
	}

	event := CDC{
		Schema: getDefaultSchema(),
	}
	event.Payload.Before = nil
	event.Payload.After = nil
	event.Payload.Source = createSource(serverName, dbName, msg, rel, lsn)
	event.Payload.Op = "t"
	event.Payload.TsMs = time.Now().UnixMilli()

	return event
}

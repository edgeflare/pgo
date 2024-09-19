package logrepl

import (
	"strconv"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// processV2 parses and processes WAL data using logical replication protocol version 2.
// It returns a slice of PostgresCDC events representing the changes in the WAL.
//
// The function takes the following parameters:
//   - walData: raw WAL data bytes
//   - relations: a map of relation IDs to their corresponding RelationMessageV2
//   - typeMap: a pgtype.Map for decoding column values
//   - inStream: a pointer to a boolean indicating whether processing is within a stream
//
// It handles various logical replication message types and delegates specific operations
// to helper functions for insert, update, delete, and truncate operations.
func processV2(walData []byte, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map, inStream *bool) []PostgresCDC {
	logicalMsg, err := pglogrepl.ParseV2(walData, *inStream)
	if err != nil {
		zap.L().Fatal("ParseV2 failed", zap.Error(err))
	}
	var cdcEvents []PostgresCDC
	switch logicalMsg := logicalMsg.(type) {
	case *pglogrepl.RelationMessageV2:
		relations[logicalMsg.RelationID] = logicalMsg
		// zap.L().Info("Relation message received", zap.Uint32("relationID", logicalMsg.RelationID))

	case *pglogrepl.BeginMessage:
		// zap.L().Info("Begin message", zap.Uint32("xid", logicalMsg.Xid))

	case *pglogrepl.CommitMessage:
		// zap.L().Info("Commit message", zap.Uint32("xid", uint32(logicalMsg.TransactionEndLSN)))

	case *pglogrepl.InsertMessageV2:
		cdcEvent := handleInsertMessageV2(logicalMsg, relations, typeMap)
		cdcEvents = append(cdcEvents, cdcEvent)
		// Remove the logging from here

	case *pglogrepl.UpdateMessageV2:
		cdcEvent := handleUpdateMessageV2(logicalMsg, relations, typeMap)
		cdcEvents = append(cdcEvents, cdcEvent)
		// Remove the logging from here

	case *pglogrepl.DeleteMessageV2:
		cdcEvent := handleDeleteMessageV2(logicalMsg, relations, typeMap)
		cdcEvents = append(cdcEvents, cdcEvent)
		// Remove the logging from here

	case *pglogrepl.TruncateMessageV2:
		cdcEvent := handleTruncateMessageV2(logicalMsg, relations)
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

// handleInsertMessageV2 processes an insert message and returns a PostgresCDC event.
//
// It takes the following parameters:
//   - msg: the InsertMessageV2 containing the inserted tuple
//   - relations: a map of relation IDs to their corresponding RelationMessageV2
//   - typeMap: a pgtype.Map for decoding column values
//
// The function constructs a PostgresCDC event with the "INSERT" operation, including
// the schema, table name, inserted data, timestamp, and transaction ID.
func handleInsertMessageV2(msg *pglogrepl.InsertMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map) PostgresCDC {
	rel, ok := relations[msg.RelationID]
	if !ok {
		zap.L().Error("unknown relation ID", zap.Uint32("relationID", msg.RelationID))
		return PostgresCDC{}
	}
	values := map[string]interface{}{}
	for idx, col := range msg.Tuple.Columns {
		colName := rel.Columns[idx].Name
		values[colName] = decodeColumn(col, typeMap, rel.Columns[idx].DataType)
	}
	return PostgresCDC{
		Operation: "INSERT",
		Schema:    rel.Namespace,
		Table:     rel.RelationName,
		Data:      values,
		Timestamp: time.Now().Unix(),
		XID:       msg.Xid,
	}
}

// handleUpdateMessageV2 processes an update message and returns a PostgresCDC event.
//
// It takes the following parameters:
//   - msg: the UpdateMessageV2 containing the updated tuple
//   - relations: a map of relation IDs to their corresponding RelationMessageV2
//   - typeMap: a pgtype.Map for decoding column values
//
// The function constructs a PostgresCDC event with the "UPDATE" operation, including
// the schema, table name, new and old data, timestamp, and transaction ID.
func handleUpdateMessageV2(msg *pglogrepl.UpdateMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map) PostgresCDC {
	rel, ok := relations[msg.RelationID]
	if !ok {
		panic("unknown relation ID " + strconv.FormatUint(uint64(msg.RelationID), 10))
	}
	newValues := map[string]interface{}{}
	oldValues := map[string]interface{}{}

	if msg.NewTuple != nil {
		for idx, col := range msg.NewTuple.Columns {
			colName := rel.Columns[idx].Name
			newValues[colName] = decodeColumn(col, typeMap, rel.Columns[idx].DataType)
		}
	}

	if msg.OldTuple != nil {
		for idx, col := range msg.OldTuple.Columns {
			colName := rel.Columns[idx].Name
			oldValues[colName] = decodeColumn(col, typeMap, rel.Columns[idx].DataType)
		}
	}

	return PostgresCDC{
		Operation: "UPDATE",
		Schema:    rel.Namespace,
		Table:     rel.RelationName,
		Data:      newValues,
		OldData:   oldValues,
		Timestamp: time.Now().Unix(),
		XID:       msg.Xid,
	}
}

// handleDeleteMessageV2 processes a delete message and returns a PostgresCDC event.
//
// It takes the following parameters:
//   - msg: the DeleteMessageV2 containing the deleted tuple
//   - relations: a map of relation IDs to their corresponding RelationMessageV2
//   - typeMap: a pgtype.Map for decoding column values
//
// The function constructs a PostgresCDC event with the "DELETE" operation, including
// the schema, table name, old data, timestamp, and transaction ID.
func handleDeleteMessageV2(msg *pglogrepl.DeleteMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map) PostgresCDC {
	rel, ok := relations[msg.RelationID]
	if !ok {
		zap.L().Error("unknown relation ID", zap.Uint32("relationID", msg.RelationID))
		return PostgresCDC{}
	}
	oldValues := map[string]interface{}{}
	for idx, col := range msg.OldTuple.Columns {
		colName := rel.Columns[idx].Name
		oldValues[colName] = decodeColumn(col, typeMap, rel.Columns[idx].DataType)
	}
	return PostgresCDC{
		Operation: "DELETE",
		Schema:    rel.Namespace,
		Table:     rel.RelationName,
		OldData:   oldValues,
		Timestamp: time.Now().Unix(),
		XID:       msg.Xid,
	}
}

// handleTruncateMessageV2 processes a truncate message and returns a PostgresCDC event.
//
// It takes the following parameters:
//   - msg: the TruncateMessageV2 containing truncate information
//   - relations: a map of relation IDs to their corresponding RelationMessageV2
//
// The function logs the truncate message and returns an empty PostgresCDC event.
// Note: This function currently does not process the truncate message fully.
func handleTruncateMessageV2(msg *pglogrepl.TruncateMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2) PostgresCDC {
	_, _ = msg, relations
	logger.Info("Truncate message received")
	return PostgresCDC{}
}

package logrepl

import (
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

func processV2(walData []byte, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map, inStream *bool) []Event {
	logicalMsg, err := pglogrepl.ParseV2(walData, *inStream)
	if err != nil {
		zap.L().Fatal("ParseV2 failed", zap.Error(err))
	}
	var events []Event
	switch logicalMsg := logicalMsg.(type) {
	case *pglogrepl.RelationMessageV2:
		relations[logicalMsg.RelationID] = logicalMsg

	case *pglogrepl.BeginMessage:
		// Handle begin message

	case *pglogrepl.CommitMessage:
		// Handle commit message

	case *pglogrepl.InsertMessageV2:
		events = append(events, handleInsertMessageV2(logicalMsg, relations, typeMap))

	case *pglogrepl.UpdateMessageV2:
		events = append(events, handleUpdateMessageV2(logicalMsg, relations, typeMap))

	case *pglogrepl.DeleteMessageV2:
		events = append(events, handleDeleteMessageV2(logicalMsg, relations, typeMap))

	case *pglogrepl.TruncateMessageV2:
		events = append(events, handleTruncateMessageV2(logicalMsg, relations))

	case *pglogrepl.TypeMessageV2:
	case *pglogrepl.OriginMessage:
	case *pglogrepl.LogicalDecodingMessageV2:
		// Handle logical decoding message
	case *pglogrepl.StreamStartMessageV2:
		*inStream = true
	case *pglogrepl.StreamStopMessageV2:
		*inStream = false
	case *pglogrepl.StreamCommitMessageV2:
	case *pglogrepl.StreamAbortMessageV2:
	default:
		zap.L().Warn("Unknown message type in pgoutput stream", zap.Any("message", logicalMsg))
	}

	return events
}

func handleInsertMessageV2(msg *pglogrepl.InsertMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map) Event {
	rel, ok := relations[msg.RelationID]
	if !ok {
		panic("unknown relation ID " + string(msg.RelationID))
	}
	values := map[string]interface{}{}
	for idx, col := range msg.Tuple.Columns {
		colName := rel.Columns[idx].Name
		switch col.DataType {
		case 'n':
			values[colName] = nil
		case 'u':
		case 't':
			val, err := decodeTextColumnData(typeMap, col.Data, rel.Columns[idx].DataType)
			if err != nil {
				panic("error decoding column data: " + err.Error())
			}
			values[colName] = val
		}
	}
	return &PostgresEvent{
		Operation: "INSERT",
		Schema:    rel.Namespace,
		Table:     rel.RelationName,
		Data:      values,
		Timestamp: time.Now().Unix(),
		XID:       msg.Xid,
	}
}

func handleUpdateMessageV2(msg *pglogrepl.UpdateMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map) Event {
	rel, ok := relations[msg.RelationID]
	if !ok {
		panic("unknown relation ID " + string(msg.RelationID))
	}
	newValues := map[string]interface{}{}
	for idx, col := range msg.NewTuple.Columns {
		colName := rel.Columns[idx].Name
		switch col.DataType {
		case 'n':
			newValues[colName] = nil
		case 'u':
		case 't':
			val, err := decodeTextColumnData(typeMap, col.Data, rel.Columns[idx].DataType)
			if err != nil {
				panic("error decoding column data: " + err.Error())
			}
			newValues[colName] = val
		}
	}
	oldValues := map[string]interface{}{}
	for idx, col := range msg.OldTuple.Columns {
		colName := rel.Columns[idx].Name
		switch col.DataType {
		case 'n':
			oldValues[colName] = nil
		case 'u':
		case 't':
			val, err := decodeTextColumnData(typeMap, col.Data, rel.Columns[idx].DataType)
			if err != nil {
				panic("error decoding column data: " + err.Error())
			}
			oldValues[colName] = val
		}
	}
	return &PostgresEvent{
		Operation: "UPDATE",
		Schema:    rel.Namespace,
		Table:     rel.RelationName,
		Data:      newValues,
		OldData:   oldValues,
		Timestamp: time.Now().Unix(),
		XID:       msg.Xid,
	}
}

func handleDeleteMessageV2(msg *pglogrepl.DeleteMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2, typeMap *pgtype.Map) Event {
	rel, ok := relations[msg.RelationID]
	if !ok {
		panic("unknown relation ID " + string(msg.RelationID))
	}
	oldValues := map[string]interface{}{}
	for idx, col := range msg.OldTuple.Columns {
		colName := rel.Columns[idx].Name
		switch col.DataType {
		case 'n':
			oldValues[colName] = nil
		case 'u':
		case 't':
			val, err := decodeTextColumnData(typeMap, col.Data, rel.Columns[idx].DataType)
			if err != nil {
				panic("error decoding column data: " + err.Error())
			}
			oldValues[colName] = val
		}
	}
	return &PostgresEvent{
		Operation: "DELETE",
		Schema:    rel.Namespace,
		Table:     rel.RelationName,
		OldData:   oldValues,
		Timestamp: time.Now().Unix(),
		XID:       msg.Xid,
	}
}

func handleTruncateMessageV2(msg *pglogrepl.TruncateMessageV2, relations map[uint32]*pglogrepl.RelationMessageV2) Event {
	// Implement truncate message handling
	return &PostgresEvent{}
}

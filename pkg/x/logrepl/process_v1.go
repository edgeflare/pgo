package logrepl

import (
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgtype"
)

func processV1(walData []byte, relations map[uint32]*pglogrepl.RelationMessage, typeMap *pgtype.Map) []Event {
	logicalMsg, err := pglogrepl.Parse(walData)
	if err != nil {
		panic("Parse logical replication message: " + err.Error())
	}
	var events []Event
	switch logicalMsg := logicalMsg.(type) {
	case *pglogrepl.RelationMessage:
		relations[logicalMsg.RelationID] = logicalMsg

	case *pglogrepl.BeginMessage:
		// Handle begin message

	case *pglogrepl.CommitMessage:
		// Handle commit message

	case *pglogrepl.InsertMessage:
		events = append(events, handleInsertMessageV1(logicalMsg, relations, typeMap))

	case *pglogrepl.UpdateMessage:
		events = append(events, handleUpdateMessageV1(logicalMsg, relations, typeMap))

	case *pglogrepl.DeleteMessage:
		events = append(events, handleDeleteMessageV1(logicalMsg, relations, typeMap))

	case *pglogrepl.TruncateMessage:
		events = append(events, handleTruncateMessageV1(logicalMsg, relations))

	case *pglogrepl.TypeMessage:
	case *pglogrepl.OriginMessage:

	case *pglogrepl.LogicalDecodingMessage:
		// Handle logical decoding message
	}

	return events
}

func handleInsertMessageV1(msg *pglogrepl.InsertMessage, relations map[uint32]*pglogrepl.RelationMessage, typeMap *pgtype.Map) Event {
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
		// XID:       uint32(msg.Type()),
	}
}

func handleUpdateMessageV1(msg *pglogrepl.UpdateMessage, relations map[uint32]*pglogrepl.RelationMessage, typeMap *pgtype.Map) Event {
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
		// XID:       msg.Xid,
	}
}

func handleDeleteMessageV1(msg *pglogrepl.DeleteMessage, relations map[uint32]*pglogrepl.RelationMessage, typeMap *pgtype.Map) Event {
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
		// XID:       msg.Xid,
	}
}

func handleTruncateMessageV1(msg *pglogrepl.TruncateMessage, relations map[uint32]*pglogrepl.RelationMessage) Event {
	// Implement truncate message handling
	return &PostgresEvent{}
}

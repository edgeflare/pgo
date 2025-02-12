package pglogrepl

import (
	"log"

	"github.com/edgeflare/pgo/pkg/pipeline/cdc"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgtype"
)

// processV1 processes a logical replication message from PostgreSQL WAL data.
//
// It takes the following parameters:
//   - walData: a byte slice containing the WAL (Write-Ahead Log) data
//   - relations: a map of relation IDs to RelationMessage objects
//   - typeMap: a pgtype.Map object for type conversions
//
// The function parses the WAL data into a logical replication message and
// processes it based on its type. It handles various message types including
// RelationMessage, BeginMessage, CommitMessage, InsertMessage, and others.
//
// For InsertMessage, it decodes the column data and logs the inserted values.
// Other message types are partially implemented or left as placeholders for
// future implementation.
//
// The function returns a slice of PostgresCDC objects and an error. Currently,
// it always returns an empty slice and nil error, serving as a placeholder for
// future CDC (Change Data Capture) functionality.
//
// This function is part of the logical replication process and is intended to
// be used in a larger system for capturing and processing database changes.
func processV1(walData []byte, relations map[uint32]*pglogrepl.RelationMessage, typeMap *pgtype.Map) ([]cdc.CDC, error) {
	logicalMsg, err := pglogrepl.Parse(walData)
	if err != nil {
		log.Fatalf("Parse logical replication message: %s", err)
	}
	log.Printf("Receive a logical replication message: %s", logicalMsg.Type())
	switch logicalMsg := logicalMsg.(type) {
	case *pglogrepl.RelationMessage:
		relations[logicalMsg.RelationID] = logicalMsg

	case *pglogrepl.BeginMessage:
		// Indicates the beginning of a group of changes in a transaction. This is only sent for committed transactions. You won't get any events from rolled back transactions.

	case *pglogrepl.CommitMessage:

	case *pglogrepl.InsertMessage:
		rel, ok := relations[logicalMsg.RelationID]
		if !ok {
			log.Fatalf("unknown relation ID %d", logicalMsg.RelationID)
		}
		values := map[string]interface{}{}
		for idx, col := range logicalMsg.Tuple.Columns {
			colName := rel.Columns[idx].Name
			switch col.DataType {
			case 'n': // null
				values[colName] = nil
			case 'u': // unchanged toast
				// This TOAST value was not changed. TOAST values are not stored in the tuple, and logical replication doesn't want to spend a disk read to fetch its value for you.
			case 't': // text
				val, err := decodeTextColumnData(typeMap, col.Data, rel.Columns[idx].DataType)
				if err != nil {
					log.Fatalln("error decoding column data:", err)
				}
				values[colName] = val
			}
		}
		log.Printf("INSERT INTO %s.%s: %v", rel.Namespace, rel.RelationName, values)

	case *pglogrepl.UpdateMessage:
		// ...
	case *pglogrepl.DeleteMessage:
		// ...
	case *pglogrepl.TruncateMessage:
		// ...

	case *pglogrepl.TypeMessage:
	case *pglogrepl.OriginMessage:

	case *pglogrepl.LogicalDecodingMessage:
		log.Printf("Logical decoding message: %q, %q", logicalMsg.Prefix, logicalMsg.Content)

	case *pglogrepl.StreamStartMessageV2:
		log.Printf("Stream start message: xid %d, first segment? %d", logicalMsg.Xid, logicalMsg.FirstSegment)
	case *pglogrepl.StreamStopMessageV2:
		log.Printf("Stream stop message")
	case *pglogrepl.StreamCommitMessageV2:
		log.Printf("Stream commit message: xid %d", logicalMsg.Xid)
	case *pglogrepl.StreamAbortMessageV2:
		log.Printf("Stream abort message: xid %d", logicalMsg.Xid)
	default:
		log.Printf("Unknown message type in pgoutput stream: %T", logicalMsg)
	}
	return []cdc.CDC{}, nil
}

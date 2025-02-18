package pglogrepl

import (
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// decodeColumn decodes a single column from a PostgreSQL logical replication message.
func decodeColumn(col *pglogrepl.TupleDataColumn, typeMap *pgtype.Map, dataType uint32) interface{} {
	switch col.DataType {
	case 'n':
		return nil
	case 'u':
		return nil // or some placeholder for unchanged toast
	case 't':
		val, err := decodeTextColumnData(typeMap, col.Data, dataType)
		if err != nil {
			zap.L().Error("error decoding column data", zap.Error(err))
			return nil
		}
		return val
	default:
		zap.L().Warn("unknown column data type", zap.Any("dataType", col.DataType))
		return nil
	}
}

// decodeTextColumnData decodes the binary data of a column into its corresponding Go type.
// It uses the provided type map to determine the appropriate codec for decoding.
func decodeTextColumnData(mi *pgtype.Map, data []byte, dataType uint32) (interface{}, error) {
	if dt, ok := mi.TypeForOID(dataType); ok {
		return dt.Codec.DecodeValue(mi, dataType, pgtype.TextFormatCode, data)
	}
	return string(data), nil
}

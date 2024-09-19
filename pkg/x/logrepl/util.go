package logrepl

import (
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// decodeColumn decodes a single column from a PostgreSQL logical replication message.
// It handles different data types and returns the decoded value as an interface{}.
//
// The function takes three parameters:
//   - col: A pointer to the TupleDataColumn containing the column data.
//   - typeMap: A pointer to the pgtype.Map used for type mapping.
//   - dataType: The OID of the PostgreSQL data type.
//
// It returns nil for null values, unchanged toast values, or in case of decoding errors.
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
//
// The function takes three parameters:
//   - mi: A pointer to the pgtype.Map used for type mapping.
//   - data: A byte slice containing the binary column data.
//   - dataType: The OID of the PostgreSQL data type.
//
// It returns the decoded value as an interface{} and an error if decoding fails.
// If the data type is not found in the type map, it returns the data as a string.
func decodeTextColumnData(mi *pgtype.Map, data []byte, dataType uint32) (interface{}, error) {
	if dt, ok := mi.TypeForOID(dataType); ok {
		return dt.Codec.DecodeValue(mi, dataType, pgtype.TextFormatCode, data)
	}
	return string(data), nil
}

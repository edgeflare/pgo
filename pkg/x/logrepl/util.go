package logrepl

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// decodeTextColumnData decodes the text format of a column's data into a suitable Go type based on its data type.
func decodeTextColumnData(typeMap *pgtype.Map, data []byte, dataType uint32) (interface{}, error) {
	if dt, ok := typeMap.TypeForOID(dataType); ok {
		value, err := dt.Codec.DecodeValue(typeMap, dataType, pgtype.TextFormatCode, data)
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	return string(data), nil
}

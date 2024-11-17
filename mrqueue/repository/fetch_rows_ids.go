package repository

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
)

func fetchRowsIDs(ctx context.Context, client mrstorage.DBConnManager, sql string, limit uint32, args ...any) (rowsIDs []uint64, err error) {
	cursor, err := client.Conn(ctx).Query(
		ctx,
		sql,
		args...,
	)
	if err != nil {
		return nil, err
	}

	defer cursor.Close()

	rowsIDs = make([]uint64, 0, limit)

	for cursor.Next() {
		var rowID uint64

		err = cursor.Scan(
			&rowID,
		)
		if err != nil {
			return nil, err
		}

		rowsIDs = append(rowsIDs, rowID)
	}

	return rowsIDs, cursor.Err()
}

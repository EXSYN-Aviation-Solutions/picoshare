package file_test

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/mtlynch/picoshare/v2/store/sqlite/file"
	"github.com/mtlynch/picoshare/v2/types"
)

type (
	mockChunkRow struct {
		id         types.EntryID
		chunkIndex int
		chunk      []byte
	}

	mockSqlTx struct {
		rows []mockChunkRow
		err  error
	}
)

var errMockSqlFailure = errors.New("fake SQL error")

func (db *mockSqlTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	chunk := args[2].([]byte)
	chunkCopy := make([]byte, len(chunk))
	copy(chunkCopy, chunk)
	db.rows = append(db.rows, mockChunkRow{
		id:         args[0].(types.EntryID),
		chunkIndex: args[1].(int),
		chunk:      chunkCopy,
	})
	return nil, db.err
}

func TestWriteFile(t *testing.T) {
	for _, tt := range []struct {
		explanation  string
		id           types.EntryID
		data         []byte
		chunkSize    int
		sqlExecErr   error
		errExpected  error
		rowsExpected []mockChunkRow
	}{
		{
			explanation: "data is smaller than chunk size",
			id:          types.EntryID("dummy-id"),
			data:        []byte("hello, world!"),
			chunkSize:   25,
			rowsExpected: []mockChunkRow{
				{
					id:         types.EntryID("dummy-id"),
					chunkIndex: 0,
					chunk:      []byte("hello, world!"),
				},
			},
		},
		{
			explanation: "data fits exactly in single chunk",
			id:          types.EntryID("dummy-id"),
			data:        []byte("01234"),
			chunkSize:   5,
			rowsExpected: []mockChunkRow{
				{
					id:         types.EntryID("dummy-id"),
					chunkIndex: 0,
					chunk:      []byte("01234"),
				},
			},
		},
		{
			explanation: "data occupies a partial chunk after the first",
			id:          types.EntryID("dummy-id"),
			data:        []byte("0123456"),
			chunkSize:   5,
			rowsExpected: []mockChunkRow{
				{
					id:         types.EntryID("dummy-id"),
					chunkIndex: 0,
					chunk:      []byte("01234"),
				},
				{
					id:         types.EntryID("dummy-id"),
					chunkIndex: 1,
					chunk:      []byte("56"),
				},
			},
		},
		{
			explanation: "data spans exactly two chunks",
			id:          types.EntryID("dummy-id"),
			data:        []byte("0123456789"),
			chunkSize:   5,
			rowsExpected: []mockChunkRow{
				{
					id:         types.EntryID("dummy-id"),
					chunkIndex: 0,
					chunk:      []byte("01234"),
				},
				{
					id:         types.EntryID("dummy-id"),
					chunkIndex: 1,
					chunk:      []byte("56789"),
				},
			},
		},
		{
			explanation: "write fails when SQL transaction returns error",
			id:          types.EntryID("dummy-id"),
			data:        []byte("0123456789"),
			chunkSize:   5,
			sqlExecErr:  errMockSqlFailure,
			errExpected: errMockSqlFailure,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			tx := mockSqlTx{
				err: tt.sqlExecErr,
			}

			w := file.NewWriter(&tx, tt.id, tt.chunkSize)
			n, err := w.Write(tt.data)

			if got, want := err, tt.errExpected; got != want {
				t.Fatalf("err=%v, want=%v", err, tt.errExpected)
			}
			if err != nil {
				return
			}
			if got, want := n, len(tt.data); got != want {
				t.Errorf("n=%d, want=%d", got, want)
			}

			if err := w.Close(); err != nil {
				t.Errorf("failed to close writer: %v", err)
			}

			if got, want := tx.rows, tt.rowsExpected; !reflect.DeepEqual(got, want) {
				t.Errorf("rows=%v, want %v", tx.rows, tt.rowsExpected)
			}
		})
	}
}

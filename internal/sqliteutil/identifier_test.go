package sqliteutil

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "ascii", input: "roads", want: `"roads"`},
		{name: "chinese", input: "县级区划", want: `"县级区划"`},
		{name: "embedded quote", input: `县级"区划`, want: `"县级""区划"`},
		{name: "empty", input: "", want: `""`},
		{name: "nul", input: "roads\x00archive", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := QuoteIdentifier(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestQuoteIdentifierExecutesSafelyInSQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "identifiers.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	tableName := `县级"区划; DROP TABLE protected_table;--`
	columnName := `名称", injected INTEGER); DROP TABLE protected_table;--`
	quotedTable, err := QuoteIdentifier(tableName)
	require.NoError(t, err)
	quotedColumn, err := QuoteIdentifier(columnName)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE protected_table (value TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf("CREATE TABLE %s (%s TEXT)", quotedTable, quotedColumn))
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", quotedTable, quotedColumn), "河南")
	require.NoError(t, err)

	var got string
	err = db.QueryRow(fmt.Sprintf("SELECT %s FROM %s", quotedColumn, quotedTable)).Scan(&got)
	require.NoError(t, err)
	assert.Equal(t, "河南", got)

	var protectedTable string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'protected_table'`).Scan(&protectedTable)
	require.NoError(t, err)
	assert.Equal(t, "protected_table", protectedTable)
}

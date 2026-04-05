package dataset

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/schema"
)

// setupTestDB creates a test database with schema initialized.
func setupTestDB(t *testing.T) *sql.DB {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)

	initializer := schema.NewInitializer(db)
	err = initializer.Initialize()
	require.NoError(t, err)

	return db
}

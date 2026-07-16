// Package sqliteutil provides shared SQLite helpers for internal data access.
package sqliteutil

import (
	"fmt"
	"strings"
)

// QuoteIdentifier safely quotes a SQLite identifier.
func QuoteIdentifier(name string) (string, error) {
	if strings.ContainsRune(name, '\x00') {
		return "", fmt.Errorf("SQLite identifier must not contain NUL")
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`, nil
}

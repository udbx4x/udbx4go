package sqliteutil

import (
	"testing"

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

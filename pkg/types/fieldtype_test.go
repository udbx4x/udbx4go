package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFieldType_String(t *testing.T) {
	tests := []struct {
		fieldType FieldType
		expected  string
	}{
		{FieldTypeBoolean, "boolean"},
		{FieldTypeByte, "byte"},
		{FieldTypeInt16, "int16"},
		{FieldTypeInt32, "int32"},
		{FieldTypeInt64, "int64"},
		{FieldTypeSingle, "single"},
		{FieldTypeDouble, "double"},
		{FieldTypeDate, "date"},
		{FieldTypeBinary, "binary"},
		{FieldTypeGeometry, "geometry"},
		{FieldTypeChar, "char"},
		{FieldTypeNText, "ntext"},
		{FieldTypeText, "text"},
		{FieldTypeTime, "time"},
		{FieldType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.fieldType.String())
		})
	}
}

func TestFromFieldTypeString(t *testing.T) {
	tests := []struct {
		input    string
		expected FieldType
		ok       bool
	}{
		{"boolean", FieldTypeBoolean, true},
		{"byte", FieldTypeByte, true},
		{"int16", FieldTypeInt16, true},
		{"int32", FieldTypeInt32, true},
		{"int64", FieldTypeInt64, true},
		{"single", FieldTypeSingle, true},
		{"double", FieldTypeDouble, true},
		{"date", FieldTypeDate, true},
		{"binary", FieldTypeBinary, true},
		{"geometry", FieldTypeGeometry, true},
		{"char", FieldTypeChar, true},
		{"ntext", FieldTypeNText, true},
		{"text", FieldTypeText, true},
		{"time", FieldTypeTime, true},
		{"unknown", FieldTypeText, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			fieldType, ok := FromFieldTypeString(tt.input)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.expected, fieldType)
			}
		})
	}
}

func TestFieldType_SQLiteType(t *testing.T) {
	tests := []struct {
		fieldType FieldType
		expected  string
	}{
		{FieldTypeBoolean, "INTEGER"},
		{FieldTypeByte, "INTEGER"},
		{FieldTypeInt16, "INTEGER"},
		{FieldTypeInt32, "INTEGER"},
		{FieldTypeInt64, "INTEGER"},
		{FieldTypeSingle, "REAL"},
		{FieldTypeDouble, "REAL"},
		{FieldTypeDate, "TEXT"},
		{FieldTypeBinary, "BLOB"},
		{FieldTypeGeometry, "BLOB"},
		{FieldTypeChar, "TEXT"},
		{FieldTypeNText, "TEXT"},
		{FieldTypeText, "TEXT"},
		{FieldTypeTime, "TEXT"},
	}

	for _, tt := range tests {
		t.Run(tt.fieldType.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.fieldType.SQLiteType())
		})
	}
}

func TestFieldType_GoType(t *testing.T) {
	tests := []struct {
		fieldType FieldType
		expected  string
	}{
		{FieldTypeBoolean, "bool"},
		{FieldTypeByte, "int8"},
		{FieldTypeInt16, "int16"},
		{FieldTypeInt32, "int32"},
		{FieldTypeInt64, "int64"},
		{FieldTypeSingle, "float32"},
		{FieldTypeDouble, "float64"},
		{FieldTypeDate, "string"},
		{FieldTypeBinary, "[]byte"},
		{FieldTypeGeometry, "[]byte"},
		{FieldTypeChar, "string"},
		{FieldTypeNText, "string"},
		{FieldTypeText, "string"},
		{FieldTypeTime, "string"},
	}

	for _, tt := range tests {
		t.Run(tt.fieldType.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.fieldType.GoType())
		})
	}
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInventoryRequiresOutput(t *testing.T) {
	code := run([]string{"inventory", "../../../data/SampleData.udbx"})

	assert.Equal(t, 2, code)
}

func TestInventoryMissingInputDoesNotCreateFile(t *testing.T) {
	input := filepath.Join(t.TempDir(), "missing.udbx")
	output := filepath.Join(t.TempDir(), "inventory.json")

	code := run([]string{"inventory", "--output", output, input})

	assert.Equal(t, 1, code)
	_, inputErr := os.Stat(input)
	assert.True(t, os.IsNotExist(inputErr), "missing input should not be created")
	_, outputErr := os.Stat(output)
	assert.True(t, os.IsNotExist(outputErr), "failed export should not create output")
}

func TestInventoryWritesJSON(t *testing.T) {
	output := filepath.Join(t.TempDir(), "nested", "inventory.json")

	code := run([]string{"inventory", "--output", output, "../../../data/SampleData.udbx"})

	require.Equal(t, 0, code)
	content, err := os.ReadFile(output)
	require.NoError(t, err)

	var inventory inventoryFile
	require.NoError(t, json.Unmarshal(content, &inventory))
	assert.Equal(t, "../../../data/SampleData.udbx", inventory.File)
	require.NotEmpty(t, inventory.Datasets)
	assert.NotEmpty(t, inventory.Datasets[0].Name)
	assert.NotEmpty(t, inventory.Datasets[0].Kind)
	assert.NotEmpty(t, inventory.Datasets[0].TableName)
	assert.GreaterOrEqual(t, inventory.Datasets[0].ObjectCount, 0)
}

func TestInventoryRefusesExistingOutputWithoutOverwrite(t *testing.T) {
	output := filepath.Join(t.TempDir(), "inventory.json")
	require.NoError(t, os.WriteFile(output, []byte(`{"existing":true}`), 0o644))

	code := run([]string{"inventory", "--output", output, "../../../data/SampleData.udbx"})

	assert.Equal(t, 2, code)
	content, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.JSONEq(t, `{"existing":true}`, string(content))
}

func TestWriteJSONAtomicallyRefusesRaceCreatedOutputWithoutOverwrite(t *testing.T) {
	output := filepath.Join(t.TempDir(), "inventory.json")
	inventory := &inventoryFile{File: "sample.udbx"}

	err := writeJSONAtomically(output, inventory, false, func() error {
		return os.WriteFile(output, []byte(`{"existing":true}`), 0o644)
	})

	require.Error(t, err)
	content, readErr := os.ReadFile(output)
	require.NoError(t, readErr)
	assert.JSONEq(t, `{"existing":true}`, string(content))
}

func TestInventoryOverwriteExistingOutput(t *testing.T) {
	output := filepath.Join(t.TempDir(), "inventory.json")
	require.NoError(t, os.WriteFile(output, []byte(`{"existing":true}`), 0o644))

	code := run([]string{"inventory", "--output", output, "--overwrite", "../../../data/SampleData.udbx"})

	require.Equal(t, 0, code)
	content, err := os.ReadFile(output)
	require.NoError(t, err)
	var inventory inventoryFile
	require.NoError(t, json.Unmarshal(content, &inventory))
	assert.Equal(t, "../../../data/SampleData.udbx", inventory.File)
	require.NotEmpty(t, inventory.Datasets)
}

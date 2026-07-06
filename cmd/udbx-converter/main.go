package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/udbx4x/udbx4go"
)

var errOutputExists = errors.New("output already exists")

type inventoryFile struct {
	File     string             `json:"file"`
	Datasets []inventoryDataset `json:"datasets"`
}

type inventoryDataset struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	TableName   string `json:"tableName"`
	ObjectCount int    `json:"objectCount"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 || args[0] != "inventory" {
		fmt.Fprintln(os.Stderr, "usage: udbx-converter inventory --output <inventory.json> [--overwrite] <file.udbx>")
		return 2
	}

	output, overwrite, input, ok := parseInventoryArgs(args[1:])
	if !ok || output == "" || input == "" {
		fmt.Fprintln(os.Stderr, "usage: udbx-converter inventory --output <inventory.json> [--overwrite] <file.udbx>")
		return 2
	}

	inventory, err := buildInventory(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build inventory: %v\n", err)
		return 1
	}
	if err := writeJSONAtomically(output, inventory, overwrite); err != nil {
		if errors.Is(err, errOutputExists) {
			fmt.Fprintf(os.Stderr, "failed to write inventory: %v\n", err)
			return 2
		}
		fmt.Fprintf(os.Stderr, "failed to write inventory: %v\n", err)
		return 1
	}
	return 0
}

func parseInventoryArgs(args []string) (string, bool, string, bool) {
	fs := flag.NewFlagSet("inventory", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	output := fs.String("output", "", "inventory JSON output path")
	overwrite := fs.Bool("overwrite", false, "overwrite existing output")
	if err := fs.Parse(args); err != nil {
		return "", false, "", false
	}
	if fs.NArg() != 1 {
		return "", false, "", false
	}
	return *output, *overwrite, fs.Arg(0), true
}

func buildInventory(path string) (*inventoryFile, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	ds, err := udbx4go.Open(path)
	if err != nil {
		return nil, err
	}
	defer ds.Close()

	datasets, err := ds.ListDatasets()
	if err != nil {
		return nil, err
	}

	inventory := &inventoryFile{
		File:     path,
		Datasets: make([]inventoryDataset, 0, len(datasets)),
	}
	for _, dataset := range datasets {
		inventory.Datasets = append(inventory.Datasets, inventoryDataset{
			Name:        dataset.Name,
			Kind:        dataset.Kind.String(),
			TableName:   dataset.TableName,
			ObjectCount: dataset.ObjectCount,
		})
	}
	return inventory, nil
}

func writeJSONAtomically(path string, inventory *inventoryFile, overwrite bool, beforePublish ...func() error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(inventory); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	for _, hook := range beforePublish {
		if err := hook(); err != nil {
			return err
		}
	}
	if overwrite {
		if err := os.Rename(tmpPath, path); err != nil {
			return fmt.Errorf("rename temporary inventory file: %w", err)
		}
		return nil
	}
	if err := os.Link(tmpPath, path); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%w: %s", errOutputExists, path)
		}
		return fmt.Errorf("publish temporary inventory file: %w", err)
	}
	return nil
}

package main

import (
	"fmt"
	udbx4go "github.com/udbx4x/udbx4go"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func main() {
	ds, err := udbx4go.Open("/Users/zhangyuting/github/udbx4x/data/SampleData.udbx")
	if err != nil {
		fmt.Printf("Failed to open SampleData.udbx: %v\n", err)
		return
	}
	defer ds.Close()

	datasets, err := ds.ListDatasets()
	if err != nil {
		fmt.Printf("Failed to list datasets: %v\n", err)
		return
	}

	for _, info := range datasets {
		fmt.Printf("\nDataset: %s, Kind: %s, Count: %d\n", info.Name, info.Kind.String(), info.ObjectCount)

		dataset, err := ds.GetDataset(info.Name)
		if err != nil {
			fmt.Printf("  Error getting dataset: %v\n", err)
			continue
		}

		fields, err := dataset.GetFields()
		if err != nil {
			fmt.Printf("  Error getting fields: %v\n", err)
			continue
		}
		fmt.Printf("  Fields: %d\n", len(fields))

		// Try to list first page
		opts := &types.QueryOptions{Limit: 1, Offset: 0}

		switch d := dataset.(type) {
		case interface{ List(opts *types.QueryOptions) ([]*types.Feature, error) }:
			features, err := d.List(opts)
			fmt.Printf("  List features: count=%d, err=%v\n", len(features), err)
			if err != nil {
				fmt.Printf("    Error details: %v\n", err)
			}
		case interface{ List(opts *types.QueryOptions) ([]*types.TabularRecord, error) }:
			records, err := d.List(opts)
			fmt.Printf("  List records: count=%d, err=%v\n", len(records), err)
			if err != nil {
				fmt.Printf("    Error details: %v\n", err)
			}
		default:
			fmt.Printf("  Unknown dataset type: %T\n", dataset)
		}
	}
}

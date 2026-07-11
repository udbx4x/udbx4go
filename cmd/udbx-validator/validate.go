package main

import (
	"os"
	"path/filepath"

	"github.com/udbx4x/udbx4go"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func ValidateFile(path string) (Report, int) {
	report := Report{File: FileInfo{Path: path}}
	stat, err := os.Stat(path)
	if err != nil {
		report.Checks = append(report.Checks, Check{ID: "open-file", Level: "error", Status: "fail", Message: err.Error()})
		report.Summary.Fail = 1
		return report, 2
	}
	report.File.Size = stat.Size()
	if realPath, err := filepath.Abs(path); err == nil {
		report.File.RealPath = realPath
	}

	ds, err := udbx4go.Open(path)
	if err != nil {
		report.Checks = append(report.Checks, Check{ID: "open-file", Level: "error", Status: "fail", Message: err.Error()})
		report.Summary.Fail = 1
		return report, 2
	}
	defer ds.Close()

	addCheck := func(check Check) {
		report.Checks = append(report.Checks, check)
		switch check.Status {
		case "pass":
			report.Summary.Pass++
		case "warn":
			report.Summary.Warn++
		case "fail":
			report.Summary.Fail++
		}
	}

	addCheck(Check{ID: "open-file", Level: "error", Status: "pass", Message: "file opened"})
	datasets, err := ds.ListDatasets()
	if err != nil {
		addCheck(Check{ID: "list-datasets", Level: "error", Status: "fail", Message: err.Error()})
		return report, 1
	}
	report.Summary.DatasetCount = len(datasets)
	addCheck(Check{ID: "list-datasets", Level: "error", Status: "pass", Message: "datasets listed"})

	for _, info := range datasets {
		validateDataset(ds, info, &report, addCheck)
	}
	if report.Summary.Fail > 0 {
		return report, 1
	}
	return report, 0
}

func validateDataset(ds *udbx4go.DataSource, info *types.DatasetInfo, report *Report, addCheck func(Check)) {
	datasetName := info.Name
	if info.Kind.String() == "unknown" {
		addCheck(Check{ID: "dataset-kind", Level: "warning", Status: "warn", Dataset: datasetName, Message: "unsupported dataset kind"})
		report.Unsupported = append(report.Unsupported, Unsupported{Dataset: datasetName, Kind: info.Kind.String(), Reason: "not in validator v1 baseline"})
		return
	}
	addCheck(Check{ID: "dataset-kind", Level: "error", Status: "pass", Dataset: datasetName, Message: "dataset kind is in v1 baseline"})

	d, err := ds.GetDataset(datasetName)
	if err != nil {
		addCheck(Check{ID: "open-dataset", Level: "warning", Status: "warn", Dataset: datasetName, Message: err.Error()})
		return
	}
	defer d.Close()

	fields, err := d.GetFields()
	if err != nil {
		addCheck(Check{ID: "field-list", Level: "error", Status: "fail", Dataset: datasetName, Message: err.Error()})
		return
	}
	for _, field := range fields {
		if field.FieldType.String() == "unknown" {
			addCheck(Check{ID: "field-type", Level: "error", Status: "fail", Dataset: datasetName, Message: "unknown field type"})
		}
	}
	addCheck(Check{ID: "field-list", Level: "error", Status: "pass", Dataset: datasetName, Message: "fields listed"})

	count, err := d.Count()
	if err != nil {
		addCheck(Check{ID: "object-count", Level: "error", Status: "fail", Dataset: datasetName, Message: err.Error()})
		return
	}
	report.Summary.ObjectCount += count
	addCheck(Check{ID: "object-count", Level: "error", Status: "pass", Dataset: datasetName, Message: "object count read"})
}

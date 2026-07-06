package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportJSONAndMarkdownUseSameChecks(t *testing.T) {
	report := Report{
		File:    FileInfo{Path: "sample.udbx", Size: 128},
		Summary: Summary{DatasetCount: 1, Pass: 1, Warn: 1, Fail: 0},
		Checks: []Check{
			{ID: "open-file", Level: "error", Status: "pass", Message: "file opened"},
			{ID: "unsupported-kind", Level: "warning", Status: "warn", Dataset: "Network", Message: "unsupported dataset kind"},
		},
		Unsupported: []Unsupported{{Dataset: "Network", Reason: "not in v1 baseline"}},
	}

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if !strings.Contains(string(payload), `"open-file"`) {
		t.Fatalf("json report did not include check id: %s", payload)
	}

	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "open-file") || !strings.Contains(markdown, "unsupported-kind") {
		t.Fatalf("markdown report did not include checks:\n%s", markdown)
	}
}

func TestValidateSampleDataProducesPassReport(t *testing.T) {
	report, exitCode := ValidateFile("../../../data/SampleData.udbx")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %#v", exitCode, report)
	}
	if report.Summary.DatasetCount == 0 {
		t.Fatalf("expected datasets in SampleData.udbx")
	}
	if report.Summary.Pass == 0 {
		t.Fatalf("expected at least one passing check")
	}
}

func TestValidateMissingFileReturnsExitCode2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.udbx")
	report, exitCode := ValidateFile(path)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d: %#v", exitCode, report)
	}
	if report.Summary.Fail == 0 {
		t.Fatalf("expected failed check for missing file")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("missing input should not be created, stat err: %v", err)
	}
}

func TestUnsupportedFormatReturnsBeforeOpeningInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.udbx")

	code := run([]string{"--format", "xml", path})

	if code != 2 {
		t.Fatalf("expected code 2, got %d", code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unsupported format should not create input, stat err: %v", err)
	}
}

func TestMainJSONOutput(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	previousStdout := stdout
	previousStderr := stderr
	stdout = &out
	stderr = &errOut
	defer func() {
		stdout = previousStdout
		stderr = previousStderr
	}()

	code := run([]string{"--format", "json", "../../../data/SampleData.udbx"})
	if code != 0 {
		t.Fatalf("expected code 0, got %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"checks"`) {
		t.Fatalf("expected json output to contain checks: %s", out.String())
	}
}

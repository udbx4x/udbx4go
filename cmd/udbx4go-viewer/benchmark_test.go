package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validBenchmarkConfig(t *testing.T) BenchmarkConfigDTO {
	t.Helper()
	dir := t.TempDir()
	return BenchmarkConfigDTO{
		RunID:                "sampledata-01",
		OutputPath:           filepath.Join(dir, "result.json"),
		Temperature:          "warm",
		MaxConcurrentQueries: 1,
		Scenario: BenchmarkScenarioDTO{
			Name:     "sampledata-multilayer",
			FilePath: sampleDataPath(t),
			Layers:   []string{"BaseMap_P", "BaseMap_L", "BaseMap_R", "County_T", "CADDT"},
			Selection: BenchmarkSelectionDTO{
				DatasetName: "BaseMap_R",
				Page:        1,
				RowIndex:    0,
			},
			ViewportSteps: []BenchmarkViewportStepDTO{{
				Bounds:           BoundingBoxDTO{MinX: 115, MinY: 38, MaxX: 118, MaxY: 42},
				ExpectedStrategy: "envelope_cache",
			}},
		},
	}
}

func writeBenchmarkConfigFixture(t *testing.T, config BenchmarkConfigDTO) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "benchmark-config.json")
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}

func TestLoadBenchmarkConfigAcceptsOneScenario(t *testing.T) {
	configPath := writeBenchmarkConfigFixture(t, validBenchmarkConfig(t))

	config, err := loadBenchmarkConfig(configPath)
	if err != nil {
		t.Fatalf("loadBenchmarkConfig() error = %v", err)
	}
	if config.RunID != "sampledata-01" || len(config.Scenario.Layers) != 5 {
		t.Fatalf("config = %+v", config)
	}
}

func TestLoadBenchmarkConfigRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*BenchmarkConfigDTO)
		want   string
	}{
		{
			name: "relative sample path",
			mutate: func(config *BenchmarkConfigDTO) {
				config.Scenario.FilePath = "data/SampleData.udbx"
			},
			want: "filePath",
		},
		{
			name: "empty layers",
			mutate: func(config *BenchmarkConfigDTO) {
				config.Scenario.Layers = nil
			},
			want: "layers",
		},
		{
			name: "invalid page",
			mutate: func(config *BenchmarkConfigDTO) {
				config.Scenario.Selection.Page = 0
			},
			want: "page",
		},
		{
			name: "same input and output path",
			mutate: func(config *BenchmarkConfigDTO) {
				config.OutputPath = config.Scenario.FilePath
			},
			want: "outputPath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validBenchmarkConfig(t)
			tt.mutate(&config)
			path := writeBenchmarkConfigFixture(t, config)
			_, err := loadBenchmarkConfig(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("loadBenchmarkConfig() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestGetBenchmarkConfigReturnsNilOutsideBenchmarkMode(t *testing.T) {
	config, err := NewApp().GetBenchmarkConfig()
	if err != nil {
		t.Fatalf("GetBenchmarkConfig() error = %v", err)
	}
	if config != nil {
		t.Fatalf("GetBenchmarkConfig() = %+v, want nil", config)
	}
}

func TestSaveBenchmarkResultWritesFinalJSONAtomically(t *testing.T) {
	config := validBenchmarkConfig(t)
	app := NewApp()
	app.benchmarkConfig = &config

	result := BenchmarkResultDTO{
		RunID:     config.RunID,
		Status:    "passed",
		StartedAt: "2026-07-14T16:00:00+08:00",
		Scenario:  config.Scenario.Name,
		Metrics: BenchmarkMetricsDTO{
			OpenFileMS:         12.5,
			LoadLayersMS:       30,
			FitVisibleLayersMS: 2,
			SelectAndFitMS:     18,
		},
	}

	if err := app.SaveBenchmarkResult(result); err != nil {
		t.Fatalf("SaveBenchmarkResult() error = %v", err)
	}
	data, err := os.ReadFile(config.OutputPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	var saved BenchmarkResultDTO
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if saved.RunID != config.RunID || saved.Status != "passed" {
		t.Fatalf("saved = %+v", saved)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(config.OutputPath), ".benchmark-result-*"))
	if err != nil {
		t.Fatalf("filepath.Glob() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary result files = %v", matches)
	}
}

func TestSaveBenchmarkResultRejectsMismatchedRun(t *testing.T) {
	config := validBenchmarkConfig(t)
	app := NewApp()
	app.benchmarkConfig = &config

	err := app.SaveBenchmarkResult(BenchmarkResultDTO{
		RunID:    "other-run",
		Status:   "failed",
		Scenario: config.Scenario.Name,
	})
	if err == nil || !strings.Contains(err.Error(), "runId") {
		t.Fatalf("SaveBenchmarkResult() error = %v", err)
	}
}

func TestBenchmarkSpatialConfigRequiresViewportStepsAndConcurrency(t *testing.T) {
	config := validBenchmarkConfig(t)
	config.MaxConcurrentQueries = 2
	config.Temperature = "warm"
	config.Scenario.ViewportSteps = []BenchmarkViewportStepDTO{{
		Bounds:           BoundingBoxDTO{MinX: 110.36, MinY: 31.39, MaxX: 116.63, MaxY: 36.36},
		ExpectedStrategy: "rtree",
	}}

	path := writeBenchmarkConfigFixture(t, config)
	loaded, err := loadBenchmarkConfig(path)
	if err != nil {
		t.Fatalf("loadBenchmarkConfig() error = %v", err)
	}
	if loaded.MaxConcurrentQueries != 2 || loaded.Temperature != "warm" {
		t.Fatalf("loaded config = %+v", loaded)
	}
	if got := loaded.Scenario.ViewportSteps[0].ExpectedStrategy; got != "rtree" {
		t.Fatalf("expected strategy = %q", got)
	}

	tests := []struct {
		name   string
		mutate func(*BenchmarkConfigDTO)
		want   string
	}{
		{"missing viewport steps", func(c *BenchmarkConfigDTO) { c.Scenario.ViewportSteps = nil }, "viewportSteps"},
		{"invalid strategy", func(c *BenchmarkConfigDTO) { c.Scenario.ViewportSteps[0].ExpectedStrategy = "scan" }, "expectedStrategy"},
		{"invalid bounds", func(c *BenchmarkConfigDTO) {
			c.Scenario.ViewportSteps[0].Bounds.MaxX = c.Scenario.ViewportSteps[0].Bounds.MinX - 1
		}, "bounds"},
		{"invalid concurrency", func(c *BenchmarkConfigDTO) { c.MaxConcurrentQueries = 4 }, "maxConcurrentQueries"},
		{"invalid temperature", func(c *BenchmarkConfigDTO) { c.Temperature = "lukewarm" }, "temperature"},
		{"bounded fallback is not an expected success strategy", func(c *BenchmarkConfigDTO) {
			c.Scenario.ViewportSteps[0].ExpectedStrategy = "bounded_sample"
		}, "expectedStrategy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := config
			candidate.Scenario.ViewportSteps = append([]BenchmarkViewportStepDTO(nil), config.Scenario.ViewportSteps...)
			tt.mutate(&candidate)
			_, err := loadBenchmarkConfig(writeBenchmarkConfigFixture(t, candidate))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("loadBenchmarkConfig() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestBenchmarkSpatialResultPersistsViewportMetrics(t *testing.T) {
	config := validBenchmarkConfig(t)
	config.MaxConcurrentQueries = 1
	config.Temperature = "cold"
	config.Scenario.ViewportSteps = []BenchmarkViewportStepDTO{{
		Bounds:           BoundingBoxDTO{MinX: 115, MinY: 38, MaxX: 118, MaxY: 42},
		ExpectedStrategy: "envelope_cache",
	}}
	app := NewApp()
	app.benchmarkConfig = &config
	result := BenchmarkResultDTO{
		RunID: config.RunID, Status: "passed", Scenario: config.Scenario.Name,
		Metrics: BenchmarkMetricsDTO{
			BackendQueryMS: []float64{12.5}, MoveendToRenderMS: []float64{42},
			MaxConcurrentQueries: 1, PendingPeak: 1, StaleResultsDiscarded: 2,
			StaleResultApplied: false, FinalFeatureCount: 164, BlankRenderCount: 0,
		},
	}
	if err := app.SaveBenchmarkResult(result); err != nil {
		t.Fatalf("SaveBenchmarkResult() error = %v", err)
	}
	data, err := os.ReadFile(config.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"moveendToRenderMs"`) || !strings.Contains(string(data), `"finalFeatureCount": 164`) {
		t.Fatalf("saved result lacks spatial metrics: %s", data)
	}
}

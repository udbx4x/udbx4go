package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type BenchmarkConfigDTO struct {
	RunID                string               `json:"runId"`
	OutputPath           string               `json:"outputPath"`
	Temperature          string               `json:"temperature"`
	MaxConcurrentQueries int                  `json:"maxConcurrentQueries"`
	Scenario             BenchmarkScenarioDTO `json:"scenario"`
}

type BenchmarkScenarioDTO struct {
	Name          string                     `json:"name"`
	FilePath      string                     `json:"filePath"`
	Layers        []string                   `json:"layers"`
	Selection     BenchmarkSelectionDTO      `json:"selection"`
	ViewportSteps []BenchmarkViewportStepDTO `json:"viewportSteps"`
}

type BenchmarkViewportStepDTO struct {
	Bounds           BoundingBoxDTO `json:"bounds"`
	ExpectedStrategy string         `json:"expectedStrategy"`
	HideLayers       []string       `json:"hideLayers,omitempty"`
	ShowLayers       []string       `json:"showLayers,omitempty"`
	RemoveLayers     []string       `json:"removeLayers,omitempty"`
}

type BenchmarkSelectionDTO struct {
	DatasetName string `json:"datasetName"`
	Page        int    `json:"page"`
	RowIndex    int    `json:"rowIndex"`
}

type BenchmarkMetricsDTO struct {
	OpenFileMS            float64   `json:"openFileMs"`
	LoadLayersMS          float64   `json:"loadLayersMs"`
	FitVisibleLayersMS    float64   `json:"fitVisibleLayersMs"`
	SelectAndFitMS        float64   `json:"selectAndFitMs"`
	BackendQueryMS        []float64 `json:"backendQueryMs"`
	MoveendToRenderMS     []float64 `json:"moveendToRenderMs"`
	MaxConcurrentQueries  int       `json:"maxConcurrentQueries"`
	PendingPeak           int       `json:"pendingPeak"`
	PendingFinal          int       `json:"pendingFinal"`
	StaleResultsDiscarded int       `json:"staleResultsDiscarded"`
	StaleResultApplied    bool      `json:"staleResultApplied"`
	FinalFeatureCount     int       `json:"finalFeatureCount"`
	BlankRenderCount      int       `json:"blankRenderCount"`
}

type BenchmarkResultDTO struct {
	RunID     string              `json:"runId"`
	Status    string              `json:"status"`
	StartedAt string              `json:"startedAt"`
	Scenario  string              `json:"scenario"`
	Metrics   BenchmarkMetricsDTO `json:"metrics"`
	Error     string              `json:"error"`
}

func loadBenchmarkConfig(path string) (*BenchmarkConfigDTO, error) {
	if path == "" {
		return nil, fmt.Errorf("benchmark config path is empty")
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("benchmark config path must be absolute: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open benchmark config: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var config BenchmarkConfigDTO
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("decode benchmark config: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	if err := validateBenchmarkConfig(config); err != nil {
		return nil, err
	}
	return &config, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("benchmark config contains multiple JSON values")
		}
		return fmt.Errorf("decode benchmark config trailing data: %w", err)
	}
	return nil
}

func validateBenchmarkConfig(config BenchmarkConfigDTO) error {
	if strings.TrimSpace(config.RunID) == "" {
		return fmt.Errorf("runId is required")
	}
	if !filepath.IsAbs(config.OutputPath) {
		return fmt.Errorf("outputPath must be absolute: %s", config.OutputPath)
	}
	if strings.TrimSpace(config.Scenario.Name) == "" {
		return fmt.Errorf("scenario.name is required")
	}
	if config.Temperature != "cold" && config.Temperature != "warm" {
		return fmt.Errorf("temperature must be cold or warm")
	}
	if config.MaxConcurrentQueries < 1 || config.MaxConcurrentQueries > 3 {
		return fmt.Errorf("maxConcurrentQueries must be between 1 and 3")
	}
	if !filepath.IsAbs(config.Scenario.FilePath) {
		return fmt.Errorf("scenario.filePath must be absolute: %s", config.Scenario.FilePath)
	}
	if filepath.Clean(config.OutputPath) == filepath.Clean(config.Scenario.FilePath) {
		return fmt.Errorf("outputPath must not overwrite scenario.filePath")
	}
	if len(config.Scenario.Layers) == 0 {
		return fmt.Errorf("scenario.layers must not be empty")
	}
	if len(config.Scenario.ViewportSteps) == 0 {
		return fmt.Errorf("scenario.viewportSteps must not be empty")
	}
	for index, step := range config.Scenario.ViewportSteps {
		if !validBenchmarkBounds(step.Bounds) {
			return fmt.Errorf("scenario.viewportSteps[%d].bounds is invalid", index)
		}
		if !validBenchmarkStrategy(step.ExpectedStrategy) {
			return fmt.Errorf("scenario.viewportSteps[%d].expectedStrategy is invalid", index)
		}
	}
	for index, layer := range config.Scenario.Layers {
		if strings.TrimSpace(layer) == "" {
			return fmt.Errorf("scenario.layers[%d] must not be empty", index)
		}
	}
	selection := config.Scenario.Selection
	if strings.TrimSpace(selection.DatasetName) == "" {
		return fmt.Errorf("scenario.selection.datasetName is required")
	}
	if selection.Page < 1 {
		return fmt.Errorf("scenario.selection.page must be at least 1")
	}
	if selection.RowIndex < 0 {
		return fmt.Errorf("scenario.selection.rowIndex must not be negative")
	}
	return nil
}

func validBenchmarkBounds(bounds BoundingBoxDTO) bool {
	values := []float64{bounds.MinX, bounds.MinY, bounds.MaxX, bounds.MaxY}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return bounds.MinX <= bounds.MaxX && bounds.MinY <= bounds.MaxY
}

func validBenchmarkStrategy(strategy string) bool {
	switch strategy {
	case "rtree", "envelope_cache":
		return true
	default:
		return false
	}
}

func (a *App) GetBenchmarkConfig() (*BenchmarkConfigDTO, error) {
	if a.benchmarkConfig == nil {
		return nil, nil
	}
	config := *a.benchmarkConfig
	config.Scenario.Layers = append([]string(nil), a.benchmarkConfig.Scenario.Layers...)
	config.Scenario.ViewportSteps = append([]BenchmarkViewportStepDTO(nil), a.benchmarkConfig.Scenario.ViewportSteps...)
	return &config, nil
}

func (a *App) SaveBenchmarkResult(result BenchmarkResultDTO) error {
	if a.benchmarkConfig == nil {
		return fmt.Errorf("benchmark mode is not active")
	}
	if result.RunID != a.benchmarkConfig.RunID {
		return fmt.Errorf("result runId %q does not match config runId %q", result.RunID, a.benchmarkConfig.RunID)
	}
	if result.Scenario != a.benchmarkConfig.Scenario.Name {
		return fmt.Errorf("result scenario %q does not match config scenario %q", result.Scenario, a.benchmarkConfig.Scenario.Name)
	}
	if result.Status != "passed" && result.Status != "failed" {
		return fmt.Errorf("result status must be passed or failed")
	}
	return writeBenchmarkResult(a.benchmarkConfig.OutputPath, result)
}

func writeBenchmarkResult(path string, result BenchmarkResultDTO) (returnErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create benchmark result directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".benchmark-result-*")
	if err != nil {
		return fmt.Errorf("create benchmark result temp file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		if returnErr != nil {
			_ = os.Remove(tempPath)
		}
	}()

	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		_ = temp.Close()
		return fmt.Errorf("encode benchmark result: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync benchmark result: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close benchmark result: %w", err)
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		return fmt.Errorf("set benchmark result permissions: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("publish benchmark result: %w", err)
	}
	return nil
}

func (a *App) QuitBenchmark() {
	if a.benchmarkConfig != nil && a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

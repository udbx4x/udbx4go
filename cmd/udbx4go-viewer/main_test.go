package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBenchmarkConfigArg(t *testing.T) {
	path, err := parseBenchmarkConfigArg([]string{"--benchmark-config", "/tmp/run.json"})
	if err != nil || path != "/tmp/run.json" {
		t.Fatalf("path = %q, err = %v", path, err)
	}

	path, err = parseBenchmarkConfigArg(nil)
	if err != nil || path != "" {
		t.Fatalf("no args path = %q, err = %v", path, err)
	}
}

func TestParseBenchmarkConfigArgRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing value", args: []string{"--benchmark-config"}, want: "requires"},
		{name: "relative path", args: []string{"--benchmark-config", "run.json"}, want: "absolute"},
		{name: "unknown flag", args: []string{"--other"}, want: "unknown"},
		{name: "duplicate flag", args: []string{"--benchmark-config", "/tmp/one.json", "--benchmark-config", "/tmp/two.json"}, want: "once"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseBenchmarkConfigArg(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseBenchmarkConfigArg() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestNewAppFromArgsLoadsBenchmarkConfig(t *testing.T) {
	config := validBenchmarkConfig(t)
	configPath := writeBenchmarkConfigFixture(t, config)

	app, err := newAppFromArgs([]string{"--benchmark-config", configPath})
	if err != nil {
		t.Fatalf("newAppFromArgs() error = %v", err)
	}
	if app.benchmarkConfigPath != filepath.Clean(configPath) {
		t.Fatalf("benchmarkConfigPath = %q", app.benchmarkConfigPath)
	}
	if app.benchmarkConfig == nil || app.benchmarkConfig.RunID != config.RunID {
		t.Fatalf("benchmarkConfig = %+v", app.benchmarkConfig)
	}
}

func TestNewAppFromArgsKeepsNormalMode(t *testing.T) {
	app, err := newAppFromArgs(nil)
	if err != nil {
		t.Fatalf("newAppFromArgs() error = %v", err)
	}
	if app.benchmarkConfig != nil || app.benchmarkConfigPath != "" {
		t.Fatalf("normal mode app = %+v", app)
	}
}

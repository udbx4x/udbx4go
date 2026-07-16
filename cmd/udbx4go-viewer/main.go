package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app, err := newAppFromArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Create application with options
	err = wails.Run(&options.App{
		Title:     "UDBX Viewer",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		Menu:             nil,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func parseBenchmarkConfigArg(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if args[0] != "--benchmark-config" {
		return "", fmt.Errorf("unknown argument: %s", args[0])
	}
	if len(args) < 2 {
		return "", fmt.Errorf("--benchmark-config requires an absolute path")
	}
	if len(args) > 2 {
		if args[2] == "--benchmark-config" {
			return "", fmt.Errorf("--benchmark-config may only be specified once")
		}
		return "", fmt.Errorf("unknown argument: %s", args[2])
	}
	if !filepath.IsAbs(args[1]) {
		return "", fmt.Errorf("--benchmark-config path must be absolute: %s", args[1])
	}
	return filepath.Clean(args[1]), nil
}

func newAppFromArgs(args []string) (*App, error) {
	configPath, err := parseBenchmarkConfigArg(args)
	if err != nil {
		return nil, err
	}
	app := NewApp()
	if configPath == "" {
		return app, nil
	}
	config, err := loadBenchmarkConfig(configPath)
	if err != nil {
		return nil, err
	}
	app.benchmarkConfigPath = configPath
	app.benchmarkConfig = config
	return app, nil
}

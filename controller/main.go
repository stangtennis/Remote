package main

import (
	"embed"
	"io/fs"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

// Version information - injected at build time via -ldflags -X
var (
	Version     = "dev"
	BuildDate   = "unknown"
	VersionInfo = ""
)

func init() {
	if VersionInfo == "" {
		VersionInfo = Version + " (built " + BuildDate + ")"
	}
}

func main() {
	// Check for update mode first (before any GUI initialization)
	if len(os.Args) >= 3 && os.Args[1] == "--update-from" {
		runUpdateMode(os.Args[2])
		return
	}

	// Create application
	app := NewApp()

	// Get frontend assets as sub-filesystem
	frontendFS, err := fs.Sub(assets, "frontend")
	if err != nil {
		fmt.Printf("Failed to load frontend assets: %v\n", err)
		os.Exit(1)
	}

	// Run Wails application
	err = wails.Run(&options.App{
		Title:     "Remote Desktop Controller " + Version,
		Width:     1100,
		Height:    750,
		MinWidth:  800,
		MinHeight: 550,
		AssetServer: &assetserver.Options{
			Assets: frontendFS,
		},
		BackgroundColour: &options.RGBA{R: 3, G: 7, B: 18, A: 255}, // --background: #030712
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

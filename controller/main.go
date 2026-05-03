package main

import (
	"embed"
	"io/fs"
	"fmt"
	"os"

	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
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

	// Disable WebRTC mDNS hostname obfuscation i WebView2.
	//
	// Edge/Chromium gemmer som default lokale IPs bag tilfældige *.local
	// hostnames i WebRTC-host-candidates (privacy-feature siden 2018).
	// Pion-agenten kan ikke resolve .local → afviser host candidates →
	// connection falder tilbage til TURN selv på samme LAN.
	//
	// Sæt env-var FØR Wails starter, så WebView2 instans bruger flagsene.
	// WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS er den officielle MS API.
	if runtime.GOOS == "windows" {
		_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
			"--disable-features=WebRtcHideLocalIpsWithMdns")
	}

	// Create application
	app := NewApp()

	// Get frontend assets as sub-filesystem
	frontendFS, err := fs.Sub(assets, "frontend")
	if err != nil {
		fmt.Printf("Failed to load frontend assets: %v\n", err)
		os.Exit(1)
	}

	// Window title
	title := "Remote Desktop Controller " + Version
	if runtime.GOOS == "darwin" {
		title = "Remote Desktop " + Version
	}

	// Run Wails application
	err = wails.Run(&options.App{
		Title:     title,
		Width:     1100,
		Height:    750,
		MinWidth:  800,
		MinHeight: 550,
		AssetServer: &assetserver.Options{
			Assets: frontendFS,
		},
		BackgroundColour: &options.RGBA{R: 3, G: 7, B: 18, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			Appearance: mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			About: &mac.AboutInfo{
				Title:   "Remote Desktop Controller",
				Message: "v" + Version,
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

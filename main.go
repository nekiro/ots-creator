package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/settings"
)

// version is set at build time (-ldflags "-X main.version=x.y.z").
var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[app.State](app.EventProjectChanged)
	application.RegisterEvent[app.State](app.EventOtherChanged)
	application.RegisterEvent[app.Progress](app.EventProgress)
}

func main() {
	var wailsApp *application.App
	emit := func(name string, data any) {
		if wailsApp != nil {
			wailsApp.Event.Emit(name, data)
		}
	}
	app.Generator = "OTS Creator " + version
	session := app.NewSession(emit)
	prefs := openSettings()
	resources := app.NewResources(session)
	var selfUpdater app.Updater // set below for release builds
	updates := app.NewUpdateService(version, func() app.Updater { return selfUpdater })
	var mainWindow *application.WebviewWindow // set below
	windows := app.NewWindowService(session, func() {
		if mainWindow != nil {
			mainWindow.Close()
		}
	})

	wailsApp = application.New(application.Options{
		Name:        "OTS Creator",
		Description: "Object editor for Open Tibia clients",
		Services: []application.Service{
			application.NewService(app.NewProjectService(session)),
			application.NewService(app.NewThingService(session)),
			application.NewService(app.NewSpriteService(session)),
			application.NewService(app.NewCompareService(session)),
			application.NewService(&app.DialogService{}),
			application.NewService(&app.ViewerService{}),
			application.NewService(updates),
			application.NewService(windows),
			application.NewService(app.NewSettingsService(prefs, session)),
			application.NewService(app.NewMarketService(session, prefs)),
		},
		Assets: application.AssetOptions{
			Handler:        application.AssetFileServerFS(assets),
			Middleware:     resources.Middleware,
			DisableLogging: true,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Self-update from GitHub releases (release builds only). The UI is our
	// own dialog driven by the standard wails:updater:* events.
	if version != "dev" {
		if err := initUpdater(wailsApp); err != nil {
			log.Printf("updater: %v", err)
		} else {
			selfUpdater = wailsApp.Updater
		}
	}

	mainWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "OTS Creator",
		Width:            1440,
		Height:           900,
		MinWidth:         1024,
		MinHeight:        640,
		Frameless:        true, // the menu bar draws Tibia-style window controls
		EnableFileDrop:   true, // see data-file-drop-target in App.svelte
		BackgroundColour: application.NewRGB(0x16, 0x16, 0x16),
		URL:              "/",
	})
	// Closing with uncompiled or unapplied changes asks first (in the UI).
	blockClose := app.BlockClose(windows)
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if blockClose() {
			e.Cancel()
		}
	})

	// Dropped files are handled by the frontend (open, import OBD/images).
	mainWindow.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		target := ""
		if d := e.Context().DropTargetDetails(); d != nil {
			target = d.ElementID
		}
		emit(app.EventFilesDropped, app.FilesDropped{Paths: e.Context().DroppedFiles(), Target: target})
	})

	// Open a client passed on the command line (file association, dev runs).
	if len(os.Args) > 1 {
		ps := app.NewProjectService(session)
		if cf, err := ps.Inspect(os.Args[1]); err != nil {
			log.Printf("open %s: %v", os.Args[1], err)
		} else if _, err := ps.Open(app.OpenRequest{DatPath: cf.DatPath, SprPath: cf.SprPath, Version: cf.Detected, Features: cf.Features}); err != nil {
			log.Printf("open %s: %v", os.Args[1], err)
		}
	}

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// openSettings loads the user settings; on errors it falls back to
// defaults (and still saves when the file can be written).
func openSettings() *settings.Store {
	path, err := settings.DefaultPath()
	if err != nil {
		log.Printf("settings: %v", err)
	}
	st, err := settings.Open(path)
	if err != nil {
		log.Printf("settings: %v", err)
	}
	return st
}

func initUpdater(a *application.App) error {
	gh, err := github.New(github.Config{Repository: app.UpdateRepository, ChecksumAsset: app.UpdateChecksums})
	if err != nil {
		return err
	}
	return a.Updater.Init(updater.Config{
		CurrentVersion: version,
		Providers:      []updater.Provider{gh},
		Window:         updater.WindowNone,
	})
}

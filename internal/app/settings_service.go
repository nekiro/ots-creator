package app

import (
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/settings"
)

type SettingsService struct {
	settings *settings.Store
	// ApplyMCP starts or stops the MCP server for the settings (nil: none).
	ApplyMCP func(enabled bool, port int) error
}

func NewSettingsService(store *settings.Store, session *Session) *SettingsService {
	service := &SettingsService{settings: store}
	session.onSaved = service.remember
	session.otbPaths = store
	return service
}

func (ss *SettingsService) Get() settings.Settings { return ss.settings.Get() }

func (ss *SettingsService) Update(prefs settings.Settings) error {
	if ss.ApplyMCP != nil {
		if err := ss.ApplyMCP(prefs.MCPServer, prefs.MCPPort); err != nil {
			return err
		}
	}
	return ss.settings.Update(prefs)
}

func (ss *SettingsService) RemoveRecent(datPath string) error {
	return ss.settings.RemoveRecent(datPath)
}

func assetsFormat(format project.Format) string {
	if format == project.FormatAssets {
		return string(format)
	}
	return ""
}

func (ss *SettingsService) remember(info project.Info) {
	_ = ss.settings.AddRecent(settings.Recent{Format: assetsFormat(info.Format), DatPath: info.DatPath, SprPath: info.SprPath, Version: info.Version, Features: info.Features})
}

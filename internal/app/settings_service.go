package app

import (
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/settings"
)

type SettingsService struct {
	settings *settings.Store
}

func NewSettingsService(store *settings.Store, session *Session) *SettingsService {
	service := &SettingsService{settings: store}
	session.onSaved = service.remember
	session.otbPaths = store
	return service
}

func (ss *SettingsService) Get() settings.Settings { return ss.settings.Get() }

func (ss *SettingsService) Update(prefs settings.Settings) error { return ss.settings.Update(prefs) }

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

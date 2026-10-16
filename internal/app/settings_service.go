package app

import (
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/settings"
)

// SettingsService exposes user preferences and recent clients.
type SettingsService struct {
	settings *settings.Store
}

// NewSettingsService returns the service. Opened and compiled clients of
// the session are added to the recent list.
func NewSettingsService(store *settings.Store, session *Session) *SettingsService {
	service := &SettingsService{settings: store}
	session.onSaved = service.remember
	session.otbPaths = store
	return service
}

// Get returns the settings and the recent clients.
func (ss *SettingsService) Get() settings.Settings { return ss.settings.Get() }

// Update saves the preferences (the recent list is not changed).
func (ss *SettingsService) Update(prefs settings.Settings) error { return ss.settings.Update(prefs) }

// RemoveRecent forgets one recent client; "" clears the list.
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

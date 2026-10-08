package app

import (
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/settings"
)

// SettingsService exposes user preferences and recent clients.
type SettingsService struct {
	st *settings.Store
}

// NewSettingsService returns the service. Opened and compiled clients of
// the session are added to the recent list.
func NewSettingsService(st *settings.Store, s *Session) *SettingsService {
	ss := &SettingsService{st: st}
	s.onSaved = ss.remember
	return ss
}

// Get returns the settings and the recent clients.
func (ss *SettingsService) Get() settings.Settings { return ss.st.Get() }

// Update saves the preferences (the recent list is not changed).
func (ss *SettingsService) Update(s settings.Settings) error { return ss.st.Update(s) }

// RemoveRecent forgets one recent client; "" clears the list.
func (ss *SettingsService) RemoveRecent(datPath string) error { return ss.st.RemoveRecent(datPath) }

func assetsFormat(f project.Format) string {
	if f == project.FormatAssets {
		return string(f)
	}
	return ""
}

func (ss *SettingsService) remember(info project.Info) {
	_ = ss.st.AddRecent(settings.Recent{Format: assetsFormat(info.Format), DatPath: info.DatPath, SprPath: info.SprPath, Version: info.Version, Features: info.Features})
}

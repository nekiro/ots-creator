package app

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nekiro/ots-creator/internal/fsutil"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/settings"
)

// MapDataConfig tells the map editor where the server data of a client
// lives.
type MapDataConfig struct {
	// ItemsMode is "auto" (items.otb when found), "otb" or "client".
	ItemsMode string `json:"itemsMode"`
	OTBPath   string `json:"otbPath"`
	XMLPath   string `json:"xmlPath"`
	// MaterialsPath is the brush file (JSON). Empty uses the default file
	// of the client version in the settings folder.
	MaterialsPath string `json:"materialsPath"`
	// ServerDir is the server data folder scanned for monsters and NPCs.
	ServerDir string `json:"serverDir"`
}

// RecentMap is a recently opened map.
type RecentMap struct {
	Path string `json:"path"`
	// Client is the client the map was edited with.
	Client string `json:"client"`
}

type mapStoreData struct {
	Profiles map[string]MapDataConfig `json:"profiles"`
	Recent   []RecentMap              `json:"recent"`
	Editor   *mapedit.Settings        `json:"editor"`
	// Keys holds the keyboard shortcuts changed by the user: action id ->
	// key combinations ("Ctrl+S"). Missing actions use their defaults.
	Keys map[string][]string `json:"keys,omitempty"`
}

// MapStore persists map editor settings next to the app settings.
type MapStore struct {
	mu   sync.Mutex
	path string
	data mapStoreData
}

// maxRecentMaps limits the recent map list.
const maxRecentMaps = 10

// OpenMapStore loads the map editor settings from the settings folder
// (defaults on errors).
func OpenMapStore() *MapStore {
	settingsPath, err := settings.DefaultPath()
	if err != nil {
		return NewMapStore("")
	}
	return NewMapStore(filepath.Join(filepath.Dir(settingsPath), "mapeditor.json"))
}

// NewMapStore loads map editor settings from path ("" keeps them in
// memory only).
func NewMapStore(path string) *MapStore {
	st := &MapStore{path: path, data: mapStoreData{Profiles: map[string]MapDataConfig{}}}
	if path == "" {
		return st
	}
	data, err := os.ReadFile(st.path)
	if err == nil {
		_ = json.Unmarshal(data, &st.data)
	}
	if st.data.Profiles == nil {
		st.data.Profiles = map[string]MapDataConfig{}
	}
	return st
}

// Dir returns the folder of the settings ("" when unknown).
func (st *MapStore) Dir() string {
	if st.path == "" {
		return ""
	}
	return filepath.Dir(st.path)
}

func (st *MapStore) saveLocked() error {
	if st.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(st.data, "", "\t")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(st.path), fsutil.DirMode); err != nil {
		return err
	}
	return fsutil.WriteAtomic(st.path, data)
}

func profileKey(client string) string { return strings.ToLower(filepath.Clean(client)) }

// Profile returns the data config of a client.
func (st *MapStore) Profile(client string) (MapDataConfig, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	config, ok := st.data.Profiles[profileKey(client)]
	return config, ok
}

// SetProfile stores the data config of a client.
func (st *MapStore) SetProfile(client string, config MapDataConfig) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.data.Profiles[profileKey(client)] = config
	return st.saveLocked()
}

// Recent returns the recent maps.
func (st *MapStore) Recent() []RecentMap {
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]RecentMap(nil), st.data.Recent...)
}

// AddRecent puts a map first in the recent list.
func (st *MapStore) AddRecent(recent RecentMap) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := []RecentMap{recent}
	for _, entry := range st.data.Recent {
		if !fsutil.SamePath(entry.Path, recent.Path) {
			out = append(out, entry)
		}
	}
	st.data.Recent = out[:min(len(out), maxRecentMaps)]
	return st.saveLocked()
}

// RemoveRecent forgets a recent map ("" clears the list).
func (st *MapStore) RemoveRecent(path string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []RecentMap
	for _, entry := range st.data.Recent {
		if path != "" && !fsutil.SamePath(entry.Path, path) {
			out = append(out, entry)
		}
	}
	st.data.Recent = out
	return st.saveLocked()
}

// EditorSettings returns the brush behavior settings.
func (st *MapStore) EditorSettings() mapedit.Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.data.Editor == nil {
		return mapedit.DefaultSettings()
	}
	return *st.data.Editor
}

// SetEditorSettings stores the brush behavior settings.
func (st *MapStore) SetEditorSettings(settings mapedit.Settings) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.data.Editor = &settings
	return st.saveLocked()
}

// Keys returns the changed keyboard shortcuts.
func (st *MapStore) Keys() map[string][]string {
	st.mu.Lock()
	defer st.mu.Unlock()
	return maps.Clone(st.data.Keys)
}

// SetKeys stores the changed keyboard shortcuts (nil restores all defaults).
func (st *MapStore) SetKeys(keys map[string][]string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.data.Keys = maps.Clone(keys)
	return st.saveLocked()
}

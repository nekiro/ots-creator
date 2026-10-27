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

type MapDataConfig struct {
	// ItemsMode is "auto" (items.otb when found), "otb" or "client".
	ItemsMode string `json:"itemsMode"`
	OTBPath   string `json:"otbPath"`
	XMLPath   string `json:"xmlPath"`
	// MaterialsPath is the brush file (JSON). Empty uses the default file
	// of the client version in the settings folder.
	MaterialsPath string `json:"materialsPath"`
	ServerDir     string `json:"serverDir"`
}

type RecentMap struct {
	Path   string `json:"path"`
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

type MapStore struct {
	mu   sync.Mutex
	path string
	data mapStoreData
}

const maxRecentMaps = 10

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
	store := &MapStore{path: path, data: mapStoreData{Profiles: map[string]MapDataConfig{}}}
	if path == "" {
		return store
	}
	data, err := os.ReadFile(store.path)
	if err == nil {
		_ = json.Unmarshal(data, &store.data)
	}
	if store.data.Profiles == nil {
		store.data.Profiles = map[string]MapDataConfig{}
	}
	return store
}

func (st *MapStore) Dir() string {
	if st.path == "" {
		return ""
	}
	return filepath.Dir(st.path)
}

func (st *MapStore) saveLocked() error {
	return fsutil.WriteJSON(st.path, st.data, "\t")
}

func recentMapPath(recent RecentMap) string { return recent.Path }

func profileKey(client string) string { return strings.ToLower(filepath.Clean(client)) }

func (st *MapStore) Profile(client string) (MapDataConfig, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	config, ok := st.data.Profiles[profileKey(client)]
	return config, ok
}

func (st *MapStore) SetProfile(client string, config MapDataConfig) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.data.Profiles[profileKey(client)] = config
	return st.saveLocked()
}

func (st *MapStore) Recent() []RecentMap {
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]RecentMap(nil), st.data.Recent...)
}

func (st *MapStore) AddRecent(recent RecentMap) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := fsutil.AppendOtherPaths([]RecentMap{recent}, st.data.Recent, recent.Path, recentMapPath)
	st.data.Recent = out[:min(len(out), maxRecentMaps)]
	return st.saveLocked()
}

func (st *MapStore) RemoveRecent(path string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []RecentMap
	if path != "" {
		out = fsutil.AppendOtherPaths(out, st.data.Recent, path, recentMapPath)
	}
	st.data.Recent = out
	return st.saveLocked()
}

func (st *MapStore) EditorSettings() mapedit.Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.data.Editor == nil {
		return mapedit.DefaultSettings()
	}
	return *st.data.Editor
}

func (st *MapStore) SetEditorSettings(settings mapedit.Settings) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.data.Editor = &settings
	return st.saveLocked()
}

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

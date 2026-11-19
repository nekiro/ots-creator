package app

import (
	"maps"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/fsutil"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/settings"
)

// MapDataConfig is the server data the map editor reads for a client. The
// items.otb is not part of it: it is the one attached to the client (see
// Session.attachItemsOTB), and each map tells whether it uses it (see
// MapItemIDs).
type MapDataConfig struct {
	// XMLPath is the items.xml with the item names ("": names of the client).
	XMLPath string `json:"xmlPath"`
	// MaterialsPath is the brush file (JSON). Empty uses the default file
	// of the client version in the settings folder.
	MaterialsPath string `json:"materialsPath"`
	ServerDir     string `json:"serverDir"`
	// LegacyOTBPath is the items.otb older versions kept here, before the
	// map editor used the one attached to the client. It is only offered
	// when a map is opened.
	LegacyOTBPath string `json:"otbPath,omitempty"`
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
	// ItemIDs keeps, by map path, whether the map uses server ids (with the
	// items.otb of the client) or client ids, as chosen when it was opened.
	ItemIDs map[string]otitems.Mode `json:"itemIds,omitempty"`
}

type MapStore struct {
	mu   sync.Mutex
	path string
	data mapStoreData
	// problem tells why the stored settings could not be loaded ("": loaded).
	problem string
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
	var loaded mapStoreData
	if err := fsutil.ReadJSON(store.path, &loaded); err != nil {
		store.problem = err.Error()
	} else {
		store.data = loaded
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

// MapItemIDs returns the item id mode chosen for the map at path.
func (st *MapStore) MapItemIDs(path string) (otitems.Mode, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	mode, ok := st.data.ItemIDs[profileKey(path)]
	return mode, ok && (mode == otitems.ModeOTB || mode == otitems.ModeClient)
}

func (st *MapStore) SetMapItemIDs(path string, mode otitems.Mode) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	key := profileKey(path)
	if st.data.ItemIDs[key] == mode {
		return nil
	}
	if st.data.ItemIDs == nil {
		st.data.ItemIDs = map[string]otitems.Mode{}
	}
	st.data.ItemIDs[key] = mode
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

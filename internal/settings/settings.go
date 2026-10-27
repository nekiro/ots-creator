package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nekiro/ots-creator/internal/fsutil"

	"github.com/nekiro/ots-creator/internal/core/client"
)

const MaxRecent = 10

const (
	BackgroundMagenta     = "magenta"
	BackgroundTransparent = "transparent"
)

type Recent struct {
	// Format is "assets" for a Tibia 12+ asset folder (DatPath is the
	// folder); empty for dat/spr.
	Format   string          `json:"format,omitempty"`
	DatPath  string          `json:"datPath"`
	SprPath  string          `json:"sprPath"`
	Version  client.Version  `json:"version"`
	Features client.Features `json:"features"`
	OpenedAt time.Time       `json:"openedAt"`
}

type Settings struct {
	CheckUpdates bool `json:"checkUpdates"`
	ReopenLast   bool `json:"reopenLast"`
	// SheetBackground fills empty pixels of exported sprite sheets
	// (BackgroundMagenta like ObjectBuilder, or BackgroundTransparent).
	SheetBackground string `json:"sheetBackground"`
	ExportFormat    string `json:"exportFormat"`
	// ObjectFormat is the file format of exported objects: "otobj" (the
	// native format) or "obd" (ObjectBuilder).
	ObjectFormat     string   `json:"objectFormat"`
	ListColumns      int      `json:"listColumns"`
	Recent           []Recent `json:"recent"`
	MarketAuthor     string   `json:"marketAuthor"`
	MarketAdminToken string   `json:"marketAdminToken"`
	// MarketShared maps market entries shared from this computer to their
	// delete tokens. Like Recent, Update keeps it as stored.
	MarketShared map[string]string `json:"marketShared"`
	// LastDirs maps a file dialog to the folder it last confirmed, so each
	// dialog opens where it was left. Like Recent, Update keeps it as stored.
	LastDirs map[string]string `json:"lastDirs"`
	// ItemsOTB maps a client (its dat path or asset folder) to the
	// items.otb kept in sync with it. Like Recent, Update keeps it as stored.
	ItemsOTB map[string]string `json:"itemsOtb"`
}

const (
	MinListColumns     = 6
	MaxListColumns     = 12
	DefaultListColumns = 7
)

func Defaults() Settings {
	return Settings{CheckUpdates: true, SheetBackground: BackgroundMagenta, ExportFormat: "png", ObjectFormat: "otobj", ListColumns: DefaultListColumns, Recent: []Recent{}, MarketShared: map[string]string{}, LastDirs: map[string]string{}, ItemsOTB: map[string]string{}}
}

func (s *Settings) normalize() {
	if s.SheetBackground != BackgroundTransparent {
		s.SheetBackground = BackgroundMagenta
	}
	switch s.ExportFormat {
	case "png", "bmp", "jpg":
	default:
		s.ExportFormat = "png"
	}
	if s.ObjectFormat != "obd" {
		s.ObjectFormat = "otobj"
	}
	if s.ListColumns == 0 {
		s.ListColumns = DefaultListColumns
	}
	s.ListColumns = min(max(s.ListColumns, MinListColumns), MaxListColumns)
	if s.Recent == nil {
		s.Recent = []Recent{}
	}
	if s.MarketShared == nil {
		s.MarketShared = map[string]string{}
	}
	if s.LastDirs == nil {
		s.LastDirs = map[string]string{}
	}
	if s.ItemsOTB == nil {
		s.ItemsOTB = map[string]string{}
	}
	if len(s.Recent) > MaxRecent {
		s.Recent = s.Recent[:MaxRecent]
	}
}

type Store struct {
	mu       sync.Mutex
	path     string
	settings Settings
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "OTS Creator", "settings.json"), nil
}

// Open loads settings from path. A missing or unreadable file gives the
// defaults; the error reports a corrupt file but the store still works.
func Open(path string) (*Store, error) {
	store := &Store{path: path, settings: Defaults()}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return store, err
	}
	loaded := Defaults()
	if err := json.Unmarshal(data, &loaded); err != nil {
		return store, err
	}
	loaded.normalize()
	store.settings = loaded
	return store, nil
}

func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	snapshot := st.settings
	snapshot.Recent = append([]Recent{}, st.settings.Recent...)
	snapshot.MarketShared = maps.Clone(st.settings.MarketShared)
	snapshot.LastDirs = maps.Clone(st.settings.LastDirs)
	snapshot.ItemsOTB = maps.Clone(st.settings.ItemsOTB)
	return snapshot
}

// Update changes the preferences. The recent list, shared market entries
// and dialog folders are kept as stored.
func (st *Store) Update(updated Settings) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	updated.Recent = st.settings.Recent
	updated.MarketShared = st.settings.MarketShared
	updated.LastDirs = st.settings.LastDirs
	updated.ItemsOTB = st.settings.ItemsOTB
	updated.normalize()
	st.settings = updated
	return st.saveLocked()
}

func (st *Store) AddRecent(recent Recent) error {
	if recent.DatPath == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if recent.OpenedAt.IsZero() {
		recent.OpenedAt = time.Now()
	}
	st.settings.Recent = fsutil.AppendOtherPaths([]Recent{recent}, st.settings.Recent, recent.DatPath, recentDatPath)
	st.settings.normalize()
	return st.saveLocked()
}

// SetMarketShared remembers the delete token of a shared market entry;
// an empty token forgets the entry.
func (st *Store) SetMarketShared(id, token string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if token == "" {
		delete(st.settings.MarketShared, id)
	} else {
		st.settings.MarketShared[id] = token
	}
	return st.saveLocked()
}

func (st *Store) SetMarketAuthor(name string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.settings.MarketAuthor == name {
		return nil
	}
	st.settings.MarketAuthor = name
	return st.saveLocked()
}

func (st *Store) LastDir(key string) string {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.settings.LastDirs[key]
}

func (st *Store) SetLastDir(key, dir string) error {
	if key == "" || dir == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.settings.LastDirs[key] == dir {
		return nil
	}
	st.settings.LastDirs[key] = dir
	return st.saveLocked()
}

func (st *Store) RemoveRecent(datPath string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	list := []Recent{}
	if datPath != "" {
		list = fsutil.AppendOtherPaths(list, st.settings.Recent, datPath, recentDatPath)
	}
	st.settings.Recent = list
	return st.saveLocked()
}

func recentDatPath(recent Recent) string { return recent.DatPath }

func (st *Store) ItemsOTB(clientPath string) string {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.settings.ItemsOTB[fsutil.PathKey(clientPath)]
}

func (st *Store) SetItemsOTB(clientPath, otbPath string) error {
	if clientPath == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.settings.ItemsOTB == nil {
		st.settings.ItemsOTB = map[string]string{}
	}
	if otbPath == "" {
		delete(st.settings.ItemsOTB, fsutil.PathKey(clientPath))
	} else {
		st.settings.ItemsOTB[fsutil.PathKey(clientPath)] = otbPath
	}
	return st.saveLocked()
}

func (st *Store) saveLocked() error {
	return fsutil.WriteJSON(st.path, st.settings, "  ")
}

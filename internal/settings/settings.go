// Package settings stores user preferences and the recently opened clients
// in a JSON file in the user's config directory.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nekiro/ots-creator/internal/client"
)

// MaxRecent is the number of remembered clients.
const MaxRecent = 10

// Sheet backgrounds.
const (
	BackgroundMagenta     = "magenta"
	BackgroundTransparent = "transparent"
)

// Recent is a client that was opened or compiled.
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

// Settings are the user preferences.
type Settings struct {
	// CheckUpdates checks GitHub for a new release on startup.
	CheckUpdates bool `json:"checkUpdates"`
	// ReopenLast opens the most recent client on startup.
	ReopenLast bool `json:"reopenLast"`
	// SheetBackground fills empty pixels of exported sprite sheets
	// (BackgroundMagenta like ObjectBuilder, or BackgroundTransparent).
	SheetBackground string `json:"sheetBackground"`
	// ExportFormat is the default image format: png, bmp or jpg.
	ExportFormat string `json:"exportFormat"`
	// ObjectFormat is the file format of exported objects: "otobj" (the
	// native format) or "obd" (ObjectBuilder).
	ObjectFormat string `json:"objectFormat"`
	// ListColumns is the number of objects per row in the object list; the
	// list panel is sized to fit them.
	ListColumns int      `json:"listColumns"`
	Recent      []Recent `json:"recent"`
	// MarketAuthor is the nickname last used to share to the market.
	MarketAuthor string `json:"marketAuthor"`
	// MarketAdminToken deletes any market entry (the API's admin token).
	MarketAdminToken string `json:"marketAdminToken"`
	// MarketShared maps market entries shared from this computer to their
	// delete tokens. Like Recent, Update keeps it as stored.
	MarketShared map[string]string `json:"marketShared"`
}

// Object list columns range.
const (
	MinListColumns     = 6
	MaxListColumns     = 12
	DefaultListColumns = 7
)

// Defaults returns the settings of a fresh install.
func Defaults() Settings {
	return Settings{CheckUpdates: true, SheetBackground: BackgroundMagenta, ExportFormat: "png", ObjectFormat: "otobj", ListColumns: DefaultListColumns, Recent: []Recent{}, MarketShared: map[string]string{}}
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
	if len(s.Recent) > MaxRecent {
		s.Recent = s.Recent[:MaxRecent]
	}
}

// Store loads and saves settings. It is safe for concurrent use.
type Store struct {
	mu   sync.Mutex
	path string
	s    Settings
}

// DefaultPath returns the settings file in the user's config directory.
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
	st := &Store{path: path, s: Defaults()}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	s := Defaults()
	if err := json.Unmarshal(data, &s); err != nil {
		return st, err
	}
	s.normalize()
	st.s = s
	return st, nil
}

// Get returns a copy of the settings.
func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	s := st.s
	s.Recent = append([]Recent{}, st.s.Recent...)
	s.MarketShared = maps.Clone(st.s.MarketShared)
	return s
}

// Update changes the preferences. The recent list and shared market
// entries are kept as stored.
func (st *Store) Update(s Settings) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	s.Recent = st.s.Recent
	s.MarketShared = st.s.MarketShared
	s.normalize()
	st.s = s
	return st.saveLocked()
}

// AddRecent moves a client to the top of the recent list.
func (st *Store) AddRecent(r Recent) error {
	if r.DatPath == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if r.OpenedAt.IsZero() {
		r.OpenedAt = time.Now()
	}
	list := []Recent{r}
	for _, x := range st.s.Recent {
		if !samePath(x.DatPath, r.DatPath) {
			list = append(list, x)
		}
	}
	st.s.Recent = list
	st.s.normalize()
	return st.saveLocked()
}

// SetMarketShared remembers the delete token of a shared market entry;
// an empty token forgets the entry.
func (st *Store) SetMarketShared(id, token string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if token == "" {
		delete(st.s.MarketShared, id)
	} else {
		st.s.MarketShared[id] = token
	}
	return st.saveLocked()
}

// SetMarketAuthor remembers the nickname used to share.
func (st *Store) SetMarketAuthor(name string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.s.MarketAuthor == name {
		return nil
	}
	st.s.MarketAuthor = name
	return st.saveLocked()
}

// RemoveRecent drops a client from the recent list; "" clears it.
func (st *Store) RemoveRecent(datPath string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	list := []Recent{}
	for _, x := range st.s.Recent {
		if datPath != "" && !samePath(x.DatPath, datPath) {
			list = append(list, x)
		}
	}
	st.s.Recent = list
	return st.saveLocked()
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (st *Store) saveLocked() error {
	if st.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(st.s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(st.path), 0o755); err != nil {
		return err
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, st.path)
}

package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/suggest"
)

var ErrNoMap = errors.New("the map is not open")

var errNoClient = errors.New("open a client first: the map editor draws the map with its sprites")

type openMap struct {
	id     int
	path   string
	// editor.Items.Mode is the item id mode of the map (see map_data.go).
	editor *mapedit.Editor
	// suggestions counts which items go together (see map_suggest.go);
	// indexing is set while a goroutine counts the changed chunks.
	suggestions *suggest.Index
	indexing    atomic.Bool
	// title names a map without a file (a map shared by another editor).
	title string
}

func (m *openMap) name() string {
	if m.path == "" && m.title != "" {
		return m.title
	}
	if m.path == "" {
		return fmt.Sprintf("untitled-%d.otbm", m.id)
	}
	return filepath.Base(m.path)
}

// MapService edits OTBM maps with the sprites of the open client. Its
// methods are split by topic: map_data.go (item data), map_files.go (new,
// open, save), map_edits.go, map_selection.go, map_tools.go,
// map_inspector.go, map_palette.go, map_materials.go and
// map_render_tables.go.
type MapService struct {
	session *Session
	store   *MapStore
	// mu guards everything; requests that only read the map share it.
	mu            sync.RWMutex
	maps          map[int]*openMap
	lastID        int
	// data is the item data of the open client by item id mode.
	data          map[otitems.Mode]*mapData
	clipboard     *mapedit.Clip
	clipboardMode otitems.Mode
	activeID      int
	// looks keeps the sprite looks of items for finding look-alikes.
	looks lookCache
	// closed is called (under the lock) when a map closes.
	closed []func(id int)
}

func NewMapService(session *Session, store *MapStore) *MapService {
	session.Warn(store.problem)
	return &MapService{session: session, store: store, maps: map[int]*openMap{}, data: map[otitems.Mode]*mapData{}}
}

// get returns an open map and the item data of its mode. The caller holds
// the write lock.
func (ms *MapService) get(id int) (*openMap, *mapData, error) {
	if _, err := ms.session.Project(); err != nil {
		return nil, nil, errNoClient
	}
	opened := ms.maps[id]
	if opened == nil {
		return nil, nil, ErrNoMap
	}
	data, err := ms.dataFor(opened)
	if err != nil {
		return nil, nil, err
	}
	return opened, data, nil
}

// read read-locks the service and returns an open map and the item data
// with the function that unlocks. Loading the item data, the first time,
// takes the write lock.
func (ms *MapService) read(id int) (*openMap, *mapData, func(), error) {
	for {
		ms.mu.RLock()
		if opened := ms.maps[id]; opened != nil {
			if data := ms.current(opened.editor.Items.Mode); data != nil {
				return opened, data, ms.mu.RUnlock, nil
			}
		}
		ms.mu.RUnlock()
		ms.mu.Lock()
		_, _, err := ms.get(id)
		ms.mu.Unlock()
		if err != nil {
			return nil, nil, nil, err
		}
	}
}

func (ms *MapService) withMap(id int, fn func(opened *openMap, data *mapData) error) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	opened, data, err := ms.get(id)
	if err != nil {
		return err
	}
	return fn(opened, data)
}

// withMapRead is withMap under the read lock, for fn that only reads the
// map: drawing and loading chunks go on while it runs.
func (ms *MapService) withMapRead(id int, fn func(opened *openMap, data *mapData) error) error {
	opened, data, unlock, err := ms.read(id)
	if err != nil {
		return err
	}
	defer unlock()
	return fn(opened, data)
}

func (ms *MapService) withData(fn func(data *mapData) error) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	data, err := ms.dataOf(ms.activeMode())
	if err != nil {
		return err
	}
	return fn(data)
}

// edit runs fn on a map and returns the change; fn returns the number of
// affected tiles or items.
func (ms *MapService) edit(id int, fn func(editor *mapedit.Editor, data *mapData) (int, error)) (MapChange, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	opened, data, err := ms.get(id)
	if err != nil {
		return MapChange{}, err
	}
	beforeMode := opened.editor.Items.Mode
	count, err := fn(opened.editor, data)
	// Converting the ids (or undoing it) changes the mode of the map: it
	// takes the data of its new mode.
	if resolved, dataErr := ms.dataFor(opened); dataErr == nil {
		opened.editor.Items = resolved.items
		if opened.editor.Brushes != resolved.brushes {
			opened.editor.SetBrushes(resolved.brushes)
		}
	} else if err == nil {
		err = dataErr
	}
	change := ms.change(opened)
	change.All = beforeMode != opened.editor.Items.Mode
	change.Count = count
	if len(change.Chunks) > 0 || change.All {
		ms.session.notify(EventMapChanged, MapChanged{ID: id, Chunks: change.Chunks, All: change.All})
	}
	return change, err
}

func (ms *MapService) editMap(id int, fn func(editor *mapedit.Editor) error) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) { return 0, fn(editor) })
}

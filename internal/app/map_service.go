package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
)

var ErrNoMap = errors.New("the map is not open")

var errNoClient = errors.New("open a client first: the map editor draws the map with its sprites")

type openMap struct {
	id     int
	path   string
	editor *mapedit.Editor
	// clientData is retained across Undo/Redo of ID conversion.
	clientData *mapData
}

func (m *openMap) name() string {
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
	data          *mapData
	clipboard     *mapedit.Clip
	clipboardMode otitems.Mode
	activeID      int
}

func NewMapService(session *Session, store *MapStore) *MapService {
	return &MapService{session: session, store: store, maps: map[int]*openMap{}}
}

// get returns an open map and the item data. The caller holds the lock.
func (ms *MapService) get(id int) (*openMap, *mapData, error) {
	_, err := ms.ensure("")
	if err != nil {
		return nil, nil, err
	}
	opened := ms.maps[id]
	if opened == nil {
		return nil, nil, ErrNoMap
	}
	return opened, ms.dataFor(opened), nil
}

// read read-locks the service and returns an open map and the item data
// with the function that unlocks. Loading the item data, the first time,
// takes the write lock.
func (ms *MapService) read(id int) (*openMap, *mapData, func(), error) {
	for {
		ms.mu.RLock()
		project, err := ms.session.Project()
		if err == nil && ms.data != nil && ms.data.project == project {
			opened := ms.maps[id]
			if opened == nil {
				ms.mu.RUnlock()
				return nil, nil, nil, ErrNoMap
			}
			return opened, ms.dataFor(opened), ms.mu.RUnlock, nil
		}
		ms.mu.RUnlock()
		ms.mu.Lock()
		_, _, err = ms.get(id)
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

func (ms *MapService) withData(fn func(data *mapData) error) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	data, err := ms.ensure("")
	if err != nil {
		return err
	}
	if opened := ms.maps[ms.activeID]; opened != nil {
		data = ms.dataFor(opened)
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
	resolved := ms.dataFor(opened)
	opened.editor.Items = resolved.items
	if opened.editor.Brushes != resolved.brushes {
		opened.editor.SetBrushes(resolved.brushes)
	}
	change := ms.change(opened)
	change.All = beforeMode != resolved.items.Mode
	change.Count = count
	if len(change.Chunks) > 0 || change.All {
		ms.session.notify(EventMapChanged, MapChanged{ID: id, Chunks: change.Chunks, All: change.All})
	}
	return change, err
}

func (ms *MapService) editMap(id int, fn func(editor *mapedit.Editor) error) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) { return 0, fn(editor) })
}

package app

import (
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
)

// ConvertMapToClientIDs converts the loaded working map, including unsaved
// edits. Saving remains an explicit action; Undo restores server IDs.
func (ms *MapService) ConvertMapToClientIDs(id int) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, data *mapData) (int, error) {
		target := data.asClientIDs(ms.store.Dir())
		count, err := editor.ConvertToClientIDs(target.items, target.brushes)
		if err == nil {
			ms.maps[id].clientData = target
		}
		return count, err
	})
}

type MapViewData struct {
	Data    DataInfo  `json:"data"`
	Items   ItemTable `json:"items"`
	Palette Palette   `json:"palette"`
}

// ViewData selects the tab's item data for the palette and material tools.
// Maps in other tabs continue to use their own item mode.
func (ms *MapService) ViewData(id int, activate bool) (view MapViewData, err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	data, err := ms.ensure("")
	if err != nil {
		return view, err
	}
	if id != 0 {
		opened := ms.maps[id]
		if opened == nil {
			return view, ErrNoMap
		}
		data = ms.dataFor(opened)
	}
	if activate {
		ms.activeID = id
	}
	return MapViewData{Data: data.info(), Items: itemTable(data.items), Palette: data.palette()}, nil
}

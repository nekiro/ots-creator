package app

import (
	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
)

// ConvertMapToClientIDs converts the loaded working map, including unsaved
// edits, from server ids to client ids. Saving remains an explicit action;
// Undo restores server IDs.
func (ms *MapService) ConvertMapToClientIDs(id int) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) {
		target, err := ms.dataOf(otitems.ModeClient)
		if err != nil {
			return 0, err
		}
		return editor.ConvertToClientIDs(target.items, target.brushes)
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
	mode := ms.serverMode()
	if id != 0 {
		opened := ms.maps[id]
		if opened == nil {
			return view, ErrNoMap
		}
		mode = opened.editor.Items.Mode
	}
	data, err := ms.dataOf(mode)
	if err != nil {
		return view, err
	}
	if activate {
		ms.activeID = id
	}
	return MapViewData{Data: data.info(), Items: itemTable(data.items), Palette: data.palette()}, nil
}

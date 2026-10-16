package app

import (
	"slices"

	"github.com/nekiro/ots-creator/internal/mapedit"
	"github.com/nekiro/ots-creator/internal/otmap"
)

// MapInfo describes an open map.
type MapInfo struct {
	ID        int    `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Changed   bool   `json:"changed"`
	CanUndo   bool   `json:"canUndo"`
	CanRedo   bool   `json:"canRedo"`
	UndoLabel string `json:"undoLabel"`
	RedoLabel string `json:"redoLabel"`
	Tiles     int    `json:"tiles"`
	// Floors has a bit set for every floor with tiles.
	Floors    uint16              `json:"floors"`
	Props     mapedit.Properties  `json:"props"`
	Selection int                 `json:"selection"`
	SelMin    otmap.Position      `json:"selMin"`
	SelMax    otmap.Position      `json:"selMax"`
	Rev       uint64              `json:"rev"`
	Bounds    [4]int              `json:"bounds"`
	HasBounds bool                `json:"hasBounds"`
	Towns     []*otmap.Town       `json:"towns"`
	Houses    []mapedit.HouseInfo `json:"houses"`
	Waypoints []*otmap.Waypoint   `json:"waypoints"`
	Zones     []otmap.Zone        `json:"zones"`
}

// info describes a map; meta adds bounds, towns, houses, waypoints and
// zones.
func (ms *MapService) info(m *openMap, meta bool) MapInfo {
	editor := m.editor
	undo, redo := editor.History()
	info := MapInfo{
		ID: m.id, Path: m.path, Name: m.name(), Changed: editor.Changed(), CanUndo: editor.CanUndo(), CanRedo: editor.CanRedo(),
		Tiles: editor.Map.TileCount(), Props: editor.Properties(), Selection: editor.SelectionCount(), Rev: editor.Map.Rev,
	}
	if len(undo) > 0 {
		info.UndoLabel = undo[len(undo)-1].Label
	}
	if len(redo) > 0 {
		info.RedoLabel = redo[0].Label
	}
	for floor, tiles := range editor.Map.FloorTiles() {
		if tiles > 0 {
			info.Floors |= 1 << floor
		}
	}
	info.SelMin, info.SelMax, _ = editor.SelectionBounds()
	if meta {
		minX, minY, maxX, maxY, ok := editor.Map.Bounds()
		info.Bounds, info.HasBounds = [4]int{minX, minY, maxX, maxY}, ok
		info.Towns = editor.Map.SortedTowns()
		info.Houses = editor.Houses()
		info.Waypoints = slices.Clone(editor.Map.Waypoints)
		info.Zones = slices.Clone(editor.Map.Zones)
	}
	return info
}

// MapChange is the result of an edit: the chunks to redraw and the map
// state. Meta is set when towns, houses, waypoints or zones changed (Info
// then lists them).
type MapChange struct {
	Chunks []uint32 `json:"chunks"`
	All    bool     `json:"all"`
	Meta   bool     `json:"meta"`
	Info   MapInfo  `json:"info"`
	// Count is the number of affected tiles or items (tools).
	Count    int      `json:"count"`
	Warnings []string `json:"warnings"`
}

func (ms *MapService) change(m *openMap) MapChange {
	chunks, meta := m.editor.TakeDirty()
	keys := make([]uint32, len(chunks))
	for i, key := range chunks {
		keys[i] = uint32(key)
	}
	return MapChange{Chunks: keys, Meta: meta, Info: ms.info(m, meta)}
}

// Info returns the state of a map (with towns, houses and waypoints).
func (ms *MapService) Info(id int) (info MapInfo, err error) {
	err = ms.withMap(id, func(m *openMap, _ *mapData) error {
		info = ms.info(m, true)
		return nil
	})
	return info, err
}

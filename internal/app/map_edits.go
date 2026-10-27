package app

import (
	"strings"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func (ms *MapService) Undo(id int) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.Undo(); return nil })
}

func (ms *MapService) Redo(id int) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.Redo(); return nil })
}

func (ms *MapService) History(id int) (steps map[string][]mapedit.ActionInfo, err error) {
	err = ms.withMap(id, func(opened *openMap, _ *mapData) error {
		undo, redo := opened.editor.History()
		steps = map[string][]mapedit.ActionInfo{"undo": undo, "redo": redo}
		return nil
	})
	return steps, err
}

func (ms *MapService) Draw(id int, request mapedit.DrawRequest) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, data *mapData) (int, error) {
		if request.Kind == mapedit.KindCreature && !request.Erase {
			if creature := data.creatures[strings.ToLower(request.Creature)]; creature != nil {
				request.NPC = creature.NPC
			}
		}
		return editor.Draw(request)
	})
}

func (ms *MapService) DoodadPreview(id int, name string, variation, size int, shape string, thickness float64) (preview *mapedit.DoodadBuffer, err error) {
	err = ms.withMap(id, func(opened *openMap, _ *mapData) error {
		preview = opened.editor.GenerateDoodad(name, variation, size, shape, thickness)
		return nil
	})
	return preview, err
}

func (ms *MapService) UpdateItem(id int, ref mapedit.ItemRef, patch mapedit.ItemPatch) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.UpdateItem(ref, patch) })
}

func (ms *MapService) RemoveItem(id int, ref mapedit.ItemRef) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.RemoveItem(ref) })
}

func (ms *MapService) AddItem(id int, pos otmap.Position, item uint16) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.AddItem(pos, item) })
}

func (ms *MapService) AddToContainer(id int, ref mapedit.ItemRef, item uint16) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.AddToContainer(ref, item) })
}

func (ms *MapService) MoveItem(id int, pos otmap.Position, from, to int) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.MoveItem(pos, from, to) })
}

func (ms *MapService) RotateItem(id int, ref mapedit.ItemRef) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.RotateItem(ref) })
}

func (ms *MapService) SwitchDoor(id int, pos otmap.Position, index int) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.SwitchDoor(pos, index) })
}

func (ms *MapService) UpdateTile(id int, pos otmap.Position, patch mapedit.TilePatch) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.UpdateTile(pos, patch); return nil })
}

func (ms *MapService) UpdateCreature(id int, pos otmap.Position, index int, patch mapedit.CreaturePatch) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.UpdateCreature(pos, index, patch) })
}

func (ms *MapService) UpdateSpawn(id int, pos otmap.Position, npc bool, radius int32) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.UpdateSpawn(pos, npc, radius) })
}

func (ms *MapService) SetProperties(id int, props mapedit.Properties) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.SetProperties(props) })
}

// SaveHouse creates (ID 0) or updates a house; Count is its id.
func (ms *MapService) SaveHouse(id int, house otmap.House) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) {
		houseID, err := editor.SaveHouse(house)
		return int(houseID), err
	})
}

func (ms *MapService) RemoveHouse(id int, house uint32) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.RemoveHouse(house); return nil })
}

func (ms *MapService) HouseTiles(id int, house uint32) (tiles []otmap.Position, err error) {
	err = ms.withMap(id, func(opened *openMap, _ *mapData) error {
		tiles = opened.editor.HouseTiles(house)
		return nil
	})
	return tiles, err
}

// SaveTown creates (ID 0) or updates a town; Count is its id.
func (ms *MapService) SaveTown(id int, town otmap.Town) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) {
		townID, err := editor.SaveTown(town)
		return int(townID), err
	})
}

func (ms *MapService) RemoveTown(id int, town uint32) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.RemoveTown(town); return nil })
}

// SaveWaypoint creates or changes a waypoint (oldName "" creates).
func (ms *MapService) SaveWaypoint(id int, oldName string, waypoint otmap.Waypoint) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.SaveWaypoint(oldName, waypoint) })
}

func (ms *MapService) RemoveWaypoint(id int, name string) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.RemoveWaypoint(name); return nil })
}

// SaveZone creates (ID 0) or renames a zone; Count is its id.
func (ms *MapService) SaveZone(id int, zone otmap.Zone) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) {
		zoneID, err := editor.SaveZone(zone)
		return int(zoneID), err
	})
}

func (ms *MapService) RemoveZone(id int, zone uint16) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.RemoveZone(zone); return nil })
}

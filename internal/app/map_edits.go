package app

import (
	"strings"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Undo reverts the last action.
func (ms *MapService) Undo(id int) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.Undo(); return nil })
}

// Redo applies the last undone action.
func (ms *MapService) Redo(id int) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.Redo(); return nil })
}

// History lists the undo and redo steps.
func (ms *MapService) History(id int) (steps map[string][]mapedit.ActionInfo, err error) {
	err = ms.withMap(id, func(opened *openMap, _ *mapData) error {
		undo, redo := opened.editor.History()
		steps = map[string][]mapedit.ActionInfo{"undo": undo, "redo": redo}
		return nil
	})
	return steps, err
}

// Draw applies a brush.
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

// DoodadPreview generates the next placement of a doodad brush.
func (ms *MapService) DoodadPreview(id int, name string, variation, size int, shape string, thickness float64) (preview *mapedit.DoodadBuffer, err error) {
	err = ms.withMap(id, func(opened *openMap, _ *mapData) error {
		preview = opened.editor.GenerateDoodad(name, variation, size, shape, thickness)
		return nil
	})
	return preview, err
}

// UpdateItem changes an item's attributes.
func (ms *MapService) UpdateItem(id int, ref mapedit.ItemRef, patch mapedit.ItemPatch) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.UpdateItem(ref, patch) })
}

// RemoveItem deletes an item.
func (ms *MapService) RemoveItem(id int, ref mapedit.ItemRef) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.RemoveItem(ref) })
}

// AddItem puts an item on a tile.
func (ms *MapService) AddItem(id int, pos otmap.Position, item uint16) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.AddItem(pos, item) })
}

// AddToContainer puts an item into a container.
func (ms *MapService) AddToContainer(id int, ref mapedit.ItemRef, item uint16) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.AddToContainer(ref, item) })
}

// MoveItem reorders an item of a tile.
func (ms *MapService) MoveItem(id int, pos otmap.Position, from, to int) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.MoveItem(pos, from, to) })
}

// RotateItem rotates an item.
func (ms *MapService) RotateItem(id int, ref mapedit.ItemRef) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.RotateItem(ref) })
}

// SwitchDoor opens or closes a door.
func (ms *MapService) SwitchDoor(id int, pos otmap.Position, index int) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.SwitchDoor(pos, index) })
}

// UpdateTile changes tile flags, house or zones.
func (ms *MapService) UpdateTile(id int, pos otmap.Position, patch mapedit.TilePatch) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.UpdateTile(pos, patch); return nil })
}

// UpdateCreature changes or removes a creature.
func (ms *MapService) UpdateCreature(id int, pos otmap.Position, index int, patch mapedit.CreaturePatch) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.UpdateCreature(pos, index, patch) })
}

// UpdateSpawn changes the radius of a spawn (0 removes it).
func (ms *MapService) UpdateSpawn(id int, pos otmap.Position, npc bool, radius int32) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.UpdateSpawn(pos, npc, radius) })
}

// SetProperties changes the map header.
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

// RemoveHouse deletes a house.
func (ms *MapService) RemoveHouse(id int, house uint32) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.RemoveHouse(house); return nil })
}

// HouseTiles returns the tiles of a house.
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

// RemoveTown deletes a town.
func (ms *MapService) RemoveTown(id int, town uint32) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.RemoveTown(town); return nil })
}

// SaveWaypoint creates or changes a waypoint (oldName "" creates).
func (ms *MapService) SaveWaypoint(id int, oldName string, waypoint otmap.Waypoint) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.SaveWaypoint(oldName, waypoint) })
}

// RemoveWaypoint deletes a waypoint.
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

// RemoveZone deletes a zone.
func (ms *MapService) RemoveZone(id int, zone uint16) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.RemoveZone(zone); return nil })
}

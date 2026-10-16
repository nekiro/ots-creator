package mapedit

import (
	"slices"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// topOrder returns the stack order of an item: 1 borders, 2 bottom items,
// 3 top items, 0 common items. Border items of materials count as 1.
func (e *Editor) topOrder(id uint16) uint8 {
	definition := e.Items.Get(id)
	var order uint8
	if definition != nil {
		order = definition.TopOrder
	}
	if order == commonTopOrder && e.Brushes.isBorder(id) {
		order = borderTopOrder
	}
	return order
}

func (e *Editor) isGround(id uint16) bool {
	definition := e.Items.Get(id)
	return definition != nil && definition.IsGround()
}

func (e *Editor) itemType(id uint16) *otitems.Type { return e.Items.Get(id) }

// addItem puts an item on a tile like the server stacks it: ground replaces
// the ground, bottom items go below common items ordered by their top
// order, common items go on top.
func (e *Editor) addItem(tile *otmap.Tile, item otmap.Item) {
	if e.isGround(item.ID) {
		tile.Ground = item
		return
	}
	if info := e.Brushes.info[item.ID]; info != nil && info.groundEquivalent != 0 {
		tile.Ground = otmap.Item{ID: info.groundEquivalent}
		tile.Items = slices.Insert(tile.Items, 0, item)
		return
	}
	order := e.topOrder(item.ID)
	if order == commonTopOrder {
		tile.Items = append(tile.Items, item)
		return
	}
	i := 0
	for ; i < len(tile.Items); i++ {
		stackedOrder := e.topOrder(tile.Items[i].ID)
		if stackedOrder == commonTopOrder || order < stackedOrder {
			break
		}
	}
	tile.Items = slices.Insert(tile.Items, i, item)
}

// addBorderItem puts a border item at the bottom of the stack.
func addBorderItem(tile *otmap.Tile, id uint16) {
	tile.Items = slices.Insert(tile.Items, 0, otmap.Item{ID: id})
}

// cleanBorders removes the border items at the bottom of the stack.
func (e *Editor) cleanBorders(tile *otmap.Tile) {
	borderCount := 0
	for borderCount < len(tile.Items) && e.Brushes.isBorder(tile.Items[borderCount].ID) {
		borderCount++
	}
	if borderCount > 0 {
		tile.Items = slices.Delete(tile.Items, 0, borderCount)
	}
}

func (e *Editor) groundBrushOf(tile *otmap.Tile) *GroundBrush {
	if tile == nil || tile.Ground.ID == 0 {
		return nil
	}
	return e.Brushes.GroundOf(tile.Ground.ID)
}

func (e *Editor) hasOptionalBorder(tile *otmap.Tile) bool {
	if tile.State&otmap.StateOptionalBorder != 0 {
		return true
	}
	for _, item := range tile.Items {
		if info := e.Brushes.info[item.ID]; info != nil && info.optionalBorder {
			return true
		}
	}
	return false
}

// IsBlocking reports whether a tile blocks walking.
func (e *Editor) IsBlocking(tile *otmap.Tile) bool { return e.blocking(tile) }

// blocking reports whether a tile blocks walking (no ground counts).
func (e *Editor) blocking(tile *otmap.Tile) bool {
	if tile == nil || tile.Ground.ID == 0 {
		return true
	}
	return e.hasFlag(tile, otitems.FlagUnpassable)
}

// hasFlag reports whether the ground or an item of a tile has flag.
func (e *Editor) hasFlag(tile *otmap.Tile, flag otitems.Flags) bool {
	if tile == nil {
		return false
	}
	if definition := e.itemType(tile.Ground.ID); definition != nil && definition.Flags.Has(flag) {
		return true
	}
	for _, item := range tile.Items {
		if definition := e.itemType(item.ID); definition != nil && definition.Flags.Has(flag) {
			return true
		}
	}
	return false
}

func (e *Editor) hasWall(tile *otmap.Tile) bool {
	for _, item := range tile.Items {
		if wall, _ := e.Brushes.wallOf(item.ID); wall != nil {
			return true
		}
	}
	return false
}

func (e *Editor) hasCarpet(tile *otmap.Tile) bool { return e.hasJoined(tile, &carpetKind) }

func (e *Editor) hasTable(tile *otmap.Tile) bool { return e.hasJoined(tile, &tableKind) }

// posAt is a shorthand for building positions from ints (callers check range).
func posAt(x, y, z int) otmap.Position {
	return otmap.Position{X: uint16(x), Y: uint16(y), Z: uint8(z)}
}

func inRange(x, y int) bool { return x >= 0 && y >= 0 && x <= 0xFFFF && y <= 0xFFFF }

// newItem creates an item, giving stackables a count of one.
func (e *Editor) newItem(id uint16) otmap.Item {
	item := otmap.Item{ID: id}
	if definition := e.itemType(id); definition != nil && definition.Flags.Has(otitems.FlagStackable) {
		item.Subtype = 1
	}
	return item
}

package mapedit

import (
	"slices"

	"github.com/nekiro/ots-creator/internal/otmap"
)

// wallShape returns the wall alignment for the neighbors that hold a
// matching wall. The full shape set is pole, ends, straight pieces,
// corners, T pieces and the intersection; their numbers equal the mask.
func wallShape(mask int, full bool) int {
	if full {
		return mask
	}
	// Reduced set (only straight pieces, the corner and the pole): only
	// the north and west neighbors decide.
	switch mask & (wallN | wallW) {
	case wallN:
		return wallVertical
	case wallW:
		return wallHorizontal
	case wallN | wallW:
		return wallNWDiagonal
	}
	return wallPole
}

// matchingWall reports whether the tile holds a wall that connects to wall.
func (e *Editor) matchingWall(tile *otmap.Tile, wall *WallBrush) bool {
	if tile == nil {
		return false
	}
	for _, item := range tile.Items {
		wallBrush, info := e.Brushes.wallOf(item.ID)
		if wallBrush == nil || wallBrush.decoration {
			continue
		}
		if wallBrush == wall || wall.friendOf(wallBrush) || wallBrush.friendOf(wall) {
			return !info.wallHate
		}
	}
	return false
}

// wallNeighborMask returns the wall neighbor bits of the tile at x, y, z
// whose tiles hold a wall that connects to wall.
func (e *Editor) wallNeighborMask(x, y, z int, wall *WallBrush) int {
	mask := 0
	for _, neighbor := range wallNeighborOffsets {
		if e.matchingWall(e.Map.At(x+neighbor.dx, y+neighbor.dy, z), wall) {
			mask |= neighbor.bit
		}
	}
	return mask
}

// wallize reshapes the walls of the tile at pos to match their neighbors.
func (e *Editor) wallize(tx *Tx, pos otmap.Position) {
	current := tx.Get(pos)
	if current == nil || !e.hasWall(current) {
		return
	}
	tile := tx.Edit(pos)
	x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
	var reshaped []otmap.Item
	items := tile.Items
	for i := 0; i < len(items); i++ {
		item := items[i]
		wallBrush, info := e.Brushes.wallOf(item.ID)
		if wallBrush == nil {
			reshaped = append(reshaped, item)
			continue
		}
		if wallBrush.decoration {
			// Decorations follow the wall below them (handled there).
			reshaped = append(reshaped, item)
			continue
		}
		mask := e.wallNeighborMask(x, y, z, wallBrush)
		if info.wallAlign == wallUntouchable {
			reshaped = append(reshaped, item)
			continue
		}
		newAlign, newID := e.fitWall(item.ID, wallBrush, info, mask)
		if newAlign < 0 {
			reshaped = append(reshaped, item)
			continue
		}
		item.ID = newID
		reshaped = append(reshaped, item)
		// Decorations right above the wall take the same shape.
		for i+1 < len(items) {
			decoration, decorationInfo := e.Brushes.wallOf(items[i+1].ID)
			if decoration == nil || !decoration.decoration {
				break
			}
			i++
			reshaped = append(reshaped, e.fitDecoration(items[i], decoration, decorationInfo, newAlign))
		}
	}
	tile.Items = reshaped
}

// fitWall returns the alignment and item of a wall for the neighbor mask,
// trying the full shape set before the reduced one. The alignment is -1
// when the brush has no fitting piece.
func (e *Editor) fitWall(id uint16, wall *WallBrush, info *itemInfo, mask int) (newAlign int, newID uint16) {
	for _, full := range []bool{true, false} {
		shape := wallShape(mask, full)
		if info.wallAlign == shape {
			return shape, id
		}
		if info.door {
			if doorID := wall.doorFor(shape, info.doorType, info.doorOpen); doorID != 0 {
				return shape, doorID
			}
			continue
		}
		if pickedID := wall.pick(shape, e.rng); pickedID != 0 {
			return shape, pickedID
		}
	}
	return -1, 0
}

// fitDecoration returns the decoration item reshaped to the alignment of
// the wall below it.
func (e *Editor) fitDecoration(item otmap.Item, decoration *WallBrush, info *itemInfo, align int) otmap.Item {
	if info.wallAlign != align {
		if info.door {
			if id := decoration.doorFor(align, info.doorType, info.doorOpen); id != 0 {
				item.ID = id
			}
		} else if id := decoration.pick(align, e.rng); id != 0 {
			item.ID = id
		}
	}
	return item
}

// doorFor returns a door of a type and state for an alignment, falling back
// to the other state.
func (w *WallBrush) doorFor(align, doorType int, open bool) uint16 {
	var fallback uint16
	for brush, depth := w, 0; brush != nil && depth < maxWallRedirects; brush, depth = brush.redirect, depth+1 {
		for _, door := range brush.nodes[align].doors {
			if door.doorType != doorType {
				continue
			}
			if door.open == open {
				return door.id
			}
			if fallback == 0 {
				fallback = door.id
			}
		}
	}
	return fallback
}

// cleanWalls removes the walls of a brush (all walls when wall is nil).
func (e *Editor) cleanWalls(tile *otmap.Tile, wall *WallBrush) {
	tile.Items = slices.DeleteFunc(tile.Items, func(item otmap.Item) bool {
		wallBrush, _ := e.Brushes.wallOf(item.ID)
		if wallBrush == nil {
			return false
		}
		if wall == nil {
			return true
		}
		_, info := e.Brushes.wallOf(item.ID)
		return wall.hasWall(item.ID, info.wallAlign) || wallBrush == wall
	})
}

// drawWall places a wall of the brush; the alignment is fixed by wallize.
func (e *Editor) drawWall(tile *otmap.Tile, wall *WallBrush) {
	if wall.decoration {
		e.drawDecoration(tile, wall)
		return
	}
	e.cleanWalls(tile, wall)
	for align := 0; align < wallAlignmentMax; align++ {
		if id := wall.pick(align, e.rng); id != 0 {
			e.addItem(tile, otmap.Item{ID: id})
			return
		}
	}
}

// cycleWall changes a wall of the brush to the next alignment (alt-click).
func (e *Editor) cycleWall(tile *otmap.Tile, wall *WallBrush) bool {
	for i := range tile.Items {
		wallBrush, info := e.Brushes.wallOf(tile.Items[i].ID)
		if wallBrush != wall {
			continue
		}
		for step := 1; step < wallAlignmentMax; step++ {
			align := (info.wallAlign + step) % wallAlignmentMax
			if id := wall.pick(align, e.rng); id != 0 {
				tile.Items[i].ID = id
				return true
			}
		}
		return false
	}
	return false
}

// drawDecoration puts a decoration of the brush on every wall of the tile.
func (e *Editor) drawDecoration(tile *otmap.Tile, decoration *WallBrush) {
	e.cleanWalls(tile, decoration)
	var decorated []otmap.Item
	for _, item := range tile.Items {
		decorated = append(decorated, item)
		wallBrush, info := e.Brushes.wallOf(item.ID)
		if wallBrush == nil || wallBrush.decoration {
			continue
		}
		var id uint16
		if info.door {
			id = decoration.doorFor(info.wallAlign, info.doorType, info.doorOpen)
		} else {
			id = decoration.nodes[info.wallAlign].items.pick(e.rng)
		}
		if id != 0 {
			decorated = append(decorated, otmap.Item{ID: id})
		}
	}
	tile.Items = decorated
}

// drawDoor turns the wall of the tile into a door of a type.
func (e *Editor) drawDoor(tx *Tx, pos otmap.Position, doorType int, open bool) bool {
	current := tx.Get(pos)
	if current == nil {
		return false
	}
	for i, item := range current.Items {
		wallBrush, info := e.Brushes.wallOf(item.ID)
		if wallBrush == nil || wallBrush.decoration {
			continue
		}
		state := open
		if info.door {
			state = info.doorOpen
		}
		id := wallBrush.doorFor(info.wallAlign, doorType, state)
		if id == 0 {
			continue
		}
		tile := tx.Edit(pos)
		tile.Items[i].ID = id
		// Decorations above follow.
		for j := i + 1; j < len(tile.Items); j++ {
			decoration, decorationInfo := e.Brushes.wallOf(tile.Items[j].ID)
			if decoration == nil || !decoration.decoration {
				break
			}
			if decorationID := decoration.doorFor(decorationInfo.wallAlign, doorType, state); decorationID != 0 {
				tile.Items[j].ID = decorationID
			}
		}
		if e.Settings.AutoDoorID && tile.HouseID != 0 {
			e.assignDoorID(tx, tile, i)
		}
		return true
	}
	return false
}

// removeDoor turns doors of the tile back into walls.
func (e *Editor) removeDoor(tx *Tx, pos otmap.Position) bool {
	current := tx.Get(pos)
	if current == nil {
		return false
	}
	for i, item := range current.Items {
		wallBrush, info := e.Brushes.wallOf(item.ID)
		if wallBrush == nil || !info.door {
			continue
		}
		tile := tx.Edit(pos)
		if id := wallBrush.pick(info.wallAlign, e.rng); id != 0 {
			tile.Items[i].ID = id
			if tile.Items[i].Ext != nil {
				tile.Items[i].Ext.DoorID = 0
				tile.Items[i].Compact()
			}
		}
		return true
	}
	return false
}

// SwitchDoor opens a closed door or closes an open one.
func (e *Editor) switchDoor(tile *otmap.Tile, i int) bool {
	wallBrush, info := e.Brushes.wallOf(tile.Items[i].ID)
	if wallBrush == nil || !info.door {
		return false
	}
	for _, door := range wallBrush.nodes[info.wallAlign].doors {
		if door.doorType == info.doorType && door.open != info.doorOpen {
			tile.Items[i].ID = door.id
			return true
		}
	}
	return false
}

// carpetPiece returns the carpet piece for neighbors holding the same
// carpet. Pieces are named by the sides the carpet continues to: three
// sides give the piece of the middle one, two adjacent sides a corner, all
// sides the center, or an inner diagonal when one corner is missing.
func carpetPiece(mask int) int {
	north, east, south, west := mask&bitN != 0, mask&bitE != 0, mask&bitS != 0, mask&bitW != 0
	switch {
	case north && east && south && west:
		missing := -1
		count := 0
		for _, corner := range []struct {
			bit, piece int
		}{{bitNW, edgeDSE}, {bitNE, edgeDSW}, {bitSW, edgeDNE}, {bitSE, edgeDNW}} {
			if mask&corner.bit == 0 {
				missing = corner.piece
				count++
			}
		}
		if count == 1 {
			return missing
		}
		return edgeCenter
	case north && east && west && !south:
		return edgeN
	case east && south && west && !north:
		return edgeS
	case north && south && west && !east:
		return edgeW
	case north && east && south && !west:
		return edgeE
	case north && west && !east && !south:
		return edgeCNW
	case north && east && !west && !south:
		return edgeCNE
	case south && west && !north && !east:
		return edgeCSW
	case south && east && !north && !west:
		return edgeCSE
	}
	return edgeCenter
}

// tablePiece returns the table piece: horizontal tables win over vertical
// ones.
func tablePiece(mask int) int {
	north, east, south, west := mask&bitN != 0, mask&bitE != 0, mask&bitS != 0, mask&bitW != 0
	switch {
	case east && west:
		return tableHorizontal
	case east:
		return tableWest
	case west:
		return tableEast
	case north && south:
		return tableVertical
	case south:
		return tableNorth
	case north:
		return tableSouth
	}
	return tableAlone
}

func (e *Editor) neighborMask(pos otmap.Position, has func(*otmap.Tile) bool) int {
	mask := 0
	x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
	for i, offset := range neighbors8 {
		if has(e.Map.At(x+offset[0], y+offset[1], z)) {
			mask |= 1 << i
		}
	}
	return mask
}

func tileHas(tile *otmap.Tile, fn func(id uint16) bool) bool {
	if tile == nil {
		return false
	}
	for _, item := range tile.Items {
		if fn(item.ID) {
			return true
		}
	}
	return false
}

// carpetize reshapes the carpets of the tile at pos.
func (e *Editor) carpetize(tx *Tx, pos otmap.Position) {
	current := tx.Get(pos)
	if current == nil || !e.hasCarpet(current) {
		return
	}
	tile := tx.Edit(pos)
	for i, item := range tile.Items {
		info := e.Brushes.info[item.ID]
		if info == nil || info.carpet == nil {
			continue
		}
		carpet := info.carpet
		mask := e.neighborMask(pos, func(neighbor *otmap.Tile) bool {
			return tileHas(neighbor, func(id uint16) bool {
				neighborInfo := e.Brushes.info[id]
				return neighborInfo != nil && neighborInfo.carpet == carpet
			})
		})
		piece := carpetPiece(mask)
		if piece == info.carpetAlign {
			continue
		}
		if id := carpet.parts[piece].pick(e.rng); id != 0 {
			tile.Items[i].ID = id
		} else if id := carpet.parts[edgeCenter].pick(e.rng); id != 0 {
			tile.Items[i].ID = id
		}
	}
}

// tableize reshapes the tables of the tile at pos.
func (e *Editor) tableize(tx *Tx, pos otmap.Position) {
	current := tx.Get(pos)
	if current == nil || !e.hasTable(current) {
		return
	}
	tile := tx.Edit(pos)
	for i, item := range tile.Items {
		info := e.Brushes.info[item.ID]
		if info == nil || info.table == nil {
			continue
		}
		table := info.table
		mask := e.neighborMask(pos, func(neighbor *otmap.Tile) bool {
			return tileHas(neighbor, func(id uint16) bool {
				neighborInfo := e.Brushes.info[id]
				return neighborInfo != nil && neighborInfo.table == table
			})
		})
		piece := tablePiece(mask)
		if piece == info.tableAlign {
			continue
		}
		if id := table.parts[piece].pick(e.rng); id != 0 {
			tile.Items[i].ID = id
		} else if id := table.parts[tableAlone].pick(e.rng); id != 0 {
			tile.Items[i].ID = id
		}
	}
}

func (e *Editor) drawCarpet(tile *otmap.Tile, carpet *CarpetBrush) {
	e.removeCarpets(tile)
	id := carpet.parts[edgeCenter].pick(e.rng)
	if id == 0 {
		for _, part := range carpet.parts {
			if id = part.pick(e.rng); id != 0 {
				break
			}
		}
	}
	if id != 0 {
		e.addItem(tile, otmap.Item{ID: id})
	}
}

func (e *Editor) removeCarpets(tile *otmap.Tile) {
	tile.Items = slices.DeleteFunc(tile.Items, func(item otmap.Item) bool {
		info := e.Brushes.info[item.ID]
		return info != nil && info.carpet != nil
	})
}

func (e *Editor) drawTable(tile *otmap.Tile, table *TableBrush) {
	e.removeTables(tile)
	id := table.parts[tableAlone].pick(e.rng)
	if id == 0 {
		for _, part := range table.parts {
			if id = part.pick(e.rng); id != 0 {
				break
			}
		}
	}
	if id != 0 {
		e.addItem(tile, otmap.Item{ID: id})
	}
}

func (e *Editor) removeTables(tile *otmap.Tile) {
	tile.Items = slices.DeleteFunc(tile.Items, func(item otmap.Item) bool {
		info := e.Brushes.info[item.ID]
		return info != nil && info.table != nil
	})
}

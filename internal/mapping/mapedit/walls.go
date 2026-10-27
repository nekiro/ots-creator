package mapedit

import (
	"slices"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
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

func (e *Editor) wallNeighborMask(x, y, z int, wall *WallBrush) int {
	return e.neighborMask(x, y, z, wallNeighborOffsets[:], func(neighbor *otmap.Tile) bool { return e.matchingWall(neighbor, wall) })
}

func (e *Editor) neighborMask(x, y, z int, neighbors []neighborOffset, has func(*otmap.Tile) bool) int {
	mask := 0
	for _, neighbor := range neighbors {
		if has(e.peek(x+neighbor.dx, y+neighbor.dy, z)) {
			mask |= neighbor.bit
		}
	}
	return mask
}

func (e *Editor) fitWalls(tx *Tx, pos otmap.Position) {
	current := tx.Get(pos)
	if current == nil || !e.hasWall(current) {
		return
	}
	x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
	var changes []pieceChange
	items := current.Items
	for i := 0; i < len(items); i++ {
		wallBrush, info := e.Brushes.wallOf(items[i].ID)
		// Decorations follow the wall below them (handled there).
		if wallBrush == nil || wallBrush.decoration || info.wallAlign == wallUntouchable {
			continue
		}
		mask := e.wallNeighborMask(x, y, z, wallBrush)
		newAlign, newID := e.fitWall(items[i].ID, wallBrush, info, mask)
		if newAlign < 0 {
			continue
		}
		changes = appendPieceChange(changes, items, i, newID)
		// Decorations right above the wall take the same shape.
		for i+1 < len(items) {
			decoration, decorationInfo := e.Brushes.wallOf(items[i+1].ID)
			if decoration == nil || !decoration.decoration {
				break
			}
			i++
			changes = appendPieceChange(changes, items, i, e.fitDecoration(items[i], decoration, decorationInfo, newAlign).ID)
		}
	}
	applyPieceChanges(tx, pos, changes)
}

type pieceChange struct {
	index int
	id    uint16
}

func appendPieceChange(changes []pieceChange, items []otmap.Item, index int, id uint16) []pieceChange {
	if items[index].ID == id {
		return changes
	}
	return append(changes, pieceChange{index, id})
}

// applyPieceChanges edits the tile at pos only when a piece changed, so an
// unchanged tile stays out of the undo history and is not marked modified.
func applyPieceChanges(tx *Tx, pos otmap.Position, changes []pieceChange) {
	if len(changes) == 0 {
		return
	}
	tile := tx.Edit(pos)
	for _, change := range changes {
		tile.Items[change.index].ID = change.id
	}
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

func (e *Editor) cleanWalls(tile *otmap.Tile, wall *WallBrush) {
	tile.Items = slices.DeleteFunc(tile.Items, func(item otmap.Item) bool {
		wallBrush, info := e.Brushes.wallOf(item.ID)
		if wallBrush == nil {
			return false
		}
		if wall == nil {
			return true
		}
		return wall.hasWall(item.ID, info.wallAlign) || wallBrush == wall
	})
}

// drawWall places a wall of the brush; the alignment is fixed by fitWalls.
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

// joinedBrush is a carpet or table brush: its items join with neighbors
// that hold the same brush.
type joinedBrush interface {
	pieces() []chanceList
}

func (c *CarpetBrush) pieces() []chanceList { return c.parts[:] }
func (t *TableBrush) pieces() []chanceList  { return t.parts[:] }

type joinedKind struct {
	brushOf func(info *itemInfo) (joinedBrush, int)
	piece   func(mask int) int
	// fallback is the piece drawn first and used when the brush lacks the
	// piece a shape wants.
	fallback int
}

var carpetKind = joinedKind{
	brushOf: func(info *itemInfo) (joinedBrush, int) {
		if info == nil || info.carpet == nil {
			return nil, 0
		}
		return info.carpet, info.carpetAlign
	},
	piece:    carpetPiece,
	fallback: edgeCenter,
}

var tableKind = joinedKind{
	brushOf: func(info *itemInfo) (joinedBrush, int) {
		if info == nil || info.table == nil {
			return nil, 0
		}
		return info.table, info.tableAlign
	},
	piece:    tablePiece,
	fallback: tableAlone,
}

func (e *Editor) joinedBrushOf(kind *joinedKind, id uint16) (joinedBrush, int) {
	return kind.brushOf(e.Brushes.info[id])
}

func (e *Editor) hasJoined(tile *otmap.Tile, kind *joinedKind) bool {
	return tileHas(tile, func(id uint16) bool {
		brush, _ := e.joinedBrushOf(kind, id)
		return brush != nil
	})
}

func (e *Editor) fitCarpets(tx *Tx, pos otmap.Position) { e.fitJoined(tx, pos, &carpetKind) }

func (e *Editor) fitTables(tx *Tx, pos otmap.Position) { e.fitJoined(tx, pos, &tableKind) }

func (e *Editor) fitJoined(tx *Tx, pos otmap.Position, kind *joinedKind) {
	current := tx.Get(pos)
	if current == nil || !e.hasJoined(current, kind) {
		return
	}
	x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
	var changes []pieceChange
	for i, item := range current.Items {
		brush, currentPiece := e.joinedBrushOf(kind, item.ID)
		if brush == nil {
			continue
		}
		mask := e.neighborMask(x, y, z, neighborOffsets8[:], func(neighbor *otmap.Tile) bool {
			return tileHas(neighbor, func(id uint16) bool {
				neighborBrush, _ := e.joinedBrushOf(kind, id)
				return neighborBrush == brush
			})
		})
		piece := kind.piece(mask)
		if piece == currentPiece {
			continue
		}
		pieces := brush.pieces()
		if id := pieces[piece].pick(e.rng); id != 0 {
			changes = appendPieceChange(changes, current.Items, i, id)
		} else if id := pieces[kind.fallback].pick(e.rng); id != 0 {
			changes = appendPieceChange(changes, current.Items, i, id)
		}
	}
	applyPieceChanges(tx, pos, changes)
}

// drawJoined replaces the carpets or tables of the tile by a piece of the
// brush; fitJoined shapes it afterwards.
func (e *Editor) drawJoined(tile *otmap.Tile, kind *joinedKind, brush joinedBrush) {
	e.removeJoined(tile, kind)
	pieces := brush.pieces()
	id := pieces[kind.fallback].pick(e.rng)
	if id == 0 {
		for _, part := range pieces {
			if id = part.pick(e.rng); id != 0 {
				break
			}
		}
	}
	if id != 0 {
		e.addItem(tile, otmap.Item{ID: id})
	}
}

func (e *Editor) removeJoined(tile *otmap.Tile, kind *joinedKind) {
	tile.Items = slices.DeleteFunc(tile.Items, func(item otmap.Item) bool {
		brush, _ := e.joinedBrushOf(kind, item.ID)
		return brush != nil
	})
}

package otmap

import "slices"

// Chunk storage. Most tiles of a loaded map are plain: items without
// attributes, no flags, house, creatures or selection. A chunk keeps those
// packed (8 bytes per tile plus 4 per item, all in two slices) and only the
// others as *Tile values. A *Tile costs 64 bytes plus its item slice, and a
// big map has millions of plain tiles.
//
// Packed tiles are read-only: Map.Tile returns a fresh copy of one, and
// Map.Put (the editor's working copies) always stores a *Tile, which
// replaces the packed tile. Map.SetTile packs a tile when it can.

// Chunk is a 32x32 block of one floor.
type Chunk struct {
	X, Y uint16 // chunk coordinates (tile >> ChunkBits)
	Z    uint8
	// Count is the number of tiles.
	Count int
	// Rev is the map revision of the last change in this chunk.
	Rev uint64

	// cells holds per tile: 0 = none, cellPtr|i = ptrs[i], else packed[cell-1].
	cells  [ChunkSize * ChunkSize]uint16
	packed []packedTile
	items  []packedItem
	ptrs   []*Tile
	// freePtrs are unused slots of ptrs; dead counts packed tiles no cell
	// refers to any more (replaced or removed).
	freePtrs []uint16
	dead     int
}

const (
	cellPtr = 0x8000
	// maxPackedItems keeps the item count of a packed tile, the ground
	// included, within its uint8.
	maxPackedItems = 254
	// minDeadToRepack is the number of dead packed tiles below which a
	// chunk is never repacked.
	minDeadToRepack = 32
)

type packedTile struct {
	offset uint32 // first item in Chunk.items
	count  uint8  // items, the ground included
	ground bool   // items[offset] is the ground
}

type packedItem struct {
	id, subtype uint16
}

// packable reports whether a tile can be stored packed.
func packable(tile *Tile) bool {
	if tile.Flags != 0 || tile.HouseID != 0 || tile.State != 0 || tile.Ext != nil || len(tile.Items) > maxPackedItems {
		return false
	}
	if ground := tile.Ground; ground.Ext != nil || ground.Selected {
		return false
	}
	for i := range tile.Items {
		if item := &tile.Items[i]; item.Ext != nil || item.Selected {
			return false
		}
	}
	return tile.Ground.ID != 0 || len(tile.Items) > 0
}

// get returns the tile at local index i: the stored *Tile, or a packed
// tile unpacked into a new Tile.
func (c *Chunk) get(i int) *Tile {
	cell := c.cells[i]
	switch {
	case cell == 0:
		return nil
	case cell&cellPtr != 0:
		return c.ptrs[cell&^cellPtr]
	default:
		return c.unpack(c.packed[cell-1], &Tile{}, nil)
	}
}

// unpack fills tile from a packed tile. Items go into buf when it has room
// (then tile is only valid until buf is reused), else into a new slice.
func (c *Chunk) unpack(stored packedTile, tile *Tile, buf []Item) *Tile {
	src := c.items[stored.offset : stored.offset+uint32(stored.count)]
	*tile = Tile{}
	if stored.ground {
		tile.Ground = Item{ID: src[0].id, Subtype: src[0].subtype}
		src = src[1:]
	}
	if len(src) > 0 {
		var items []Item
		if cap(buf) >= len(src) {
			items = buf[:len(src)]
		} else {
			items = make([]Item, len(src))
		}
		for i, item := range src {
			items[i] = Item{ID: item.id, Subtype: item.subtype}
		}
		tile.Items = items
	}
	return tile
}

// set stores tile at local index i (nil removes) and reports whether a tile
// was there before. Packing is up to the caller (pack).
func (c *Chunk) set(i int, tile *Tile, pack bool) (had bool) {
	cell := c.cells[i]
	had = cell != 0
	// Release what the cell holds.
	switch {
	case cell&cellPtr != 0:
		slot := cell &^ cellPtr
		if tile != nil && !(pack && packable(tile)) {
			c.ptrs[slot] = tile // replace in place
			return had
		}
		c.ptrs[slot] = nil
		c.freePtrs = append(c.freePtrs, slot)
	case cell != 0:
		c.dead++
	}
	c.cells[i] = 0
	if tile == nil {
		c.maybeRepack()
		return had
	}
	if pack && packable(tile) {
		c.cells[i] = c.addPacked(tile)
	} else {
		var slot uint16
		if freeCount := len(c.freePtrs); freeCount > 0 {
			slot = c.freePtrs[freeCount-1]
			c.freePtrs = c.freePtrs[:freeCount-1]
		} else {
			slot = uint16(len(c.ptrs))
			c.ptrs = append(c.ptrs, nil)
		}
		c.ptrs[slot] = tile
		c.cells[i] = cellPtr | slot
	}
	c.maybeRepack()
	return had
}

func (c *Chunk) addPacked(tile *Tile) uint16 {
	stored := packedTile{offset: uint32(len(c.items))}
	if tile.Ground.ID != 0 {
		stored.ground = true
		c.items = append(c.items, packedItem{tile.Ground.ID, tile.Ground.Subtype})
	}
	for _, item := range tile.Items {
		c.items = append(c.items, packedItem{item.ID, item.Subtype})
	}
	stored.count = uint8(len(c.items) - int(stored.offset))
	c.packed = append(c.packed, stored)
	return uint16(len(c.packed))
}

// maybeRepack drops dead packed tiles once they are half of the chunk.
func (c *Chunk) maybeRepack() {
	if c.dead < minDeadToRepack || c.dead*2 < len(c.packed) {
		return
	}
	live := len(c.packed) - c.dead
	packed := make([]packedTile, 0, live)
	items := make([]packedItem, 0, len(c.items))
	for i, cell := range c.cells {
		if cell == 0 || cell&cellPtr != 0 {
			continue
		}
		stored := c.packed[cell-1]
		offset := uint32(len(items))
		items = append(items, c.items[stored.offset:stored.offset+uint32(stored.count)]...)
		stored.offset = offset
		packed = append(packed, stored)
		c.cells[i] = uint16(len(packed))
	}
	c.packed, c.items, c.dead = packed, slices.Clip(items), 0
}

// clip trims the slices of a chunk after loading.
func (c *Chunk) clip() {
	if cap(c.items) > len(c.items) {
		c.items = append([]packedItem(nil), c.items...)
	}
	if cap(c.packed) > len(c.packed) {
		c.packed = append([]packedTile(nil), c.packed...)
	}
}

// eachPtr calls fn for every tile stored as *Tile.
func (c *Chunk) eachPtr(fn func(tile *Tile)) {
	for _, tile := range c.ptrs {
		if tile != nil {
			fn(tile)
		}
	}
}

// Peek returns the tile at local (x, y), nil when there is none. A packed
// tile is unpacked into scratch and buf (see unpack): it is only valid
// until they are reused.
func (c *Chunk) Peek(x, y int, scratch *Tile, buf []Item) *Tile {
	cell := c.cells[y<<ChunkBits|x]
	switch {
	case cell == 0:
		return nil
	case cell&cellPtr != 0:
		return c.ptrs[cell&^cellPtr]
	default:
		return c.unpack(c.packed[cell-1], scratch, buf)
	}
}

// Each calls fn for every tile of the chunk. Packed tiles are passed as a
// reused copy: tile is only valid during the call (Clone it to keep it), and
// changing it does not change the map.
func (c *Chunk) Each(fn func(pos Position, tile *Tile)) {
	var scratch Tile
	var buf [16]Item
	for i := range c.cells {
		cell := c.cells[i]
		if cell == 0 {
			continue
		}
		pos := Position{c.X<<ChunkBits | uint16(i&(ChunkSize-1)), c.Y<<ChunkBits | uint16(i>>ChunkBits), c.Z}
		if cell&cellPtr != 0 {
			fn(pos, c.ptrs[cell&^cellPtr])
		} else {
			fn(pos, c.unpack(c.packed[cell-1], &scratch, buf[:]))
		}
	}
}

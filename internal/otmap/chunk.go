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

	// cells holds per tile: 0 = none, cellPtr|i = ptrs[i], else packed[v-1].
	cells  [ChunkSize * ChunkSize]uint16
	packed []packedTile
	items  []packedItem
	ptrs   []*Tile
	// freePtrs are unused slots of ptrs; dead counts packed tiles no cell
	// refers to any more (replaced or removed).
	freePtrs []uint16
	dead     int
}

const cellPtr = 0x8000

type packedTile struct {
	off    uint32 // first item in Chunk.items
	n      uint8  // items, the ground included
	ground bool   // items[off] is the ground
}

type packedItem struct {
	id, subtype uint16
}

// packable reports whether a tile can be stored packed.
func packable(t *Tile) bool {
	if t.Flags != 0 || t.HouseID != 0 || t.State != 0 || t.Ext != nil || len(t.Items) > 254 {
		return false
	}
	if g := t.Ground; g.Ext != nil || g.Selected {
		return false
	}
	for i := range t.Items {
		if it := &t.Items[i]; it.Ext != nil || it.Selected {
			return false
		}
	}
	return t.Ground.ID != 0 || len(t.Items) > 0
}

// get returns the tile at local index i: the stored *Tile, or a packed
// tile unpacked into a new Tile.
func (c *Chunk) get(i int) *Tile {
	v := c.cells[i]
	switch {
	case v == 0:
		return nil
	case v&cellPtr != 0:
		return c.ptrs[v&^cellPtr]
	default:
		return c.unpack(c.packed[v-1], &Tile{}, nil)
	}
}

// unpack fills t from a packed tile. Items go into buf when it has room
// (then t is only valid until buf is reused), else into a new slice.
func (c *Chunk) unpack(pt packedTile, t *Tile, buf []Item) *Tile {
	src := c.items[pt.off : pt.off+uint32(pt.n)]
	*t = Tile{}
	if pt.ground {
		t.Ground = Item{ID: src[0].id, Subtype: src[0].subtype}
		src = src[1:]
	}
	if len(src) > 0 {
		var items []Item
		if cap(buf) >= len(src) {
			items = buf[:len(src)]
		} else {
			items = make([]Item, len(src))
		}
		for k, it := range src {
			items[k] = Item{ID: it.id, Subtype: it.subtype}
		}
		t.Items = items
	}
	return t
}

// set stores t at local index i (nil removes) and reports whether a tile
// was there before. Packing is up to the caller (pack).
func (c *Chunk) set(i int, t *Tile, pack bool) (had bool) {
	v := c.cells[i]
	had = v != 0
	// Release what the cell holds.
	switch {
	case v&cellPtr != 0:
		slot := v &^ cellPtr
		if t != nil && !(pack && packable(t)) {
			c.ptrs[slot] = t // replace in place
			return had
		}
		c.ptrs[slot] = nil
		c.freePtrs = append(c.freePtrs, slot)
	case v != 0:
		c.dead++
	}
	c.cells[i] = 0
	if t == nil {
		c.maybeRepack()
		return had
	}
	if pack && packable(t) {
		c.cells[i] = c.addPacked(t)
	} else {
		var slot uint16
		if n := len(c.freePtrs); n > 0 {
			slot = c.freePtrs[n-1]
			c.freePtrs = c.freePtrs[:n-1]
		} else {
			slot = uint16(len(c.ptrs))
			c.ptrs = append(c.ptrs, nil)
		}
		c.ptrs[slot] = t
		c.cells[i] = cellPtr | slot
	}
	c.maybeRepack()
	return had
}

func (c *Chunk) addPacked(t *Tile) uint16 {
	pt := packedTile{off: uint32(len(c.items))}
	if t.Ground.ID != 0 {
		pt.ground = true
		c.items = append(c.items, packedItem{t.Ground.ID, t.Ground.Subtype})
	}
	for _, it := range t.Items {
		c.items = append(c.items, packedItem{it.ID, it.Subtype})
	}
	pt.n = uint8(len(c.items) - int(pt.off))
	c.packed = append(c.packed, pt)
	return uint16(len(c.packed))
}

// maybeRepack drops dead packed tiles once they are half of the chunk.
func (c *Chunk) maybeRepack() {
	if c.dead < 32 || c.dead*2 < len(c.packed) {
		return
	}
	live := len(c.packed) - c.dead
	packed := make([]packedTile, 0, live)
	items := make([]packedItem, 0, len(c.items))
	for i, v := range c.cells {
		if v == 0 || v&cellPtr != 0 {
			continue
		}
		pt := c.packed[v-1]
		off := uint32(len(items))
		items = append(items, c.items[pt.off:pt.off+uint32(pt.n)]...)
		pt.off = off
		packed = append(packed, pt)
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
func (c *Chunk) eachPtr(fn func(t *Tile)) {
	for _, t := range c.ptrs {
		if t != nil {
			fn(t)
		}
	}
}

// Peek returns the tile at local (x, y), nil when there is none. A packed
// tile is unpacked into scratch and buf (see unpack): it is only valid
// until they are reused.
func (c *Chunk) Peek(x, y int, scratch *Tile, buf []Item) *Tile {
	v := c.cells[y<<ChunkBits|x]
	switch {
	case v == 0:
		return nil
	case v&cellPtr != 0:
		return c.ptrs[v&^cellPtr]
	default:
		return c.unpack(c.packed[v-1], scratch, buf)
	}
}

// Each calls fn for every tile of the chunk. Packed tiles are passed as a
// reused copy: t is only valid during the call (Clone it to keep it), and
// changing it does not change the map.
func (c *Chunk) Each(fn func(p Position, t *Tile)) {
	var scratch Tile
	var buf [16]Item
	for i := range c.cells {
		v := c.cells[i]
		if v == 0 {
			continue
		}
		p := Position{c.X<<ChunkBits | uint16(i&(ChunkSize-1)), c.Y<<ChunkBits | uint16(i>>ChunkBits), c.Z}
		if v&cellPtr != 0 {
			fn(p, c.ptrs[v&^cellPtr])
		} else {
			fn(p, c.unpack(c.packed[v-1], &scratch, buf[:]))
		}
	}
}

// Package otmap is the Open Tibia map model (OTBM) with its auxiliary files:
// houses, spawns (one file with monsters and NPCs, or separate
// monster and NPC files) and zones.
//
// Tiles live in 32x32 chunks per floor so drawing and saving touch only what
// exists, and items are small values: the rarely used attributes live in a
// separate struct, so a large map stays compact in memory.
package otmap

import (
	"cmp"
	"slices"
)

// SpawnFormat tells how spawns are stored.
type SpawnFormat string

const (
	// SpawnsSingle is one spawn file with monsters and NPCs.
	SpawnsSingle SpawnFormat = "single"
	// SpawnsSeparate are separate monster and NPC spawn files.
	SpawnsSeparate SpawnFormat = "separate"
)

// OTBMVersion is the only OTBM version the editor writes: version 2, which
// servers load (they reject newer ones). Files of versions 1 to 4 are
// read and saved as version 2. Waypoints are written too: servers read them
// in any version.
const OTBMVersion uint32 = 2

// otbmNewest is the newest OTBM version the reader knows.
const otbmNewest uint32 = 4

// Map is an OTBM map. It is not safe for concurrent use.
type Map struct {
	Width       uint16      `json:"width"`
	Height      uint16      `json:"height"`
	ItemsMajor  uint32      `json:"itemsMajor"`
	ItemsMinor  uint32      `json:"itemsMinor"`
	Description string      `json:"description"`
	SpawnFile   string      `json:"spawnFile"`
	NpcFile     string      `json:"npcFile"`
	HouseFile   string      `json:"houseFile"`
	ZoneFile    string      `json:"zoneFile"`
	SpawnFormat SpawnFormat `json:"spawnFormat"`

	Towns     map[uint32]*Town  `json:"-"`
	Houses    map[uint32]*House `json:"-"`
	Waypoints []*Waypoint       `json:"-"`
	Zones     []Zone            `json:"-"`

	chunks map[ChunkKey]*Chunk
	tiles  int
	floors [MaxFloor + 1]int
	// Rev increases on every tile change (see Touch).
	Rev uint64 `json:"-"`
}

// ForgetAuxFileNames clears the names of the house, spawn, npc and zone
// files, so that saving under a new name names them after the new map.
func (m *Map) ForgetAuxFileNames() {
	m.HouseFile, m.SpawnFile, m.NpcFile, m.ZoneFile = "", "", "", ""
}

// New returns an empty map.
func New(width, height uint16) *Map {
	return &Map{
		Width: width, Height: height, SpawnFormat: SpawnsSingle,
		Towns: map[uint32]*Town{}, Houses: map[uint32]*House{},
		chunks: map[ChunkKey]*Chunk{},
	}
}

// TileCount returns the number of tiles.
func (m *Map) TileCount() int { return m.tiles }

// FloorTiles returns the number of tiles on each floor.
func (m *Map) FloorTiles() [MaxFloor + 1]int { return m.floors }

// Tile returns the tile at pos or nil. Treat it as read-only: a packed tile
// (see chunk.go) comes back as a new copy.
func (m *Map) Tile(pos Position) *Tile {
	chunk := m.chunks[KeyOf(pos)]
	if chunk == nil {
		return nil
	}
	return chunk.get(localIndex(pos))
}

// Peek returns the tile at pos or nil without copying a packed tile: it is
// unpacked into scratch and buf (see Chunk.Peek) and only valid until they
// are reused. Treat it as read-only.
func (m *Map) Peek(pos Position, scratch *Tile, buf []Item) *Tile {
	chunk := m.chunks[KeyOf(pos)]
	if chunk == nil {
		return nil
	}
	index := localIndex(pos)
	return chunk.Peek(index&(ChunkSize-1), index>>ChunkBits, scratch, buf)
}

// PeekAt is Peek for x, y, z; out of range coordinates are nil.
func (m *Map) PeekAt(x, y, z int, scratch *Tile, buf []Item) *Tile {
	if x < 0 || y < 0 || z < 0 || x > 0xFFFF || y > 0xFFFF || z > MaxFloor {
		return nil
	}
	return m.Peek(Position{uint16(x), uint16(y), uint8(z)}, scratch, buf)
}

// stored returns the tile at pos when it is stored as *Tile (see
// EachStored), else nil: a cheap lookup for creatures, spawns and houses.
func (m *Map) stored(pos Position) *Tile {
	chunk := m.chunks[KeyOf(pos)]
	if chunk == nil {
		return nil
	}
	if cell := chunk.cells[localIndex(pos)]; cell&cellPtr != 0 {
		return chunk.ptrs[cell&^cellPtr]
	}
	return nil
}

// Has reports whether there is a tile at pos.
func (m *Map) Has(pos Position) bool {
	chunk := m.chunks[KeyOf(pos)]
	return chunk != nil && chunk.cells[localIndex(pos)] != 0
}

// At returns the tile at x, y, z or nil; out of range coordinates are nil.
func (m *Map) At(x, y, z int) *Tile {
	if x < 0 || y < 0 || z < 0 || x > 0xFFFF || y > 0xFFFF || z > MaxFloor {
		return nil
	}
	return m.Tile(Position{uint16(x), uint16(y), uint8(z)})
}

// SetTile stores a tile (nil or empty removes it) and marks the chunk
// changed. The map may pack the tile: keep no pointer to it.
func (m *Map) SetTile(pos Position, tile *Tile) {
	if tile != nil && tile.Empty() {
		tile = nil
	}
	m.put(pos, tile, true)
}

// Put stores a tile even when it is empty (an editor working copy); nil
// removes the tile. Prune drops it again when it stays empty. The map keeps
// tile itself, so changes to it show on the map.
func (m *Map) Put(pos Position, tile *Tile) { m.put(pos, tile, false) }

func (m *Map) put(pos Position, tile *Tile, pack bool) {
	key := KeyOf(pos)
	chunk := m.chunks[key]
	if chunk == nil {
		if tile == nil {
			return
		}
		chunkX, chunkY, z := key.Split()
		chunk = &Chunk{X: chunkX, Y: chunkY, Z: z}
		m.chunks[key] = chunk
	}
	had := chunk.set(localIndex(pos), tile, pack)
	switch {
	case !had && tile != nil:
		chunk.Count++
		m.tiles++
		m.floors[pos.Z]++
	case had && tile == nil:
		chunk.Count--
		m.tiles--
		m.floors[pos.Z]--
	}
	m.Rev++
	chunk.Rev = m.Rev
	if chunk.Count == 0 {
		delete(m.chunks, key)
	}
}

// Prune removes the tile at pos when it is empty.
func (m *Map) Prune(pos Position) {
	if tile := m.Tile(pos); tile != nil && tile.Empty() {
		m.Put(pos, nil)
	}
}

// Touch marks the chunk of pos changed (after editing a tile in place).
func (m *Map) Touch(pos Position) {
	if chunk := m.chunks[KeyOf(pos)]; chunk != nil {
		m.Rev++
		chunk.Rev = m.Rev
	}
}

// Chunk returns a chunk or nil.
func (m *Map) Chunk(key ChunkKey) *Chunk { return m.chunks[key] }

// ChunkRev returns the revision of a chunk (0 when it does not exist).
func (m *Map) ChunkRev(key ChunkKey) uint64 {
	if chunk := m.chunks[key]; chunk != nil {
		return chunk.Rev
	}
	return 0
}

// Chunks returns all chunks sorted by floor, then by 256x256 area, then by
// position: the order OTBM tile areas are written in.
func (m *Map) Chunks() []*Chunk {
	out := make([]*Chunk, 0, len(m.chunks))
	for _, chunk := range m.chunks {
		out = append(out, chunk)
	}
	slices.SortFunc(out, func(left, right *Chunk) int {
		return cmp.Or(
			cmp.Compare(left.Z, right.Z),
			cmp.Compare(left.Y>>3, right.Y>>3),
			cmp.Compare(left.X>>3, right.X>>3),
			cmp.Compare(left.Y, right.Y),
			cmp.Compare(left.X, right.X),
		)
	})
	return out
}

// Each calls fn for every tile in save order. fn must not add or remove
// tiles; tile is only valid during the call (see Chunk.Each).
func (m *Map) Each(fn func(pos Position, tile *Tile)) {
	for _, chunk := range m.Chunks() {
		chunk.Each(fn)
	}
}

// EachStored calls fn in save order for every tile stored as *Tile: those
// with flags, a house, a state, an ext (creatures, spawns, zones) or
// selected items (see packable). Plain packed tiles are skipped without
// unpacking, so scans for such tiles should use it instead of Each.
func (m *Map) EachStored(fn func(pos Position, tile *Tile)) {
	for _, chunk := range m.Chunks() {
		chunk.eachStored(fn)
	}
}

// EachIn calls fn for every tile inside the rectangle on floors
// minZ..maxZ.
func (m *Map) EachIn(minX, minY, maxX, maxY int, minZ, maxZ int, fn func(pos Position, tile *Tile)) {
	minX, minY = max(minX, 0), max(minY, 0)
	maxX, maxY = min(maxX, 0xFFFF), min(maxY, 0xFFFF)
	for z := max(minZ, 0); z <= min(maxZ, MaxFloor); z++ {
		for chunkY := minY >> ChunkBits; chunkY <= maxY>>ChunkBits; chunkY++ {
			for chunkX := minX >> ChunkBits; chunkX <= maxX>>ChunkBits; chunkX++ {
				chunk := m.chunks[MakeKey(uint16(chunkX), uint16(chunkY), uint8(z))]
				if chunk == nil {
					continue
				}
				chunk.Each(func(pos Position, tile *Tile) {
					if int(pos.X) >= minX && int(pos.X) <= maxX && int(pos.Y) >= minY && int(pos.Y) <= maxY {
						fn(pos, tile)
					}
				})
			}
		}
	}
}

// Bounds returns the area that holds tiles (ok is false for an empty map).
func (m *Map) Bounds() (minX, minY, maxX, maxY int, ok bool) {
	first := true
	for _, chunk := range m.chunks {
		chunk.Each(func(pos Position, _ *Tile) {
			x, y := int(pos.X), int(pos.Y)
			if first {
				minX, minY, maxX, maxY, first = x, y, x, y, false
				return
			}
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
		})
	}
	return minX, minY, maxX, maxY, !first
}

// SortedTowns returns the towns by id.
func (m *Map) SortedTowns() []*Town {
	out := make([]*Town, 0, len(m.Towns))
	for _, town := range m.Towns {
		out = append(out, town)
	}
	slices.SortFunc(out, func(left, right *Town) int { return cmp.Compare(left.ID, right.ID) })
	return out
}

// SortedHouses returns the houses by id.
func (m *Map) SortedHouses() []*House {
	out := make([]*House, 0, len(m.Houses))
	for _, house := range m.Houses {
		out = append(out, house)
	}
	slices.SortFunc(out, func(left, right *House) int { return cmp.Compare(left.ID, right.ID) })
	return out
}

// Waypoint returns the waypoint with a name or nil.
func (m *Map) Waypoint(name string) *Waypoint {
	for _, waypoint := range m.Waypoints {
		if waypoint.Name == name {
			return waypoint
		}
	}
	return nil
}

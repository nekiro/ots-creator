package maprender

import (
	"encoding/binary"
	"math"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Tile flags of the chunk format.
const (
	tileSelected = 1 << iota
	tileModified
	tileHouse
	tilePZ
	tileNoPVP
	tileNoLogout
	tilePVPZone
	tileCreatures
	tileSpawn
	tileSpawnNPC
	tileZones
	tileOptionalBorder
	tileBlocking
	tileText
	// tileGround marks a tile whose first item is its ground.
	tileGround
	// tileOpaque marks a tile with a full ground or an opaque thing: it
	// covers creatures on the floors under it (client: Tile::isFullyOpaque).
	tileOpaque
)

// Item flags of the chunk format.
const (
	itemSelected = 1 << iota
	itemIDs      // action or unique id
	itemText
	itemTeleport
	itemContents
	itemDoorID
)

// Creature flags of the chunk format; the direction is in bits 2-3.
const (
	creatureNPC = 1 << iota
	creatureSelected
	creatureDirectionShift = iota
)

const (
	// maxTileText bounds the item text sent with a tile (hover labels show
	// at most 12 lines of 40 characters).
	maxTileText = 1024
	// maxTileItems leaves room for the ground in the u8 item count.
	maxTileItems = math.MaxUint8 - 1
	// maxListLength bounds the u8 counted lists (creatures, zones, names).
	maxListLength = math.MaxUint8
	// chunkBufferSize is the initial buffer of a chunk response.
	chunkBufferSize = 64 << 10
)

// chunkWriter appends little endian values to a buffer.
type chunkWriter struct {
	buf []byte
	// looks tells the opacity of things (tileOpaque).
	looks *geometryBuilder
}

func (w *chunkWriter) u8(value uint8)   { w.buf = append(w.buf, value) }
func (w *chunkWriter) u16(value uint16) { w.buf = binary.LittleEndian.AppendUint16(w.buf, value) }
func (w *chunkWriter) u32(value uint32) { w.buf = binary.LittleEndian.AppendUint32(w.buf, value) }

func (w *chunkWriter) bool(value bool) {
	if value {
		w.u8(1)
	} else {
		w.u8(0)
	}
}

// shortString writes a string of at most 255 bytes with its u8 length.
func (w *chunkWriter) shortString(text string) {
	text = text[:min(len(text), maxListLength)]
	w.u8(uint8(len(text)))
	w.buf = append(w.buf, text...)
}

// EncodeChunks encodes the tiles of chunks for the editor (little endian),
// per chunk:
//
//	u32 key, u16 tile count, then per tile:
//	u16 local index (y*32+x), u16 flags, [u32 house id],
//	u8 item count (ground first when the tile has one; flag bit 0x4000 of
//	   the tile marks a ground), items: u16 id, u16 subtype, u8 flags,
//	[creatures: u8 count, per creature: u8 flags (1 npc, 2 selected,
//	   dir<<2), u16 look type, u8 head, body, legs, feet, addons,
//	   u16 mount, u16 look item, u8 name length, name],
//	[spawn: u16 radius, u8 selected], [npc spawn: same],
//	[zones: u8 count, u16 ids],
//	[text of the topmost item with one: u16 length, text]
func EncodeChunks(scene *Scene, keys []otmap.ChunkKey) []byte {
	writer := &chunkWriter{buf: make([]byte, 0, chunkBufferSize), looks: newGeometryBuilder(scene, GeometryOptions{})}
	for _, key := range keys {
		writer.chunk(scene, key)
	}
	return writer.buf
}

func (w *chunkWriter) chunk(scene *Scene, key otmap.ChunkKey) {
	w.u32(uint32(key))
	countAt := len(w.buf)
	w.u16(0)
	chunk := scene.Map.Chunk(key)
	if chunk == nil {
		return
	}
	tiles := 0
	chunk.Each(func(position otmap.Position, tile *otmap.Tile) {
		tiles++
		w.tile(scene, position, tile)
	})
	binary.LittleEndian.PutUint16(w.buf[countAt:], uint16(tiles))
}

func (w *chunkWriter) tile(scene *Scene, position otmap.Position, tile *otmap.Tile) {
	localX, localY := int(position.X&(otmap.ChunkSize-1)), int(position.Y&(otmap.ChunkSize-1))
	w.u16(uint16(localY<<otmap.ChunkBits | localX))
	creatures := tile.Creatures()
	text := topText(tile)
	flags := tileFlags(scene, tile, len(creatures) > 0, text != "")
	if w.looks.isFullyOpaque(tile) {
		flags |= tileOpaque
	}
	w.u16(flags)
	if tile.HouseID != 0 {
		w.u32(tile.HouseID)
	}
	w.items(tile)
	if len(creatures) > 0 {
		w.creatures(scene, creatures)
	}
	for _, spawn := range []*otmap.Spawn{tile.Spawn(), tile.SpawnNPC()} {
		if spawn != nil {
			w.u16(uint16(min(max(spawn.Radius, 0), math.MaxUint16)))
			w.bool(spawn.Selected)
		}
	}
	if zones := tile.Zones(); len(zones) > 0 {
		zones = zones[:min(len(zones), maxListLength)]
		w.u8(uint8(len(zones)))
		for _, id := range zones {
			w.u16(id)
		}
	}
	if text != "" {
		text = text[:min(len(text), maxTileText)]
		w.u16(uint16(len(text)))
		w.buf = append(w.buf, text...)
	}
}

func tileFlags(scene *Scene, tile *otmap.Tile, hasCreatures, hasText bool) uint16 {
	var flags uint16
	for _, flag := range []struct {
		on  bool
		bit uint16
	}{
		{tile.Selected(), tileSelected},
		{tile.State&otmap.StateModified != 0, tileModified},
		{tile.HouseID != 0, tileHouse},
		{tile.Flags&otmap.FlagPZ != 0, tilePZ},
		{tile.Flags&otmap.FlagNoPVP != 0, tileNoPVP},
		{tile.Flags&otmap.FlagNoLogout != 0, tileNoLogout},
		{tile.Flags&otmap.FlagPVPZone != 0, tilePVPZone},
		{hasCreatures, tileCreatures},
		{tile.Spawn() != nil, tileSpawn},
		{tile.SpawnNPC() != nil, tileSpawnNPC},
		{len(tile.Zones()) > 0, tileZones},
		{tile.State&otmap.StateOptionalBorder != 0, tileOptionalBorder},
		{scene.IsBlocking(tile), tileBlocking},
		{hasText, tileText},
		{tile.Ground.ID != 0, tileGround},
	} {
		if flag.on {
			flags |= flag.bit
		}
	}
	return flags
}

func (w *chunkWriter) items(tile *otmap.Tile) {
	items := tile.Items[:min(len(tile.Items), maxTileItems)]
	count := len(items)
	if tile.Ground.ID != 0 {
		count++
	}
	w.u8(uint8(count))
	if tile.Ground.ID != 0 {
		w.item(tile.Ground)
	}
	for _, item := range items {
		w.item(item)
	}
}

func (w *chunkWriter) item(item otmap.Item) {
	w.u16(item.ID)
	w.u16(item.Subtype)
	var flags uint8
	if item.Selected {
		flags |= itemSelected
	}
	if ext := item.Ext; ext != nil {
		if ext.ActionID != 0 || ext.UniqueID != 0 {
			flags |= itemIDs
		}
		if ext.Text != "" || ext.Description != "" {
			flags |= itemText
		}
		if ext.Dest != nil {
			flags |= itemTeleport
		}
		if len(ext.Items) > 0 {
			flags |= itemContents
		}
		if ext.DoorID != 0 {
			flags |= itemDoorID
		}
	}
	w.u8(flags)
}

func (w *chunkWriter) creatures(scene *Scene, creatures []otmap.Creature) {
	creatures = creatures[:min(len(creatures), maxListLength)]
	w.u8(uint8(len(creatures)))
	for _, creature := range creatures {
		flags := (creature.Direction & 3) << creatureDirectionShift
		if creature.NPC {
			flags |= creatureNPC
		}
		if creature.Selected {
			flags |= creatureSelected
		}
		outfit := scene.Outfit(creature.Name)
		w.u8(flags)
		w.u16(outfit.LookType)
		w.u8(outfit.Head)
		w.u8(outfit.Body)
		w.u8(outfit.Legs)
		w.u8(outfit.Feet)
		w.u8(outfit.Addons)
		w.u16(outfit.Mount)
		w.u16(outfit.LookItem)
		w.shortString(creature.Name)
	}
}

// topText is the text of the topmost item of the tile that has one.
func topText(tile *otmap.Tile) string {
	for i := len(tile.Items) - 1; i >= 0; i-- {
		if ext := tile.Items[i].Ext; ext != nil && ext.Text != "" {
			return ext.Text
		}
	}
	if ext := tile.Ground.Ext; ext != nil {
		return ext.Text
	}
	return ""
}

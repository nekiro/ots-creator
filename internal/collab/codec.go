package collab

import (
	"errors"
	"math"

	"github.com/nekiro/ots-creator/internal/formats/binio"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// The binary encoding of edits: little endian, strings and byte slices with
// a u32 length. Every count is checked against the bytes left, so a broken
// or hostile message fails instead of allocating a lot.

// Edit is one change of the shared map. Seq is its place in the order of
// the host (0 when a guest sends it), Author the peer that made it and
// Local the number the author gave it (to recognize its own edits).
type Edit struct {
	Seq    uint64
	Author uint32
	Local  uint64
	Label  string
	Tiles  []mapedit.TileState
	Meta   *mapedit.MetaData
}

// maxItemDepth bounds containers in containers.
const maxItemDepth = 32

var errTooDeep = errors.New("containers are nested too deep")

func encodeEdit(edit *Edit) []byte {
	w := binio.NewWriter(64 + 32*len(edit.Tiles))
	w.U32(uint32(edit.Seq))
	w.U32(uint32(edit.Seq >> 32))
	w.U32(edit.Author)
	w.U32(uint32(edit.Local))
	w.U32(uint32(edit.Local >> 32))
	putString(w, edit.Label)
	w.U32(uint32(len(edit.Tiles)))
	for _, state := range edit.Tiles {
		putPosition(w, state.Pos)
		putTile(w, state.Tile)
	}
	w.Bool(edit.Meta != nil)
	if edit.Meta != nil {
		putMeta(w, edit.Meta)
	}
	return w.Bytes()
}

func decodeEdit(data []byte) (*Edit, error) {
	r := binio.NewReader(data)
	edit := &Edit{}
	edit.Seq = uint64(r.U32()) | uint64(r.U32())<<32
	edit.Author = r.U32()
	edit.Local = uint64(r.U32()) | uint64(r.U32())<<32
	edit.Label = getString(r)
	count := getCount(r, 6)
	edit.Tiles = make([]mapedit.TileState, 0, count)
	for range count {
		pos := getPosition(r)
		tile := getTile(r)
		if r.Err() != nil {
			break
		}
		edit.Tiles = append(edit.Tiles, mapedit.TileState{Pos: pos, Tile: tile})
	}
	if r.Bool() {
		edit.Meta = getMeta(r)
	}
	if r.Err() != nil {
		return nil, r.Err()
	}
	return edit, nil
}

func putString(w *binio.Writer, text string) {
	w.U32(uint32(len(text)))
	w.Write([]byte(text))
}

func getString(r *binio.Reader) string {
	length := getCount(r, 1)
	return string(r.Bytes(length))
}

func putBytes(w *binio.Writer, data []byte) {
	w.U32(uint32(len(data)))
	w.Write(data)
}

func getBytes(r *binio.Reader) []byte {
	length := getCount(r, 1)
	return append([]byte(nil), r.Bytes(length)...)
}

// getCount reads a u32 count of records of at least minSize bytes each and
// fails when the rest of the data cannot hold them.
func getCount(r *binio.Reader, minSize int) int {
	count := int(r.U32())
	if r.Err() != nil {
		return 0
	}
	if count < 0 || count > r.Remaining()/max(minSize, 1) {
		r.Fail(errors.New("a count is larger than the message"))
		return 0
	}
	return count
}

func putPosition(w *binio.Writer, pos otmap.Position) {
	w.U16(pos.X)
	w.U16(pos.Y)
	w.U8(pos.Z)
}

func getPosition(r *binio.Reader) otmap.Position {
	pos := otmap.Position{X: r.U16(), Y: r.U16(), Z: r.U8()}
	if pos.Z > otmap.MaxFloor {
		r.Fail(errors.New("a floor is out of range"))
	}
	return pos
}

func putTile(w *binio.Writer, tile *otmap.Tile) {
	if tile.Empty() {
		w.U8(0)
		return
	}
	w.U8(1)
	putItem(w, tile.Ground)
	w.U32(uint32(len(tile.Items)))
	for _, item := range tile.Items {
		putItem(w, item)
	}
	w.U32(tile.Flags)
	w.U32(tile.HouseID)
	ext := tile.Ext
	if ext == nil {
		ext = &otmap.TileExt{}
	}
	w.U32(uint32(len(ext.Creatures)))
	for _, creature := range ext.Creatures {
		putString(w, creature.Name)
		w.Bool(creature.NPC)
		w.I32(creature.SpawnTime)
		w.U8(creature.Direction)
		w.U8(creature.Weight)
	}
	putSpawn(w, ext.Spawn)
	putSpawn(w, ext.SpawnNPC)
	w.U32(uint32(len(ext.Zones)))
	for _, zone := range ext.Zones {
		w.U16(zone)
	}
}

func getTile(r *binio.Reader) *otmap.Tile {
	if r.U8() == 0 {
		return nil
	}
	tile := &otmap.Tile{}
	tile.Ground = getItem(r, 0)
	if count := getCount(r, 5); count > 0 {
		tile.Items = make([]otmap.Item, count)
		for i := range tile.Items {
			tile.Items[i] = getItem(r, 0)
		}
	}
	tile.Flags = r.U32()
	tile.HouseID = r.U32()
	ext := &otmap.TileExt{}
	if count := getCount(r, 11); count > 0 {
		ext.Creatures = make([]otmap.Creature, count)
		for i := range ext.Creatures {
			ext.Creatures[i] = otmap.Creature{Name: getString(r), NPC: r.Bool(), SpawnTime: r.I32(), Direction: r.U8(), Weight: r.U8()}
		}
	}
	ext.Spawn = getSpawn(r)
	ext.SpawnNPC = getSpawn(r)
	if count := getCount(r, 2); count > 0 {
		ext.Zones = make([]uint16, count)
		for i := range ext.Zones {
			ext.Zones[i] = r.U16()
		}
	}
	tile.Ext = ext
	tile.Compact()
	return tile
}

func putSpawn(w *binio.Writer, spawn *otmap.Spawn) {
	w.Bool(spawn != nil)
	if spawn != nil {
		w.I32(spawn.Radius)
	}
}

func getSpawn(r *binio.Reader) *otmap.Spawn {
	if !r.Bool() {
		return nil
	}
	return &otmap.Spawn{Radius: r.I32()}
}

func putItem(w *binio.Writer, item otmap.Item) {
	w.U16(item.ID)
	w.U16(item.Subtype)
	ext := item.Ext
	if !item.Complex() {
		w.U8(0)
		return
	}
	w.U8(1)
	w.U16(ext.ActionID)
	w.U16(ext.UniqueID)
	putString(w, ext.Text)
	putString(w, ext.Description)
	w.Bool(ext.Dest != nil)
	if ext.Dest != nil {
		putPosition(w, *ext.Dest)
	}
	w.U16(ext.DepotID)
	w.U8(ext.DoorID)
	w.U32(uint32(len(ext.Items)))
	for _, inner := range ext.Items {
		putItem(w, inner)
	}
	w.U32(uint32(len(ext.Attrs)))
	for _, attr := range ext.Attrs {
		putString(w, attr.Key)
		w.U8(attr.Type)
		putString(w, attr.Str)
		bits := math.Float64bits(attr.Num)
		w.U32(uint32(bits))
		w.U32(uint32(bits >> 32))
		w.I32(attr.Int)
		w.Bool(attr.Value)
	}
	w.U32(uint32(len(ext.Raw)))
	for _, raw := range ext.Raw {
		w.U8(raw.Code)
		putBytes(w, raw.Data)
	}
}

func getItem(r *binio.Reader, depth int) otmap.Item {
	item := otmap.Item{ID: r.U16(), Subtype: r.U16()}
	if r.U8() == 0 {
		return item
	}
	if depth >= maxItemDepth {
		r.Fail(errTooDeep)
		return item
	}
	ext := &otmap.ItemExt{ActionID: r.U16(), UniqueID: r.U16(), Text: getString(r), Description: getString(r)}
	if r.Bool() {
		dest := getPosition(r)
		ext.Dest = &dest
	}
	ext.DepotID = r.U16()
	ext.DoorID = r.U8()
	if count := getCount(r, 5); count > 0 {
		ext.Items = make([]otmap.Item, count)
		for i := range ext.Items {
			ext.Items[i] = getItem(r, depth+1)
			if r.Err() != nil {
				return item
			}
		}
	}
	if count := getCount(r, 22); count > 0 {
		ext.Attrs = make([]otmap.Attr, count)
		for i := range ext.Attrs {
			attr := otmap.Attr{Key: getString(r), Type: r.U8(), Str: getString(r)}
			attr.Num = math.Float64frombits(uint64(r.U32()) | uint64(r.U32())<<32)
			attr.Int = r.I32()
			attr.Value = r.Bool()
			ext.Attrs[i] = attr
		}
	}
	if count := getCount(r, 5); count > 0 {
		ext.Raw = make([]otmap.RawAttr, count)
		for i := range ext.Raw {
			ext.Raw[i] = otmap.RawAttr{Code: r.U8(), Data: getBytes(r)}
		}
	}
	item.Ext = ext
	item.Compact()
	return item
}

func putMeta(w *binio.Writer, meta *mapedit.MetaData) {
	w.U16(meta.Width)
	w.U16(meta.Height)
	putString(w, meta.Description)
	putString(w, string(meta.SpawnFormat))
	w.U32(uint32(len(meta.Towns)))
	for _, town := range meta.Towns {
		w.U32(town.ID)
		putString(w, town.Name)
		putPosition(w, town.Temple)
	}
	w.U32(uint32(len(meta.Houses)))
	for _, house := range meta.Houses {
		w.U32(house.ID)
		putString(w, house.Name)
		w.U32(house.TownID)
		w.I32(house.Rent)
		w.Bool(house.Guildhall)
		putPosition(w, house.Exit)
		w.U32(house.ClientID)
		w.I32(house.Beds)
	}
	w.U32(uint32(len(meta.Waypoints)))
	for _, waypoint := range meta.Waypoints {
		putString(w, waypoint.Name)
		putPosition(w, waypoint.Pos)
	}
	w.U32(uint32(len(meta.Zones)))
	for _, zone := range meta.Zones {
		w.U16(zone.ID)
		putString(w, zone.Name)
	}
}

func getMeta(r *binio.Reader) *mapedit.MetaData {
	meta := &mapedit.MetaData{Width: r.U16(), Height: r.U16(), Description: getString(r), SpawnFormat: otmap.SpawnFormat(getString(r))}
	switch meta.SpawnFormat {
	case "", otmap.SpawnsSingle, otmap.SpawnsSeparate:
	default:
		meta.SpawnFormat = ""
	}
	if count := getCount(r, 13); count > 0 {
		meta.Towns = make([]otmap.Town, count)
		for i := range meta.Towns {
			meta.Towns[i] = otmap.Town{ID: r.U32(), Name: getString(r), Temple: getPosition(r)}
		}
	}
	if count := getCount(r, 30); count > 0 {
		meta.Houses = make([]otmap.House, count)
		for i := range meta.Houses {
			meta.Houses[i] = otmap.House{ID: r.U32(), Name: getString(r), TownID: r.U32(), Rent: r.I32(), Guildhall: r.Bool(), Exit: getPosition(r), ClientID: r.U32(), Beds: r.I32()}
		}
	}
	if count := getCount(r, 9); count > 0 {
		meta.Waypoints = make([]otmap.Waypoint, count)
		for i := range meta.Waypoints {
			meta.Waypoints[i] = otmap.Waypoint{Name: getString(r), Pos: getPosition(r)}
		}
	}
	if count := getCount(r, 6); count > 0 {
		meta.Zones = make([]otmap.Zone, count)
		for i := range meta.Zones {
			meta.Zones[i] = otmap.Zone{ID: r.U16(), Name: getString(r)}
		}
	}
	return meta
}

package maprender

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

type fakeThings map[uint32]*thing.Thing

func (things fakeThings) Thing(_ thing.Category, id uint32) (*thing.Thing, error) {
	if found, ok := things[id]; ok {
		return found, nil
	}
	return nil, errors.New("missing")
}

func oneSpriteThing(sprite uint32) *thing.Thing {
	return &thing.Thing{FrameGroups: []*thing.FrameGroup{{
		Width: 1, Height: 1, Layers: 1, PatternX: 1, PatternY: 1, PatternZ: 1, Frames: 1, Sprites: []uint32{sprite},
	}}}
}

// testScene has a ground (100, sprite 7) and an item without client data
// (101) on tile (1, 2, 7).
func testScene(t *testing.T) *Scene {
	t.Helper()
	items := otitems.NewDB(otitems.ModeClient, []*otitems.Type{
		{ID: 100, ClientID: 100, Flags: otitems.FlagGround},
		{ID: 101, ClientID: 101},
	})
	m := otmap.New(64, 64)
	m.SetTile(otmap.Position{X: 1, Y: 2, Z: 7}, &otmap.Tile{
		Ground:  otmap.Item{ID: 100},
		Items:   []otmap.Item{{ID: 101, Ext: &otmap.ItemExt{Text: "hello", ActionID: 5}}},
		HouseID: 9,
	})
	return &Scene{
		Map: m, Items: items, Things: fakeThings{100: oneSpriteThing(7)}, SpriteSize: 32,
		IsBlocking: func(*otmap.Tile) bool { return true },
		Outfit:     func(string) materials.Outfit { return materials.Outfit{} },
	}
}

func TestEncodeChunks(t *testing.T) {
	scene := testScene(t)
	key := otmap.KeyOf(otmap.Position{X: 1, Y: 2, Z: 7})
	data := EncodeChunks(scene, []otmap.ChunkKey{key, otmap.MakeKey(1, 1, 7)})
	if binary.LittleEndian.Uint32(data) != uint32(key) || binary.LittleEndian.Uint16(data[4:]) != 1 {
		t.Fatalf("header %v", data[:6])
	}
	tile := data[6:]
	if index := binary.LittleEndian.Uint16(tile); index != 2<<otmap.ChunkBits|1 {
		t.Fatalf("local index %d", index)
	}
	flags := binary.LittleEndian.Uint16(tile[2:])
	for _, bit := range []uint16{tileHouse, tileBlocking, tileText, tileGround} {
		if flags&bit == 0 {
			t.Fatalf("flags %b miss %b", flags, bit)
		}
	}
	if house := binary.LittleEndian.Uint32(tile[4:]); house != 9 || tile[8] != 2 {
		t.Fatalf("house %d, %d items", house, tile[8])
	}
	// ground: id, subtype, flags; then the item with ids and text.
	if binary.LittleEndian.Uint16(tile[9:]) != 100 || binary.LittleEndian.Uint16(tile[14:]) != 101 || tile[18] != itemIDs|itemText {
		t.Fatalf("items %v", tile[9:19])
	}
	if length := binary.LittleEndian.Uint16(tile[19:]); length != 5 || string(tile[21:26]) != "hello" {
		t.Fatalf("text %q", tile[21:])
	}
	// The missing chunk has no tiles.
	if rest := tile[26:]; len(rest) != 6 || binary.LittleEndian.Uint16(rest[4:]) != 0 {
		t.Fatalf("empty chunk %v", rest)
	}
}

func TestBuildGeometryParts(t *testing.T) {
	scene := testScene(t)
	keys := []otmap.ChunkKey{otmap.MakeKey(0, 0, 7)}
	parts, rawSize, err := BuildGeometryParts(scene, keys, GeometryOptions{}, false)
	if err != nil || len(parts) != 1 || rawSize != len(parts[0]) {
		t.Fatalf("parts %d, raw %d, %v", len(parts), rawSize, err)
	}
	part := parts[0]
	quads, sprites := binary.LittleEndian.Uint32(part[4:]), binary.LittleEndian.Uint32(part[16:])
	// The ground sprite and a red marker for the item without sprites.
	if quads != 2 || sprites != 1 {
		t.Fatalf("%d quads, %d sprites", quads, sprites)
	}
	ground, marker := part[20:36], part[36:52]
	if binary.LittleEndian.Uint32(ground[4:]) != 7 || int16(binary.LittleEndian.Uint16(ground)) != 32 {
		t.Fatalf("ground quad %v", ground)
	}
	if sprite := binary.LittleEndian.Uint32(marker[4:]); sprite&spriteRect == 0 || marker[8] != unitByte(missingSpriteColor.red) {
		t.Fatalf("marker quad %v", marker)
	}
	packed, packedRaw, err := BuildGeometry(scene, keys, GeometryOptions{ViewFlags: ViewHouses, House: 9})
	if err != nil || packedRaw != rawSize || len(packed) == 0 {
		t.Fatalf("packed %d bytes, raw %d, %v", len(packed), packedRaw, err)
	}
}

func TestTileTint(t *testing.T) {
	scene := testScene(t)
	tile := scene.Map.Tile(otmap.Position{X: 1, Y: 2, Z: 7})
	builder := newGeometryBuilder(scene, GeometryOptions{ViewFlags: ViewHouses | ViewPathing, House: 9})
	if color := builder.tileTint(tile, true); color.red != houseTint || color.green != blockingTint || color.blue != blockingTint {
		t.Fatalf("own house tint %+v", color)
	}
	builder.options.House = 1
	if color := builder.tileTint(tile, false); color.green != otherHouseTint {
		t.Fatalf("other house tint %+v", color)
	}
	builder.options.ViewFlags = ViewHighlight
	if color := builder.tileTint(tile, true); color.red != highlightByItemCount[0] || color.blue != 1 {
		t.Fatalf("highlight tint %+v", color)
	}
}

func TestStackPattern(t *testing.T) {
	cases := map[int][2]int{0: {0, 0}, 1: {0, 0}, 4: {3, 0}, 5: {0, 1}, 9: {0, 1}, 10: {1, 1}, 24: {1, 1}, 25: {2, 1}, 49: {2, 1}, 50: {3, 1}, 100: {3, 1}}
	for count, want := range cases {
		if x, y := stackPattern(count); x != want[0] || y != want[1] {
			t.Errorf("stackPattern(%d) = %d, %d, want %v", count, x, y, want)
		}
	}
}

func TestUnitByte(t *testing.T) {
	if unitByte(-1) != 0 || unitByte(0.5) != 128 || unitByte(2) != 255 {
		t.Fatal("unitByte")
	}
}

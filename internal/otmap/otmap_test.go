package otmap

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/otitems"
)

func testItems() *otitems.DB {
	return otitems.NewDB(otitems.ModeOTB, []*otitems.Type{
		{ID: 100, ClientID: 100, Flags: otitems.FlagGround},
		{ID: 101, ClientID: 101, Flags: otitems.FlagGround},
		{ID: 1050, ClientID: 1050, TopOrder: 2},
		{ID: 1987, ClientID: 1987, Flags: otitems.FlagContainer, Kind: otitems.KindContainer},
		{ID: 2148, ClientID: 2148, Flags: otitems.FlagStackable},
		{ID: 2006, ClientID: 2006, Flags: otitems.FlagFluidContainer},
		{ID: 1387, ClientID: 1387, Kind: otitems.KindTeleport},
		{ID: 80, Flags: otitems.FlagMeta},
	})
}

func sampleMap() *Map {
	m := New(2048, 2048)
	m.Description = "test map"
	m.SetTile(Position{100, 100, 7}, &Tile{Ground: Item{ID: 100}, Flags: FlagPZ})
	dest := Position{200, 201, 6}
	m.SetTile(Position{101, 100, 7}, &Tile{
		Ground: Item{ID: 101, Ext: &ItemExt{ActionID: 1000}},
		Items: []Item{
			{ID: 1050},
			{ID: 2148, Subtype: 37},
			{ID: 1387, Ext: &ItemExt{Dest: &dest, UniqueID: 5000, Text: "hello\xfe", Description: "desc"}},
			{ID: 1987, Ext: &ItemExt{Items: []Item{{ID: 2148, Subtype: 3}, {ID: 2006, Subtype: 2}}}},
			{ID: 80},
		},
	})
	m.SetTile(Position{300, 300, 8}, &Tile{Ground: Item{ID: 100}, HouseID: 7})
	m.Houses[7] = &House{ID: 7, Name: "Flat", TownID: 1, Rent: 500, Exit: Position{301, 300, 8}, Guildhall: true}
	m.Towns[1] = &Town{ID: 1, Name: "Thais", Temple: Position{100, 100, 7}}
	m.Waypoints = []*Waypoint{{Name: "temple", Pos: Position{100, 100, 7}}}
	center := m.Tile(Position{100, 100, 7})
	center.EnsureExt().Spawn = &Spawn{Radius: 3}
	center.Ext.Creatures = []Creature{{Name: "Rat", SpawnTime: 60}}
	m.SetTile(Position{102, 101, 7}, &Tile{Ground: Item{ID: 100}, Ext: &TileExt{Creatures: []Creature{{Name: "Guide", NPC: true, SpawnTime: 30, Direction: 2}}}})
	return m
}

func TestSaveLoadTFS(t *testing.T) {
	items := testItems()
	dir := t.TempDir()
	path := filepath.Join(dir, "world.otbm")
	m := sampleMap()
	warns, err := Save(path, m, items, Format{})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	for _, f := range []string{"world-house.xml", "world-spawn.xml"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("%s not written: %v", f, err)
		}
	}
	got, warns, err := Load(path, items, Format{})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("load warnings: %v", warns)
	}
	if got.Description != "test map" || got.Width != 2048 {
		t.Fatalf("header %+v", got)
	}
	tile := got.Tile(Position{101, 100, 7})
	if tile == nil || tile.Ground.ID != 101 || tile.Ground.ActionID() != 1000 {
		t.Fatalf("ground tile = %+v", tile)
	}
	want := sampleMap().Tile(Position{101, 100, 7}).Items[:4] // the meta item is not saved
	if !reflect.DeepEqual(tile.Items, want) {
		t.Fatalf("items\n got %+v\nwant %+v", tile.Items, want)
	}
	if got.Tile(Position{100, 100, 7}).Flags != FlagPZ {
		t.Fatal("tile flags lost")
	}
	if h := got.Houses[7]; h == nil || h.Name != "Flat" || !h.Guildhall || h.Exit != (Position{301, 300, 8}) || h.Rent != 500 {
		t.Fatalf("house = %+v", h)
	}
	if got.Tile(Position{300, 300, 8}).HouseID != 7 {
		t.Fatal("house tile lost")
	}
	if tw := got.Towns[1]; tw == nil || tw.Name != "Thais" {
		t.Fatal("town lost")
	}
	if wp := got.Waypoint("temple"); wp == nil || wp.Pos != (Position{100, 100, 7}) {
		t.Fatal("waypoint lost")
	}
	center := got.Tile(Position{100, 100, 7})
	if center.Spawn() == nil || center.Spawn().Radius != 3 || len(center.Creatures()) != 1 || center.Creatures()[0].Name != "Rat" {
		t.Fatalf("spawn = %+v", center.Ext)
	}
	npc := got.Tile(Position{102, 101, 7}).Creatures()
	if len(npc) != 1 || !npc[0].NPC || npc[0].Direction != 2 {
		t.Fatalf("npc = %+v", npc)
	}
}

func TestSaveLoadSeparateSpawns(t *testing.T) {
	items := testItems()
	dir := t.TempDir()
	path := filepath.Join(dir, "world.otbm")
	m := sampleMap()
	m.SpawnFormat = SpawnsSeparate
	npcTile := m.Tile(Position{102, 101, 7})
	npcTile.EnsureExt().SpawnNPC = &Spawn{Radius: 1}
	m.Zones = []Zone{{ID: 3, Name: "arena"}}
	m.Tile(Position{100, 100, 7}).Ext.Zones = []uint16{3}
	if _, err := Save(path, m, items, Format{}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"world-monster.xml", "world-npc.xml", "world-zones.xml"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("%s not written", f)
		}
	}
	data, _ := os.ReadFile(filepath.Join(dir, "world-npc.xml"))
	if !strings.Contains(string(data), `<npc name="Guide"`) {
		t.Fatalf("npc file:\n%s", data)
	}
	got, warns, err := Load(path, items, Format{})
	if err != nil || len(warns) != 0 {
		t.Fatal(err, warns)
	}
	if got.SpawnFormat != SpawnsSeparate || len(got.Zones) != 1 || got.Zones[0].Name != "arena" {
		t.Fatalf("separate spawn data lost: %v %v", got.SpawnFormat, got.Zones)
	}
	if z := got.Tile(Position{100, 100, 7}).Zones(); len(z) != 1 || z[0] != 3 {
		t.Fatalf("tile zones = %v", z)
	}
	if got.Tile(Position{102, 101, 7}).SpawnNPC() == nil || !got.Tile(Position{102, 101, 7}).Creatures()[0].NPC {
		t.Fatal("npc spawn lost")
	}
}

func TestUnsavedCreatureWarning(t *testing.T) {
	m := New(100, 100)
	m.SetTile(Position{10, 10, 7}, &Tile{Ext: &TileExt{Creatures: []Creature{{Name: "Rat"}}}})
	warns, err := Save(filepath.Join(t.TempDir(), "m.otbm"), m, testItems(), Format{})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "1 creatures") {
		t.Fatalf("warns = %v", warns)
	}
}

func TestChunks(t *testing.T) {
	m := New(1000, 1000)
	m.SetTile(Position{40, 40, 7}, &Tile{Ground: Item{ID: 100}})
	m.SetTile(Position{41, 40, 7}, &Tile{Ground: Item{ID: 100}})
	if m.TileCount() != 2 {
		t.Fatal("count")
	}
	k := KeyOf(Position{40, 40, 7})
	if cx, cy, z := k.Split(); cx != 1 || cy != 1 || z != 7 {
		t.Fatalf("split %d %d %d", cx, cy, z)
	}
	rev := m.ChunkRev(k)
	m.SetTile(Position{40, 40, 7}, nil)
	if m.ChunkRev(k) <= rev || m.TileCount() != 1 {
		t.Fatal("removal not tracked")
	}
	m.SetTile(Position{41, 40, 7}, &Tile{})
	if m.Chunk(k) != nil || m.TileCount() != 0 {
		t.Fatal("empty chunk kept")
	}
	n := 0
	m.SetTile(Position{5, 5, 7}, &Tile{Ground: Item{ID: 100}})
	m.SetTile(Position{500, 5, 7}, &Tile{Ground: Item{ID: 100}})
	m.EachIn(0, 0, 10, 10, 7, 7, func(Position, *Tile) { n++ })
	if n != 1 {
		t.Fatalf("EachIn found %d", n)
	}
}

func TestCountU16(t *testing.T) {
	items := testItems()
	m := New(256, 256)
	m.SetTile(Position{10, 10, 7}, &Tile{Ground: Item{ID: 100}, Items: []Item{{ID: 2148, Subtype: 1000, Ext: &ItemExt{ActionID: 7}}}})
	f := Format{CountU16: true}
	data, err := EncodeOTBM(m, items, f)
	if err != nil {
		t.Fatal(err)
	}
	got, warns, err := ReadOTBM(data, items, f)
	if err != nil || len(warns) > 0 {
		t.Fatal(err, warns)
	}
	it := got.Tile(Position{10, 10, 7}).Items[0]
	if it.Subtype != 1000 || it.Ext == nil || it.Ext.ActionID != 7 {
		t.Fatalf("got %+v", it)
	}
	// The standard format caps counts at 255.
	data, _ = EncodeOTBM(m, items, Format{})
	got, _, _ = ReadOTBM(data, items, Format{})
	if it := got.Tile(Position{10, 10, 7}).Items[0]; it.Subtype != 255 {
		t.Fatalf("standard format: count %d", it.Subtype)
	}
}

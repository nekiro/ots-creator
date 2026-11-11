package otmap

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nekiro/ots-creator/internal/core/otitems"
)

// normalizedTiles returns every tile in a comparable form: editor state
// cleared, meta items dropped, raw and map attributes sorted.
func normalizedTiles(gameMap *Map, items Items) map[Position]*Tile {
	meta := func(id uint16) bool {
		itemType := items.Get(id)
		return itemType != nil && itemType.Flags.Has(otitems.FlagMeta)
	}
	var normalizeItems func(list []Item) []Item
	normalizeItem := func(item Item) Item {
		item = item.Clone()
		item.Selected = false
		if item.Ext != nil {
			item.Ext.Items = normalizeItems(item.Ext.Items)
			slices.SortStableFunc(item.Ext.Raw, func(left, right RawAttr) int { return cmp.Compare(left.Code, right.Code) })
			slices.SortStableFunc(item.Ext.Attrs, func(left, right Attr) int { return cmp.Compare(left.Key, right.Key) })
			item.Compact()
		}
		return item
	}
	normalizeItems = func(list []Item) []Item {
		var out []Item
		for _, item := range list {
			if !meta(item.ID) {
				out = append(out, normalizeItem(item))
			}
		}
		return out
	}
	tiles := map[Position]*Tile{}
	gameMap.Each(func(pos Position, tile *Tile) {
		clone := tile.Clone()
		clone.State = 0
		if meta(clone.Ground.ID) {
			clone.Ground = Item{}
		}
		clone.Ground = normalizeItem(clone.Ground)
		clone.Items = normalizeItems(clone.Items)
		if clone.Ext != nil {
			for i := range clone.Ext.Creatures {
				clone.Ext.Creatures[i].Selected = false
			}
			for _, spawn := range []*Spawn{clone.Ext.Spawn, clone.Ext.SpawnNPC} {
				if spawn != nil {
					spawn.Selected = false
				}
			}
			if len(clone.Ext.Zones) == 0 {
				clone.Ext.Zones = nil
			}
			clone.Compact()
		}
		if !clone.Empty() {
			tiles[pos] = clone
		}
	})
	return tiles
}

func compareTiles(t *testing.T, want, got map[Position]*Tile) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("tiles: got %d, want %d", len(got), len(want))
	}
	shown := 0
	for pos, wantTile := range want {
		gotTile := got[pos]
		if !reflect.DeepEqual(wantTile, gotTile) {
			if shown++; shown > 5 {
				t.Fatalf("more tiles differ")
			}
			t.Errorf("tile %v:\n got  %s\n want %s", pos, describeTile(gotTile), describeTile(wantTile))
		}
	}
}

func describeTile(tile *Tile) string {
	if tile == nil {
		return "nil"
	}
	text := fmt.Sprintf("%+v", *tile)
	if tile.Ext != nil {
		text += fmt.Sprintf(" ext %+v", *tile.Ext)
	}
	describe := func(item Item) {
		if item.Ext != nil {
			text += fmt.Sprintf(" item %d ext %+v", item.ID, *item.Ext)
		}
	}
	describe(tile.Ground)
	for _, item := range tile.Items {
		describe(item)
	}
	return text
}

func textSampleMap() *Map {
	m := sampleMap()
	m.Attributes = []CustomAttr{{Code: 28, Name: "Chest quest number", Type: "u16"}, {Code: 26, Name: "Door quest value", Type: "i32"}, {Code: 30, Name: "note", Type: "string"}}
	m.Zones = []Zone{{ID: 3, Name: "Arena"}, {ID: 1, Name: "Hunt"}}
	m.Waypoints = append(m.Waypoints, &Waypoint{Name: "depot \"main\"", Pos: Position{110, 100, 7}})
	m.ItemsMajor, m.ItemsMinor = 3, 20
	m.SetTile(Position{103, 100, 7}, &Tile{
		Ground: Item{ID: 100},
		Flags:  FlagNoLogout | FlagRefresh | 1<<1,
		Items: []Item{{ID: 1050, Ext: &ItemExt{
			Raw: []RawAttr{
				{Code: 26, Data: []byte{0xFF, 0xFF, 0xFF, 0xFF}},
				{Code: 28, Data: []byte{0x39, 0x05}},
				{Code: attrWrittenBy, Data: []byte{3, 0, 'S', 'a', 'm'}},
				{Code: attrWrittenDate, Data: []byte{1, 2, 3, 4}},
				{Code: 30, Data: []byte{2, 0, 0xE9, '\n'}},
			},
			Attrs: []Attr{
				{Key: "rarity", Type: AttrInteger, Int: -3},
				{Key: "Custom Key", Type: AttrString, Str: "a \"b\"\\"},
				{Key: "weight", Type: AttrDouble, Num: 2},
				{Key: "scale", Type: AttrFloat, Num: float64(float32(0.1))},
				{Key: "big", Type: AttrDouble, Num: 1e300},
				{Key: "on", Type: AttrBoolean, Value: true},
			},
		}}},
		Ext: &TileExt{Zones: []uint16{3, 1}},
	})
	m.SetTile(Position{104, 100, 7}, &Tile{Flags: FlagPZ})
	m.SetTile(Position{105, 100, 7}, &Tile{Items: []Item{{ID: 1050, Ext: &ItemExt{DepotID: 4, DoorID: 2}}}})
	npcCenter := m.Tile(Position{102, 101, 7}).Clone()
	npcCenter.EnsureExt().SpawnNPC = &Spawn{Radius: 2}
	m.SetTile(Position{102, 101, 7}, npcCenter)
	m.SetTile(Position{50, 60, 8}, &Tile{Ext: &TileExt{Creatures: []Creature{{Name: "Lost", SpawnTime: -1, Direction: 3, Weight: 5}}}})
	return m
}

func TestOTMAPRoundTrip(t *testing.T) {
	items := testItems()
	dir := t.TempDir()
	path := filepath.Join(dir, "map.otmap")
	original := textSampleMap()
	if _, err := Save(path, original, items, Format{}); err != nil {
		t.Fatal(err)
	}
	got, warns, err := Load(path, items, Format{})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	compareTiles(t, normalizedTiles(original, items), normalizedTiles(got, items))
	if got.Width != original.Width || got.Height != original.Height || got.Description != original.Description ||
		got.ItemsMajor != 3 || got.ItemsMinor != 20 || got.SpawnFormat != SpawnsSingle {
		t.Errorf("header %+v", got)
	}
	if !reflect.DeepEqual(got.Towns, original.Towns) || !reflect.DeepEqual(got.Houses, original.Houses) || !reflect.DeepEqual(got.Zones, []Zone{{1, "Hunt"}, {3, "Arena"}}) {
		t.Errorf("towns %v houses %v zones %v", got.Towns, got.Houses, got.Zones)
	}
	if len(got.Waypoints) != 2 || got.Waypoints[1].Name != "temple" || got.Waypoints[0].Name != "depot \"main\"" {
		t.Errorf("waypoints %v", got.Waypoints)
	}
	wantAttrs := slices.Clone(original.Attributes)
	slices.SortFunc(wantAttrs, func(left, right CustomAttr) int { return cmp.Compare(left.Code, right.Code) })
	if !reflect.DeepEqual(got.Attributes, wantAttrs) {
		t.Errorf("attributes %v, want %v", got.Attributes, wantAttrs)
	}

	// Saving what was read gives the same bytes.
	world, sectors, err := EncodeOTMAP(original, items)
	if err != nil {
		t.Fatal(err)
	}
	world2, sectors2, err := EncodeOTMAP(got, items)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(world, world2) || !reflect.DeepEqual(sectors, sectors2) {
		t.Fatalf("not canonical:\n%s\n%s", world, world2)
	}
	for name, data := range sectors {
		onDisk, err := os.ReadFile(filepath.Join(dir, sectorsDirName, name))
		if err != nil || !bytes.Equal(onDisk, data) {
			t.Fatalf("%s on disk differs: %v", name, err)
		}
	}
}

func TestOTMAPText(t *testing.T) {
	items := testItems()
	world, sectors, err := EncodeOTMAP(textSampleMap(), items)
	if err != nil {
		t.Fatal(err)
	}
	wantWorld := `otmap 1
items server
otb 3 20
size 2048 2048
description "test map"
attribute 26 door_quest_value i32 "Door quest value"
attribute 28 chest_quest_number u16 "Chest quest number"
attribute 30 note string
town 1 "Thais" temple=100,100,7
house 7 "Flat" town=1 exit=301,300,8 rent=500 guildhall
zone 1 "Hunt"
zone 3 "Arena"
waypoint "depot \"main\"" 110,100,7
waypoint "temple" 100,100,7
`
	if string(world) != wantWorld {
		t.Errorf("world file:\n%s\nwant:\n%s", world, wantWorld)
	}
	wantSector := `otmap-sector 1
100 100 7 [pz] 100
101 100 7 101(aid=1000) 1050 2148:37 1387(uid=5000 dest=200,201,6 text="hello\xFE" desc="desc") 1987{2148:3 2006:2}
103 100 7 [nologout refresh flags=2 zone=3,1] 100 1050(writer="Sam" written=67305985 door_quest_value=-1 chest_quest_number=1337 note="\xE9\n" @"Custom Key"="a \"b\"\\" @big=1e+300 @on=true @rarity=-3 @scale=0.1f @weight=2.0)
104 100 7 [pz] -
105 100 7 - 1050(depot=4 door=2)
102 101 7 100
spawn 100 100 7 radius=3
npcspawn 102 101 7 radius=2
monster 100 100 7 "Rat" spawntime=60
npc 102 101 7 "Guide" spawntime=30 dir=s
`
	if got := string(sectors["3_3.sector"]); got != wantSector {
		t.Errorf("sector 3_3:\n%s\nwant:\n%s", got, wantSector)
	}
	if got := string(sectors["1_1.sector"]); got != "otmap-sector 1\nmonster 50 60 8 \"Lost\" spawntime=-1 dir=w weight=5\n" {
		t.Errorf("sector 1_1:\n%s", got)
	}
}

func writeOTMAPFiles(t *testing.T, world string, sectors map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, sectorsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "map.otmap")
	if err := os.WriteFile(path, []byte(world), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, text := range sectors {
		if err := os.WriteFile(filepath.Join(dir, sectorsDirName, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestOTMAPLenientInput(t *testing.T) {
	items := testItems()
	path := writeOTMAPFiles(t, "otmap 1\r\nitems server\r\n\r\nhouse 7 \"Flat\" town=2 exit=1,1,7\r\n", map[string]string{
		"0_0.sector": "otmap-sector 1\r\n5 5 7\t100   1050\r\n\n1 1 7 [house=7 house=7] 100 1987{ 2148:3  }\r\nmonster 2 2 7 \"Rat\"\r\nspawn 1 1 7 radius=0\r\n2 2 7 [house=9] 100\r\n",
	})
	got, warns, err := Load(path, items, Format{})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(warns, "\n")
	for _, want := range []string{"not in order", "town 2", "house 9"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings %q lack %q", joined, want)
		}
	}
	if tile := got.Tile(Position{5, 5, 7}); tile == nil || tile.Ground.ID != 100 || len(tile.Items) != 1 {
		t.Errorf("tile 5,5,7: %+v", tile)
	}
	if tile := got.Tile(Position{1, 1, 7}); tile == nil || tile.Spawn() == nil || tile.Spawn().Radius != 1 || tile.Items[0].Ext.Items[0].Subtype != 3 {
		t.Errorf("tile 1,1,7: %+v", tile)
	}
	if got.Houses[9] == nil || got.Width != 6 || got.Height != 6 {
		t.Errorf("house 9 %v, size %dx%d", got.Houses[9], got.Width, got.Height)
	}
}

func TestOTMAPErrors(t *testing.T) {
	world := "otmap 1\nitems server\nattribute 28 chest u16\n"
	for _, test := range []struct{ world, sector, want string }{
		{"otmap 2\nitems server\n", "", "unsupported version"},
		{"otmap 1\n", "", "no items record"},
		{"otmap 1\nitems server\ntown 1 \"A\" temple=1,1,7\ntown 1 \"B\" temple=1,1,7\n", "", "map.otmap:4:1: town 1 defined twice"},
		{"otmap 1\nitems server\nattribute 4 aid u16\n", "", "standard OTBM attribute"},
		{"otmap 1\nitems server\nbogus 1\n", "", "unknown record"},
		{world, "otmap-sector 2\n", "expected the header"},
		{world, "otmap-sector 1\n40 1 7 100\n", "0_0.sector:2:1: 40,1,7 is outside sector 0_0"},
		{world, "otmap-sector 1\n1 1 16 100\n", "number out of range (0-15)"},
		{world, "otmap-sector 1\n1 1 7 100\n1 1 7 101\n", "0_0.sector:3:1: tile 1,1,7 written twice"},
		{world, "otmap-sector 1\n1 1 7 100(foo=1)\n", "unknown attribute \"foo\""},
		{world, "otmap-sector 1\n1 1 7 100(chest=70000)\n", "number out of range (0-65535)"},
		{world, "otmap-sector 1\n1 1 7 100(text=\"a)\n", "unterminated string"},
		{world, "otmap-sector 1\n1 1 7 0\n", "item id 0"},
		{world, "otmap-sector 1\n1 1 7 100 x\n", "expected a number"},
		{world, "otmap-sector 1\n1 1 7 100(aid=1)101\n", "expected a space"},
		{world, "otmap-sector 1\nspawn 1 1 7 radius=1\nspawn 1 1 7 radius=2\n", "two spawn areas"},
		{world, "otmap-sector 1\nnpc 1 1 7 \"A\" dir=up\n", "direction must be"},
		{world, "otmap-sector 1\n1 1 7 100(@x=1.5x)\n", "expected"},
	} {
		sectors := map[string]string{}
		if test.sector != "" {
			sectors["0_0.sector"] = test.sector
		}
		_, _, err := Load(writeOTMAPFiles(t, test.world, sectors), testItems(), Format{})
		if err == nil || !strings.Contains(filepath.ToSlash(err.Error()), test.want) {
			t.Errorf("%q / %q: error %v, want %q", test.world, test.sector, err, test.want)
		}
	}
}

func TestOTMAPRemovesStaleSectors(t *testing.T) {
	items := testItems()
	dir := t.TempDir()
	path := filepath.Join(dir, "map.otmap")
	m := New(2048, 2048)
	m.SetTile(Position{10, 10, 7}, &Tile{Ground: Item{ID: 100}})
	m.SetTile(Position{100, 10, 7}, &Tile{Ground: Item{ID: 100}})
	if _, err := Save(path, m, items, Format{}); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, sectorsDirName, "notes.txt")
	if err := os.WriteFile(other, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.SetTile(Position{100, 10, 7}, nil)
	if _, err := Save(path, m, items, Format{}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, sectorsDirName))
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !reflect.DeepEqual(names, []string{"0_0.sector", "notes.txt"}) {
		t.Fatalf("files %v", names)
	}
}

func TestOTBMKeepsAttributesFile(t *testing.T) {
	items := testItems()
	dir := t.TempDir()
	m := textSampleMap()
	// OTBM with one spawn file keeps neither NPC areas nor creatures
	// outside every area.
	m.SetTile(Position{50, 60, 8}, nil)
	npcCenter := m.Tile(Position{102, 101, 7}).Clone()
	npcCenter.Ext.SpawnNPC = nil
	m.SetTile(Position{102, 101, 7}, npcCenter)
	if _, err := Save(filepath.Join(dir, "map.otmap"), m, items, Format{}); err != nil {
		t.Fatal(err)
	}
	fromText, _, err := Load(filepath.Join(dir, "map.otmap"), items, Format{})
	if err != nil {
		t.Fatal(err)
	}
	otbmPath := filepath.Join(dir, "world.otbm")
	if _, err := Save(otbmPath, fromText, items, Format{}); err != nil {
		t.Fatal(err)
	}
	if !fsutilFileExists(filepath.Join(dir, "world"+attrsFileSuffix)) {
		t.Fatal("attributes file not written")
	}
	fromBinary, warns, err := Load(otbmPath, items, Format{})
	if err != nil || len(warns) != 0 {
		t.Fatalf("load: %v %v", warns, err)
	}
	compareTiles(t, normalizedTiles(fromText, items), normalizedTiles(fromBinary, items))
}

func fsutilFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// TestOTMAPRealMap converts a real map (skipped unless OTS_MAP is set):
//
//	OTS_MAP=path/map.otbm [OTS_ITEMS=path/items.otb] [OTS_OTMAP_OUT=dir] [OTS_COUNT_U16=1] go test -run TestOTMAPRealMap -v ./internal/mapping/otmap
//
// It reads the OTBM, writes OTMAP to a temporary directory, reads it back,
// compares every tile, and writes OTBM from the OTMAP map to compare again.
func TestOTMAPRealMap(t *testing.T) {
	path := os.Getenv("OTS_MAP")
	if path == "" {
		t.Skip("OTS_MAP not set")
	}
	var items Items
	if otbPath := os.Getenv("OTS_ITEMS"); otbPath != "" {
		data, err := os.ReadFile(otbPath)
		if err != nil {
			t.Fatal(err)
		}
		db, err := otitems.ParseOTB(data)
		if err != nil {
			t.Fatal(err)
		}
		items = db
	} else {
		types := make([]*otitems.Type, 0, 60000)
		for id := uint16(100); id < 60000; id++ {
			types = append(types, &otitems.Type{ID: id, ClientID: id})
		}
		items = otitems.NewDB(otitems.ModeOTB, types)
	}
	format := Format{CountU16: os.Getenv("OTS_COUNT_U16") != ""}
	start := time.Now()
	fromOTBM, warns, err := Load(path, items, format)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("OTBM load %v, %d warnings, %d tiles", time.Since(start), len(warns), fromOTBM.TileCount())
	if len(warns) > 0 {
		t.Logf("first OTBM warnings: %v", warns[:min(len(warns), 5)])
	}
	dir := os.Getenv("OTS_OTMAP_OUT")
	if dir == "" {
		dir = t.TempDir()
	}
	textPath := filepath.Join(dir, "map.otmap")
	start = time.Now()
	if _, err := Save(textPath, fromOTBM, items, Format{}); err != nil {
		t.Fatal(err)
	}
	t.Logf("OTMAP save %v", time.Since(start))
	start = time.Now()
	if _, err := Save(textPath, fromOTBM, items, Format{}); err != nil {
		t.Fatal(err)
	}
	t.Logf("OTMAP save without changes %v", time.Since(start))
	var files, size, largest int64
	entries, _ := os.ReadDir(filepath.Join(dir, sectorsDirName))
	for _, entry := range entries {
		info, _ := entry.Info()
		files++
		size += info.Size()
		largest = max(largest, info.Size())
	}
	otbmInfo, _ := os.Stat(path)
	t.Logf("OTMAP: %d sector files, %.1f MB (OTBM %.1f MB), largest sector %.0f KB", files, float64(size)/1e6, float64(otbmInfo.Size())/1e6, float64(largest)/1e3)
	start = time.Now()
	fromText, textWarns, err := Load(textPath, items, Format{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("OTMAP load %v, warnings %v", time.Since(start), textWarns)
	want := normalizedTiles(fromOTBM, items)
	compareTiles(t, want, normalizedTiles(fromText, items))
	if !reflect.DeepEqual(fromOTBM.Houses, fromText.Houses) || !reflect.DeepEqual(fromOTBM.Towns, fromText.Towns) {
		t.Errorf("houses or towns differ")
	}
	otbmCopy := filepath.Join(dir, "copy.otbm")
	if _, err := Save(otbmCopy, fromText, items, format); err != nil {
		t.Fatal(err)
	}
	fromCopy, _, err := Load(otbmCopy, items, format)
	if err != nil {
		t.Fatal(err)
	}
	compareTiles(t, want, normalizedTiles(fromCopy, items))
}

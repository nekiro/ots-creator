package otmap

import (
	"reflect"
	"strings"
	"testing"
)

func TestHousesRoundTrip(t *testing.T) {
	source := sampleMap()
	source.Houses[9] = &House{ID: 9, Name: "Shop & Co", TownID: 1, Rent: 100, Exit: Position{5, 6, 7}, ClientID: 42, Beds: 2}
	data := WriteHouses(source)
	if !strings.Contains(string(data), `size="1"`) || !strings.Contains(string(data), "Shop &amp; Co") {
		t.Fatalf("houses.xml:\n%s", data)
	}

	loaded := New(2048, 2048)
	warnings, err := ReadHouses(loaded, data)
	if err != nil || len(warnings) != 0 {
		t.Fatal(err, warnings)
	}
	if !reflect.DeepEqual(loaded.SortedHouses(), source.SortedHouses()) {
		t.Fatalf("houses %+v, want %+v", loaded.SortedHouses()[0], source.SortedHouses()[0])
	}
}

func TestReadHousesDefaults(t *testing.T) {
	m := New(100, 100)
	m.Houses[3] = &House{ID: 3, Name: "old"}
	warnings, err := ReadHouses(m, []byte(`<houses><house name="" houseid="3" guildhall="1"/><house name="x"/><other/></houses>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings %v", warnings)
	}
	if house := m.Houses[3]; house.Name != "House #3" || !house.Guildhall || len(m.Houses) != 1 {
		t.Fatalf("house %+v", house)
	}
	if _, err := ReadHouses(m, []byte("<houses")); err == nil {
		t.Fatal("broken xml must fail")
	}
}

func TestHouseSizes(t *testing.T) {
	m := New(100, 100)
	m.SetTile(Position{1, 1, 7}, &Tile{Ground: Item{ID: 100}, HouseID: 4})
	m.SetTile(Position{2, 1, 7}, &Tile{Ground: Item{ID: 100}, HouseID: 4})
	m.SetTile(Position{60, 1, 7}, &Tile{Ground: Item{ID: 100}, HouseID: 5})
	m.SetTile(Position{3, 1, 7}, &Tile{Ground: Item{ID: 100}})
	if sizes := HouseSizes(m); !reflect.DeepEqual(sizes, map[uint32]int{4: 2, 5: 1}) {
		t.Fatalf("sizes %v", sizes)
	}
}

func TestSpawnsRoundTrip(t *testing.T) {
	source := sampleMap()
	monsters, npcs, unsaved := WriteSpawns(source)
	if npcs != nil || unsaved != 0 {
		t.Fatalf("npcs %v unsaved %d", npcs, unsaved)
	}
	loaded := New(2048, 2048)
	warnings, err := ReadSpawns(loaded, monsters, false)
	if err != nil || len(warnings) != 0 {
		t.Fatal(err, warnings)
	}
	center := loaded.Tile(Position{100, 100, 7})
	if center.Spawn() == nil || center.Spawn().Radius != 3 {
		t.Fatalf("center %+v", center)
	}
	if creatures := center.Creatures(); len(creatures) != 1 || creatures[0].Name != "Rat" || creatures[0].SpawnTime != 60 {
		t.Fatalf("rat %+v", creatures)
	}
	guide := loaded.Tile(Position{102, 101, 7}).Creatures()
	if len(guide) != 1 || !guide[0].NPC || guide[0].Direction != 2 {
		t.Fatalf("guide %+v", guide)
	}
	// Written again, the files are the same.
	again, _, _ := WriteSpawns(loaded)
	if string(again) != string(monsters) {
		t.Fatalf("second write differs:\n%s\n%s", monsters, again)
	}
}

func TestReadSpawnsWarnings(t *testing.T) {
	m := New(100, 100)
	data := `<spawns>
		<spawn centerx="0" centery="5" centerz="7" radius="1"/>
		<spawn centerx="10" centery="10" centerz="7" radius="0">
			<monster name="" x="0" y="0"/>
			<monster name="Rat" x="-20" y="0"/>
			<monster name="Bat" x="1" y="0" z="6" direction="9" weight="5"/>
			<item name="ignored"/>
		</spawn>
		<spawn centerx="10" centery="10" centerz="7" radius="2"/>
		<other/>
	</spawns>`
	warnings, err := ReadSpawns(m, []byte(data), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 3 {
		t.Fatalf("warnings %v", warnings)
	}
	if spawn := m.Tile(Position{10, 10, 7}).Spawn(); spawn == nil || spawn.Radius != 1 {
		t.Fatalf("spawn %+v", spawn)
	}
	bat := m.Tile(Position{11, 10, 6}).Creatures()
	if len(bat) != 1 || bat[0].Direction != 3 || bat[0].Weight != 5 {
		t.Fatalf("bat %+v", bat)
	}

	npcData := `<npcs><npc centerx="10" centery="10" centerz="7" radius="2"><npc name="Guide" x="0" y="1"/></npc>` +
		`<npc centerx="10" centery="10" centerz="7" radius="2"/></npcs>`
	warnings, err = ReadSpawns(m, []byte(npcData), true)
	if err != nil || len(warnings) != 1 {
		t.Fatal(err, warnings)
	}
	if m.Tile(Position{10, 10, 7}).SpawnNPC() == nil || !m.Tile(Position{10, 11, 7}).Creatures()[0].NPC {
		t.Fatal("npc spawn not read")
	}
	if _, err := ReadSpawns(m, []byte("<x"), false); err == nil {
		t.Fatal("broken xml must fail")
	}
}

func TestZonesRoundTrip(t *testing.T) {
	source := New(10, 10)
	source.Zones = []Zone{{ID: 1, Name: "arena"}, {ID: 7, Name: "quest <1>"}}
	loaded := New(10, 10)
	if err := ReadZones(loaded, WriteZones(source)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Zones, source.Zones) {
		t.Fatalf("zones %+v", loaded.Zones)
	}
	if err := ReadZones(loaded, []byte("<zones")); err == nil {
		t.Fatal("broken xml must fail")
	}
}

func TestBounds(t *testing.T) {
	m := New(1000, 1000)
	if _, _, _, _, ok := m.Bounds(); ok {
		t.Fatal("empty map has no bounds")
	}
	m.SetTile(Position{50, 70, 7}, &Tile{Ground: Item{ID: 100}})
	m.SetTile(Position{10, 300, 6}, &Tile{Ground: Item{ID: 100}})
	m.SetTile(Position{400, 20, 8}, &Tile{Ground: Item{ID: 100}})
	minX, minY, maxX, maxY, ok := m.Bounds()
	if !ok || minX != 10 || minY != 20 || maxX != 400 || maxY != 300 {
		t.Fatalf("bounds %d,%d %d,%d", minX, minY, maxX, maxY)
	}
}

func TestSortedTownsAndHouses(t *testing.T) {
	m := New(10, 10)
	for _, id := range []uint32{5, 1, 3} {
		m.Towns[id] = &Town{ID: id}
		m.Houses[id*10] = &House{ID: id * 10}
	}
	var townIDs, houseIDs []uint32
	for _, town := range m.SortedTowns() {
		townIDs = append(townIDs, town.ID)
	}
	for _, house := range m.SortedHouses() {
		houseIDs = append(houseIDs, house.ID)
	}
	if !reflect.DeepEqual(townIDs, []uint32{1, 3, 5}) || !reflect.DeepEqual(houseIDs, []uint32{10, 30, 50}) {
		t.Fatalf("towns %v houses %v", townIDs, houseIDs)
	}
}

func TestPruneAndTouch(t *testing.T) {
	m := New(100, 100)
	pos := Position{5, 5, 7}
	m.Put(pos, &Tile{})
	if !m.Has(pos) || m.TileCount() != 1 {
		t.Fatal("Put must keep an empty tile")
	}
	key := KeyOf(pos)
	before := m.ChunkRev(key)
	m.Touch(pos)
	if m.ChunkRev(key) <= before {
		t.Fatal("Touch must raise the chunk revision")
	}
	m.Touch(Position{90, 90, 7}) // no chunk: nothing happens
	m.Prune(pos)
	if m.Has(pos) || m.TileCount() != 0 || m.Chunk(key) != nil || m.ChunkRev(key) != 0 {
		t.Fatal("Prune must drop an empty tile")
	}

	m.SetTile(pos, &Tile{Ground: Item{ID: 100}})
	m.Prune(pos)
	if !m.Has(pos) {
		t.Fatal("Prune must keep a tile with a ground")
	}
}

func TestChunkPeek(t *testing.T) {
	m := New(100, 100)
	m.SetTile(Position{3, 4, 7}, &Tile{Ground: Item{ID: 100}, Items: []Item{{ID: 200}}})
	m.SetTile(Position{5, 4, 7}, &Tile{Ground: Item{ID: 101}, Flags: FlagPZ})
	chunk := m.Chunk(KeyOf(Position{3, 4, 7}))
	var scratch Tile
	buffer := make([]Item, 4)
	if chunk.Peek(0, 0, &scratch, buffer) != nil {
		t.Fatal("no tile at 0,0")
	}
	packed := chunk.Peek(3, 4, &scratch, buffer)
	if packed != &scratch || packed.Ground.ID != 100 || len(packed.Items) != 1 || &packed.Items[0] != &buffer[0] {
		t.Fatalf("packed tile %+v", packed)
	}
	if stored := chunk.Peek(5, 4, &scratch, buffer); stored != m.Tile(Position{5, 4, 7}) || stored.Flags != FlagPZ {
		t.Fatalf("stored tile %+v", stored)
	}
}

func TestEachInAndWaypoint(t *testing.T) {
	m := sampleMap()
	var found []Position
	m.EachIn(-5, -5, 101, 100, 0, 7, func(pos Position, _ *Tile) { found = append(found, pos) })
	if !reflect.DeepEqual(found, []Position{{100, 100, 7}, {101, 100, 7}}) {
		t.Fatalf("found %v", found)
	}
	if m.Waypoint("temple") == nil || m.Waypoint("none") != nil {
		t.Fatal("waypoint lookup")
	}
	if m.At(-1, 0, 7) != nil || m.At(100, 100, 7) == nil {
		t.Fatal("At")
	}
}

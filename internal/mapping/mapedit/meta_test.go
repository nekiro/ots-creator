package mapedit

import (
	"reflect"
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func metaLastLabel(e *Editor) string {
	undo, _ := e.History()
	if len(undo) == 0 {
		return ""
	}
	return undo[len(undo)-1].Label
}

func metaWaypointNames(e *Editor) []string {
	var names []string
	for _, waypoint := range e.Map.Waypoints {
		names = append(names, waypoint.Name)
	}
	return names
}

func TestPropertiesReturnsMapHeader(t *testing.T) {
	e := testEditor(t)
	e.Map.Description, e.Map.HouseFile, e.Map.SpawnFile, e.Map.NpcFile, e.Map.ZoneFile = "desc", "h.xml", "s.xml", "n.xml", "z.xml"
	e.Map.SpawnFormat = otmap.SpawnsSeparate

	want := Properties{Description: "desc", Width: 1000, Height: 1000, HouseFile: "h.xml", SpawnFile: "s.xml", NpcFile: "n.xml", ZoneFile: "z.xml", SpawnFormat: otmap.SpawnsSeparate}
	if got := e.Properties(); got != want {
		t.Fatalf("Properties() = %+v, want %+v", got, want)
	}
}

func TestSetPropertiesStoresAllFields(t *testing.T) {
	e := testEditor(t)
	props := Properties{Description: "new", Width: 2000, Height: 1500, HouseFile: "h.xml", SpawnFile: "s.xml", NpcFile: "n.xml", ZoneFile: "z.xml", SpawnFormat: otmap.SpawnsSeparate}

	if err := e.SetProperties(props); err != nil {
		t.Fatal(err)
	}
	if got := e.Properties(); got != props {
		t.Fatalf("Properties() = %+v, want %+v", got, props)
	}
}

func TestSetPropertiesAppliesDefaults(t *testing.T) {
	e := testEditor(t)
	if err := e.SetProperties(Properties{Description: "test", Width: 0, Height: 0, SpawnFormat: "bogus"}); err != nil {
		t.Fatal(err)
	}
	want := Properties{Description: "test", Width: 1, Height: 1, SpawnFormat: otmap.SpawnsSingle}
	if got := e.Properties(); got != want {
		t.Fatalf("Properties() = %+v, want %+v (size at least 1, unknown spawn format becomes single)", got, want)
	}
}

func TestSetPropertiesIsUndoable(t *testing.T) {
	e := testEditor(t)
	before := e.Properties()
	e.SetProperties(Properties{Description: "changed", Width: 5, Height: 5})

	if label := e.Undo(); label != "Map properties" {
		t.Fatalf("undo label = %q, want %q", label, "Map properties")
	}
	if got := e.Properties(); got != before {
		t.Fatalf("after undo Properties() = %+v, want %+v", got, before)
	}
}

func TestSaveHouseNewGetsFirstFreeID(t *testing.T) {
	e := testEditor(t)
	e.Map.Houses[1] = &otmap.House{ID: 1, Name: "One"}
	e.Map.Houses[3] = &otmap.House{ID: 3, Name: "Three"}

	id, err := e.SaveHouse(otmap.House{Name: "New", Rent: 100})
	if err != nil || id != 2 {
		t.Fatalf("SaveHouse(new) = %d, %v, want id 2 (the lowest free id)", id, err)
	}
	if got := e.Map.Houses[2]; got == nil || got.Name != "New" || got.Rent != 100 || got.ID != 2 {
		t.Fatalf("house 2 = %+v, want the new house with id 2", got)
	}
	if label := metaLastLabel(e); label != "Add house" {
		t.Fatalf("undo label = %q, want %q", label, "Add house")
	}
}

func TestSaveHouseWithIDUpdatesIt(t *testing.T) {
	e := testEditor(t)
	e.Map.Houses[4] = &otmap.House{ID: 4, Name: "Old", Rent: 10}

	id, err := e.SaveHouse(otmap.House{ID: 4, Name: "Renamed", Rent: 20, TownID: 2})
	if err != nil || id != 4 {
		t.Fatalf("SaveHouse(id 4) = %d, %v, want id 4", id, err)
	}
	want := otmap.House{ID: 4, Name: "Renamed", Rent: 20, TownID: 2}
	if got := *e.Map.Houses[4]; got != want || len(e.Map.Houses) != 1 {
		t.Fatalf("house 4 = %+v (houses %d), want %+v and no new house", got, len(e.Map.Houses), want)
	}
	if label := metaLastLabel(e); label != "Edit house" {
		t.Fatalf("undo label = %q, want %q", label, "Edit house")
	}
}

func TestSaveHouseRequiresName(t *testing.T) {
	e := testEditor(t)
	id, err := e.SaveHouse(otmap.House{Name: "  "})
	if id != 0 || errText(err) != "the house needs a name" {
		t.Fatalf("SaveHouse(blank name) = %d, %v, want 0 and %q", id, err, "the house needs a name")
	}
	if len(e.Map.Houses) != 0 || e.CanUndo() {
		t.Fatal("a rejected house was stored or left an undo step")
	}
}

func TestSaveHouseUndoRemovesNewHouse(t *testing.T) {
	e := testEditor(t)
	e.SaveHouse(otmap.House{Name: "New"})
	e.Undo()
	if len(e.Map.Houses) != 0 {
		t.Fatalf("houses after undo = %v, want none", e.Map.Houses)
	}
}

func TestRemoveHouseDeletesItAndClearsItsTiles(t *testing.T) {
	e := testEditor(t)
	e.Map.Houses[1] = &otmap.House{ID: 1, Name: "Gone"}
	e.Map.Houses[2] = &otmap.House{ID: 2, Name: "Kept"}
	e.Map.SetTile(at(1, 1), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 1, Flags: otmap.FlagPZ | otmap.FlagNoLogout})
	e.Map.SetTile(at(2, 1), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 2, Flags: otmap.FlagPZ})

	e.RemoveHouse(1)
	if e.Map.Houses[1] != nil || e.Map.Houses[2] == nil {
		t.Fatalf("houses = %v, want only house 2 left", e.Map.Houses)
	}
	cleared := e.Map.Tile(at(1, 1))
	if cleared.HouseID != 0 || cleared.Flags != otmap.FlagNoLogout {
		t.Fatalf("tile of the removed house: house=%d flags=%d, want house 0 and only the PZ flag cleared", cleared.HouseID, cleared.Flags)
	}
	if kept := e.Map.Tile(at(2, 1)); kept.HouseID != 2 || kept.Flags != otmap.FlagPZ {
		t.Fatalf("tile of house 2: house=%d flags=%d, want it untouched", kept.HouseID, kept.Flags)
	}
}

func TestRemoveHouseUndoRestoresHouseAndTiles(t *testing.T) {
	e := testEditor(t)
	e.Map.Houses[1] = &otmap.House{ID: 1, Name: "House"}
	e.Map.SetTile(at(1, 1), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 1, Flags: otmap.FlagPZ})

	e.RemoveHouse(1)
	if label := e.Undo(); label != "Remove house" {
		t.Fatalf("undo label = %q, want %q", label, "Remove house")
	}
	if e.Map.Houses[1] == nil || e.Map.Tile(at(1, 1)).HouseID != 1 || e.Map.Tile(at(1, 1)).Flags != otmap.FlagPZ {
		t.Fatal("undo did not bring back the house and its PZ tile")
	}
}

func TestHouseTilesListsTilesOfHouse(t *testing.T) {
	e := testEditor(t)
	for _, tile := range []struct {
		pos   otmap.Position
		house uint32
	}{{at(1, 1), 1}, {at(2, 1), 2}, {at(1, 2), 1}, {posAt(1, 1, 6), 1}} {
		e.Map.SetTile(tile.pos, &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: tile.house})
	}

	want := []otmap.Position{posAt(1, 1, 6), at(1, 1), at(1, 2)}
	if got := e.HouseTiles(1); !reflect.DeepEqual(got, want) {
		t.Fatalf("HouseTiles(1) = %v, want %v", got, want)
	}
	if got := e.HouseTiles(9); got != nil {
		t.Fatalf("HouseTiles(9) = %v, want none", got)
	}
}

func TestHousesListsHousesByIDWithSizes(t *testing.T) {
	e := testEditor(t)
	e.Map.Houses[2] = &otmap.House{ID: 2, Name: "Two"}
	e.Map.Houses[1] = &otmap.House{ID: 1, Name: "One"}
	e.Map.SetTile(at(1, 1), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 2})
	e.Map.SetTile(at(2, 1), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 2})

	want := []HouseInfo{{House: otmap.House{ID: 1, Name: "One"}, Size: 0}, {House: otmap.House{ID: 2, Name: "Two"}, Size: 2}}
	if got := e.Houses(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Houses() = %+v, want %+v", got, want)
	}
}

func TestSaveTownNewGetsFirstFreeID(t *testing.T) {
	e := testEditor(t)
	e.Map.Towns[1] = &otmap.Town{ID: 1, Name: "One"}

	id, err := e.SaveTown(otmap.Town{Name: "Two", Temple: at(5, 5)})
	if err != nil || id != 2 {
		t.Fatalf("SaveTown(new) = %d, %v, want id 2", id, err)
	}
	want := otmap.Town{ID: 2, Name: "Two", Temple: at(5, 5)}
	if got := e.Map.Towns[2]; got == nil || *got != want {
		t.Fatalf("town 2 = %+v, want %+v", got, want)
	}
}

func TestSaveTownNewFillsFirstGapInIDs(t *testing.T) {
	cases := []struct {
		name     string
		existing []uint32
		want     uint32
	}{
		{"no towns", nil, 1},
		{"gap after the first", []uint32{1, 3}, 2},
		{"gap at the start", []uint32{2, 3}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			for _, id := range tc.existing {
				e.Map.Towns[id] = &otmap.Town{ID: id, Name: "Existing"}
			}
			id, err := e.SaveTown(otmap.Town{Name: "New"})
			if err != nil || id != tc.want {
				t.Fatalf("SaveTown(new) = %d, %v, want id %d", id, err, tc.want)
			}
		})
	}
}

func TestSaveTownWithIDUpdatesIt(t *testing.T) {
	e := testEditor(t)
	e.Map.Towns[3] = &otmap.Town{ID: 3, Name: "Old"}

	id, err := e.SaveTown(otmap.Town{ID: 3, Name: "New", Temple: at(7, 7)})
	if err != nil || id != 3 {
		t.Fatalf("SaveTown(id 3) = %d, %v, want 3", id, err)
	}
	if got := *e.Map.Towns[3]; got.Name != "New" || got.Temple != at(7, 7) || len(e.Map.Towns) != 1 {
		t.Fatalf("town 3 = %+v, want it renamed and moved, no new town", got)
	}
}

func TestSaveTownRequiresName(t *testing.T) {
	e := testEditor(t)
	if id, err := e.SaveTown(otmap.Town{Name: ""}); id != 0 || errText(err) != "the town needs a name" {
		t.Fatalf("SaveTown(no name) = %d, %v, want 0 and %q", id, err, "the town needs a name")
	}
	if len(e.Map.Towns) != 0 {
		t.Fatal("a rejected town was stored")
	}
}

func TestRemoveTownKeepsHouseTownIDs(t *testing.T) {
	e := testEditor(t)
	e.Map.Towns[1] = &otmap.Town{ID: 1, Name: "Town"}
	e.Map.Houses[1] = &otmap.House{ID: 1, Name: "House", TownID: 1}

	e.RemoveTown(1)
	if len(e.Map.Towns) != 0 {
		t.Fatalf("towns = %v, want none", e.Map.Towns)
	}
	if e.Map.Houses[1].TownID != 1 {
		t.Fatalf("house town = %d, want 1 kept", e.Map.Houses[1].TownID)
	}
	if e.Undo(); e.Map.Towns[1] == nil {
		t.Fatal("undo did not bring the town back")
	}
}

func TestSaveWaypointAddsTrimmedWaypoint(t *testing.T) {
	e := testEditor(t)
	if err := e.SaveWaypoint("", otmap.Waypoint{Name: " start ", Pos: at(5, 5)}); err != nil {
		t.Fatal(err)
	}
	if got := e.Map.Waypoint("start"); got == nil || got.Pos != at(5, 5) {
		t.Fatalf("waypoint start = %+v, want it at 5,5 with the name trimmed", got)
	}
}

func TestSaveWaypointRejectsDuplicateName(t *testing.T) {
	e := testEditor(t)
	e.SaveWaypoint("", otmap.Waypoint{Name: "start", Pos: at(5, 5)})
	e.SaveWaypoint("", otmap.Waypoint{Name: "other", Pos: at(6, 6)})

	cases := []struct {
		name    string
		oldName string
	}{
		{"new waypoint", ""},
		{"renamed waypoint", "other"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := e.SaveWaypoint(tc.oldName, otmap.Waypoint{Name: "start", Pos: at(9, 9)})
			if want := `a waypoint named "start" exists`; errText(err) != want {
				t.Fatalf("SaveWaypoint = %v, want %q", err, want)
			}
			if got := metaWaypointNames(e); !slices.Equal(got, []string{"start", "other"}) || e.Map.Waypoint("start").Pos != at(5, 5) {
				t.Fatalf("waypoints = %v, want them unchanged", got)
			}
		})
	}
}

func TestSaveWaypointRequiresName(t *testing.T) {
	e := testEditor(t)
	if err := e.SaveWaypoint("", otmap.Waypoint{Name: "   "}); errText(err) != "the waypoint needs a name" {
		t.Fatalf("SaveWaypoint(blank) = %v, want %q", err, "the waypoint needs a name")
	}
}

func TestSaveWaypointRenamesAndMoves(t *testing.T) {
	e := testEditor(t)
	e.SaveWaypoint("", otmap.Waypoint{Name: "start", Pos: at(5, 5)})

	if err := e.SaveWaypoint("start", otmap.Waypoint{Name: "begin", Pos: at(8, 8)}); err != nil {
		t.Fatal(err)
	}
	want := []*otmap.Waypoint{{Name: "begin", Pos: at(8, 8)}}
	if !reflect.DeepEqual(e.Map.Waypoints, want) {
		t.Fatalf("waypoints = %v, want only begin at 8,8", metaWaypointNames(e))
	}
}

func TestSaveWaypointChangesCaseOfOwnName(t *testing.T) {
	e := testEditor(t)
	e.SaveWaypoint("", otmap.Waypoint{Name: "start", Pos: at(5, 5)})

	if err := e.SaveWaypoint("start", otmap.Waypoint{Name: "Start", Pos: at(5, 5)}); err != nil {
		t.Fatalf("renaming start to Start = %v, want no duplicate error", err)
	}
	if got := metaWaypointNames(e); !slices.Equal(got, []string{"Start"}) {
		t.Fatalf("waypoints = %v, want [Start]", got)
	}
}

func TestSaveWaypointWithUnknownOldNameAddsIt(t *testing.T) {
	e := testEditor(t)
	if err := e.SaveWaypoint("missing", otmap.Waypoint{Name: "new", Pos: at(5, 5)}); err != nil {
		t.Fatal(err)
	}
	if got := metaWaypointNames(e); !slices.Equal(got, []string{"new"}) {
		t.Fatalf("waypoints = %v, want [new]", got)
	}
}

func TestRemoveWaypointDeletesByName(t *testing.T) {
	e := testEditor(t)
	e.SaveWaypoint("", otmap.Waypoint{Name: "a", Pos: at(5, 5)})
	e.SaveWaypoint("", otmap.Waypoint{Name: "b", Pos: at(6, 6)})

	e.RemoveWaypoint("a")
	if got := metaWaypointNames(e); !slices.Equal(got, []string{"b"}) {
		t.Fatalf("waypoints = %v, want [b]", got)
	}
	if label := e.Undo(); label != "Remove waypoint" || !slices.Equal(metaWaypointNames(e), []string{"a", "b"}) {
		t.Fatalf("undo %q left waypoints %v, want both back", label, metaWaypointNames(e))
	}
}

func TestSaveZoneNewGetsFirstFreeID(t *testing.T) {
	e := testEditor(t)
	e.Map.Zones = []otmap.Zone{{ID: 1, Name: "one"}, {ID: 3, Name: "three"}}

	id, err := e.SaveZone(otmap.Zone{Name: "arena"})
	if err != nil || id != 2 {
		t.Fatalf("SaveZone(new) = %d, %v, want id 2", id, err)
	}
	want := []otmap.Zone{{ID: 1, Name: "one"}, {ID: 3, Name: "three"}, {ID: 2, Name: "arena"}}
	if !reflect.DeepEqual(e.Map.Zones, want) {
		t.Fatalf("zones = %+v, want %+v", e.Map.Zones, want)
	}
}

func TestSaveZoneWithIDRenamesIt(t *testing.T) {
	e := testEditor(t)
	e.Map.Zones = []otmap.Zone{{ID: 1, Name: "one"}, {ID: 2, Name: "two"}}

	if id, err := e.SaveZone(otmap.Zone{ID: 2, Name: "renamed"}); err != nil || id != 2 {
		t.Fatalf("SaveZone(id 2) = %d, %v, want 2", id, err)
	}
	want := []otmap.Zone{{ID: 1, Name: "one"}, {ID: 2, Name: "renamed"}}
	if !reflect.DeepEqual(e.Map.Zones, want) {
		t.Fatalf("zones = %+v, want %+v", e.Map.Zones, want)
	}
}

func TestSaveZoneWithUnknownIDAddsIt(t *testing.T) {
	e := testEditor(t)
	if id, err := e.SaveZone(otmap.Zone{ID: 7, Name: "seven"}); err != nil || id != 7 {
		t.Fatalf("SaveZone(id 7) = %d, %v, want 7", id, err)
	}
	if want := []otmap.Zone{{ID: 7, Name: "seven"}}; !reflect.DeepEqual(e.Map.Zones, want) {
		t.Fatalf("zones = %+v, want %+v", e.Map.Zones, want)
	}
}

func TestSaveZoneRequiresName(t *testing.T) {
	e := testEditor(t)
	if id, err := e.SaveZone(otmap.Zone{Name: " "}); id != 0 || errText(err) != "the zone needs a name" {
		t.Fatalf("SaveZone(blank) = %d, %v, want 0 and %q", id, err, "the zone needs a name")
	}
}

func TestRemoveZoneDeletesItAndClearsTiles(t *testing.T) {
	e := testEditor(t)
	e.Map.Zones = []otmap.Zone{{ID: 1, Name: "one"}, {ID: 2, Name: "two"}}
	e.Map.SetTile(at(1, 1), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Zones: []uint16{1, 2}}})
	e.Map.SetTile(at(2, 1), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Zones: []uint16{1}}})

	e.RemoveZone(1)
	if want := []otmap.Zone{{ID: 2, Name: "two"}}; !reflect.DeepEqual(e.Map.Zones, want) {
		t.Fatalf("zones = %+v, want %+v", e.Map.Zones, want)
	}
	if got := e.Map.Tile(at(1, 1)).Zones(); !slices.Equal(got, []uint16{2}) {
		t.Fatalf("zones of 1,1 = %v, want [2]", got)
	}
	if ext := e.Map.Tile(at(2, 1)).Ext; ext != nil {
		t.Fatalf("ext of 2,1 = %+v, want it dropped with its only zone", ext)
	}
}

func metaSource() *otmap.Map {
	source := otmap.New(100, 100)
	source.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: []otmap.Item{{ID: stoneID}}})
	return source
}

func TestImportPlacesTilesAtOffset(t *testing.T) {
	e := testEditor(t)
	changed, err := e.Import(metaSource(), ImportOptions{DX: 5, DY: 6, DZ: -1})
	if changed != 1 || err != nil {
		t.Fatalf("Import = %d, %v, want 1 changed tile and no error", changed, err)
	}
	if got := toolsGroundAndItems(e.Map.Tile(posAt(15, 16, 6))); !slices.Equal(got, []uint16{grassA, stoneID}) {
		t.Fatalf("tile at 15,16,6 = %v, want the imported grass and stone", got)
	}
	if e.Map.Tile(at(10, 10)) != nil {
		t.Fatal("a tile was imported without the offset")
	}
}

func TestImportSkipsTilesOutsideMapAndReportsIt(t *testing.T) {
	e := testEditor(t)
	source := metaSource()
	source.SetTile(at(50, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}})

	changed, err := e.Import(source, ImportOptions{DX: -20})
	if want := "some tiles were outside the map and were skipped"; changed != 1 || errText(err) != want {
		t.Fatalf("Import = %d, %v, want 1 and %q", changed, err, want)
	}
	if got := toolsGroundAndItems(e.Map.Tile(at(30, 10))); !slices.Equal(got, []uint16{sandID}) {
		t.Fatalf("tile at 30,10 = %v, want the sand that fit", got)
	}
}

func TestImportMergesIntoExistingTile(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: bagID}}, Flags: otmap.FlagNoPVP,
		Ext: &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Rat"}}, Zones: []uint16{1}}})
	source := otmap.New(100, 100)
	source.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: []otmap.Item{{ID: borderN}, {ID: stoneID}}, Flags: otmap.FlagPZ,
		Ext: &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Wolf"}}, Spawn: &otmap.Spawn{Radius: 2}, SpawnNPC: &otmap.Spawn{Radius: 1}, Zones: []uint16{2}}})

	e.Import(source, ImportOptions{})
	tile := e.Map.Tile(at(10, 10))
	if got := toolsGroundAndItems(tile); !slices.Equal(got, []uint16{grassA, borderN, bagID, stoneID}) {
		t.Fatalf("merged tile = %v, want grass ground, the border below and the items stacked", got)
	}
	if tile.Flags != otmap.FlagNoPVP|otmap.FlagPZ {
		t.Fatalf("flags = %d, want both flags", tile.Flags)
	}
	if want := []otmap.Creature{{Name: "Rat"}, {Name: "Wolf"}}; !reflect.DeepEqual(tile.Creatures(), want) {
		t.Fatalf("creatures = %+v, want Rat and Wolf", tile.Creatures())
	}
	if tile.Spawn() == nil || tile.Spawn().Radius != 2 || tile.SpawnNPC() == nil || tile.SpawnNPC().Radius != 1 {
		t.Fatalf("spawns = %+v / %+v, want the imported spawns", tile.Spawn(), tile.SpawnNPC())
	}
	if !slices.Equal(tile.Zones(), []uint16{1, 2}) {
		t.Fatalf("zones = %v, want [1 2]", tile.Zones())
	}
}

func TestImportMergeKeepsGroundWhenSourceHasNone(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
	source := otmap.New(100, 100)
	source.SetTile(at(10, 10), &otmap.Tile{Items: []otmap.Item{{ID: stoneID}}})

	e.Import(source, ImportOptions{})
	if got := toolsGroundAndItems(e.Map.Tile(at(10, 10))); !slices.Equal(got, []uint16{sandID, stoneID}) {
		t.Fatalf("tile = %v, want the sand kept and the stone added", got)
	}
}

func TestImportReplaceOverwritesExistingTile(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: bagID}}, Flags: otmap.FlagNoPVP})

	e.Import(metaSource(), ImportOptions{Replace: true})
	tile := e.Map.Tile(at(10, 10))
	if got := toolsGroundAndItems(tile); !slices.Equal(got, []uint16{grassA, stoneID}) || tile.Flags != 0 {
		t.Fatalf("tile = %v flags=%d, want only the imported grass and stone", got, tile.Flags)
	}
}

func TestImportedTilesAreModifiedAndNotSelected(t *testing.T) {
	e := testEditor(t)
	source := metaSource()
	selected := source.Tile(at(10, 10)).Clone()
	selected.SetSelected(true)
	source.SetTile(at(10, 10), selected)

	e.Import(source, ImportOptions{})
	tile := e.Map.Tile(at(10, 10))
	if tile.Selected() || e.SelectionCount() != 0 {
		t.Fatal("imported tile is selected, want the selection dropped")
	}
	if tile.State&otmap.StateModified == 0 {
		t.Fatal("imported tile is not marked modified")
	}
}

func metaHouseSource() *otmap.Map {
	source := otmap.New(100, 100)
	source.Houses[1] = &otmap.House{ID: 1, Name: "Theirs", Exit: at(10, 11)}
	source.Houses[2] = &otmap.House{ID: 2, Name: "Second"}
	source.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 1})
	source.SetTile(at(11, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 2})
	return source
}

func TestImportHousesMerge(t *testing.T) {
	for _, mode := range []string{"merge", ""} {
		t.Run("mode "+mode, func(t *testing.T) {
			e := testEditor(t)
			e.Map.Houses[1] = &otmap.House{ID: 1, Name: "Mine"}

			e.Import(metaHouseSource(), ImportOptions{DX: 100, Houses: mode})
			if got := e.Map.Houses[1].Name; got != "Mine" {
				t.Fatalf("house 1 = %q, want the existing house kept", got)
			}
			want := otmap.House{ID: 2, Name: "Second"}
			if got := e.Map.Houses[2]; got == nil || *got != want {
				t.Fatalf("house 2 = %+v, want %+v (a zero exit stays zero)", got, want)
			}
			if first, second := e.Map.Tile(at(110, 10)).HouseID, e.Map.Tile(at(111, 10)).HouseID; first != 1 || second != 2 {
				t.Fatalf("tile houses = %d, %d, want ids kept (1, 2)", first, second)
			}
		})
	}
}

func TestImportHousesMergeMovesExit(t *testing.T) {
	e := testEditor(t)
	e.Import(metaHouseSource(), ImportOptions{DX: 100, DY: 1, DZ: -1})
	if got := e.Map.Houses[1].Exit; got != posAt(110, 12, 6) {
		t.Fatalf("exit = %+v, want it moved by the offset to 110,12,6", got)
	}
}

func TestImportHousesRenumber(t *testing.T) {
	e := testEditor(t)
	e.Map.Houses[1] = &otmap.House{ID: 1, Name: "Mine"}

	e.Import(metaHouseSource(), ImportOptions{DX: 100, Houses: "renumber"})
	names := map[uint32]string{}
	for id, house := range e.Map.Houses {
		names[id] = house.Name
	}
	if want := map[uint32]string{1: "Mine", 2: "Theirs", 3: "Second"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("houses = %v, want %v (imported houses take the next free ids)", names, want)
	}
	if got := e.Map.Houses[2].Exit; got != at(110, 11) {
		t.Fatalf("exit of renumbered house = %+v, want it moved to 110,11", got)
	}
	if first, second := e.Map.Tile(at(110, 10)).HouseID, e.Map.Tile(at(111, 10)).HouseID; first != 2 || second != 3 {
		t.Fatalf("tile houses = %d, %d, want the new ids 2, 3", first, second)
	}
}

func TestImportHousesSkip(t *testing.T) {
	e := testEditor(t)
	e.Import(metaHouseSource(), ImportOptions{DX: 100, Houses: "skip"})
	if len(e.Map.Houses) != 0 {
		t.Fatalf("houses = %v, want none imported", e.Map.Houses)
	}
	if first, second := e.Map.Tile(at(110, 10)).HouseID, e.Map.Tile(at(111, 10)).HouseID; first != 0 || second != 0 {
		t.Fatalf("tile houses = %d, %d, want both cleared", first, second)
	}
}

func TestImportHouseIDOverridesTargetTileOnMerge(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(110, 10), &otmap.Tile{Ground: otmap.Item{ID: grassA}, HouseID: 7})

	e.Import(metaHouseSource(), ImportOptions{DX: 100})
	if got := e.Map.Tile(at(110, 10)).HouseID; got != 1 {
		t.Fatalf("merged tile house = %d, want the imported house 1", got)
	}
}

func metaTownSource() *otmap.Map {
	source := metaSource()
	source.Towns[1] = &otmap.Town{ID: 1, Name: "Theirs", Temple: at(1, 1)}
	source.Towns[5] = &otmap.Town{ID: 5, Name: "Other", Temple: at(10, 10)}
	source.Waypoints = append(source.Waypoints, &otmap.Waypoint{Name: "start", Pos: at(2, 2)}, &otmap.Waypoint{Name: "far", Pos: at(12, 12)})
	return source
}

func TestImportTownsAddsMissingTownsAndWaypointsAtOffset(t *testing.T) {
	e := testEditor(t)
	e.Map.Towns[1] = &otmap.Town{ID: 1, Name: "Mine", Temple: at(50, 50)}
	e.Map.Waypoints = append(e.Map.Waypoints, &otmap.Waypoint{Name: "start", Pos: at(50, 50)})

	e.Import(metaTownSource(), ImportOptions{DX: 90, DY: 80, Towns: true})
	if got := *e.Map.Towns[1]; got.Name != "Mine" || got.Temple != at(50, 50) {
		t.Fatalf("town 1 = %+v, want the existing town kept", got)
	}
	if got := e.Map.Towns[5]; got == nil || *got != (otmap.Town{ID: 5, Name: "Other", Temple: at(100, 90)}) {
		t.Fatalf("town 5 = %+v, want Other with the temple moved to 100,90", got)
	}
	want := []*otmap.Waypoint{{Name: "start", Pos: at(50, 50)}, {Name: "far", Pos: at(102, 92)}}
	if !reflect.DeepEqual(e.Map.Waypoints, want) {
		t.Fatalf("waypoints = %v, want start kept and far added at 102,92", metaWaypointNames(e))
	}
}

func TestImportWithoutTownsSkipsTownsAndWaypoints(t *testing.T) {
	e := testEditor(t)
	e.Import(metaTownSource(), ImportOptions{})
	if len(e.Map.Towns) != 0 || len(e.Map.Waypoints) != 0 {
		t.Fatalf("towns %v waypoints %v, want none imported", e.Map.Towns, metaWaypointNames(e))
	}
}

func TestImportIsOneUndoStep(t *testing.T) {
	e := testEditor(t)
	e.Import(metaHouseSource(), ImportOptions{DX: 100})

	if label := e.Undo(); label != "Import map" {
		t.Fatalf("undo label = %q, want %q", label, "Import map")
	}
	if len(e.Map.Houses) != 0 || e.Map.TileCount() != 0 {
		t.Fatalf("after undo: %d houses, %d tiles, want the map empty again", len(e.Map.Houses), e.Map.TileCount())
	}
}

func TestMinimapColor(t *testing.T) {
	cases := []struct {
		name  string
		tile  otmap.Tile
		color uint8
	}{
		{"top colored item wins", otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: stoneID}, {ID: treeID}, {ID: coinID}}}, 30},
		{"falls back to the ground", otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: coinID}, {ID: 9999}}}, 10},
		{"ground without color", otmap.Tile{Ground: otmap.Item{ID: grassA}}, 0},
		{"no ground, no colored item", otmap.Tile{Items: []otmap.Item{{ID: coinID}}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Items.Get(sandID).MinimapColor = 10
			e.Items.Get(stoneID).MinimapColor = 20
			e.Items.Get(treeID).MinimapColor = 30
			if got := e.MinimapColor(&tc.tile); got != tc.color {
				t.Fatalf("MinimapColor = %d, want %d", got, tc.color)
			}
		})
	}
}

func TestMinimapRGB(t *testing.T) {
	cases := []struct {
		index            uint8
		red, green, blue uint8
	}{
		{0, 0, 0, 0},
		{1, 0, 0, 51},
		{6, 0, 51, 0},
		{36, 51, 0, 0},
		{215, 255, 255, 255},
		{216, 0, 0, 0}, // outside the 6x6x6 palette
		{255, 0, 0, 0},
	}
	for _, tc := range cases {
		red, green, blue := MinimapRGB(tc.index)
		if red != tc.red || green != tc.green || blue != tc.blue {
			t.Errorf("MinimapRGB(%d) = %d,%d,%d, want %d,%d,%d", tc.index, red, green, blue, tc.red, tc.green, tc.blue)
		}
	}
}

package mapedit

import (
	"testing"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// checkEditor returns an editor whose sand is a floor change (stairs),
// coins are ladders and bags are teleports.
func checkEditor(t *testing.T) *Editor {
	t.Helper()
	e := testEditor(t)
	e.Items.Get(sandID).Flags |= otitems.FlagFloorChange
	e.Items.Get(coinID).Name = "wooden ladder"
	e.Items.Get(bagID).Kind = otitems.KindTeleport
	return e
}

func groundRect(e *Editor, x0, y0, x1, y1, z int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			e.Map.SetTile(posAt(x, y, z), &otmap.Tile{Ground: otmap.Item{ID: grassA}})
		}
	}
}

func putItem(e *Editor, pos otmap.Position, item otmap.Item) {
	tile := e.Map.Tile(pos)
	tile.Items = append(tile.Items, item)
	e.Map.SetTile(pos, tile)
}

func TestFindPath(t *testing.T) {
	e := checkEditor(t)
	groundRect(e, 10, 10, 20, 20, 7)
	// A wall across x 15 with a closed door at y 18.
	for y := 10; y <= 20; y++ {
		item := otmap.Item{ID: wallV}
		if y == 18 {
			item.ID = doorHClosed
		}
		putItem(e, posAt(15, y, 7), item)
	}
	result, err := e.FindPath(at(10, 10), at(20, 10), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found || result.Steps != 16 {
		t.Fatalf("path through the door: %+v", result)
	}
	door := false
	for _, point := range result.Waypoints {
		door = door || (point.Via == ViaDoor && point.Pos == at(15, 18))
	}
	if !door || result.Waypoints[0].Pos != at(10, 10) || result.Waypoints[len(result.Waypoints)-1].Pos != at(20, 10) {
		t.Fatalf("waypoints %+v", result.Waypoints)
	}

	// Without the door there is no path: the closest tile is next to the wall.
	e.Map.SetTile(at(15, 18), &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: []otmap.Item{{ID: wallV}}})
	result, _ = e.FindPath(at(10, 10), at(20, 10), 0)
	if result.Found || result.Closest == nil || result.Closest.X != 14 {
		t.Fatalf("walled off: %+v", result)
	}
	if _, err := e.FindPath(at(15, 12), at(20, 10), 0); err == nil {
		t.Fatal("a wall is a start")
	}

	// Stairs down at 12,12 lead to floor 8, a ladder there back up, a
	// teleport to the other side.
	e.Map.SetTile(at(12, 12), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
	groundRect(e, 11, 11, 13, 13, 8)
	result, _ = e.FindPath(at(10, 10), posAt(13, 13, 8), 0)
	if !result.Found || result.Waypoints[len(result.Waypoints)-1].Via != "" {
		t.Fatalf("stairs down: %+v", result)
	}
	putItem(e, posAt(11, 11, 8), otmap.Item{ID: coinID})
	putItem(e, posAt(13, 11, 8), otmap.Item{ID: bagID, Ext: &otmap.ItemExt{Dest: ptr(at(20, 20))}})
	result, _ = e.FindPath(posAt(13, 13, 8), at(19, 19), 0)
	if !result.Found || result.Steps != 4 {
		t.Fatalf("teleport: %+v", result)
	}
	result, _ = e.FindPath(posAt(11, 12, 8), at(11, 10), 0)
	if !result.Found {
		t.Fatalf("ladder: %+v", result)
	}

	result, _ = e.FindPath(at(10, 10), at(20, 20), 5)
	if result.Found || !result.Truncated {
		t.Fatalf("limit: %+v", result)
	}
}

func TestValidate(t *testing.T) {
	e := checkEditor(t)
	groundRect(e, 10, 10, 20, 20, 7)
	putItem(e, at(11, 11), otmap.Item{ID: wallV})
	e.Map.Towns = map[uint32]*otmap.Town{1: {ID: 1, Name: "Town", Temple: at(11, 11)}}
	e.Map.Houses = map[uint32]*otmap.House{
		1: {ID: 1, Name: "Inside", Exit: at(12, 12)},
		2: {ID: 2, Name: "Empty"},
	}
	tile := e.Map.Tile(at(12, 12))
	tile.HouseID = 1
	e.Map.SetTile(at(12, 12), tile)
	tile = e.Map.Tile(at(13, 13))
	tile.HouseID = 9
	tile.Ext = &otmap.TileExt{Zones: []uint16{4}}
	e.Map.SetTile(at(13, 13), tile)
	tile = e.Map.Tile(at(19, 13))
	tile.Ext = &otmap.TileExt{Spawn: &otmap.Spawn{Radius: 1}}
	e.Map.SetTile(at(19, 13), tile)
	e.Map.Waypoints = []*otmap.Waypoint{{Name: "nowhere", Pos: at(50, 50)}}
	putItem(e, at(14, 14), otmap.Item{ID: bagID})
	putItem(e, at(15, 15), otmap.Item{ID: stoneID, Ext: &otmap.ItemExt{UniqueID: 5000}})
	putItem(e, at(16, 16), otmap.Item{ID: stoneID, Ext: &otmap.ItemExt{UniqueID: 5000}})
	putItem(e, at(17, 17), otmap.Item{ID: 4242})
	tile = e.Map.Tile(at(11, 11))
	tile.Ext = &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Rat"}}}
	e.Map.SetTile(at(11, 11), tile)
	e.Map.SetTile(at(30, 30), &otmap.Tile{Ground: otmap.Item{ID: sandID}})

	validation := e.Validate(100)
	want := map[string]int{
		ProblemTemple: 1, ProblemHouseExit: 2, ProblemHouseTiles: 1, ProblemUnknownHome: 1, ProblemWaypoint: 1,
		ProblemTeleport: 1, ProblemUniqueID: 1, ProblemCreature: 1, ProblemSpawn: 1, ProblemUnknownItem: 1,
		ProblemFloorChange: 1, ProblemZone: 1,
	}
	for kind, count := range want {
		if validation.Counts[kind] != count {
			t.Errorf("%s: %d, want %d (%+v)", kind, validation.Counts[kind], count, validation.Problems)
		}
	}
	if len(validation.Counts) != len(want) || len(validation.Problems) != len(want)+1 {
		t.Fatalf("counts %v", validation.Counts)
	}
	if limited := e.Validate(3); len(limited.Problems) != 3 || !limited.More || limited.Counts[ProblemTemple] != 1 {
		t.Fatalf("limit: %+v", limited)
	}
}

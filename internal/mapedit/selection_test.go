package mapedit

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/otmap"
)

// selectionSandWithStone paints a 3x3 sand square around (70, 70) with a
// stone on the center, without automagic borders.
func selectionSandWithStone(t *testing.T) *Editor {
	t.Helper()
	e := testEditor(t)
	e.Settings.Automagic = false
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(70, 70)), Size: 1})
	e.Draw(DrawRequest{Kind: KindRaw, Item: stoneID, Points: points(at(70, 70))})
	return e
}

// selectionSquare lists the positions of a square on floor 7, sorted like
// Editor.Selected.
func selectionSquare(x0, y0, x1, y1 int) []otmap.Position {
	var positions []otmap.Position
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			positions = append(positions, at(x, y))
		}
	}
	return positions
}

// selectionDesks draws a row of desks on y=30 from x0 to x1.
func selectionDesks(e *Editor, x0, x1 int) {
	for x := x0; x <= x1; x++ {
		e.Draw(DrawRequest{Kind: KindTable, Name: "desk", Points: points(at(x, 30))})
	}
}

// selectionDeskRow returns the items of the tiles on y=30 from x0 to x1.
func selectionDeskRow(e *Editor, x0, x1 int) [][]uint16 {
	var row [][]uint16
	for x := x0; x <= x1; x++ {
		row = append(row, itemIDs(e.Map.Tile(at(x, 30))))
	}
	return row
}

// selectionGrassBorders are the borders grass at (60, 60) puts on the sand
// around it.
var selectionGrassBorders = map[[2]int]uint16{
	{60, 59}: borderS, {60, 61}: borderN, {59, 60}: borderE, {61, 60}: borderW,
	{59, 59}: borderCSE, {61, 59}: borderCSW, {59, 61}: borderCNE, {61, 61}: borderCNW,
}

func TestSelectRectSelectsEveryTileOfTheArea(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, false)
	if got, want := e.Selected(), selectionSquare(69, 69, 71, 71); !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want the 9 sand tiles %v", got, want)
	}
	center := e.Map.Tile(at(70, 70))
	if !center.Ground.Selected || !center.Items[0].Selected {
		t.Fatalf("center tile %+v: ground and stone should both be selected", center)
	}
}

func TestSelectRectReplacesPreviousSelection(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 69, 71), FloorsCurrent, false, false, false)
	e.SelectRect(*rect(71, 69, 71, 71), FloorsCurrent, false, false, false)
	if got, want := e.Selected(), selectionSquare(71, 69, 71, 71); !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want only the second column %v", got, want)
	}
}

func TestSelectRectAddKeepsPreviousSelection(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 69, 71), FloorsCurrent, false, false, false)
	e.SelectRect(*rect(71, 69, 71, 71), FloorsCurrent, true, false, false)
	want := []otmap.Position{at(69, 69), at(71, 69), at(69, 70), at(71, 70), at(69, 71), at(71, 71)}
	if got := e.Selected(); !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want both columns %v", got, want)
	}
}

func TestSelectRectAddCompletesPartlySelectedTile(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectItem(at(70, 70), 0, false)
	e.SelectRect(*rect(70, 70, 70, 70), FloorsCurrent, true, false, false)
	center := e.Map.Tile(at(70, 70))
	if !center.Ground.Selected || !center.Items[0].Selected {
		t.Fatalf("center tile %+v: adding a rect over a tile with only the stone selected should select the ground too", center)
	}
}

func TestSelectRectAddLeavesFullySelectedTileAlone(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(70, 70, 70, 70), FloorsCurrent, false, false, false)
	revision := e.Map.Rev
	e.SelectRect(*rect(70, 70, 70, 70), FloorsCurrent, true, false, false)
	if e.Map.Rev != revision {
		t.Fatalf("map revision changed from %d to %d: a fully selected tile should not be edited again", revision, e.Map.Rev)
	}
	if e.SelectionCount() != 1 {
		t.Fatalf("SelectionCount() = %d, want 1", e.SelectionCount())
	}
}

func TestSelectRectRemoveDeselectsOnlyTheArea(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, false)
	e.SelectRect(*rect(69, 69, 69, 71), FloorsCurrent, false, true, false)
	want := []otmap.Position{at(70, 69), at(71, 69), at(70, 70), at(71, 70), at(70, 71), at(71, 71)}
	if got := e.Selected(); !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want the area without the removed column %v", got, want)
	}
	if e.Map.Tile(at(69, 70)).Ground.Selected {
		t.Fatal("removed column still has a selected ground")
	}
}

func TestSelectRectFloorModesChooseFloors(t *testing.T) {
	cases := []struct {
		name   string
		mode   string
		floor  int
		floors []int
	}{
		{"current floor only", FloorsCurrent, 7, []int{7}},
		{"unknown mode is the current floor", "all", 7, []int{7}},
		{"lower above ground reaches down to the ground floor", FloorsLower, 5, []int{5, 6, 7}},
		{"lower on the ground floor is the ground floor", FloorsLower, 7, []int{7}},
		{"lower underground reaches two floors down", FloorsLower, 10, []int{10, 11, 12}},
		{"lower near the bottom stops at the last floor", FloorsLower, 14, []int{14, 15}},
		{"visible above ground is every floor to the ground floor", FloorsVisible, 5, []int{0, 1, 2, 3, 4, 5, 6, 7}},
		{"visible underground reaches two floors down", FloorsVisible, 9, []int{9, 10, 11}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			for z := 0; z <= otmap.MaxFloor; z++ {
				e.Map.SetTile(posAt(50, 50, z), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
			}
			e.SelectRect(Rect{From: posAt(50, 50, tc.floor), To: posAt(50, 50, tc.floor)}, tc.mode, false, false, false)
			var got []int
			for _, pos := range e.Selected() {
				got = append(got, int(pos.Z))
			}
			if !slices.Equal(got, tc.floors) {
				t.Fatalf("mode %q from floor %d selected floors %v, want %v", tc.mode, tc.floor, got, tc.floors)
			}
		})
	}
}

func TestSelectRectCompensateShiftsLowerFloors(t *testing.T) {
	cases := []struct {
		name       string
		compensate bool
		want       []otmap.Position
	}{
		{"compensated", true, []otmap.Position{posAt(50, 50, 8), posAt(51, 51, 9), posAt(52, 52, 10)}},
		{"not compensated", false, []otmap.Position{posAt(50, 50, 8), posAt(50, 50, 9), posAt(50, 50, 10)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			for z := 8; z <= 10; z++ {
				e.Map.SetTile(posAt(50, 50, z), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
				shift := z - 8
				e.Map.SetTile(posAt(50+shift, 50+shift, z), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
			}
			e.SelectRect(Rect{From: posAt(50, 50, 8), To: posAt(50, 50, 8)}, FloorsLower, false, false, tc.compensate)
			if got := e.Selected(); !slices.Equal(got, tc.want) {
				t.Fatalf("Selected() = %v, want %v (compensate shifts one tile right and down per floor)", got, tc.want)
			}
		})
	}
}

func TestSelectionChangesAreNotRecordedInHistory(t *testing.T) {
	e := selectionSandWithStone(t)
	undoBefore, _ := e.History()
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, false)
	e.SelectAt(at(70, 70), false, true, true)
	e.SelectItem(at(70, 70), -1, true)
	e.SelectTiles(points(at(69, 69)), true)
	e.ClearSelection()
	if undoAfter, _ := e.History(); len(undoAfter) != len(undoBefore) {
		t.Fatalf("history grew from %d to %d steps: selection is not an undo step", len(undoBefore), len(undoAfter))
	}
}

func TestSelectionDoesNotMarkTilesModified(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
	e.SelectTiles(points(at(10, 10)), false)
	if tile := e.Map.Tile(at(10, 10)); tile.State&otmap.StateModified != 0 {
		t.Fatalf("tile state %08b: selecting is not a modification", tile.State)
	}
}

func TestSelectAtPicksTopThingByPriority(t *testing.T) {
	fullTile := func() *otmap.Tile {
		return &otmap.Tile{
			Ground: otmap.Item{ID: sandID},
			Items:  []otmap.Item{{ID: stoneID}, {ID: coinID}},
			Ext: &otmap.TileExt{
				Creatures: []otmap.Creature{{Name: "rat"}, {Name: "wolf"}},
				Spawn:     &otmap.Spawn{Radius: 3},
				SpawnNPC:  &otmap.Spawn{Radius: 2},
			},
		}
	}
	cases := []struct {
		name          string
		tile          *otmap.Tile
		showCreatures bool
		showSpawns    bool
		want          string
		selected      func(tile *otmap.Tile) bool
	}{
		{"top creature first", fullTile(), true, true, "creature",
			func(tile *otmap.Tile) bool { return tile.Ext.Creatures[1].Selected && !tile.Ext.Creatures[0].Selected }},
		{"spawn when creatures are hidden", fullTile(), false, true, "spawn",
			func(tile *otmap.Tile) bool { return tile.Ext.Spawn.Selected && !tile.Ext.SpawnNPC.Selected }},
		{"npc spawn without a monster spawn", &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{SpawnNPC: &otmap.Spawn{Radius: 2}}}, true, true, "spawn",
			func(tile *otmap.Tile) bool { return tile.Ext.SpawnNPC.Selected }},
		{"top item when creatures and spawns are hidden", fullTile(), false, false, "item",
			func(tile *otmap.Tile) bool {
				return tile.Items[1].Selected && !tile.Items[0].Selected && !tile.Ground.Selected
			}},
		{"ground of a tile without items", &otmap.Tile{Ground: otmap.Item{ID: sandID}}, true, true, "item",
			func(tile *otmap.Tile) bool { return tile.Ground.Selected }},
		{"nothing on a tile with only a house", &otmap.Tile{HouseID: 1}, true, true, "",
			func(tile *otmap.Tile) bool { return !tile.Selected() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Map.SetTile(at(10, 10), tc.tile)
			if got := e.SelectAt(at(10, 10), false, tc.showCreatures, tc.showSpawns); got != tc.want {
				t.Fatalf("SelectAt = %q, want %q", got, tc.want)
			}
			if tile := e.Map.Tile(at(10, 10)); !tc.selected(tile) {
				t.Fatalf("tile after SelectAt %+v (ext %+v): wrong part selected", tile, tile.Ext)
			}
		})
	}
}

func TestSelectAtSelectsTopItemOnly(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, false)
	if what := e.SelectAt(at(70, 70), false, true, true); what != "item" {
		t.Fatalf("SelectAt = %q, want \"item\"", what)
	}
	tile := e.Map.Tile(at(70, 70))
	if tile.Ground.Selected || !tile.Items[0].Selected {
		t.Fatalf("tile %+v: only the stone on top should be selected", tile)
	}
	if e.SelectionCount() != 1 {
		t.Fatalf("SelectionCount() = %d, want 1: a click replaces the selection", e.SelectionCount())
	}
}

func TestSelectAtToggleFlipsTopThingAndKeepsOthers(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectTiles(points(at(69, 69)), false)
	e.SelectAt(at(70, 70), true, true, true)
	if tile := e.Map.Tile(at(70, 70)); !tile.Items[0].Selected || tile.Ground.Selected {
		t.Fatalf("first toggle: tile %+v, want only the stone selected", tile)
	}
	if !e.Map.Tile(at(69, 69)).Selected() {
		t.Fatal("toggle dropped the other selected tile")
	}
	e.SelectAt(at(70, 70), true, true, true)
	if tile := e.Map.Tile(at(70, 70)); tile.Selected() {
		t.Fatalf("second toggle: tile %+v, want nothing selected", tile)
	}
	if got, want := e.Selected(), points(at(69, 69)); !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want %v", got, want)
	}
}

func TestSelectAtToggleFlipsCreatureSpawnAndGround(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{
		Creatures: []otmap.Creature{{Name: "rat"}}, Spawn: &otmap.Spawn{Radius: 1}, SpawnNPC: &otmap.Spawn{Radius: 1},
	}})
	steps := []struct {
		showCreatures, showSpawns bool
		want                      string
		selected                  func(tile *otmap.Tile) bool
	}{
		{true, true, "creature", func(tile *otmap.Tile) bool { return tile.Ext.Creatures[0].Selected }},
		{true, true, "creature", func(tile *otmap.Tile) bool { return !tile.Ext.Creatures[0].Selected }},
		{false, true, "spawn", func(tile *otmap.Tile) bool { return tile.Ext.Spawn.Selected }},
		{false, true, "spawn", func(tile *otmap.Tile) bool { return !tile.Ext.Spawn.Selected }},
		{false, false, "item", func(tile *otmap.Tile) bool { return tile.Ground.Selected }},
		{false, false, "item", func(tile *otmap.Tile) bool { return !tile.Ground.Selected }},
	}
	for i, step := range steps {
		if got := e.SelectAt(at(10, 10), true, step.showCreatures, step.showSpawns); got != step.want {
			t.Fatalf("step %d: SelectAt = %q, want %q", i, got, step.want)
		}
		if tile := e.Map.Tile(at(10, 10)); !step.selected(tile) {
			t.Fatalf("step %d: tile %+v ext %+v: toggle did not flip the %s", i, tile, tile.Ext, step.want)
		}
	}
	e.Map.SetTile(at(11, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{SpawnNPC: &otmap.Spawn{Radius: 1}}})
	e.SelectAt(at(11, 10), true, true, true)
	e.SelectAt(at(11, 10), true, true, true)
	if tile := e.Map.Tile(at(11, 10)); tile.Ext.SpawnNPC.Selected {
		t.Fatal("second toggle left the NPC spawn selected")
	}
}

func TestSelectAtEmptyPositionClearsSelection(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, false)
	if got := e.SelectAt(at(5, 5), false, true, true); got != "" {
		t.Fatalf("SelectAt on a missing tile = %q, want \"\"", got)
	}
	if e.SelectionCount() != 0 {
		t.Fatalf("SelectionCount() = %d, want 0: clicking nothing deselects", e.SelectionCount())
	}
}

func TestSelectItemSelectsOneItemOrTheGround(t *testing.T) {
	cases := []struct {
		name         string
		index        int
		wantGround   bool
		wantStone    bool
		wantSelected bool
	}{
		{"item index", 0, false, true, true},
		{"ground index", -1, true, false, true},
		{"index past the stack", 5, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := selectionSandWithStone(t)
			e.SelectItem(at(70, 70), tc.index, false)
			tile := e.Map.Tile(at(70, 70))
			if tile.Ground.Selected != tc.wantGround || tile.Items[0].Selected != tc.wantStone {
				t.Fatalf("index %d: ground selected %v stone selected %v, want %v and %v", tc.index, tile.Ground.Selected, tile.Items[0].Selected, tc.wantGround, tc.wantStone)
			}
			if got := e.SelectionCount() == 1; got != tc.wantSelected {
				t.Fatalf("SelectionCount() = %d, want tile selected = %v", e.SelectionCount(), tc.wantSelected)
			}
		})
	}
}

func TestSelectItemGroundIndexOnGroundlessTileSelectsNothing(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Items: []otmap.Item{{ID: stoneID}}})
	e.SelectItem(at(10, 10), -1, false)
	if tile := e.Map.Tile(at(10, 10)); tile.Selected() {
		t.Fatalf("tile %+v: a tile without ground has no ground to select", tile)
	}
}

func TestSelectItemAddKeepsOrReplacesSelection(t *testing.T) {
	cases := []struct {
		name string
		add  bool
		want []otmap.Position
	}{
		{"add keeps", true, []otmap.Position{at(69, 69), at(70, 70)}},
		{"replace drops", false, []otmap.Position{at(70, 70)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := selectionSandWithStone(t)
			e.SelectTiles(points(at(69, 69)), false)
			e.SelectItem(at(70, 70), 0, tc.add)
			if got := e.Selected(); !slices.Equal(got, tc.want) {
				t.Fatalf("Selected() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSelectItemOnMissingTileOnlyClearsSelection(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectTiles(points(at(69, 69)), false)
	e.SelectItem(at(5, 5), 0, false)
	if e.SelectionCount() != 0 || e.Map.Tile(at(5, 5)) != nil {
		t.Fatalf("SelectionCount() = %d, tile = %+v: want no selection and no tile created", e.SelectionCount(), e.Map.Tile(at(5, 5)))
	}
}

func TestSelectTilesSelectsWholeTilesAndSkipsMissing(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectTiles(points(at(70, 70), at(5, 5)), false)
	if got, want := e.Selected(), points(at(70, 70)); !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want %v", got, want)
	}
	if tile := e.Map.Tile(at(70, 70)); !tile.Ground.Selected || !tile.Items[0].Selected {
		t.Fatalf("tile %+v: everything on it should be selected", tile)
	}
	if e.Map.Tile(at(5, 5)) != nil {
		t.Fatal("SelectTiles created a tile where there was none")
	}
}

func TestSelectTilesAddKeepsOrReplacesSelection(t *testing.T) {
	cases := []struct {
		name string
		add  bool
		want []otmap.Position
	}{
		{"add keeps", true, []otmap.Position{at(69, 69), at(71, 71)}},
		{"replace drops", false, []otmap.Position{at(71, 71)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := selectionSandWithStone(t)
			e.SelectTiles(points(at(69, 69)), false)
			e.SelectTiles(points(at(71, 71)), tc.add)
			if got := e.Selected(); !slices.Equal(got, tc.want) {
				t.Fatalf("Selected() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClearSelectionDeselectsEverything(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, false)
	e.ClearSelection()
	if e.SelectionCount() != 0 {
		t.Fatalf("SelectionCount() = %d, want 0", e.SelectionCount())
	}
	for _, pos := range selectionSquare(69, 69, 71, 71) {
		if e.Map.Tile(pos).Selected() {
			t.Fatalf("tile %v still selected", pos)
		}
	}
}

func TestClearSelectionWithNothingSelectedTouchesNoTile(t *testing.T) {
	e := selectionSandWithStone(t)
	revision := e.Map.Rev
	e.ClearSelection()
	if e.Map.Rev != revision {
		t.Fatalf("map revision changed from %d to %d with nothing selected", revision, e.Map.Rev)
	}
}

func TestRebuildSelectionFindsSelectedTilesOnTheMap(t *testing.T) {
	e := testEditor(t)
	// Tiles stored straight into the map, like a loaded map, are not tracked.
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID, Selected: true}})
	e.Map.SetTile(posAt(12, 11, 6), &otmap.Tile{Items: []otmap.Item{{ID: stoneID, Selected: true}}})
	e.Map.SetTile(at(11, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
	if e.SelectionCount() != 0 {
		t.Fatalf("SelectionCount() before rebuild = %d, want 0", e.SelectionCount())
	}
	e.RebuildSelection()
	if got, want := e.Selected(), []otmap.Position{posAt(12, 11, 6), at(10, 10)}; !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want %v (sorted by floor first)", got, want)
	}
}

func TestSelectedIsSortedByFloorRowAndColumn(t *testing.T) {
	e := testEditor(t)
	positions := []otmap.Position{posAt(5, 1, 8), at(3, 2), at(9, 1), posAt(1, 9, 6), at(2, 2)}
	for _, pos := range positions {
		e.Map.SetTile(pos, &otmap.Tile{Ground: otmap.Item{ID: sandID}})
	}
	e.SelectTiles(positions, false)
	want := []otmap.Position{posAt(1, 9, 6), at(9, 1), at(2, 2), at(3, 2), posAt(5, 1, 8)}
	if got := e.Selected(); !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want %v", got, want)
	}
}

func TestSelectionBoundsSpansSelectedTiles(t *testing.T) {
	e := testEditor(t)
	if _, _, ok := e.SelectionBounds(); ok {
		t.Fatal("SelectionBounds() ok with nothing selected")
	}
	positions := []otmap.Position{posAt(130, 131, 7), posAt(131, 130, 7), posAt(132, 133, 8)}
	for _, pos := range positions {
		e.Map.SetTile(pos, &otmap.Tile{Ground: otmap.Item{ID: sandID}})
	}
	e.SelectTiles(positions, false)
	lowest, highest, ok := e.SelectionBounds()
	if !ok || lowest != posAt(130, 130, 7) || highest != posAt(132, 133, 8) {
		t.Fatalf("SelectionBounds() = %v %v %v, want {130 130 7} {132 133 8} true", lowest, highest, ok)
	}
}

func TestCopyWithNothingSelectedReturnsNil(t *testing.T) {
	e := selectionSandWithStone(t)
	if clip := e.Copy(); clip != nil {
		t.Fatalf("Copy() = %+v, want nil", clip)
	}
}

func TestCopyReturnsSelectionRelativeToTopLeft(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, false)
	clip := e.Copy()
	if clip == nil || len(clip.Tiles) != 9 || clip.Width != 3 || clip.Height != 3 {
		t.Fatalf("clip = %+v, want 9 tiles in a 3x3 area", clip)
	}
	center := clip.Tiles[4]
	if center.DX != 1 || center.DY != 1 || center.DZ != 0 {
		t.Fatalf("center offset = %d,%d,%d, want 1,1,0", center.DX, center.DY, center.DZ)
	}
	if center.Ground == nil || center.Ground.ID != sandID || center.Ground.Selected {
		t.Fatalf("center ground = %+v, want unselected sand", center.Ground)
	}
	if len(center.Items) != 1 || center.Items[0].ID != stoneID || center.Items[0].Selected {
		t.Fatalf("center items = %+v, want one unselected stone", center.Items)
	}
	if tile := e.Map.Tile(at(70, 70)); !tile.Selected() || len(tile.Items) != 1 {
		t.Fatalf("Copy changed the map: %+v", tile)
	}
}

func TestCopyTakesOnlySelectedParts(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectItem(at(70, 70), 0, false)
	clip := e.Copy()
	if clip == nil || len(clip.Tiles) != 1 || clip.Width != 1 || clip.Height != 1 {
		t.Fatalf("clip = %+v, want the one tile", clip)
	}
	clipTile := clip.Tiles[0]
	if clipTile.Ground != nil || len(clipTile.Items) != 1 || clipTile.Items[0].ID != stoneID {
		t.Fatalf("clip tile = %+v, want only the stone", clipTile)
	}
}

func TestCopyCarriesHouseAndFlagsOnlyWithTheGround(t *testing.T) {
	cases := []struct {
		name      string
		index     int
		wantHouse uint32
		wantFlags uint32
	}{
		{"ground selected", -1, 5, otmap.FlagPZ},
		{"only an item selected", 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: stoneID}}, HouseID: 5, Flags: otmap.FlagPZ})
			e.SelectItem(at(10, 10), tc.index, false)
			clipTile := e.Copy().Tiles[0]
			if clipTile.HouseID != tc.wantHouse || clipTile.Flags != tc.wantFlags {
				t.Fatalf("clip house %d flags %d, want %d and %d", clipTile.HouseID, clipTile.Flags, tc.wantHouse, tc.wantFlags)
			}
		})
	}
}

func TestCopyTakesSelectedCreaturesAndSpawns(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{
		Creatures: []otmap.Creature{{Name: "rat", Selected: true}, {Name: "wolf"}},
		Spawn:     &otmap.Spawn{Radius: 3, Selected: true},
		SpawnNPC:  &otmap.Spawn{Radius: 2, Selected: true},
	}})
	e.RebuildSelection()
	clipTile := e.Copy().Tiles[0]
	if len(clipTile.Creatures) != 1 || clipTile.Creatures[0].Name != "rat" || clipTile.Creatures[0].Selected {
		t.Fatalf("clip creatures = %+v, want only the unselected copy of the selected rat", clipTile.Creatures)
	}
	if clipTile.Spawn == nil || clipTile.Spawn.Radius != 3 || clipTile.Spawn.Selected {
		t.Fatalf("clip spawn = %+v, want an unselected radius 3 spawn", clipTile.Spawn)
	}
	if clipTile.SpawnNPC == nil || clipTile.SpawnNPC.Radius != 2 || clipTile.SpawnNPC.Selected {
		t.Fatalf("clip npc spawn = %+v, want an unselected radius 2 spawn", clipTile.SpawnNPC)
	}
	if clipTile.Ground != nil {
		t.Fatalf("clip ground = %+v, the ground was not selected", clipTile.Ground)
	}
}

func TestCopySkipsUnselectedSpawns(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{
		Creatures: []otmap.Creature{{Name: "rat", Selected: true}},
		Spawn:     &otmap.Spawn{Radius: 3},
	}})
	e.RebuildSelection()
	if clipTile := e.Copy().Tiles[0]; clipTile.Spawn != nil || clipTile.SpawnNPC != nil {
		t.Fatalf("clip tile = %+v, want no spawns", clipTile)
	}
}

func TestCutRemovesSelectionAndReturnsIt(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectItem(at(70, 70), 0, false)
	clip := e.Cut()
	if clip == nil || len(clip.Tiles) != 1 || len(clip.Tiles[0].Items) != 1 || clip.Tiles[0].Items[0].ID != stoneID {
		t.Fatalf("Cut() = %+v, want the stone", clip)
	}
	tile := e.Map.Tile(at(70, 70))
	if tile.Ground.ID != sandID || len(tile.Items) != 0 {
		t.Fatalf("tile after cut %+v, want only the sand left", tile)
	}
	if undo, _ := e.History(); undo[len(undo)-1].Label != "Cut" {
		t.Fatalf("last undo step %q, want \"Cut\"", undo[len(undo)-1].Label)
	}
}

func TestCutRemovesCreaturesAndSpawns(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{
		Creatures: []otmap.Creature{{Name: "rat", Selected: true}, {Name: "wolf"}},
		Spawn:     &otmap.Spawn{Radius: 3, Selected: true},
		SpawnNPC:  &otmap.Spawn{Radius: 2, Selected: true},
	}})
	e.RebuildSelection()
	e.Cut()
	tile := e.Map.Tile(at(10, 10))
	if tile.Ext == nil || len(tile.Ext.Creatures) != 1 || tile.Ext.Creatures[0].Name != "wolf" || tile.Ext.Spawn != nil || tile.Ext.SpawnNPC != nil {
		t.Fatalf("tile ext after cut %+v, want only the unselected wolf", tile.Ext)
	}
}

func TestCutWithNothingSelectedChangesNothing(t *testing.T) {
	e := selectionSandWithStone(t)
	undoBefore, _ := e.History()
	if clip := e.Cut(); clip != nil {
		t.Fatalf("Cut() = %+v, want nil", clip)
	}
	if undoAfter, _ := e.History(); len(undoAfter) != len(undoBefore) {
		t.Fatal("Cut with nothing selected added an undo step")
	}
}

func TestPasteEmptyClipboardFails(t *testing.T) {
	e := testEditor(t)
	for _, clip := range []*Clip{nil, {Width: 1, Height: 1}} {
		if got := errText(e.Paste(clip, at(10, 10))); got != "the clipboard is empty" {
			t.Fatalf("Paste(%+v) error = %v, want \"the clipboard is empty\"", clip, got)
		}
	}
	if e.CanUndo() {
		t.Fatal("a failed paste added an undo step")
	}
}

func TestPastePlacesClipAtTargetAndSelectsOnlyIt(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, false)
	clip := e.Copy()
	if err := e.Paste(clip, at(100, 100)); err != nil {
		t.Fatal(err)
	}
	if got, want := e.Selected(), selectionSquare(100, 100, 102, 102); !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want the pasted tiles %v (the old selection is dropped)", got, want)
	}
	pasted := e.Map.Tile(at(101, 101))
	if pasted.Ground.ID != sandID || !slices.Equal(itemIDs(pasted), []uint16{stoneID}) {
		t.Fatalf("pasted center %+v, want sand with a stone", pasted)
	}
	if !pasted.Ground.Selected || !pasted.Items[0].Selected {
		t.Fatalf("pasted center %+v, want everything selected", pasted)
	}
	if undo, _ := e.History(); undo[len(undo)-1] != (ActionInfo{Label: "Paste", Tiles: 18}) {
		t.Fatalf("last undo step %+v, want Paste of 9 new tiles and 9 deselected ones", undo[len(undo)-1])
	}
}

func TestPasteMergesItemsOntoExistingTile(t *testing.T) {
	e := selectionSandWithStone(t)
	e.Map.SetTile(at(100, 100), &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: []otmap.Item{{ID: coinID}}})
	clip := &Clip{Width: 1, Height: 1, Tiles: []ClipTile{{Items: []otmap.Item{{ID: stoneID}}}}}
	if err := e.Paste(clip, at(100, 100)); err != nil {
		t.Fatal(err)
	}
	tile := e.Map.Tile(at(100, 100))
	if tile.Ground.ID != grassA || !slices.Equal(itemIDs(tile), []uint16{coinID, stoneID}) {
		t.Fatalf("tile %+v, want the grass and coin kept with the stone on top", tile)
	}
	if tile.Ground.Selected || tile.Items[0].Selected || !tile.Items[1].Selected {
		t.Fatalf("tile %+v, want only the pasted stone selected", tile)
	}
}

func TestPasteAcrossFloorsKeepsFloorOffsets(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(70, 70), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
	e.Map.SetTile(posAt(71, 71, 8), &otmap.Tile{Ground: otmap.Item{ID: grassA}})
	e.SelectTiles(points(at(70, 70), posAt(71, 71, 8)), false)
	clip := e.Copy()
	if clip.Width != 2 || clip.Height != 2 || clip.Tiles[1].DZ != 1 {
		t.Fatalf("clip = %+v, want 2x2 with the second tile one floor down", clip)
	}
	if err := e.Paste(clip, posAt(100, 100, 6)); err != nil {
		t.Fatal(err)
	}
	if tile := e.Map.Tile(posAt(100, 100, 6)); tile == nil || tile.Ground.ID != sandID {
		t.Fatalf("tile at 100,100,6 = %+v, want sand", tile)
	}
	if tile := e.Map.Tile(posAt(101, 101, 7)); tile == nil || tile.Ground.ID != grassA {
		t.Fatalf("tile at 101,101,7 = %+v, want grass", tile)
	}
}

func TestPasteSkipsTilesOutsideTheMap(t *testing.T) {
	e := testEditor(t)
	clip := &Clip{Width: 1, Height: 1, Tiles: []ClipTile{
		{Ground: &otmap.Item{ID: sandID}},
		{DZ: 1, Ground: &otmap.Item{ID: grassA}},
	}}
	if err := e.Paste(clip, posAt(10, 10, otmap.MaxFloor)); err != nil {
		t.Fatal(err)
	}
	if got, want := e.Selected(), points(posAt(10, 10, otmap.MaxFloor)); !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want only the tile on the last floor %v", got, want)
	}
}

func TestPasteRestoresHouseFlagsCreaturesAndSpawns(t *testing.T) {
	e := testEditor(t)
	clip := &Clip{Width: 1, Height: 1, Tiles: []ClipTile{{
		Ground: &otmap.Item{ID: sandID}, HouseID: 5, Flags: otmap.FlagPZ,
		Creatures: []otmap.Creature{{Name: "rat"}},
		Spawn:     &otmap.Spawn{Radius: 3},
		SpawnNPC:  &otmap.Spawn{Radius: 2},
	}}}
	if err := e.Paste(clip, at(10, 10)); err != nil {
		t.Fatal(err)
	}
	tile := e.Map.Tile(at(10, 10))
	if tile.HouseID != 5 || tile.Flags != otmap.FlagPZ {
		t.Fatalf("house %d flags %d, want 5 and PZ", tile.HouseID, tile.Flags)
	}
	ext := tile.Ext
	if ext == nil || len(ext.Creatures) != 1 || !ext.Creatures[0].Selected || ext.Spawn == nil || !ext.Spawn.Selected || ext.Spawn.Radius != 3 || ext.SpawnNPC == nil || !ext.SpawnNPC.Selected || ext.SpawnNPC.Radius != 2 {
		t.Fatalf("ext %+v, want the selected rat, spawn and npc spawn", ext)
	}
}

func TestPasteGroundKeepsTargetHouseWhenClipHasNone(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: grassA}, HouseID: 7, Flags: otmap.FlagNoLogout})
	clip := &Clip{Width: 1, Height: 1, Tiles: []ClipTile{{Ground: &otmap.Item{ID: sandID}}}}
	if err := e.Paste(clip, at(10, 10)); err != nil {
		t.Fatal(err)
	}
	if tile := e.Map.Tile(at(10, 10)); tile.Ground.ID != sandID || tile.HouseID != 7 || tile.Flags != otmap.FlagNoLogout {
		t.Fatalf("tile %+v, want sand on the kept house 7 with no-logout", tile)
	}
}

func TestPasteGroundBordersTheTilesAround(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(60, 60)), Size: 2})
	clip := &Clip{Width: 1, Height: 1, Tiles: []ClipTile{{Ground: &otmap.Item{ID: grassA}}}}
	if err := e.Paste(clip, at(60, 60)); err != nil {
		t.Fatal(err)
	}
	for xy, id := range selectionGrassBorders {
		if got := itemIDs(e.Map.Tile(at(xy[0], xy[1]))); !slices.Equal(got, []uint16{id}) {
			t.Errorf("tile %v items %v, want the grass border [%d]", xy, got, id)
		}
	}
}

func TestPasteWithoutAutomagicAddsNoBorders(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(60, 60)), Size: 2})
	e.Settings.Automagic = false
	clip := &Clip{Width: 1, Height: 1, Tiles: []ClipTile{{Ground: &otmap.Item{ID: grassA}}}}
	if err := e.Paste(clip, at(60, 60)); err != nil {
		t.Fatal(err)
	}
	for xy := range selectionGrassBorders {
		if got := itemIDs(e.Map.Tile(at(xy[0], xy[1]))); len(got) != 0 {
			t.Errorf("tile %v items %v, want no borders with automagic off", xy, got)
		}
	}
}

func TestPasteTableReshapesTablesAround(t *testing.T) {
	e := testEditor(t)
	selectionDesks(e, 30, 31)
	clip := &Clip{Width: 1, Height: 1, Tiles: []ClipTile{{Items: []otmap.Item{{ID: tableAloneID}}}}}
	if err := e.Paste(clip, at(32, 30)); err != nil {
		t.Fatal(err)
	}
	want := [][]uint16{{tableWestID}, {tableH}, {tableEID}}
	if got := selectionDeskRow(e, 30, 32); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("desk row = %v, want %v: the pasted desk joins the row", got, want)
	}
}

func TestMoveSelectionMovesTilesAndKeepsThemSelected(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, true)
	if err := e.MoveSelection(10, 0, 0); err != nil {
		t.Fatal(err)
	}
	if e.Map.Tile(at(70, 70)) != nil {
		t.Fatal("source tile at 70,70 not emptied")
	}
	moved := e.Map.Tile(at(80, 70))
	if moved == nil || moved.Ground.ID != sandID || !slices.Equal(itemIDs(moved), []uint16{stoneID}) || !moved.Selected() {
		t.Fatalf("moved tile = %+v, want selected sand with a stone", moved)
	}
	if got, want := e.Selected(), selectionSquare(79, 69, 81, 71); !slices.Equal(got, want) {
		t.Fatalf("Selected() = %v, want %v", got, want)
	}
	if undo, _ := e.History(); undo[len(undo)-1].Label != "Move" {
		t.Fatalf("last undo step %q, want \"Move\"", undo[len(undo)-1].Label)
	}
}

func TestMoveSelectionChangesFloor(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectTiles(points(at(70, 70)), false)
	if err := e.MoveSelection(0, 0, 1); err != nil {
		t.Fatal(err)
	}
	moved := e.Map.Tile(posAt(70, 70, 8))
	if moved == nil || moved.Ground.ID != sandID || !slices.Equal(itemIDs(moved), []uint16{stoneID}) {
		t.Fatalf("tile one floor down = %+v, want sand with a stone", moved)
	}
	if e.Map.Tile(at(70, 70)) != nil {
		t.Fatal("source tile not emptied")
	}
}

func TestMoveSelectionLeavingTheMapFails(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(posAt(10, 10, otmap.MaxFloor), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
	e.Map.SetTile(posAt(0, 5, 7), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
	cases := []struct {
		name       string
		pos        otmap.Position
		dx, dy, dz int
	}{
		{"below the last floor", posAt(10, 10, otmap.MaxFloor), 0, 0, 1},
		{"left of x 0", posAt(0, 5, 7), -1, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e.SelectTiles(points(tc.pos), false)
			if got := errText(e.MoveSelection(tc.dx, tc.dy, tc.dz)); got != "the selection would leave the map" {
				t.Fatalf("MoveSelection error = %v, want \"the selection would leave the map\"", got)
			}
			if tile := e.Map.Tile(tc.pos); tile == nil || tile.Ground.ID != sandID {
				t.Fatalf("tile %v = %+v, want it left in place", tc.pos, tile)
			}
		})
	}
	if e.CanUndo() {
		t.Fatal("a failed move added an undo step")
	}
}

func TestMoveSelectionWithoutOffsetOrSelectionDoesNothing(t *testing.T) {
	e := selectionSandWithStone(t)
	undoBefore, _ := e.History()
	if err := e.MoveSelection(1, 0, 0); err != nil {
		t.Fatalf("move with nothing selected: %v", err)
	}
	e.SelectTiles(points(at(70, 70)), false)
	if err := e.MoveSelection(0, 0, 0); err != nil {
		t.Fatalf("move by zero: %v", err)
	}
	if undoAfter, _ := e.History(); len(undoAfter) != len(undoBefore) {
		t.Fatal("a move that moves nothing added an undo step")
	}
}

func TestMoveSelectionCarriesHouseAndFlagsWithTheGround(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: stoneID}}, HouseID: 5, Flags: otmap.FlagPZ})
	e.SelectItem(at(10, 10), -1, false)
	if err := e.MoveSelection(1, 0, 0); err != nil {
		t.Fatal(err)
	}
	if target := e.Map.Tile(at(11, 10)); target.HouseID != 5 || target.Flags != otmap.FlagPZ || target.Ground.ID != sandID {
		t.Fatalf("target %+v, want sand with house 5 and PZ", target)
	}
	if source := e.Map.Tile(at(10, 10)); source.HouseID != 0 || source.Flags != 0 || !slices.Equal(itemIDs(source), []uint16{stoneID}) {
		t.Fatalf("source %+v, want only the stone left, without house or flags", source)
	}
}

func TestMoveSelectionOfAnItemLeavesHouseInPlace(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: stoneID}}, HouseID: 5})
	e.SelectItem(at(10, 10), 0, false)
	if err := e.MoveSelection(1, 0, 0); err != nil {
		t.Fatal(err)
	}
	if source := e.Map.Tile(at(10, 10)); source.HouseID != 5 || len(source.Items) != 0 {
		t.Fatalf("source %+v, want the house kept and the stone gone", source)
	}
	if target := e.Map.Tile(at(11, 10)); target.HouseID != 0 || !slices.Equal(itemIDs(target), []uint16{stoneID}) {
		t.Fatalf("target %+v, want just the stone", target)
	}
}

func TestMoveSelectionRedoesBordersAtSourceAndTarget(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(60, 60)), Size: 2})
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(60, 60))})
	e.SelectItem(at(60, 60), -1, false)
	if err := e.MoveSelection(1, 0, 0); err != nil {
		t.Fatal(err)
	}
	// The grass is now at 61,60 and 60,60 has no ground at all.
	want := map[[2]int][]uint16{
		{59, 60}: nil, {59, 59}: nil, {59, 61}: nil,
		{60, 59}: {borderCSE}, {61, 59}: {borderS}, {62, 59}: {borderCSW},
		{62, 60}: {borderW}, {61, 60}: nil,
		{60, 61}: {borderCNE}, {61, 61}: {borderN}, {62, 61}: {borderCNW},
	}
	for xy, ids := range want {
		if got := itemIDs(e.Map.Tile(at(xy[0], xy[1]))); !slices.Equal(got, ids) {
			t.Errorf("tile %v items %v, want %v", xy, got, ids)
		}
	}
	if e.Map.Tile(at(60, 60)) != nil {
		t.Errorf("source tile = %+v, want it removed", e.Map.Tile(at(60, 60)))
	}
}

func TestMoveSelectionOfPlainItemKeepsBordersAlone(t *testing.T) {
	e := testEditor(t)
	// Grass without its borders (automagic off), then a stone beside it.
	e.Settings.Automagic = false
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(60, 60)), Size: 2})
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(60, 60))})
	e.Draw(DrawRequest{Kind: KindRaw, Item: stoneID, Points: points(at(61, 60))})
	e.Settings.Automagic = true
	e.SelectItem(at(61, 60), 0, false)
	if err := e.MoveSelection(0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if got := itemIDs(e.Map.Tile(at(60, 59))); len(got) != 0 {
		t.Fatalf("tile north of grass items %v: moving a stone should not redo borders", got)
	}
	if got := itemIDs(e.Map.Tile(at(61, 61))); !slices.Equal(got, []uint16{stoneID}) {
		t.Fatalf("stone target items %v, want only the stone", got)
	}
}

func TestMoveSelectionReshapesTablesAround(t *testing.T) {
	e := testEditor(t)
	selectionDesks(e, 30, 32)
	e.SelectTiles(points(at(32, 30)), false)
	if err := e.MoveSelection(2, 0, 0); err != nil {
		t.Fatal(err)
	}
	want := [][]uint16{{tableWestID}, {tableEID}, nil, nil, {tableAloneID}}
	if got := selectionDeskRow(e, 30, 34); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("desk row = %v, want %v: the row end and the moved desk reshape", got, want)
	}
}

func TestDeleteSelectionRemovesSelectedTilesAndCountsThem(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, false)
	if n := e.DeleteSelection(""); n != 9 {
		t.Fatalf("DeleteSelection = %d, want 9", n)
	}
	for _, pos := range selectionSquare(69, 69, 71, 71) {
		if e.Map.Tile(pos) != nil {
			t.Fatalf("tile %v left after delete", pos)
		}
	}
	if e.SelectionCount() != 0 {
		t.Fatalf("SelectionCount() = %d, want 0", e.SelectionCount())
	}
	if undo, _ := e.History(); undo[len(undo)-1].Label != "Delete" {
		t.Fatalf("last undo step %q, want the default label \"Delete\"", undo[len(undo)-1].Label)
	}
}

func TestDeleteSelectionUsesGivenLabel(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectTiles(points(at(70, 70)), false)
	e.DeleteSelection("Erase area")
	if undo, _ := e.History(); undo[len(undo)-1].Label != "Erase area" {
		t.Fatalf("last undo step %q, want \"Erase area\"", undo[len(undo)-1].Label)
	}
}

func TestDeleteSelectionWithNothingSelectedReturnsZero(t *testing.T) {
	e := selectionSandWithStone(t)
	undoBefore, _ := e.History()
	if n := e.DeleteSelection(""); n != 0 {
		t.Fatalf("DeleteSelection = %d, want 0", n)
	}
	if undoAfter, _ := e.History(); len(undoAfter) != len(undoBefore) {
		t.Fatal("deleting nothing added an undo step")
	}
}

func TestDeleteSelectionKeepsUnselectedItems(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectItem(at(70, 70), -1, false)
	e.DeleteSelection("")
	if tile := e.Map.Tile(at(70, 70)); tile == nil || tile.Ground.ID != 0 || !slices.Equal(itemIDs(tile), []uint16{stoneID}) {
		t.Fatalf("tile %+v, want the stone without ground", tile)
	}
}

func TestDeleteSelectionUndoRestoresTilesAndSelection(t *testing.T) {
	e := selectionSandWithStone(t)
	e.SelectRect(*rect(69, 69, 71, 71), FloorsCurrent, false, false, false)
	e.DeleteSelection("")
	if label := e.Undo(); label != "Delete" {
		t.Fatalf("Undo() = %q, want \"Delete\"", label)
	}
	if tile := e.Map.Tile(at(70, 70)); tile == nil || !slices.Equal(itemIDs(tile), []uint16{stoneID}) {
		t.Fatalf("tile after undo %+v, want sand with a stone", tile)
	}
	if e.SelectionCount() != 9 {
		t.Fatalf("SelectionCount() after undo = %d, want the 9 restored selected tiles", e.SelectionCount())
	}
}

func TestDeleteSelectionRedoesBordersAround(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(60, 60)), Size: 2})
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(60, 60))})
	e.SelectItem(at(60, 60), -1, false)
	e.DeleteSelection("")
	for xy := range selectionGrassBorders {
		if got := itemIDs(e.Map.Tile(at(xy[0], xy[1]))); len(got) != 0 {
			t.Errorf("tile %v items %v, want the grass border gone with the grass", xy, got)
		}
	}
}

func TestDeleteSelectionReshapesTablesAround(t *testing.T) {
	e := testEditor(t)
	selectionDesks(e, 30, 32)
	e.SelectTiles(points(at(32, 30)), false)
	e.DeleteSelection("")
	want := [][]uint16{{tableWestID}, {tableEID}, nil}
	if got := selectionDeskRow(e, 30, 32); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("desk row = %v, want %v: the middle desk becomes the east end", got, want)
	}
}

func TestBorderizeSelectionAddsMissingBorders(t *testing.T) {
	e := testEditor(t)
	e.Settings.Automagic = false
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(60, 60)), Size: 2})
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(60, 60))})
	e.SelectRect(*rect(58, 58, 62, 62), FloorsCurrent, false, false, false)
	if n := e.BorderizeSelection(); n != len(selectionGrassBorders) {
		t.Fatalf("BorderizeSelection = %d, want %d bordered tiles", n, len(selectionGrassBorders))
	}
	for xy, id := range selectionGrassBorders {
		if got := itemIDs(e.Map.Tile(at(xy[0], xy[1]))); !slices.Equal(got, []uint16{id}) {
			t.Errorf("tile %v items %v, want [%d]", xy, got, id)
		}
	}
	if undo, _ := e.History(); undo[len(undo)-1].Label != "Borderize selection" {
		t.Fatalf("last undo step %q, want \"Borderize selection\"", undo[len(undo)-1].Label)
	}
}

func TestBorderizeSelectionFixesWalls(t *testing.T) {
	e := testEditor(t)
	// Three horizontal wall items placed raw: borderize turns them into a
	// wall line, the west end a pole.
	for x := 20; x <= 22; x++ {
		e.Map.SetTile(at(x, 20), &otmap.Tile{Items: []otmap.Item{{ID: wallV}}})
	}
	e.SelectRect(*rect(20, 20, 22, 20), FloorsCurrent, false, false, false)
	e.BorderizeSelection()
	want := [][]uint16{{wallPoleID}, {wallH}, {wallH}}
	var got [][]uint16
	for x := 20; x <= 22; x++ {
		got = append(got, itemIDs(e.Map.Tile(at(x, 20))))
	}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("walls = %v, want %v", got, want)
	}
}

func TestRandomizeSelectionRerollsGroundsOfRandomBrushes(t *testing.T) {
	e := testEditor(t)
	e.Settings.Automagic = false
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(50, 50)), Size: 2})
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(48, 48))})
	e.Map.SetTile(at(47, 47), &otmap.Tile{Items: []otmap.Item{{ID: stoneID}}})
	e.SelectRect(*rect(47, 47, 52, 52), FloorsCurrent, false, false, false)
	before := map[otmap.Position]uint16{}
	for _, pos := range e.Selected() {
		before[pos] = e.Map.Tile(pos).Ground.ID
	}
	changedCount := e.RandomizeSelection()
	differing := 0
	for pos, ground := range before {
		after := e.Map.Tile(pos).Ground.ID
		if after != ground {
			differing++
		}
		if ground == grassA || ground == grassB {
			if after != grassA && after != grassB {
				t.Errorf("tile %v ground %d, want a grass item", pos, after)
			}
		} else if after != ground {
			t.Errorf("tile %v ground changed from %d to %d: only grass randomizes", pos, ground, after)
		}
	}
	if changedCount != differing || changedCount == 0 {
		t.Fatalf("RandomizeSelection = %d, %d grounds changed: want them equal and above 0", changedCount, differing)
	}
	if undo, _ := e.History(); undo[len(undo)-1].Label != "Randomize selection" {
		t.Fatalf("last undo step %q, want \"Randomize selection\"", undo[len(undo)-1].Label)
	}
}

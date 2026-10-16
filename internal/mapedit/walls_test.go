package mapedit

import (
	"reflect"
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/materials"
	"github.com/nekiro/ots-creator/internal/otitems"
	"github.com/nekiro/ots-creator/internal/otmap"
)

// Item ids of the extra materials built by wallsEditor.
const (
	fullWallBase    = 1000 // "full wall": a piece for every alignment, id = base + alignment
	fullDoorH       = 1100 // normal door, closed, horizontal
	fullDoorHOpen   = 1101
	fullDoorV       = 1102
	fullDoorVOpen   = 1103
	fullLockedH     = 1104 // locked door, only closed
	fullWindowH     = 1105
	fullWindowHOpen = 1106
	fullHatedDoorH  = 1107 // magic door that neighbors do not connect to
	ivyBase         = 1200 // "ivy" decoration: pieces for alignments 0..14 (no intersection)
	ivyDoorH        = 1300
	ivyDoorHOpen    = 1301
	ivyDoorV        = 1302
	ivyDoorVOpen    = 1303
	mossBase        = 1400 // "moss" decoration: pieces for alignments 0..15
	brickH          = 1500 // "brick wall": only horizontal, redirects to "full wall"
	woodBase        = 1600 // "wood wall": full set, friend of "full wall"
	plankH          = 1700 // "plank wall": only horizontal
	loopPoleA       = 1800 // "loop a" redirects to "loop b" and back
	loopPoleB       = 1801
	fullRugBase     = 1900 // "full rug": every carpet piece, id = base + edge
	runnerN         = 2000 // "runner": only the "n" piece
	fullDeskBase    = 2100 // "full desk": every table piece, id = base + table alignment
	shortDeskH      = 2200 // "short desk": only horizontal
)

// wallsEditor is testEditor with extra wall, decoration, carpet and table
// materials the fixture cannot express.
func wallsEditor(t *testing.T) *Editor {
	t.Helper()
	base := testEditor(t)
	var types []*otitems.Type
	base.Items.Each(func(itemType *otitems.Type) { types = append(types, itemType) })
	addType := func(id uint16, topOrder uint8, door bool) {
		itemType := &otitems.Type{ID: id, ClientID: id, Flags: otitems.FlagUnpassable, TopOrder: topOrder, Name: "item"}
		if door {
			itemType.Kind = otitems.KindDoor
		}
		types = append(types, itemType)
	}
	wallPieces := func(baseID uint16, count int) map[string]*materials.WallPart {
		parts := map[string]*materials.WallPart{}
		for align := 0; align < count; align++ {
			parts[materials.WallAlignments[align]] = &materials.WallPart{Items: []materials.Chance{{ID: baseID + uint16(align), Chance: 1}}}
			addType(baseID+uint16(align), bottomTopOrder, false)
		}
		return parts
	}

	fullWalls := wallPieces(fullWallBase, wallAlignmentMax)
	fullWalls["horizontal"].Doors = []materials.Door{
		{ID: fullDoorH, Type: "normal"}, {ID: fullDoorHOpen, Type: "normal", Open: true},
		{ID: fullLockedH, Type: "locked"},
		{ID: fullWindowH, Type: "window"}, {ID: fullWindowHOpen, Type: "window", Open: true},
		{ID: fullHatedDoorH, Type: "magic", Hate: true},
	}
	fullWalls["vertical"].Doors = []materials.Door{{ID: fullDoorV, Type: "normal"}, {ID: fullDoorVOpen, Type: "normal", Open: true}}
	ivy := wallPieces(ivyBase, wallAlignmentMax-2)
	ivy["horizontal"].Doors = []materials.Door{{ID: ivyDoorH, Type: "normal"}, {ID: ivyDoorHOpen, Type: "normal", Open: true}}
	ivy["vertical"].Doors = []materials.Door{{ID: ivyDoorV, Type: "normal"}, {ID: ivyDoorVOpen, Type: "normal", Open: true}}
	for _, id := range []uint16{fullDoorH, fullDoorHOpen, fullDoorV, fullDoorVOpen, fullLockedH, fullWindowH, fullWindowHOpen, fullHatedDoorH, ivyDoorH, ivyDoorHOpen, ivyDoorV, ivyDoorVOpen} {
		addType(id, bottomTopOrder, true)
	}
	for _, id := range []uint16{brickH, plankH, loopPoleA, loopPoleB} {
		addType(id, bottomTopOrder, false)
	}

	rugParts := map[string][]materials.Chance{}
	for name, edge := range edgeIndex {
		rugParts[name] = []materials.Chance{{ID: fullRugBase + uint16(edge), Chance: 1}}
		addType(fullRugBase+uint16(edge), commonTopOrder, false)
	}
	deskParts := map[string][]materials.Chance{}
	for align, name := range materials.TableAlignments {
		deskParts[name] = []materials.Chance{{ID: fullDeskBase + uint16(align), Chance: 1}}
		addType(fullDeskBase+uint16(align), commonTopOrder, false)
	}
	addType(runnerN, commonTopOrder, false)
	addType(shortDeskH, commonTopOrder, false)

	singlePart := func(align string, id uint16) map[string]*materials.WallPart {
		return map[string]*materials.WallPart{align: {Items: []materials.Chance{{ID: id, Chance: 1}}}}
	}
	extra := []*materials.Brush{
		{Name: "full wall", Type: materials.TypeWall, Walls: fullWalls},
		{Name: "ivy", Type: materials.TypeDecoration, Walls: ivy},
		{Name: "moss", Type: materials.TypeDecoration, Walls: wallPieces(mossBase, wallAlignmentMax-1)},
		{Name: "brick wall", Type: materials.TypeWall, Walls: singlePart("horizontal", brickH), Redirect: "full wall"},
		{Name: "wood wall", Type: materials.TypeWall, Walls: wallPieces(woodBase, wallAlignmentMax-1), Friends: []string{"full wall"}},
		{Name: "plank wall", Type: materials.TypeWall, Walls: singlePart("horizontal", plankH)},
		{Name: "loop a", Type: materials.TypeWall, Walls: singlePart("pole", loopPoleA), Redirect: "loop b"},
		{Name: "loop b", Type: materials.TypeWall, Walls: singlePart("pole", loopPoleB), Redirect: "loop a"},
		{Name: "full rug", Type: materials.TypeCarpet, Parts: rugParts},
		{Name: "runner", Type: materials.TypeCarpet, Parts: map[string][]materials.Chance{"n": {{ID: runnerN, Chance: 1}}}},
		{Name: "full desk", Type: materials.TypeTable, Parts: deskParts},
		{Name: "short desk", Type: materials.TypeTable, Parts: map[string][]materials.Chance{"horizontal": {{ID: shortDeskH, Chance: 1}}}},
	}
	db := otitems.NewDB(otitems.ModeOTB, types)
	allMaterials := *base.Brushes.Materials
	allMaterials.Brushes = append(slices.Clone(allMaterials.Brushes), extra...)
	editor := New(base.Map, db, NewBrushes(db, &allMaterials))
	editor.Seed(1)
	return editor
}

// wallsDraw applies a draw request and fails the test on an error.
func wallsDraw(t *testing.T, editor *Editor, req DrawRequest) int {
	t.Helper()
	changed, err := editor.Draw(req)
	if err != nil {
		t.Fatalf("Draw(%s %q): unexpected error %v", req.Kind, req.Name, err)
	}
	return changed
}

// wallsPut places a tile with the given items, without any automatic
// reshaping.
func wallsPut(editor *Editor, pos otmap.Position, ids ...uint16) *otmap.Tile {
	tile := &otmap.Tile{}
	for _, id := range ids {
		tile.Items = append(tile.Items, otmap.Item{ID: id})
	}
	editor.Map.Put(pos, tile)
	return tile
}

// wallsExpect checks the item ids on the tile at x, y.
func wallsExpect(t *testing.T, editor *Editor, x, y int, want []uint16, why string) {
	t.Helper()
	if got := itemIDs(editor.Map.Tile(at(x, y))); !slices.Equal(got, want) {
		t.Errorf("tile %d,%d: got %v, want %v (%s)", x, y, got, want, why)
	}
}

// wallsNeighbors returns the center and the positions of its wall
// neighbors in mask.
func wallsNeighbors(centerX, centerY, mask int) []otmap.Position {
	positions := []otmap.Position{at(centerX, centerY)}
	for _, neighbor := range wallNeighborOffsets {
		if mask&neighbor.bit != 0 {
			positions = append(positions, at(centerX+neighbor.dx, centerY+neighbor.dy))
		}
	}
	return positions
}

func wallsMaskName(mask int) string {
	name := ""
	for _, side := range []struct {
		bit   int
		label string
	}{{wallN, "N"}, {wallW, "W"}, {wallE, "E"}, {wallS, "S"}} {
		if mask&side.bit != 0 {
			name += side.label
		}
	}
	if name == "" {
		return "none"
	}
	return name
}

func TestWallShapeEqualsNeighborMaskWithFullShapeSet(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		t.Run(wallsMaskName(mask), func(t *testing.T) {
			editor := wallsEditor(t)
			wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: wallsNeighbors(50, 50, mask)})
			wallsExpect(t, editor, 50, 50, []uint16{fullWallBase + uint16(mask)},
				"a brush with every shape uses the alignment numbered like the neighbor mask "+materials.WallAlignments[mask])
		})
	}
}

func TestWallShapeFallsBackToReducedShapeSet(t *testing.T) {
	// The stone wall has only pole, corner, horizontal and vertical: masks
	// without their own piece use only the north and west neighbors.
	want := map[int]uint16{
		0: wallPoleID, wallN: wallV, wallW: wallH, wallN | wallW: wallC,
		wallE: wallPoleID, wallN | wallE: wallV, wallW | wallE: wallH, wallN | wallW | wallE: wallC,
		wallS: wallPoleID, wallN | wallS: wallV, wallW | wallS: wallH, wallN | wallW | wallS: wallC,
		wallE | wallS: wallPoleID, wallN | wallE | wallS: wallV, wallW | wallE | wallS: wallH, wallN | wallW | wallE | wallS: wallC,
	}
	for mask := 0; mask < 16; mask++ {
		t.Run(wallsMaskName(mask), func(t *testing.T) {
			editor := testEditor(t)
			wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: wallsNeighbors(50, 50, mask)})
			wallsExpect(t, editor, 50, 50, []uint16{want[mask]}, "reduced set: N gives vertical, W horizontal, N+W corner, else pole")
		})
	}
}

func TestWallLineGetsEndsAndStraightPiece(t *testing.T) {
	editor := wallsEditor(t)
	for x := 10; x <= 12; x++ {
		wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(x, 10))})
	}
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallE}, "west end continues east")
	wallsExpect(t, editor, 11, 10, []uint16{fullWallBase + wallHorizontal}, "middle is horizontal")
	wallsExpect(t, editor, 12, 10, []uint16{fullWallBase + wallW}, "east end continues west")
}

func TestWallLineWestEndIsPoleInReducedSet(t *testing.T) {
	editor := testEditor(t)
	for x := 20; x <= 22; x++ {
		wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(x, 20))})
	}
	wallsExpect(t, editor, 20, 20, []uint16{wallPoleID}, "the west end has no west neighbor and the reduced set has no west end piece")
	wallsExpect(t, editor, 21, 20, []uint16{wallH}, "middle is horizontal")
	wallsExpect(t, editor, 22, 20, []uint16{wallH}, "east end has a west neighbor")
}

func TestWallBelowLineEndTurnsVertical(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(20, 20, 22, 20)})
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(22, 21))})
	wallsExpect(t, editor, 22, 21, []uint16{wallV}, "a wall with a north neighbor is vertical")
	wallsExpect(t, editor, 22, 20, []uint16{wallH}, "the line end keeps horizontal (west neighbor decides)")
}

func TestWallRectDrawsBoxWithCorners(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Rect: rect(10, 10, 13, 12)})
	want := map[otmap.Position]int{
		at(10, 10): wallE | wallS, at(11, 10): wallHorizontal, at(12, 10): wallHorizontal, at(13, 10): wallW | wallS,
		at(10, 11): wallVertical, at(13, 11): wallVertical,
		at(10, 12): wallN | wallE, at(11, 12): wallHorizontal, at(12, 12): wallHorizontal, at(13, 12): wallN | wallW,
	}
	for y := 10; y <= 12; y++ {
		for x := 10; x <= 13; x++ {
			if align, ok := want[at(x, y)]; ok {
				wallsExpect(t, editor, x, y, []uint16{fullWallBase + uint16(align)}, "box outline piece "+materials.WallAlignments[align])
			} else {
				wallsExpect(t, editor, x, y, nil, "a wall rectangle draws only the outline")
			}
		}
	}
}

func TestWallBoxWithReducedSetUsesCornerPiece(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(10, 10, 13, 12)})
	want := map[otmap.Position]uint16{
		at(10, 10): wallPoleID, at(11, 10): wallH, at(12, 10): wallH, at(13, 10): wallH,
		at(10, 11): wallV, at(13, 11): wallV,
		at(10, 12): wallV, at(11, 12): wallH, at(12, 12): wallH, at(13, 12): wallC,
	}
	for pos, id := range want {
		wallsExpect(t, editor, int(pos.X), int(pos.Y), []uint16{id}, "reduced set box: NW pole, SE corner, others by north/west neighbor")
	}
}

func TestWallTJunction(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Rect: rect(10, 10, 12, 10)})
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(11, 11))})
	wallsExpect(t, editor, 11, 10, []uint16{fullWallBase + wallW + wallE + wallS}, "a line with a wall below the middle makes a T")
	wallsExpect(t, editor, 11, 11, []uint16{fullWallBase + wallN}, "the stem continues north")
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallE}, "line ends stay ends")
	wallsExpect(t, editor, 12, 10, []uint16{fullWallBase + wallW}, "line ends stay ends")
}

func TestWallBrushSizeDrawsHollowSquare(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(11, 11)), Size: 1})
	wallsExpect(t, editor, 11, 11, nil, "the center of a sized wall brush stays empty")
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallE + wallS}, "the square's NW corner")
	wallsExpect(t, editor, 12, 12, []uint16{fullWallBase + wallN + wallW}, "the square's SE corner")
}

func TestEraseWallReshapesNeighbors(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Rect: rect(10, 10, 12, 10)})
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(11, 10)), Erase: true})
	wallsExpect(t, editor, 11, 10, nil, "the erased wall is gone")
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallPole}, "a wall without neighbors is a pole")
	wallsExpect(t, editor, 12, 10, []uint16{fullWallBase + wallPole}, "a wall without neighbors is a pole")
}

func TestEraseWallRemovesOnlyThatBrushWalls(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase, stoneID)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(10, 10)), Erase: true})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase, stoneID}, "erasing stone wall keeps other brushes' walls and items")
	if changed := wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(30, 30)), Erase: true}); changed != 0 {
		t.Errorf("erase on an empty tile changed %d tiles, want 0", changed)
	}
}

func TestEraseWallRemovesDoorsOfThatBrush(t *testing.T) {
	editor := testEditor(t)
	wallsPut(editor, at(10, 10), doorHClosed)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(10, 10)), Erase: true})
	wallsExpect(t, editor, 10, 10, nil, "a door belongs to its wall brush and is erased with it")
}

func TestDrawWallTwiceKeepsOneWall(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(10, 10))})
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{wallPoleID}, "drawing a wall replaces the brush's own wall")
}

func TestWallReshapeKeepsOtherItems(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase, stoneID)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(11, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallE, stoneID}, "the wall reshapes, the stone stays")
}

func TestWallWithoutFittingShapeKeepsItsItem(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "plank wall", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{plankH}, "a brush without a pole keeps the piece it drew")
}

func TestUntouchableWallKeepsItsShape(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase+wallUntouchable)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(11, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallUntouchable}, "untouchable walls never reshape")
	wallsExpect(t, editor, 11, 10, []uint16{fullWallBase + wallW}, "but neighbors still connect to them")
}

func TestFriendWallsConnect(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(10, 10))})
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "wood wall", Points: points(at(11, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallE}, "full wall connects to wood wall that names it a friend")
	wallsExpect(t, editor, 11, 10, []uint16{woodBase + wallW}, "wood wall connects to its friend")
}

func TestUnrelatedWallsDoNotConnect(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(10, 10))})
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(11, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{wallPoleID}, "stone wall ignores full wall")
	wallsExpect(t, editor, 11, 10, []uint16{fullWallBase + wallPole}, "full wall ignores stone wall")
}

func TestHatedDoorDoesNotConnectNeighbors(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Rect: rect(10, 10, 12, 10)})
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "magic", Points: points(at(11, 10))})
	wallsExpect(t, editor, 11, 10, []uint16{fullHatedDoorH}, "the magic door replaces the horizontal wall")
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallPole}, "walls do not connect to a hated door")
	wallsExpect(t, editor, 12, 10, []uint16{fullWallBase + wallPole}, "walls do not connect to a hated door")
}

func TestWallPickFollowsRedirectForMissingShape(t *testing.T) {
	editor := wallsEditor(t)
	brick := editor.Brushes.Wall("brick wall")
	if got := brick.pick(wallHorizontal, editor.rng); got != brickH {
		t.Errorf("pick(horizontal) = %d, want %d (brick's own piece)", got, brickH)
	}
	if got := brick.pick(wallVertical, editor.rng); got != fullWallBase+wallVertical {
		t.Errorf("pick(vertical) = %d, want %d (from the redirect target)", got, fullWallBase+wallVertical)
	}
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "brick wall", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallPole}, "a lone brick wall takes the pole of its redirect target")
}

func TestWallHasWallFollowsRedirect(t *testing.T) {
	editor := wallsEditor(t)
	brick := editor.Brushes.Wall("brick wall")
	cases := []struct {
		name  string
		id    uint16
		align int
		want  bool
	}{
		{"own piece", brickH, wallHorizontal, true},
		{"redirect target piece", fullWallBase + wallVertical, wallVertical, true},
		{"redirect target door", fullDoorV, wallVertical, true},
		{"other brush", wallV, wallVertical, false},
		{"wrong alignment", brickH, wallVertical, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := brick.hasWall(testCase.id, testCase.align); got != testCase.want {
				t.Errorf("hasWall(%d, %d) = %v, want %v", testCase.id, testCase.align, got, testCase.want)
			}
		})
	}
}

func TestEraseWithRedirectingBrushRemovesTargetWalls(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullDoorV)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "brick wall", Points: points(at(10, 10)), Erase: true})
	wallsExpect(t, editor, 10, 10, nil, "the brick brush owns the full wall pieces it redirects to")
}

func TestWallRedirectCycleStops(t *testing.T) {
	editor := wallsEditor(t)
	loop := editor.Brushes.Wall("loop a")
	if got := loop.pick(wallHorizontal, editor.rng); got != 0 {
		t.Errorf("pick(horizontal) = %d, want 0 (no brush of the cycle has it)", got)
	}
	if got := loop.pick(wallPole, editor.rng); got != loopPoleA {
		t.Errorf("pick(pole) = %d, want %d", got, loopPoleA)
	}
	if loop.hasWall(fullWallBase, wallPole) {
		t.Error("hasWall of an unknown item in a redirect cycle must be false")
	}
	if got := loop.doorFor(wallPole, doorTypeNormal, false); got != 0 {
		t.Errorf("doorFor = %d, want 0", got)
	}
}

func TestDecorationMatchesWallShape(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Rect: rect(10, 10, 12, 10)})
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "ivy", Rect: rect(10, 10, 12, 10)})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallE, ivyBase + wallE}, "ivy takes the west end shape")
	wallsExpect(t, editor, 11, 10, []uint16{fullWallBase + wallHorizontal, ivyBase + wallHorizontal}, "ivy takes the horizontal shape")
	wallsExpect(t, editor, 12, 10, []uint16{fullWallBase + wallW, ivyBase + wallW}, "ivy takes the east end shape")
}

func TestDecorationOnDoorUsesDecorationDoor(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullDoorHOpen)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "ivy", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullDoorHOpen, ivyDoorHOpen}, "an open door gets the open ivy door")
}

func TestDecorationRedrawKeepsOne(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "ivy", Points: points(at(10, 10))})
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "ivy", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase, ivyBase}, "drawing ivy again replaces the old ivy")
}

func TestSecondDecorationBrushKeepsFirst(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "ivy", Points: points(at(10, 10))})
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "moss", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase, mossBase, ivyBase}, "moss goes right above the wall, ivy stays")
}

func TestDecorationWithoutPieceForShapeAddsNothing(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase+wallN+wallW+wallE+wallS)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "ivy", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallN + wallW + wallE + wallS}, "ivy has no intersection piece")
}

func TestDecorationFollowsWallReshape(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase, ivyBase)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(11, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallE, ivyBase + wallE}, "the ivy above the pole becomes a west end too")
}

func TestDecorationKeepsPieceWhenMissingNewShape(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase, ivyBase)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(10, 9), at(9, 10), at(11, 10), at(10, 11))})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallN + wallW + wallE + wallS, ivyBase}, "ivy has no intersection piece and keeps its pole")
}

func TestDecorationDoesNotConnectWalls(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), ivyBase)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(11, 10))})
	wallsExpect(t, editor, 11, 10, []uint16{fullWallBase + wallPole}, "a decoration alone is not a wall neighbor")
}

func TestEraseDecorationKeepsWall(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase, ivyBase)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "ivy", Points: points(at(10, 10)), Erase: true})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase}, "erasing ivy removes only the ivy")
}

func TestCleanWallsWithoutBrushRemovesEveryWall(t *testing.T) {
	editor := wallsEditor(t)
	tile := &otmap.Tile{Items: []otmap.Item{{ID: wallH}, {ID: fullDoorH}, {ID: ivyDoorH}, {ID: stoneID}}}
	editor.cleanWalls(tile, nil)
	if got := itemIDs(tile); !slices.Equal(got, []uint16{stoneID}) {
		t.Errorf("got %v, want only the stone: walls, doors and decorations of any brush go", got)
	}
}

func TestAltClickCyclesWallShape(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(10, 10))})
	// The stone wall has pieces for pole (0), corner (3), horizontal (6)
	// and vertical (9); the cycle wraps back to the pole.
	for _, want := range []uint16{wallC, wallH, wallV, wallPoleID} {
		wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(10, 10)), Alt: true})
		wallsExpect(t, editor, 10, 10, []uint16{want}, "alt click moves to the next alignment with a piece")
	}
}

func TestAltClickOnWallWithOneShapeKeepsIt(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), plankH)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "plank wall", Points: points(at(10, 10)), Alt: true})
	wallsExpect(t, editor, 10, 10, []uint16{plankH}, "there is no other shape to cycle to")
}

func TestAltClickIgnoresOtherBrushesAndEmptyTiles(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase, stoneID)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(10, 10)), Alt: true})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase, stoneID}, "alt click with stone wall leaves a full wall alone")
	if changed := wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(30, 30)), Alt: true}); changed != 0 {
		t.Errorf("alt click on an empty tile changed %d tiles, want 0", changed)
	}
	wallsExpect(t, editor, 30, 30, nil, "alt click does not draw")
}

func TestAltWithBrushSizeDrawsWalls(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(11, 11)), Size: 1, Alt: true})
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallE + wallS}, "alt only cycles shapes on a single click")
}

func TestDoorReplacesWallClosed(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(20, 20, 22, 20)})
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "normal", Points: points(at(21, 20))})
	wallsExpect(t, editor, 21, 20, []uint16{doorHClosed}, "a door click places a closed door")
	wallsExpect(t, editor, 20, 20, []uint16{wallPoleID}, "neighbors keep their shape")
	wallsExpect(t, editor, 22, 20, []uint16{wallH}, "neighbors still connect to the door")
}

func TestAltDoorIsPlacedOpen(t *testing.T) {
	editor := testEditor(t)
	wallsPut(editor, at(10, 10), wallH)
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "normal", Points: points(at(10, 10)), Alt: true})
	wallsExpect(t, editor, 10, 10, []uint16{doorHOpn}, "alt places the door open")
}

func TestUnknownDoorTypeDrawsNormalDoor(t *testing.T) {
	editor := testEditor(t)
	wallsPut(editor, at(10, 10), wallH)
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "no such type", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{doorHClosed}, "an unknown door type falls back to normal")
}

func TestDoorNeedsWallWithDoorForItsShape(t *testing.T) {
	cases := []struct {
		name     string
		wall     string // wall brush of the line drawn through 10,10
		line     *Rect
		doorType string
		want     []uint16
	}{
		{"wall shape without doors", "stone wall", rect(10, 9, 10, 11), "normal", []uint16{wallV}},
		{"door type the wall lacks", "full wall", rect(9, 10, 11, 10), "archway", []uint16{fullWallBase + wallHorizontal}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			editor := wallsEditor(t)
			wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: testCase.wall, Rect: testCase.line})
			wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: testCase.doorType, Points: points(at(10, 10))})
			wallsExpect(t, editor, 10, 10, testCase.want, "nothing to turn into a door")
		})
	}
	for _, items := range [][]uint16{{stoneID}, {ivyBase + wallHorizontal}} {
		editor := wallsEditor(t)
		wallsPut(editor, at(10, 10), items...)
		wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "normal", Points: points(at(10, 10))})
		wallsExpect(t, editor, 10, 10, items, "a tile without a wall gets no door")
	}
	editor := wallsEditor(t)
	if changed := wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "normal", Points: points(at(30, 30))}); changed != 0 {
		t.Errorf("door on an empty position changed %d tiles, want 0", changed)
	}
}

func TestDoorTypeChangeKeepsOpenState(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullDoorHOpen)
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "window", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullWindowHOpen}, "redrawing a door keeps it open even without alt")
}

func TestDoorFallsBackToOtherState(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase+wallHorizontal)
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "locked", Points: points(at(10, 10)), Alt: true})
	wallsExpect(t, editor, 10, 10, []uint16{fullLockedH}, "the locked door exists only closed")
}

func TestDoorOnRedirectingWallUsesTargetDoor(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), brickH)
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "normal", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullDoorH}, "brick wall has no doors; its redirect target does")
}

func TestDoorDecorationFollowsDoor(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullWallBase+wallHorizontal, ivyBase+wallHorizontal, stoneID)
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "normal", Points: points(at(10, 10)), Alt: true})
	wallsExpect(t, editor, 10, 10, []uint16{fullDoorHOpen, ivyDoorHOpen, stoneID}, "the ivy above turns into an open ivy door, the stone stays")
}

func TestDoorGetsHouseDoorID(t *testing.T) {
	cases := []struct {
		name       string
		houseID    uint32
		autoDoorID bool
		wantDoorID uint8
	}{
		{"house tile", 1, true, 1},
		{"auto door id off", 1, false, 0},
		{"no house", 0, true, 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			editor := testEditor(t)
			editor.Settings.AutoDoorID = testCase.autoDoorID
			wallsPut(editor, at(10, 10), wallH).HouseID = testCase.houseID
			wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "normal", Points: points(at(10, 10))})
			door := editor.Map.Tile(at(10, 10)).Items[0]
			var got uint8
			if door.Ext != nil {
				got = door.Ext.DoorID
			}
			if door.ID != doorHClosed || got != testCase.wantDoorID {
				t.Errorf("door %d with id %d, want %d with id %d", door.ID, got, doorHClosed, testCase.wantDoorID)
			}
		})
	}
}

func TestSwitchDoorTogglesOpenAndClosed(t *testing.T) {
	editor := testEditor(t)
	wallsPut(editor, at(10, 10), doorHClosed)
	for _, want := range []uint16{doorHOpn, doorHClosed} {
		if err := editor.SwitchDoor(at(10, 10), 0); err != nil {
			t.Fatalf("SwitchDoor: unexpected error %v", err)
		}
		wallsExpect(t, editor, 10, 10, []uint16{want}, "switching flips open and closed")
	}
}

func TestSwitchDoorErrors(t *testing.T) {
	cases := []struct {
		name  string
		items []uint16
		index int
		want  any
	}{
		{"empty tile", nil, 0, errText(ErrNoItem)},
		{"index out of range", []uint16{fullDoorH}, 1, errText(ErrNoItem)},
		{"negative index", []uint16{fullDoorH}, -1, errText(ErrNoItem)},
		{"plain wall", []uint16{fullWallBase}, 0, "no door brush knows this door"},
		{"door without other state", []uint16{fullLockedH}, 0, "no door brush knows this door"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			editor := wallsEditor(t)
			if testCase.items != nil {
				wallsPut(editor, at(10, 10), testCase.items...)
			}
			if got := errText(editor.SwitchDoor(at(10, 10), testCase.index)); got != testCase.want {
				t.Errorf("error %v, want %v", got, testCase.want)
			}
			wallsExpect(t, editor, 10, 10, testCase.items, "a failed switch changes nothing")
		})
	}
}

func TestEraseDoorPutsWallBack(t *testing.T) {
	cases := []struct {
		name      string
		door      uint16
		neighbor  uint16 // wall put on both sides of the door
		neighbors []otmap.Position
		want      uint16
	}{
		{"closed horizontal", doorHClosed, wallH, points(at(9, 10), at(11, 10)), wallH},
		{"open horizontal", doorHOpn, wallH, points(at(9, 10), at(11, 10)), wallH},
		{"vertical", fullDoorVOpen, fullWallBase + wallVertical, points(at(10, 9), at(10, 11)), fullWallBase + wallVertical},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			editor := wallsEditor(t)
			wallsPut(editor, at(10, 10), testCase.door)
			for _, pos := range testCase.neighbors {
				wallsPut(editor, pos, testCase.neighbor)
			}
			wallsDraw(t, editor, DrawRequest{Kind: KindDoor, Erase: true, Points: points(at(10, 10))})
			wallsExpect(t, editor, 10, 10, []uint16{testCase.want}, "the door turns back into the wall of its shape")
		})
	}
}

func TestEraseDoorAfterSwitchPutsWallBack(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(20, 20, 22, 20)})
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "normal", Points: points(at(21, 20))})
	if err := editor.SwitchDoor(at(21, 20), 0); err != nil {
		t.Fatalf("SwitchDoor: unexpected error %v", err)
	}
	wallsExpect(t, editor, 21, 20, []uint16{doorHOpn}, "the door is open")
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, Erase: true, Points: points(at(21, 20))})
	wallsExpect(t, editor, 21, 20, []uint16{wallH}, "erasing the door puts the horizontal wall back")
}

func TestEraseDoorClearsDoorID(t *testing.T) {
	cases := []struct {
		name    string
		ext     otmap.ItemExt
		wantExt *otmap.ItemExt
	}{
		{"only a door id", otmap.ItemExt{DoorID: 3}, nil},
		{"door id and action id", otmap.ItemExt{DoorID: 3, ActionID: 100}, &otmap.ItemExt{ActionID: 100}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			editor := testEditor(t)
			ext := testCase.ext
			editor.Map.Put(at(10, 10), &otmap.Tile{HouseID: 1, Items: []otmap.Item{{ID: doorHClosed, Ext: &ext}}})
			wallsPut(editor, at(9, 10), wallH)
			wallsPut(editor, at(11, 10), wallH)
			wallsDraw(t, editor, DrawRequest{Kind: KindDoor, Erase: true, Points: points(at(10, 10))})
			wall := editor.Map.Tile(at(10, 10)).Items[0]
			if wall.ID != wallH {
				t.Errorf("item %d, want wall %d", wall.ID, wallH)
			}
			if !reflect.DeepEqual(wall.Ext, testCase.wantExt) {
				t.Errorf("ext %+v, want %+v (a wall has no door id, other attributes stay)", wall.Ext, testCase.wantExt)
			}
		})
	}
}

func TestEraseDoorOnTileWithoutDoorChangesNothing(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(9, 10, 11, 10)})
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, Erase: true, Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{wallH}, "a plain wall stays")
	wallsPut(editor, at(30, 30), stoneID)
	if changed := wallsDraw(t, editor, DrawRequest{Kind: KindDoor, Erase: true, Points: points(at(30, 30), at(50, 50))}); changed != 0 {
		t.Errorf("changed %d tiles, want 0", changed)
	}
	wallsExpect(t, editor, 30, 30, []uint16{stoneID}, "no door to erase")
}

func TestDoorKeepsTypeAndStateWhenWallTurnsVertical(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Rect: rect(9, 10, 11, 10)})
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "normal", Points: points(at(10, 10)), Alt: true})
	wallsExpect(t, editor, 10, 10, []uint16{fullDoorHOpen}, "an open horizontal door")
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(9, 10), at(11, 10)), Erase: true})
	wallsExpect(t, editor, 10, 10, []uint16{fullDoorHOpen}, "there is no pole door, so the door stays")
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(10, 9), at(10, 11))})
	wallsExpect(t, editor, 10, 10, []uint16{fullDoorVOpen}, "between north and south walls the door turns vertical and stays open")
}

func TestDecorationDoorFollowsDoorReshape(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), fullDoorH, ivyDoorH)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(10, 9), at(10, 11))})
	wallsExpect(t, editor, 10, 10, []uint16{fullDoorV, ivyDoorV}, "the ivy door turns vertical with the door")
}

func TestDoorKeepsShapeWhenNewShapeHasNoDoor(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Rect: rect(9, 10, 11, 10)})
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "normal", Points: points(at(10, 10))})
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Points: points(at(10, 11))})
	wallsExpect(t, editor, 10, 10, []uint16{fullDoorH}, "a T has no door: the reduced shape (horizontal) keeps the door")
}

func TestWallsWithoutAutomagicStayPoles(t *testing.T) {
	editor := testEditor(t)
	editor.Settings.Automagic = false
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(20, 20, 22, 20)})
	for x := 20; x <= 22; x++ {
		wallsExpect(t, editor, x, 20, []uint16{wallPoleID}, "without automagic walls keep the first piece of the brush")
	}
}

func TestEraseWallWithoutAutomagicLeavesNeighbors(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(20, 20, 22, 20)})
	editor.Settings.Automagic = false
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(21, 20)), Erase: true})
	wallsExpect(t, editor, 21, 20, nil, "the wall is erased")
	wallsExpect(t, editor, 22, 20, []uint16{wallH}, "without automagic the neighbor keeps its shape")
}

func TestDoorWithoutAutomagicKeepsNeighbors(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindWall, Name: "full wall", Rect: rect(10, 10, 12, 10)})
	editor.Settings.Automagic = false
	wallsDraw(t, editor, DrawRequest{Kind: KindDoor, DoorType: "magic", Points: points(at(11, 10))})
	wallsExpect(t, editor, 11, 10, []uint16{fullHatedDoorH}, "the door is placed")
	wallsExpect(t, editor, 10, 10, []uint16{fullWallBase + wallE}, "without automagic the neighbor is not reshaped")
}

func TestWallCarpetTableUnknownBrushFails(t *testing.T) {
	for _, kind := range []string{KindWall, KindCarpet, KindTable} {
		t.Run(kind, func(t *testing.T) {
			editor := testEditor(t)
			changed, err := editor.Draw(DrawRequest{Kind: kind, Name: "nothing", Points: points(at(10, 10))})
			want := ErrNoBrush.Error() + ": " + kind + ` "nothing"`
			if errText(err) != want || changed != 0 {
				t.Errorf("Draw = %d, %v; want 0, %q", changed, errText(err), want)
			}
		})
	}
}

// Carpets.

func TestCarpetPieceForNeighborMask(t *testing.T) {
	allSides := bitN | bitE | bitS | bitW
	allCorners := bitNW | bitNE | bitSW | bitSE
	cases := []struct {
		name string
		mask int
		want int
	}{
		{"no neighbors", 0, edgeCenter},
		{"everything", allSides | allCorners, edgeCenter},
		{"sides without corners", allSides, edgeCenter},
		{"NW corner missing", allSides | bitNE | bitSW | bitSE, edgeDSE},
		{"NE corner missing", allSides | bitNW | bitSW | bitSE, edgeDSW},
		{"SW corner missing", allSides | bitNW | bitNE | bitSE, edgeDNE},
		{"SE corner missing", allSides | bitNW | bitNE | bitSW, edgeDNW},
		{"two corners missing", allSides | bitNW | bitNE, edgeCenter},
		{"no south", bitN | bitE | bitW | allCorners, edgeN},
		{"no north", bitE | bitS | bitW, edgeS},
		{"no east", bitN | bitS | bitW, edgeW},
		{"no west", bitN | bitE | bitS, edgeE},
		{"north and west", bitN | bitW | bitNW, edgeCNW},
		{"north and east", bitN | bitE, edgeCNE},
		{"south and west", bitS | bitW, edgeCSW},
		{"south and east", bitS | bitE | bitSE, edgeCSE},
		{"only north", bitN, edgeCenter},
		{"north and south", bitN | bitS, edgeCenter},
		{"east and west", bitE | bitW, edgeCenter},
		{"only corners", allCorners, edgeCenter},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := carpetPiece(testCase.mask); got != testCase.want {
				t.Errorf("carpetPiece(%08b) = %d, want %d", testCase.mask, got, testCase.want)
			}
		})
	}
}

func TestSingleCarpetIsCenter(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "rug", Points: points(at(30, 30))})
	wallsExpect(t, editor, 30, 30, []uint16{rugC}, "a carpet alone uses the center piece")
}

func TestCarpetSquareGetsEdgesAndCorners(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "full rug", Rect: rect(10, 10, 12, 12)})
	// Pieces are named by the sides the carpet continues to.
	want := [3][3]int{
		{edgeCSE, edgeS, edgeCSW},
		{edgeE, edgeCenter, edgeW},
		{edgeCNE, edgeN, edgeCNW},
	}
	for row := range want {
		for column, edge := range want[row] {
			wallsExpect(t, editor, 10+column, 10+row, []uint16{fullRugBase + uint16(edge)}, "3x3 carpet piece")
		}
	}
}

func TestCarpetFallsBackToCenterForMissingPiece(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "rug", Points: points(at(30, 30)), Size: 1})
	for y := 29; y <= 31; y++ {
		for x := 29; x <= 31; x++ {
			want := uint16(rugC)
			if x == 30 && y == 31 {
				// The south row's middle continues north, east and west:
				// piece "n", the only edge the rug has.
				want = rugN
			}
			wallsExpect(t, editor, x, y, []uint16{want}, "rug has only center and n pieces")
		}
	}
}

func TestCarpetCenterTurnsInnerDiagonalWhenCornerRemoved(t *testing.T) {
	cases := []struct {
		name    string
		corner  otmap.Position
		wantRug int
	}{
		{"NW", at(10, 10), edgeDSE},
		{"NE", at(12, 10), edgeDSW},
		{"SW", at(10, 12), edgeDNE},
		{"SE", at(12, 12), edgeDNW},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			editor := wallsEditor(t)
			wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "full rug", Rect: rect(10, 10, 12, 12)})
			wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "full rug", Points: points(testCase.corner), Erase: true})
			wallsExpect(t, editor, int(testCase.corner.X), int(testCase.corner.Y), nil, "the corner is erased")
			wallsExpect(t, editor, 11, 11, []uint16{fullRugBase + uint16(testCase.wantRug)}, "one missing corner gives an inner diagonal")
		})
	}
}

func TestRemovingCarpetReshapesNeighbors(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "full rug", Rect: rect(10, 10, 12, 12)})
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "full rug", Points: points(at(10, 10)), Erase: true})
	wallsExpect(t, editor, 11, 10, []uint16{fullRugBase + edgeCSE}, "the north edge lost its west neighbor: continues south and east")
	wallsExpect(t, editor, 10, 11, []uint16{fullRugBase + edgeCSE}, "the west edge lost its north neighbor: continues south and east")
	wallsExpect(t, editor, 12, 10, []uint16{fullRugBase + edgeCSW}, "unaffected corner keeps its piece")
}

func TestAddingCarpetReshapesNeighbors(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "full rug", Points: points(at(10, 10))})
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "full rug", Points: points(at(11, 10), at(10, 11))})
	wallsExpect(t, editor, 10, 10, []uint16{fullRugBase + edgeCSE}, "the first carpet now continues south and east")
}

func TestDifferentCarpetsDoNotJoin(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "full rug", Points: points(at(10, 10), at(10, 11))})
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "rug", Points: points(at(11, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullRugBase + edgeCenter}, "only same-brush neighbors count (south only gives center)")
	wallsExpect(t, editor, 11, 10, []uint16{rugC}, "the rug ignores the full rug")
}

func TestCarpetReshapeKeepsOtherItems(t *testing.T) {
	editor := wallsEditor(t)
	wallsPut(editor, at(10, 10), stoneID)
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "full rug", Points: points(at(10, 10), at(11, 10), at(10, 11))})
	wallsExpect(t, editor, 10, 10, []uint16{stoneID, fullRugBase + edgeCSE}, "the carpet reshapes, the stone stays")
}

func TestDrawingCarpetReplacesOtherCarpet(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "rug", Points: points(at(10, 10))})
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "full rug", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullRugBase + edgeCenter}, "a tile holds one carpet")
}

func TestCarpetWithoutCenterUsesFirstPiece(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "runner", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{runnerN}, "runner has only an n piece and keeps it")
}

func TestEraseCarpetOnTileWithoutCarpetChangesNothing(t *testing.T) {
	editor := testEditor(t)
	wallsPut(editor, at(10, 10), stoneID)
	if changed := wallsDraw(t, editor, DrawRequest{Kind: KindCarpet, Name: "rug", Points: points(at(10, 10)), Erase: true}); changed != 0 {
		t.Errorf("changed %d tiles, want 0", changed)
	}
	wallsExpect(t, editor, 10, 10, []uint16{stoneID}, "no carpet to erase")
}

// Tables.

func TestTablePieceForNeighborMask(t *testing.T) {
	cases := []struct {
		name string
		mask int
		want int
	}{
		{"no neighbors", 0, tableAlone},
		{"only corners", bitNW | bitNE | bitSW | bitSE, tableAlone},
		{"east", bitE, tableWest},
		{"west", bitW, tableEast},
		{"east and west", bitE | bitW, tableHorizontal},
		{"south", bitS, tableNorth},
		{"north", bitN, tableSouth},
		{"north and south", bitN | bitS, tableVertical},
		{"east wins over north", bitE | bitN, tableWest},
		{"west wins over south", bitW | bitS, tableEast},
		{"all sides", bitN | bitE | bitS | bitW, tableHorizontal},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := tablePiece(testCase.mask); got != testCase.want {
				t.Errorf("tablePiece(%08b) = %d, want %d", testCase.mask, got, testCase.want)
			}
		})
	}
}

func TestSingleTableIsAlone(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Points: points(at(40, 40))})
	wallsExpect(t, editor, 40, 40, []uint16{tableAloneID}, "a table alone uses the alone piece")
}

func TestAddingTableReshapesNeighbor(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Points: points(at(40, 40))})
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Points: points(at(41, 40))})
	wallsExpect(t, editor, 40, 40, []uint16{tableWestID}, "the old table becomes the west end")
	wallsExpect(t, editor, 41, 40, []uint16{tableEID}, "the new table is the east end")
}

func TestTableLineGetsEndsAndMiddle(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Rect: rect(160, 160, 163, 160)})
	for x, want := range map[int]uint16{160: tableWestID, 161: tableH, 162: tableH, 163: tableEID} {
		wallsExpect(t, editor, x, 160, []uint16{want}, "west end, horizontal middle, east end")
	}
}

func TestVerticalTableLine(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "full desk", Rect: rect(10, 10, 10, 12)})
	wallsExpect(t, editor, 10, 10, []uint16{fullDeskBase + tableNorth}, "north end")
	wallsExpect(t, editor, 10, 11, []uint16{fullDeskBase + tableVertical}, "vertical middle")
	wallsExpect(t, editor, 10, 12, []uint16{fullDeskBase + tableSouth}, "south end")
}

func TestTableFallsBackToAloneForMissingPiece(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Rect: rect(10, 10, 10, 12)})
	for y := 10; y <= 12; y++ {
		wallsExpect(t, editor, 10, y, []uint16{tableAloneID}, "desk has no vertical pieces")
	}
}

func TestRemovingTableReshapesNeighbors(t *testing.T) {
	editor := testEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Rect: rect(10, 10, 12, 10)})
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Points: points(at(11, 10)), Erase: true})
	wallsExpect(t, editor, 11, 10, nil, "the table is erased")
	wallsExpect(t, editor, 10, 10, []uint16{tableAloneID}, "the ends are alone again")
	wallsExpect(t, editor, 12, 10, []uint16{tableAloneID}, "the ends are alone again")
}

func TestDifferentTablesDoNotJoin(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Points: points(at(10, 10))})
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "full desk", Points: points(at(11, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{tableAloneID}, "desk ignores full desk")
	wallsExpect(t, editor, 11, 10, []uint16{fullDeskBase + tableAlone}, "full desk ignores desk")
}

func TestTableReshapeKeepsOtherItems(t *testing.T) {
	editor := testEditor(t)
	wallsPut(editor, at(10, 10), stoneID)
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Points: points(at(10, 10), at(11, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{stoneID, tableWestID}, "the table reshapes, the stone stays")
}

func TestDrawingTableReplacesOtherTable(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Points: points(at(10, 10))})
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "full desk", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{fullDeskBase + tableAlone}, "a tile holds one table")
}

func TestTableWithoutAlonePieceUsesFirstPiece(t *testing.T) {
	editor := wallsEditor(t)
	wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "short desk", Points: points(at(10, 10))})
	wallsExpect(t, editor, 10, 10, []uint16{shortDeskH}, "short desk has only a horizontal piece and keeps it")
}

func TestEraseTableOnTileWithoutTableChangesNothing(t *testing.T) {
	editor := testEditor(t)
	wallsPut(editor, at(10, 10), stoneID)
	if changed := wallsDraw(t, editor, DrawRequest{Kind: KindTable, Name: "desk", Points: points(at(10, 10)), Erase: true}); changed != 0 {
		t.Errorf("changed %d tiles, want 0", changed)
	}
	wallsExpect(t, editor, 10, 10, []uint16{stoneID}, "no table to erase")
}

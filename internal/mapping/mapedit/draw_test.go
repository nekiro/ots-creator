package mapedit

import (
	"errors"
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func drawSetGround(e *Editor, ground uint16, positions ...otmap.Position) {
	for _, pos := range positions {
		e.Map.SetTile(pos, &otmap.Tile{Ground: otmap.Item{ID: ground}})
	}
}

func drawGroundAt(e *Editor, pos otmap.Position) uint16 {
	tile := e.Map.Tile(pos)
	if tile == nil {
		return 0
	}
	return tile.Ground.ID
}

func drawIsGrass(id uint16) bool { return id == grassA || id == grassB }

func drawAddHouse(t *testing.T, e *Editor, name string) uint32 {
	t.Helper()
	id, err := e.SaveHouse(otmap.House{Name: name, TownID: 1})
	if err != nil {
		t.Fatalf("SaveHouse(%q) = %v, want no error", name, err)
	}
	return id
}

func drawLastLabel(e *Editor) string {
	undo, _ := e.History()
	if len(undo) == 0 {
		return ""
	}
	return undo[len(undo)-1].Label
}

func TestDrawLabelsUndoStepAfterKind(t *testing.T) {
	cases := []struct {
		name  string
		req   DrawRequest
		label string
	}{
		{"ground", DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(10, 10))}, "Ground"},
		{"raw", DrawRequest{Kind: KindRaw, Item: stoneID, Points: points(at(10, 10))}, "Raw"},
		{"camel case kind", DrawRequest{Kind: KindHouseExit, House: 1, Points: points(at(10, 10))}, "HouseExit"},
		{"erase", DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(10, 10)), Erase: true}, "Erase ground"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			drawSetGround(e, sandID, at(10, 10))
			drawAddHouse(t, e, "Villa")
			if _, err := e.Draw(tc.req); err != nil {
				t.Fatalf("Draw = %v, want no error", err)
			}
			if got := drawLastLabel(e); got != tc.label {
				t.Fatalf("undo label = %q, want %q (kind with its first letter upper-cased, \"Erase <kind>\" when erasing)", got, tc.label)
			}
		})
	}
}

func TestDrawWithoutPointsOrRectDoesNothing(t *testing.T) {
	e := testEditor(t)
	changed, err := e.Draw(DrawRequest{Kind: KindGround, Name: "sand"})
	if changed != 0 || err != nil {
		t.Fatalf("Draw = %d, %v; want 0, nil for a request without points or rect", changed, err)
	}
	if e.CanUndo() {
		t.Fatal("an empty request added an undo step, want none")
	}
}

func TestDrawWithEmptyKindReturnsError(t *testing.T) {
	e := testEditor(t)
	_, err := e.Draw(DrawRequest{Points: points(at(10, 10))})
	if !errors.Is(err, ErrNoBrush) {
		t.Fatalf("Draw with empty kind = %v, want ErrNoBrush", err)
	}
}

func TestDrawUnknownKindReturnsErrNoBrush(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	changed, err := e.Draw(DrawRequest{Kind: "nope", Points: points(at(10, 10))})
	if !errors.Is(err, ErrNoBrush) || errText(err) != "unknown brush: nope" || changed != 0 {
		t.Fatalf("Draw = %d, %v; want 0, \"unknown brush: nope\"", changed, err)
	}
}

func TestDrawUnknownBrushReturnsErrNoBrush(t *testing.T) {
	cases := []struct {
		name    string
		req     DrawRequest
		message string
	}{
		{"ground", DrawRequest{Kind: KindGround, Name: "lava"}, `unknown brush: ground "lava"`},
		{"wall", DrawRequest{Kind: KindWall, Name: "brick"}, `unknown brush: wall "brick"`},
		{"carpet", DrawRequest{Kind: KindCarpet, Name: "desk"}, `unknown brush: carpet "desk"`},
		{"table", DrawRequest{Kind: KindTable, Name: "rug"}, `unknown brush: table "rug"`},
		{"doodad", DrawRequest{Kind: KindDoodad, Name: "bush"}, `unknown brush: doodad "bush"`},
		{"raw item zero", DrawRequest{Kind: KindRaw, Item: 0}, "unknown brush: item 0"},
		{"raw unknown item", DrawRequest{Kind: KindRaw, Item: 9999}, "unknown brush: item 9999"},
		{"house zero", DrawRequest{Kind: KindHouse, House: 0}, "unknown brush: house 0"},
		{"missing house", DrawRequest{Kind: KindHouse, House: 42}, "unknown brush: house 42"},
		{"house exit of missing house", DrawRequest{Kind: KindHouseExit, House: 42}, "unknown brush: house 42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			drawSetGround(e, sandID, at(10, 10))
			tc.req.Points = points(at(10, 10))
			changed, err := e.Draw(tc.req)
			if !errors.Is(err, ErrNoBrush) || errText(err) != tc.message {
				t.Fatalf("Draw = %v, want %q wrapping ErrNoBrush", err, tc.message)
			}
			if changed != 0 {
				t.Fatalf("Draw changed %d tiles, want 0 for an unknown brush", changed)
			}
		})
	}
}

func TestBrushAreaCoversShapeAndSize(t *testing.T) {
	cases := []struct {
		name  string
		size  int
		shape string
		count int
	}{
		{"size 0 square", 0, "square", 1},
		{"negative size is one tile", -3, "square", 1},
		{"size 1 square", 1, "square", 9},
		{"size 2 square", 2, "square", 25},
		{"empty shape is square", 1, "", 9},
		{"size 0 circle", 0, "circle", 1},
		{"size 1 circle", 1, "circle", 5},
		{"size 2 circle", 2, "circle", 13},
		{"size 3 circle", 3, "circle", 29},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			area := BrushArea(at(50, 50), tc.size, tc.shape)
			if len(area) != tc.count {
				t.Fatalf("BrushArea(size %d, %q) has %d tiles, want %d", tc.size, tc.shape, len(area), tc.count)
			}
			if !slices.Contains(area, at(50, 50)) {
				t.Fatal("BrushArea left out its center")
			}
		})
	}
}

func TestBrushAreaCircleOfSizeOneIsAPlus(t *testing.T) {
	got := BrushArea(at(50, 50), 1, "circle")
	want := []otmap.Position{at(50, 49), at(49, 50), at(50, 50), at(51, 50), at(50, 51)}
	if !slices.Equal(got, want) {
		t.Fatalf("circle of size 1 = %v, want center and its four direct neighbors %v (diagonals are outside)", got, want)
	}
}

func TestBrushAreaCircleKeepsTilesExactlyOnRadius(t *testing.T) {
	area := BrushArea(at(50, 50), 2, "circle")
	if !slices.Contains(area, at(52, 50)) {
		t.Fatal("circle of size 2 left out (52,50) at distance exactly 2, want it inside")
	}
	if slices.Contains(area, at(52, 51)) {
		t.Fatal("circle of size 2 holds (52,51) at distance 2.24, want it outside")
	}
}

func TestBrushAreaSkipsNegativeCoordinates(t *testing.T) {
	area := BrushArea(at(0, 0), 1, "square")
	want := []otmap.Position{at(0, 0), at(1, 0), at(0, 1), at(1, 1)}
	if !slices.Equal(area, want) {
		t.Fatalf("BrushArea at the map corner = %v, want only non-negative tiles %v", area, want)
	}
}

func TestDrawGroundPaintsBrushArea(t *testing.T) {
	cases := []struct {
		name  string
		size  int
		shape string
		count int
	}{
		{"single tile", 0, "square", 1},
		{"square size 1", 1, "square", 9},
		{"square size 2", 2, "square", 25},
		{"circle size 1", 1, "circle", 5},
		{"circle size 2", 2, "circle", 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			changed, err := e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(50, 50)), Size: tc.size, Shape: tc.shape})
			if err != nil || changed != tc.count {
				t.Fatalf("Draw = %d, %v; want %d tiles of sand (sand has no borders)", changed, err, tc.count)
			}
			for _, pos := range BrushArea(at(50, 50), tc.size, tc.shape) {
				if drawGroundAt(e, pos) != sandID {
					t.Fatalf("ground at %v = %d, want sand %d", pos, drawGroundAt(e, pos), sandID)
				}
			}
		})
	}
}

func TestDrawSeveralPointsPaintsEachArea(t *testing.T) {
	e := testEditor(t)
	changed, _ := e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(10, 10), at(11, 10), at(30, 30)), Size: 1})
	// Two overlapping 3x3 squares (12 tiles) and one apart (9 tiles).
	if changed != 21 {
		t.Fatalf("Draw changed %d tiles, want 21 (overlapping tiles count once)", changed)
	}
}

func TestDrawRectPaintsEveryTile(t *testing.T) {
	cases := []struct {
		name string
		rect *Rect
	}{
		{"corners in order", rect(10, 10, 12, 11)},
		{"corners reversed", rect(12, 11, 10, 10)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			changed, err := e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Rect: tc.rect})
			if err != nil || changed != 6 {
				t.Fatalf("Draw = %d, %v; want the 6 tiles of the 3x2 rectangle", changed, err)
			}
			for y := 10; y <= 11; y++ {
				for x := 10; x <= 12; x++ {
					if drawGroundAt(e, at(x, y)) != sandID {
						t.Fatalf("ground at (%d,%d) = %d, want sand", x, y, drawGroundAt(e, at(x, y)))
					}
				}
			}
		})
	}
}

func TestDrawWallRectPaintsOnlyOutline(t *testing.T) {
	e := testEditor(t)
	if _, err := e.Draw(DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(10, 10, 12, 12)}); err != nil {
		t.Fatal(err)
	}
	for y := 10; y <= 12; y++ {
		for x := 10; x <= 12; x++ {
			tile := e.Map.Tile(at(x, y))
			inner := x == 11 && y == 11
			if inner && tile != nil {
				t.Fatalf("inner tile (11,11) = %v, want no tile (a wall rect is an outline)", itemIDs(tile))
			}
			if !inner && (tile == nil || !e.hasWall(tile)) {
				t.Fatalf("outline tile (%d,%d) = %v, want a wall", x, y, itemIDs(tile))
			}
		}
	}
}

func TestDrawWallWithSizePaintsSquareOutline(t *testing.T) {
	e := testEditor(t)
	changed, err := e.Draw(DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(20, 20)), Size: 1})
	if err != nil || changed != 8 {
		t.Fatalf("Draw = %d, %v; want the 8 outline tiles of a 3x3 square", changed, err)
	}
	if tile := e.Map.Tile(at(20, 20)); tile != nil {
		t.Fatalf("center = %v, want no tile", itemIDs(tile))
	}
}

func TestDrawRectAboveMaxDrawTilesPaintsNothing(t *testing.T) {
	e := testEditor(t)
	// 501x501 tiles is above maxDrawTiles.
	changed, err := e.Draw(DrawRequest{Kind: KindRaw, Item: stoneID, Rect: rect(0, 0, 500, 500)})
	if changed != 0 || err != nil {
		t.Fatalf("Draw = %d, %v; want 0, nil for a rect above maxDrawTiles", changed, err)
	}
}

func TestDrawAreaStopsAddingPointsAboveMaxDrawTiles(t *testing.T) {
	e := testEditor(t)
	// Each 401x401 square has 160801 tiles: two exceed maxDrawTiles.
	area := e.area(DrawRequest{Kind: KindRaw, Points: points(at(200, 200), at(601, 200), at(200, 601)), Size: 200})
	if len(area.list) != 2*160801 {
		t.Fatalf("area has %d tiles, want %d (two squares, then it stops)", len(area.list), 2*160801)
	}
	if area.seen[at(200, 601)] {
		t.Fatal("area holds the third center, want points after maxDrawTiles left out")
	}
}

func TestFloodFillReplacesConnectedGround(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Rect: rect(10, 10, 12, 12)})
	if _, err := e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Fill: true, Points: points(at(11, 11))}); err != nil {
		t.Fatal(err)
	}
	for y := 10; y <= 12; y++ {
		for x := 10; x <= 12; x++ {
			if !drawIsGrass(drawGroundAt(e, at(x, y))) {
				t.Fatalf("ground at (%d,%d) = %d, want grass: it is connected sand", x, y, drawGroundAt(e, at(x, y)))
			}
		}
	}
	if drawGroundAt(e, at(13, 11)) != 0 {
		t.Fatalf("ground at (13,11) = %d, want none: empty tiles are not sand", drawGroundAt(e, at(13, 11)))
	}
}

func TestFloodFillStopsAtOtherGround(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10), at(11, 10), at(13, 10), at(14, 10))
	drawSetGround(e, grassA, at(12, 10))
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Fill: true, Points: points(at(10, 10))})
	if !drawIsGrass(drawGroundAt(e, at(11, 10))) {
		t.Fatalf("ground at (11,10) = %d, want grass", drawGroundAt(e, at(11, 10)))
	}
	for _, pos := range []otmap.Position{at(13, 10), at(14, 10)} {
		if drawGroundAt(e, pos) != sandID {
			t.Fatalf("ground at %v = %d, want sand: grass at (12,10) separates it from the start", pos, drawGroundAt(e, pos))
		}
	}
}

func TestFloodFillMovesOnlyOrthogonally(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10), at(11, 11))
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Fill: true, Points: points(at(10, 10))})
	if drawGroundAt(e, at(11, 11)) != sandID {
		t.Fatalf("diagonal sand = %d, want sand: fill does not cross corners", drawGroundAt(e, at(11, 11)))
	}
}

func TestFloodFillWithSameGroundDoesNothing(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10), at(11, 10))
	changed, err := e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Fill: true, Points: points(at(10, 10))})
	if changed != 0 || err != nil {
		t.Fatalf("Draw = %d, %v; want 0, nil when the area already has the brush", changed, err)
	}
}

func TestFloodFillOnGroundWithoutBrushDoesNothing(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, stoneID, at(10, 10))
	if filled := e.floodFill(at(10, 10), "sand"); len(filled.list) != 0 {
		t.Fatalf("fill covered %d tiles, want 0: a ground no brush knows has nothing to match", len(filled.list))
	}
}

func TestFloodFillWithUnknownBrushDoesNothing(t *testing.T) {
	e := testEditor(t)
	if filled := e.floodFill(at(10, 10), "lava"); len(filled.list) != 0 {
		t.Fatalf("fill covered %d tiles, want 0 for an unknown brush", len(filled.list))
	}
}

func TestFloodFillOfEmptyTilesStaysInsideRadius(t *testing.T) {
	e := testEditor(t)
	filled := e.floodFill(at(500, 500), "sand")
	side := 2*floodFillRadius + 1
	if len(filled.list) != side*side {
		t.Fatalf("fill covered %d tiles, want %d (a %dx%d window around the start)", len(filled.list), side*side, side, side)
	}
	if filled.seen[at(500+floodFillRadius+1, 500)] {
		t.Fatal("fill went past floodFillRadius")
	}
}

func TestAltGroundReplacesOnlyStartingGround(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10), at(11, 10))
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Alt: true, Stroke: 1, Points: points(at(10, 10)), Size: 1})
	for _, pos := range []otmap.Position{at(10, 10), at(11, 10)} {
		if !drawIsGrass(drawGroundAt(e, pos)) {
			t.Fatalf("ground at %v = %d, want grass over the starting sand", pos, drawGroundAt(e, pos))
		}
	}
	if drawGroundAt(e, at(9, 10)) != 0 {
		t.Fatalf("ground at (9,10) = %d, want none: Alt only replaces the ground it started on", drawGroundAt(e, at(9, 10)))
	}
}

func TestAltGroundStartingOnEmptyFillsOnlyEmptyTiles(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Alt: true, Stroke: 1, Points: points(at(11, 10)), Size: 1})
	if !drawIsGrass(drawGroundAt(e, at(11, 10))) {
		t.Fatalf("ground at (11,10) = %d, want grass on the empty start", drawGroundAt(e, at(11, 10)))
	}
	if drawGroundAt(e, at(10, 10)) != sandID {
		t.Fatalf("ground at (10,10) = %d, want sand kept: the stroke started on an empty tile", drawGroundAt(e, at(10, 10)))
	}
}

func TestAltGroundStrokeKeepsGroundOfItsFirstRequest(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Alt: true, Stroke: 7, Points: points(at(10, 10))})
	// The second request of the stroke starts on an empty tile but must
	// still replace only sand.
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Alt: true, Stroke: 7, Points: points(at(30, 30))})
	if drawGroundAt(e, at(30, 30)) != 0 {
		t.Fatalf("ground at (30,30) = %d, want none: the stroke replaces the sand it started on", drawGroundAt(e, at(30, 30)))
	}
}

func TestDrawGroundEraseRemovesOnlyThatBrush(t *testing.T) {
	e := testEditor(t)
	e.Settings.Automagic = false
	drawSetGround(e, sandID, at(10, 10))
	drawSetGround(e, grassA, at(11, 10))
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Erase: true, Points: points(at(10, 10)), Size: 1})
	if tile := e.Map.Tile(at(10, 10)); tile != nil {
		t.Fatalf("erased sand tile = %+v, want it removed", tile)
	}
	if drawGroundAt(e, at(11, 10)) != grassA {
		t.Fatalf("ground at (11,10) = %d, want grass kept: erasing sand leaves other grounds", drawGroundAt(e, at(11, 10)))
	}
}

func TestDrawRawAddsItemOnTop(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	e.Draw(DrawRequest{Kind: KindRaw, Item: stoneID, Points: points(at(10, 10))})
	e.Draw(DrawRequest{Kind: KindRaw, Item: bagID, Points: points(at(10, 10))})
	if got := itemIDs(e.Map.Tile(at(10, 10))); !slices.Equal(got, []uint16{stoneID, bagID}) {
		t.Fatalf("items = %v, want [stone bag]", got)
	}
}

func TestDrawRawGroundItemReplacesGround(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	e.Draw(DrawRequest{Kind: KindRaw, Item: grassA, Points: points(at(10, 10))})
	if drawGroundAt(e, at(10, 10)) != grassA {
		t.Fatalf("ground = %d, want raw ground %d", drawGroundAt(e, at(10, 10)), grassA)
	}
}

func TestDrawRawStackableGetsCountOne(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindRaw, Item: coinID, Points: points(at(10, 10))})
	tile := e.Map.Tile(at(10, 10))
	if tile == nil || len(tile.Items) != 1 || tile.Items[0].Subtype != 1 {
		t.Fatalf("tile = %+v, want one coin with count 1", tile)
	}
}

func TestDrawRawBottomItemReplacesBottomItems(t *testing.T) {
	cases := []struct {
		name          string
		replaceBottom bool
		want          []uint16
	}{
		{"RawReplaceBottom on", true, []uint16{wallH}},
		{"RawReplaceBottom off", false, []uint16{wallV, wallH}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Settings.RawReplaceBottom = tc.replaceBottom
			e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: wallV}}})
			e.Draw(DrawRequest{Kind: KindRaw, Item: wallH, Points: points(at(10, 10))})
			if got := itemIDs(e.Map.Tile(at(10, 10))); !slices.Equal(got, tc.want) {
				t.Fatalf("items = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDrawRawReplaceBottomKeepsCommonItems(t *testing.T) {
	e := testEditor(t)
	e.Settings.RawReplaceBottom = true
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: stoneID}}})
	e.Draw(DrawRequest{Kind: KindRaw, Item: stoneID, Points: points(at(10, 10))})
	if got := itemIDs(e.Map.Tile(at(10, 10))); !slices.Equal(got, []uint16{stoneID, stoneID}) {
		t.Fatalf("items = %v, want two stones: only bottom items are replaced", got)
	}
}

func TestDrawRawEraseRemovesItemAndGround(t *testing.T) {
	cases := []struct {
		name       string
		item       uint16
		wantGround uint16
		wantItems  []uint16
	}{
		{"item", stoneID, sandID, []uint16{bagID}},
		{"ground", sandID, 0, []uint16{stoneID, bagID, stoneID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: stoneID}, {ID: bagID}, {ID: stoneID}}})
			e.Draw(DrawRequest{Kind: KindRaw, Item: tc.item, Erase: true, Points: points(at(10, 10))})
			tile := e.Map.Tile(at(10, 10))
			if tile.Ground.ID != tc.wantGround || !slices.Equal(itemIDs(tile), tc.wantItems) {
				t.Fatalf("tile = ground %d items %v, want ground %d items %v", tile.Ground.ID, itemIDs(tile), tc.wantGround, tc.wantItems)
			}
		})
	}
}

func TestDrawRawEraseSkipsTilesWithoutItem(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	changed, err := e.Draw(DrawRequest{Kind: KindRaw, Item: stoneID, Erase: true, Points: points(at(10, 10), at(11, 10))})
	if changed != 0 || err != nil {
		t.Fatalf("Draw = %d, %v; want 0, nil when no tile holds the item", changed, err)
	}
}

// drawEraserTile is a tile with a ground, a border, a plain stone and a
// stone with an action id.
func drawEraserTile(groundActionID uint16) *otmap.Tile {
	ground := otmap.Item{ID: sandID}
	if groundActionID != 0 {
		ground.Ensure().ActionID = groundActionID
	}
	unique := otmap.Item{ID: stoneID}
	unique.Ensure().ActionID = 1000
	return &otmap.Tile{Ground: ground, Items: []otmap.Item{{ID: borderN}, {ID: stoneID}, unique}}
}

func TestEraserRemovesItems(t *testing.T) {
	cases := []struct {
		name        string
		leaveUnique bool
		erase       bool
		wantGround  uint16
		wantItems   []uint16
	}{
		{"leave unique keeps borders and items with attributes", true, false, sandID, []uint16{borderN, stoneID}},
		{"without leave unique removes all items", false, false, sandID, nil},
		{"erase with leave unique removes ground and borders", true, true, 0, []uint16{stoneID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Settings.Automagic = false
			e.Settings.EraserLeaveUnique = tc.leaveUnique
			e.Map.SetTile(at(10, 10), drawEraserTile(0))
			e.Draw(DrawRequest{Kind: KindEraser, Erase: tc.erase, Points: points(at(10, 10))})
			tile := e.Map.Tile(at(10, 10))
			if tile == nil || tile.Ground.ID != tc.wantGround || !slices.Equal(itemIDs(tile), tc.wantItems) {
				t.Fatalf("tile = %+v (items %v), want ground %d items %v", tile, itemIDs(tile), tc.wantGround, tc.wantItems)
			}
			if len(tc.wantItems) == 1 && !tile.Items[0].Complex() {
				t.Fatal("the kept stone is the plain one, want the one with an action id")
			}
		})
	}
}

func TestEraserEraseWithoutLeaveUniqueRemovesTile(t *testing.T) {
	e := testEditor(t)
	e.Settings.Automagic = false
	e.Settings.EraserLeaveUnique = false
	e.Map.SetTile(at(10, 10), drawEraserTile(0))
	e.Draw(DrawRequest{Kind: KindEraser, Erase: true, Points: points(at(10, 10))})
	if tile := e.Map.Tile(at(10, 10)); tile != nil {
		t.Fatalf("tile = %+v, want it gone: everything was erased", tile)
	}
}

func TestEraserEraseKeepsGroundWithAttributes(t *testing.T) {
	e := testEditor(t)
	e.Settings.Automagic = false
	e.Map.SetTile(at(10, 10), drawEraserTile(2000))
	e.Draw(DrawRequest{Kind: KindEraser, Erase: true, Points: points(at(10, 10))})
	if drawGroundAt(e, at(10, 10)) != sandID {
		t.Fatalf("ground = %d, want sand kept: it has an action id and EraserLeaveUnique is on", drawGroundAt(e, at(10, 10)))
	}
}

func TestEraserSkipsEmptyTiles(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Flags: otmap.FlagPZ})
	changed, _ := e.Draw(DrawRequest{Kind: KindEraser, Points: points(at(10, 10), at(11, 10))})
	if changed != 0 {
		t.Fatalf("Draw changed %d tiles, want 0 for tiles without ground or items", changed)
	}
}

func TestEraserReborders(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(11, 10))
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(10, 10))})
	if got := itemIDs(e.Map.Tile(at(11, 10))); !slices.Equal(got, []uint16{borderW}) {
		t.Fatalf("sand east of grass = %v, want [%d] (test setup)", got, borderW)
	}
	e.Settings.EraserLeaveUnique = false
	e.Draw(DrawRequest{Kind: KindEraser, Erase: true, Points: points(at(10, 10))})
	if got := itemIDs(e.Map.Tile(at(11, 10))); len(got) != 0 {
		t.Fatalf("sand east of erased grass = %v, want its grass border removed", got)
	}
}

func TestDrawFlagSetsFlagOnGroundTiles(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	changed, _ := e.Draw(DrawRequest{Kind: KindFlag, Flag: otmap.FlagNoLogout, Points: points(at(10, 10), at(11, 10))})
	if changed != 1 || e.Map.Tile(at(10, 10)).Flags != otmap.FlagNoLogout {
		t.Fatalf("Draw changed %d, flags %d; want 1 tile with the no-logout flag", changed, e.Map.Tile(at(10, 10)).Flags)
	}
	if e.Map.Tile(at(11, 10)) != nil {
		t.Fatal("flag created a tile without ground, want it skipped")
	}
}

func TestDrawFlagSkipsTilesAlreadyFlagged(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Flags: otmap.FlagPZ})
	changed, _ := e.Draw(DrawRequest{Kind: KindFlag, Flag: otmap.FlagPZ, Points: points(at(10, 10))})
	if changed != 0 {
		t.Fatalf("Draw changed %d tiles, want 0: the flag is already set", changed)
	}
}

func TestDrawFlagEraseClearsOnlyThatFlag(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Flags: otmap.FlagPZ | otmap.FlagNoPVP})
	drawSetGround(e, sandID, at(11, 10))
	changed, _ := e.Draw(DrawRequest{Kind: KindFlag, Flag: otmap.FlagPZ, Erase: true, Points: points(at(10, 10), at(11, 10))})
	if changed != 1 || e.Map.Tile(at(10, 10)).Flags != otmap.FlagNoPVP {
		t.Fatalf("Draw changed %d, flags %d; want 1 tile left with only no-pvp", changed, e.Map.Tile(at(10, 10)).Flags)
	}
}

func TestDrawZoneAddsZoneOnce(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	e.Draw(DrawRequest{Kind: KindZone, Zone: 3, Points: points(at(10, 10), at(11, 10))})
	changed, _ := e.Draw(DrawRequest{Kind: KindZone, Zone: 3, Points: points(at(10, 10))})
	if got := e.Map.Tile(at(10, 10)).Zones(); !slices.Equal(got, []uint16{3}) || changed != 0 {
		t.Fatalf("zones = %v, second draw changed %d; want [3] and 0", got, changed)
	}
	if e.Map.Tile(at(11, 10)) != nil {
		t.Fatal("zone created a tile without ground, want it skipped")
	}
}

func TestDrawZoneEraseRemovesZone(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Zones: []uint16{3, 4}}})
	e.Map.SetTile(at(11, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Zones: []uint16{3}}})
	e.Draw(DrawRequest{Kind: KindZone, Zone: 3, Erase: true, Points: points(at(10, 10), at(11, 10))})
	if got := e.Map.Tile(at(10, 10)).Zones(); !slices.Equal(got, []uint16{4}) {
		t.Fatalf("zones = %v, want [4]", got)
	}
	if ext := e.Map.Tile(at(11, 10)).Ext; ext != nil {
		t.Fatalf("ext = %+v, want nil after its last zone was erased", ext)
	}
}

func TestDrawZoneEraseSkipsTilesWithoutZone(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	changed, _ := e.Draw(DrawRequest{Kind: KindZone, Zone: 3, Erase: true, Points: points(at(10, 10))})
	if changed != 0 {
		t.Fatalf("Draw changed %d tiles, want 0", changed)
	}
}

func TestDrawOptionalBorderMarksTile(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	e.Draw(DrawRequest{Kind: KindOptionalBorder, Points: points(at(10, 10))})
	if e.Map.Tile(at(10, 10)).State&otmap.StateOptionalBorder == 0 {
		t.Fatal("tile not marked for an optional border")
	}
}

func TestDrawOptionalBorderEraseUnmarksTile(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, State: otmap.StateOptionalBorder})
	drawSetGround(e, sandID, at(11, 10))
	changed, _ := e.Draw(DrawRequest{Kind: KindOptionalBorder, Erase: true, Points: points(at(10, 10), at(11, 10))})
	if e.Map.Tile(at(10, 10)).State&otmap.StateOptionalBorder != 0 {
		t.Fatal("tile still marked for an optional border after erase")
	}
	if changed != 1 {
		t.Fatalf("Draw changed %d tiles, want 1: the unmarked tile is skipped", changed)
	}
}

func TestDrawHouseAssignsTilesToHouse(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(10, 10)), Size: 1})
	id := drawAddHouse(t, e, "Villa")
	if id != 1 {
		t.Fatalf("SaveHouse id = %d, want 1 (first free id)", id)
	}
	e.Draw(DrawRequest{Kind: KindHouse, House: id, Points: points(at(10, 10)), Size: 1})
	tile := e.Map.Tile(at(10, 10))
	if tile.HouseID != 1 || tile.Flags&otmap.FlagPZ == 0 {
		t.Fatalf("house tile = %+v, want house 1 and a protection zone", tile)
	}
	if houses := e.Houses(); len(houses) != 1 || houses[0].Size != 9 {
		t.Fatalf("houses = %+v, want one house of 9 tiles", houses)
	}
}

func TestDrawHouseSkipsTilesWithoutGround(t *testing.T) {
	e := testEditor(t)
	id := drawAddHouse(t, e, "Villa")
	e.Map.SetTile(at(10, 10), &otmap.Tile{Items: []otmap.Item{{ID: stoneID}}})
	changed, err := e.Draw(DrawRequest{Kind: KindHouse, House: id, Points: points(at(10, 10), at(11, 10))})
	if changed != 0 || err != nil {
		t.Fatalf("Draw = %d, %v; want 0, nil: houses need ground", changed, err)
	}
}

func TestDrawHouseSkipsTilesAlreadyInHouse(t *testing.T) {
	e := testEditor(t)
	id := drawAddHouse(t, e, "Villa")
	drawSetGround(e, sandID, at(10, 10))
	e.Draw(DrawRequest{Kind: KindHouse, House: id, Points: points(at(10, 10))})
	changed, _ := e.Draw(DrawRequest{Kind: KindHouse, House: id, Points: points(at(10, 10))})
	if changed != 0 {
		t.Fatalf("Draw changed %d tiles, want 0 for a tile already in the house", changed)
	}
}

func TestDrawHouseRemovesLooseItems(t *testing.T) {
	unique := otmap.Item{ID: stoneID}
	unique.Ensure().ActionID = 1000
	cases := []struct {
		name         string
		removeItems  bool
		wantItemsIDs []uint16
	}{
		{"HouseRemoveItems on keeps unmoveable and items with attributes", true, []uint16{treeID, stoneID}},
		{"HouseRemoveItems off keeps everything", false, []uint16{stoneID, treeID, bagID, stoneID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Settings.HouseRemoveItems = tc.removeItems
			id := drawAddHouse(t, e, "Villa")
			e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: stoneID}, {ID: treeID}, {ID: bagID}, unique.Clone()}})
			e.Draw(DrawRequest{Kind: KindHouse, House: id, Points: points(at(10, 10))})
			if got := itemIDs(e.Map.Tile(at(10, 10))); !slices.Equal(got, tc.wantItemsIDs) {
				t.Fatalf("items = %v, want %v", got, tc.wantItemsIDs)
			}
		})
	}
}

func drawDoorIDs(e *Editor, positions ...otmap.Position) []uint8 {
	var ids []uint8
	for _, pos := range positions {
		door := e.Map.Tile(pos).Items[0]
		var doorID uint8
		if door.Ext != nil {
			doorID = door.Ext.DoorID
		}
		ids = append(ids, doorID)
	}
	return ids
}

func TestDrawHouseGivesDoorsFreeDoorIDs(t *testing.T) {
	cases := []struct {
		name       string
		autoDoorID bool
		want       []uint8
	}{
		{"AutoDoorID on", true, []uint8{1, 2}},
		{"AutoDoorID off", false, []uint8{0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Settings.AutoDoorID = tc.autoDoorID
			id := drawAddHouse(t, e, "Villa")
			for _, pos := range []otmap.Position{at(10, 10), at(11, 10)} {
				e.Map.SetTile(pos, &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: doorHClosed}}})
			}
			e.Draw(DrawRequest{Kind: KindHouse, House: id, Points: points(at(10, 10))})
			e.Draw(DrawRequest{Kind: KindHouse, House: id, Points: points(at(11, 10))})
			if got := drawDoorIDs(e, at(10, 10), at(11, 10)); !slices.Equal(got, tc.want) {
				t.Fatalf("door ids = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDrawHouseKeepsExistingDoorID(t *testing.T) {
	e := testEditor(t)
	id := drawAddHouse(t, e, "Villa")
	door := otmap.Item{ID: doorHClosed}
	door.Ensure().DoorID = 5
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{door}})
	e.Draw(DrawRequest{Kind: KindHouse, House: id, Points: points(at(10, 10))})
	if got := drawDoorIDs(e, at(10, 10)); got[0] != 5 {
		t.Fatalf("door id = %d, want 5 kept", got[0])
	}
}

func TestDrawHouseDoorIDsAreCountedPerHouse(t *testing.T) {
	e := testEditor(t)
	first := drawAddHouse(t, e, "Villa")
	second := drawAddHouse(t, e, "Hut")
	for _, pos := range []otmap.Position{at(10, 10), at(20, 10)} {
		e.Map.SetTile(pos, &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: doorHClosed}}})
	}
	e.Draw(DrawRequest{Kind: KindHouse, House: first, Points: points(at(10, 10))})
	e.Draw(DrawRequest{Kind: KindHouse, House: second, Points: points(at(20, 10))})
	if got := drawDoorIDs(e, at(10, 10), at(20, 10)); !slices.Equal(got, []uint8{1, 1}) {
		t.Fatalf("door ids = %v, want [1 1]: each house numbers its own doors", got)
	}
}

func TestDrawHouseEraseClearsHouseAndDoorIDs(t *testing.T) {
	e := testEditor(t)
	id := drawAddHouse(t, e, "Villa")
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: doorHClosed}}})
	e.Draw(DrawRequest{Kind: KindHouse, House: id, Points: points(at(10, 10))})
	e.Draw(DrawRequest{Kind: KindHouse, House: id, Erase: true, Points: points(at(10, 10))})
	tile := e.Map.Tile(at(10, 10))
	if tile.HouseID != 0 || tile.Flags&otmap.FlagPZ != 0 || tile.Items[0].Ext != nil {
		t.Fatalf("tile = %+v, door ext %+v; want no house, no protection zone, no door id", tile, tile.Items[0].Ext)
	}
}

func TestDrawHouseEraseKeepsOtherHouses(t *testing.T) {
	e := testEditor(t)
	first := drawAddHouse(t, e, "Villa")
	second := drawAddHouse(t, e, "Hut")
	drawSetGround(e, sandID, at(10, 10), at(11, 10))
	e.Draw(DrawRequest{Kind: KindHouse, House: second, Points: points(at(10, 10))})
	changed, _ := e.Draw(DrawRequest{Kind: KindHouse, House: first, Erase: true, Points: points(at(10, 10), at(11, 10))})
	if changed != 0 || e.Map.Tile(at(10, 10)).HouseID != second {
		t.Fatalf("Draw changed %d, house %d; want 0 and house %d kept", changed, e.Map.Tile(at(10, 10)).HouseID, second)
	}
}

func TestDrawHouseEraseWorksForDeletedHouse(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 9, Flags: otmap.FlagPZ})
	changed, err := e.Draw(DrawRequest{Kind: KindHouse, House: 9, Erase: true, Points: points(at(10, 10))})
	if changed != 1 || err != nil || e.Map.Tile(at(10, 10)).HouseID != 0 {
		t.Fatalf("Draw = %d, %v; want the leftover tile of a deleted house cleared", changed, err)
	}
}

func TestRemoveHouseClearsPaintedTiles(t *testing.T) {
	e := testEditor(t)
	id := drawAddHouse(t, e, "Villa")
	drawSetGround(e, sandID, at(10, 10))
	e.Draw(DrawRequest{Kind: KindHouse, House: id, Points: points(at(10, 10))})
	e.RemoveHouse(id)
	if tile := e.Map.Tile(at(10, 10)); tile.HouseID != 0 || tile.Flags&otmap.FlagPZ != 0 {
		t.Fatalf("tile = %+v, want house and protection zone cleared", tile)
	}
}

func TestDrawHouseExitSetsExit(t *testing.T) {
	e := testEditor(t)
	id := drawAddHouse(t, e, "Villa")
	drawSetGround(e, sandID, at(12, 12), at(13, 12))
	// The last point of a stroke wins.
	if _, err := e.Draw(DrawRequest{Kind: KindHouseExit, House: id, Points: points(at(13, 12), at(12, 12))}); err != nil {
		t.Fatal(err)
	}
	if e.Map.Houses[id].Exit != at(12, 12) {
		t.Fatalf("exit = %v, want (12,12)", e.Map.Houses[id].Exit)
	}
}

func TestDrawHouseExitRejectsInvalidTiles(t *testing.T) {
	cases := []struct {
		name string
		tile *otmap.Tile
	}{
		{"no tile", nil},
		{"no ground", &otmap.Tile{Items: []otmap.Item{{ID: stoneID}}}},
		{"inside a house", &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 1}},
		{"blocking", &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: treeID}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			id := drawAddHouse(t, e, "Villa")
			if tc.tile != nil {
				e.Map.SetTile(at(12, 12), tc.tile)
			}
			_, err := e.Draw(DrawRequest{Kind: KindHouseExit, House: id, Points: points(at(12, 12))})
			if errText(err) != "a house exit must be on a walkable tile outside the house" {
				t.Fatalf("Draw = %v, want the house exit error", err)
			}
			if e.Map.Houses[id].Exit != (otmap.Position{}) {
				t.Fatalf("exit = %v, want unset", e.Map.Houses[id].Exit)
			}
		})
	}
}

func TestUndoRestoresHouseExit(t *testing.T) {
	e := testEditor(t)
	id := drawAddHouse(t, e, "Villa")
	drawSetGround(e, sandID, at(12, 12))
	e.Draw(DrawRequest{Kind: KindHouseExit, House: id, Points: points(at(12, 12))})
	e.Undo()
	if e.Map.Houses[id].Exit != (otmap.Position{}) {
		t.Fatalf("exit after undo = %v, want unset", e.Map.Houses[id].Exit)
	}
}

func TestDrawHouseExitEraseClearsExitAtPoint(t *testing.T) {
	cases := []struct {
		name  string
		point otmap.Position
		want  otmap.Position
	}{
		{"at the exit", at(12, 12), otmap.Position{}},
		{"elsewhere", at(20, 20), at(12, 12)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			id := drawAddHouse(t, e, "Villa")
			e.Map.Houses[id].Exit = at(12, 12)
			if _, err := e.Draw(DrawRequest{Kind: KindHouseExit, House: id, Erase: true, Points: points(tc.point)}); err != nil {
				t.Fatal(err)
			}
			if e.Map.Houses[id].Exit != tc.want {
				t.Fatalf("exit = %v, want %v", e.Map.Houses[id].Exit, tc.want)
			}
		})
	}
}

func TestDrawWaypointAddsNewWaypoint(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindWaypoint, Waypoint: "start", Points: points(at(10, 10))})
	if len(e.Map.Waypoints) != 1 || *e.Map.Waypoints[0] != (otmap.Waypoint{Name: "start", Pos: at(10, 10)}) {
		t.Fatalf("waypoints = %v, want start at (10,10)", e.Map.Waypoints)
	}
}

func TestDrawWaypointMovesExistingWaypoint(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindWaypoint, Waypoint: "start", Points: points(at(10, 10))})
	e.Draw(DrawRequest{Kind: KindWaypoint, Waypoint: "start", Points: points(at(20, 20))})
	if len(e.Map.Waypoints) != 1 || e.Map.Waypoints[0].Pos != at(20, 20) {
		t.Fatalf("waypoints = %v, want one start moved to (20,20)", e.Map.Waypoints)
	}
}

func TestDrawWaypointWithoutNameFails(t *testing.T) {
	e := testEditor(t)
	_, err := e.Draw(DrawRequest{Kind: KindWaypoint, Points: points(at(10, 10))})
	if errText(err) != "choose a waypoint" || len(e.Map.Waypoints) != 0 {
		t.Fatalf("Draw = %v, waypoints %v; want \"choose a waypoint\" and none added", err, e.Map.Waypoints)
	}
}

func TestDrawWaypointEraseRemovesWaypointAtPoint(t *testing.T) {
	e := testEditor(t)
	e.Map.Waypoints = []*otmap.Waypoint{{Name: "a", Pos: at(10, 10)}, {Name: "b", Pos: at(20, 20)}}
	e.Draw(DrawRequest{Kind: KindWaypoint, Erase: true, Points: points(at(10, 10))})
	if len(e.Map.Waypoints) != 1 || e.Map.Waypoints[0].Name != "b" {
		t.Fatalf("waypoints = %v, want only b left", e.Map.Waypoints)
	}
}

func TestDrawWaypointEraseElsewhereAddsNoUndoStep(t *testing.T) {
	e := testEditor(t)
	e.Map.Waypoints = []*otmap.Waypoint{{Name: "a", Pos: at(10, 10)}}
	e.Draw(DrawRequest{Kind: KindWaypoint, Erase: true, Points: points(at(11, 10))})
	if len(e.Map.Waypoints) != 1 || e.CanUndo() {
		t.Fatalf("waypoints = %v, can undo %v; want a kept and no undo step", e.Map.Waypoints, e.CanUndo())
	}
}

func TestDrawSpawnUsesSizeAsRadius(t *testing.T) {
	cases := []struct {
		name   string
		size   int
		radius int32
	}{
		{"size 0 is radius 1", 0, 1},
		{"size 3", 3, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Draw(DrawRequest{Kind: KindSpawn, Size: tc.size, Points: points(at(10, 10))})
			spawn := e.Map.Tile(at(10, 10)).Spawn()
			if spawn == nil || spawn.Radius != tc.radius {
				t.Fatalf("spawn = %+v, want radius %d", spawn, tc.radius)
			}
		})
	}
}

func TestDrawSpawnNPCSetsNPCSpawn(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindSpawnNPC, Size: 2, Points: points(at(10, 10))})
	tile := e.Map.Tile(at(10, 10))
	if tile.SpawnNPC() == nil || tile.SpawnNPC().Radius != 2 || tile.Spawn() != nil {
		t.Fatalf("tile ext = %+v, want only an NPC spawn of radius 2", tile.Ext)
	}
}

func TestDrawSpawnEraseRemovesOnlyItsKind(t *testing.T) {
	cases := []struct {
		name      string
		kind      string
		wantSpawn bool
		wantNPC   bool
	}{
		{"monster spawn", KindSpawn, false, true},
		{"NPC spawn", KindSpawnNPC, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Spawn: &otmap.Spawn{Radius: 1}, SpawnNPC: &otmap.Spawn{Radius: 1}}})
			e.Draw(DrawRequest{Kind: tc.kind, Erase: true, Points: points(at(10, 10))})
			tile := e.Map.Tile(at(10, 10))
			if (tile.Spawn() != nil) != tc.wantSpawn || (tile.SpawnNPC() != nil) != tc.wantNPC {
				t.Fatalf("tile ext = %+v, want monster spawn %v, NPC spawn %v", tile.Ext, tc.wantSpawn, tc.wantNPC)
			}
		})
	}
}

func TestDrawSpawnEraseRemovesEmptyTile(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindSpawn, Points: points(at(10, 10))})
	changed, _ := e.Draw(DrawRequest{Kind: KindSpawn, Erase: true, Points: points(at(10, 10), at(11, 10))})
	if changed != 1 || e.Map.Tile(at(10, 10)) != nil {
		t.Fatalf("Draw changed %d, tile %+v; want 1 change and the empty tile removed", changed, e.Map.Tile(at(10, 10)))
	}
}

func TestDrawCreaturePlacesCreatureWithDefaults(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(12, 12))
	e.Draw(DrawRequest{Kind: KindCreature, Creature: "Rat", Direction: 2, Points: points(at(12, 12))})
	creatures := e.Map.Tile(at(12, 12)).Creatures()
	want := otmap.Creature{Name: "Rat", SpawnTime: 60, Direction: 2}
	if len(creatures) != 1 || creatures[0] != want {
		t.Fatalf("creatures = %+v, want %+v (default spawn time 60)", creatures, want)
	}
}

func TestDrawCreatureKeepsGivenSpawnTime(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(12, 12))
	e.Draw(DrawRequest{Kind: KindCreature, Creature: "Rat", SpawnTime: 90, Points: points(at(12, 12))})
	if got := e.Map.Tile(at(12, 12)).Creatures()[0].SpawnTime; got != 90 {
		t.Fatalf("spawn time = %d, want 90", got)
	}
}

func TestDrawCreatureReplacesCreatureOnTile(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(12, 12))
	e.Draw(DrawRequest{Kind: KindCreature, Creature: "Rat", Points: points(at(12, 12))})
	e.Draw(DrawRequest{Kind: KindCreature, Creature: "Wolf", Points: points(at(12, 12))})
	creatures := e.Map.Tile(at(12, 12)).Creatures()
	if len(creatures) != 1 || creatures[0].Name != "Wolf" {
		t.Fatalf("creatures = %+v, want only the wolf", creatures)
	}
}

func TestDrawCreatureWithoutNameFails(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(12, 12))
	_, err := e.Draw(DrawRequest{Kind: KindCreature, Points: points(at(12, 12))})
	if errText(err) != "choose a creature" {
		t.Fatalf("Draw = %v, want \"choose a creature\"", err)
	}
}

func TestDrawCreatureSkipsUnwalkableTiles(t *testing.T) {
	cases := []struct {
		name string
		tile *otmap.Tile
		alt  bool
		want int
	}{
		{"no tile", nil, false, 0},
		{"no ground", &otmap.Tile{Items: []otmap.Item{{ID: stoneID}}}, false, 0},
		{"blocking", &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: treeID}}}, false, 0},
		{"blocking with Alt", &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: treeID}}}, true, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			if tc.tile != nil {
				e.Map.SetTile(at(12, 12), tc.tile)
			}
			e.Draw(DrawRequest{Kind: KindCreature, Creature: "Rat", Alt: tc.alt, Points: points(at(12, 12))})
			var got int
			if tile := e.Map.Tile(at(12, 12)); tile != nil {
				got = len(tile.Creatures())
			}
			if got != tc.want {
				t.Fatalf("creatures on tile = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDrawCreatureCreatesSpawnOutsideSpawns(t *testing.T) {
	cases := []struct {
		name            string
		autoCreateSpawn bool
		wantSpawn       bool
	}{
		{"AutoCreateSpawn on", true, true},
		{"AutoCreateSpawn off", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Settings.AutoCreateSpawn = tc.autoCreateSpawn
			drawSetGround(e, sandID, at(12, 12))
			e.Draw(DrawRequest{Kind: KindCreature, Creature: "Rat", Points: points(at(12, 12))})
			spawn := e.Map.Tile(at(12, 12)).Spawn()
			if (spawn != nil) != tc.wantSpawn || (spawn != nil && spawn.Radius != 1) {
				t.Fatalf("spawn = %+v, want spawn of radius 1: %v", spawn, tc.wantSpawn)
			}
		})
	}
}

func TestDrawCreatureInsideSpawnCreatesNoSpawn(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(11, 12), at(12, 12))
	e.Draw(DrawRequest{Kind: KindCreature, Creature: "Rat", Points: points(at(12, 12))})
	e.Draw(DrawRequest{Kind: KindCreature, Creature: "Rat", Points: points(at(11, 12))})
	if e.Map.Tile(at(11, 12)).Spawn() != nil {
		t.Fatal("second creature got its own spawn inside the first spawn, want none")
	}
}

func TestRemoveEmptySpawnsKeepsSpawnsWithCreatures(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(11, 12), at(12, 12))
	e.Draw(DrawRequest{Kind: KindCreature, Creature: "Rat", Points: points(at(12, 12))})
	e.Draw(DrawRequest{Kind: KindCreature, Creature: "Rat", Points: points(at(11, 12))})
	if removed := e.RemoveEmptySpawns(); removed != 0 {
		t.Fatalf("RemoveEmptySpawns = %d, want 0: the spawn holds creatures", removed)
	}
}

func TestDrawNPCCreatesSpawnOfMapFormat(t *testing.T) {
	cases := []struct {
		name     string
		format   otmap.SpawnFormat
		wantNPC  bool
		wantMain bool
	}{
		{"separate files use NPC spawn", otmap.SpawnsSeparate, true, false},
		{"single file uses monster spawn", otmap.SpawnsSingle, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Map.SpawnFormat = tc.format
			drawSetGround(e, sandID, at(12, 12))
			e.Draw(DrawRequest{Kind: KindCreature, Creature: "Banker", NPC: true, Points: points(at(12, 12))})
			tile := e.Map.Tile(at(12, 12))
			if !tile.Creatures()[0].NPC {
				t.Fatal("creature not marked as NPC")
			}
			if (tile.SpawnNPC() != nil) != tc.wantNPC || (tile.Spawn() != nil) != tc.wantMain {
				t.Fatalf("tile ext = %+v, want NPC spawn %v, monster spawn %v", tile.Ext, tc.wantNPC, tc.wantMain)
			}
		})
	}
}

func TestDrawCreatureEraseRemovesCreatures(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(12, 12), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Rat"}}}})
	changed, _ := e.Draw(DrawRequest{Kind: KindCreature, Erase: true, Points: points(at(12, 12), at(13, 12))})
	tile := e.Map.Tile(at(12, 12))
	if changed != 1 || tile.Ext != nil {
		t.Fatalf("Draw changed %d, ext %+v; want 1 and no ext left", changed, tile.Ext)
	}
}

func TestCoveredFindsSpawnAround(t *testing.T) {
	cases := []struct {
		name   string
		format otmap.SpawnFormat
		ext    otmap.TileExt
		target otmap.Position
		npc    bool
		want   bool
	}{
		{"inside radius", otmap.SpawnsSingle, otmap.TileExt{Spawn: &otmap.Spawn{Radius: 3}}, at(13, 13), false, true},
		{"outside radius", otmap.SpawnsSingle, otmap.TileExt{Spawn: &otmap.Spawn{Radius: 3}}, at(14, 10), false, false},
		{"other floor", otmap.SpawnsSingle, otmap.TileExt{Spawn: &otmap.Spawn{Radius: 3}}, posAt(10, 10, 6), false, false},
		{"single file counts NPC spawn for monsters", otmap.SpawnsSingle, otmap.TileExt{SpawnNPC: &otmap.Spawn{Radius: 3}}, at(11, 10), false, true},
		{"separate files: NPC ignores monster spawn", otmap.SpawnsSeparate, otmap.TileExt{Spawn: &otmap.Spawn{Radius: 3}}, at(11, 10), true, false},
		{"separate files: NPC uses NPC spawn", otmap.SpawnsSeparate, otmap.TileExt{SpawnNPC: &otmap.Spawn{Radius: 3}}, at(11, 10), true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Map.SpawnFormat = tc.format
			ext := tc.ext
			e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &ext})
			if got := e.covered(tc.target, tc.npc); got != tc.want {
				t.Fatalf("covered(%v, npc %v) = %v, want %v", tc.target, tc.npc, got, tc.want)
			}
		})
	}
}

func TestDrawWallEraseRemovesWall(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindWall, Name: "stone wall", Points: points(at(10, 10))})
	e.Draw(DrawRequest{Kind: KindWall, Name: "stone wall", Erase: true, Points: points(at(10, 10), at(11, 10))})
	if tile := e.Map.Tile(at(10, 10)); tile != nil {
		t.Fatalf("tile = %v, want the wall erased", itemIDs(tile))
	}
}

func TestDrawWallAltClickChangesWallShape(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(10, 10, 12, 10)})
	changed, _ := e.Draw(DrawRequest{Kind: KindWall, Name: "stone wall", Alt: true, Points: points(at(11, 10), at(20, 20))})
	tile := e.Map.Tile(at(11, 10))
	if changed != 1 || len(tile.Items) != 1 || tile.Items[0].ID == wallH || !e.hasWall(tile) {
		t.Fatalf("Draw changed %d, items %v; want the horizontal wall turned into another wall shape and the empty tile skipped", changed, itemIDs(tile))
	}
}

func TestDrawDoorDispatchesDoorTypeAndAlt(t *testing.T) {
	cases := []struct {
		name     string
		doorType string
		alt      bool
		want     uint16
	}{
		{"normal door", "normal", false, doorHClosed},
		{"unknown door type is normal", "", false, doorHClosed},
		{"Alt places it open", "normal", true, doorHOpn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Draw(DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(10, 10, 12, 10)})
			e.Draw(DrawRequest{Kind: KindDoor, DoorType: tc.doorType, Alt: tc.alt, Points: points(at(11, 10))})
			if got := itemIDs(e.Map.Tile(at(11, 10))); !slices.Equal(got, []uint16{tc.want}) {
				t.Fatalf("items = %v, want [%d]", got, tc.want)
			}
		})
	}
}

func TestDrawDoorEraseTurnsDoorBackIntoWall(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(10, 10, 12, 10)})
	e.Draw(DrawRequest{Kind: KindDoor, Points: points(at(11, 10))})
	e.Draw(DrawRequest{Kind: KindDoor, Erase: true, Points: points(at(11, 10))})
	if got := itemIDs(e.Map.Tile(at(11, 10))); !slices.Equal(got, []uint16{wallH}) {
		t.Fatalf("items = %v, want [%d]", got, wallH)
	}
}

func TestDrawCarpetAndTableDispatch(t *testing.T) {
	cases := []struct {
		kind string
		name string
		want uint16
	}{
		{KindCarpet, "rug", rugC},
		{KindTable, "desk", tableAloneID},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			e := testEditor(t)
			drawSetGround(e, sandID, at(10, 10))
			e.Draw(DrawRequest{Kind: tc.kind, Name: tc.name, Points: points(at(10, 10))})
			if got := itemIDs(e.Map.Tile(at(10, 10))); !slices.Equal(got, []uint16{tc.want}) {
				t.Fatalf("items = %v, want [%d]", got, tc.want)
			}
			e.Draw(DrawRequest{Kind: tc.kind, Name: tc.name, Erase: true, Points: points(at(10, 10))})
			if got := itemIDs(e.Map.Tile(at(10, 10))); len(got) != 0 {
				t.Fatalf("items after erase = %v, want none", got)
			}
		})
	}
}

func TestDrawDoodadPlacesDoodad(t *testing.T) {
	e := testEditor(t)
	drawSetGround(e, sandID, at(10, 10))
	e.Draw(DrawRequest{Kind: KindDoodad, Name: "trees", Thickness: 100, Points: points(at(10, 10))})
	if got := itemIDs(e.Map.Tile(at(10, 10))); !slices.Equal(got, []uint16{treeID}) {
		t.Fatalf("items = %v, want [tree]", got)
	}
}

func TestDrawDoodadEraseRemovesDoodadItems(t *testing.T) {
	e := testEditor(t)
	// Grass and the bag belong to no doodad (sand and stone are part of
	// the "pit" stamp).
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: []otmap.Item{{ID: bagID}, {ID: treeID}}})
	drawSetGround(e, grassA, at(11, 10))
	changed, _ := e.Draw(DrawRequest{Kind: KindDoodad, Name: "trees", Erase: true, Points: points(at(10, 10)), Size: 1})
	if got := itemIDs(e.Map.Tile(at(10, 10))); !slices.Equal(got, []uint16{bagID}) || changed != 1 {
		t.Fatalf("items = %v, changed %d; want [bag] and only the tile with the tree changed", got, changed)
	}
}

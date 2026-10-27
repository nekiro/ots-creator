package mapedit

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func doodadEditor(t *testing.T, brushes ...*materials.Brush) *Editor {
	t.Helper()
	return brushesTestEditor(t, brushesAddBrush(brushes...))
}

func doodadSingles(singles ...materials.Single) materials.Alternative {
	return materials.Alternative{Singles: singles}
}

func doodadSingleOf(id uint16, chance int) materials.Single {
	return materials.Single{Chance: chance, Item: materials.ItemSpec{ID: id}}
}

func doodadPutTile(e *Editor, pos otmap.Position, ground uint16, items ...uint16) {
	e.Map.SetTile(pos, &otmap.Tile{Ground: otmap.Item{ID: ground}, Items: tileopsItems(items...)})
}

func doodadGround(tile *otmap.Tile) uint16 {
	if tile == nil {
		return 0
	}
	return tile.Ground.ID
}

func doodadDraw(e *Editor, name string, pos otmap.Position, alt bool) {
	e.Draw(DrawRequest{Kind: KindDoodad, Name: name, Points: points(pos), Alt: alt})
}

func doodadErase(e *Editor, name string, pos otmap.Position) int {
	changed, _ := e.Draw(DrawRequest{Kind: KindDoodad, Name: name, Points: points(pos), Erase: true})
	return changed
}

func TestDoodadGenerateOfUnknownBrushIsEmpty(t *testing.T) {
	e := testEditor(t)
	buffer := e.GenerateDoodad("lava", 1, 2, "circle", 0)
	want := &DoodadBuffer{Brush: "lava", Variation: 1, Size: 2, Shape: "circle"}
	if !reflect.DeepEqual(buffer, want) {
		t.Errorf("buffer = %+v, want %+v (no tiles)", buffer, want)
	}
	if e.Doodad() != buffer {
		t.Error("Doodad() is not the generated buffer")
	}
}

func TestDoodadIsNilBeforeGenerating(t *testing.T) {
	if got := testEditor(t).Doodad(); got != nil {
		t.Errorf("Doodad() = %+v, want nil before any generation", got)
	}
}

func TestDoodadGenerateStoresBufferAsPreview(t *testing.T) {
	e := testEditor(t)
	buffer := e.GenerateDoodad("trees", 0, 0, "square", 0)
	if e.Doodad() != buffer {
		t.Error("Doodad() is not the generated buffer")
	}
}

func TestDoodadGenerateSizeZeroPlacesOneObjectAtCursor(t *testing.T) {
	buffer := testEditor(t).GenerateDoodad("trees", 0, 0, "square", 0)
	want := []DoodadTile{{X: 0, Y: 0, Z: 0, Items: []otmap.Item{{ID: treeID}}}}
	if !reflect.DeepEqual(buffer.Tiles, want) {
		t.Errorf("tiles = %+v, want one tree at the cursor", buffer.Tiles)
	}
}

func TestDoodadGenerateOneSizeIgnoresSize(t *testing.T) {
	buffer := testEditor(t).GenerateDoodad("pit", 0, 3, "square", 0)
	var offsets [][3]int
	for _, tile := range buffer.Tiles {
		offsets = append(offsets, [3]int{tile.X, tile.Y, tile.Z})
	}
	if want := [][3]int{{0, 0, 0}, {1, 0, 0}}; !slices.Equal(offsets, want) {
		t.Errorf("tile offsets = %v, want the composite placed once %v", offsets, want)
	}
}

func TestDoodadGenerateMarksCompositeTilesWithGroundAsReplace(t *testing.T) {
	buffer := testEditor(t).GenerateDoodad("pit", 0, 0, "square", 0)
	want := []DoodadTile{
		{X: 0, Y: 0, Items: []otmap.Item{{ID: sandID}, {ID: stoneID}, {ID: borderN}}, Replace: true},
		{X: 1, Y: 0, Items: []otmap.Item{{ID: treeID}}},
	}
	if !reflect.DeepEqual(buffer.Tiles, want) {
		t.Errorf("tiles = %+v, want %+v (only the tile with ground replaces)", buffer.Tiles, want)
	}
}

func TestDoodadGenerateSquareScattersWithinSize(t *testing.T) {
	buffer := testEditor(t).GenerateDoodad("trees", 0, 2, "square", 0)
	if len(buffer.Tiles) < 2 {
		t.Fatalf("%d tiles, want several trees for size 2", len(buffer.Tiles))
	}
	for _, tile := range buffer.Tiles {
		if tile.X < -2 || tile.X > 2 || tile.Y < -2 || tile.Y > 2 || tile.Z != 0 {
			t.Errorf("tile at %d,%d,%d is outside the size 2 square", tile.X, tile.Y, tile.Z)
		}
	}
}

func TestDoodadGenerateCircleScattersWithinRadius(t *testing.T) {
	buffer := testEditor(t).GenerateDoodad("trees", 0, 3, "circle", 0)
	if len(buffer.Tiles) < 2 {
		t.Fatalf("%d tiles, want several trees for size 3", len(buffer.Tiles))
	}
	for _, tile := range buffer.Tiles {
		if math.Sqrt(float64(tile.X*tile.X+tile.Y*tile.Y)) >= 3+circleEdgeSlack {
			t.Errorf("tile at %d,%d is outside the radius 3 circle", tile.X, tile.Y)
		}
	}
}

func TestDoodadGenerateScatterNeverStacksObjectsOnOneTile(t *testing.T) {
	e := doodadEditor(t, &materials.Brush{Name: "pair", Type: materials.TypeDoodad,
		Doodad: &materials.DoodadOptions{Thickness: 1, ThicknessCeiling: 1},
		Alternatives: []materials.Alternative{
			{Composites: []materials.Composite{{Chance: 1, Tiles: []materials.CompositeTile{
				{X: 0, Y: 0, Items: []materials.ItemSpec{{ID: stoneID}}},
				{X: 1, Y: 0, Items: []materials.ItemSpec{{ID: bagID}}},
			}}}},
		}})
	for _, name := range []string{"trees", "pair"} {
		buffer := e.GenerateDoodad(name, 0, 2, "square", 0)
		for _, tile := range buffer.Tiles {
			if len(tile.Items) != 1 {
				t.Errorf("%s: tile %d,%d has items %+v, want one object per tile", name, tile.X, tile.Y, tile.Items)
			}
		}
	}
}

func TestDoodadGenerateThicknessOverridesBrushDensity(t *testing.T) {
	e := testEditor(t)
	// Size 2 covers 26 tiles: a thickness of 0.01 gives 0 objects, raised to 1.
	if got := len(e.GenerateDoodad("trees", 0, 2, "square", 0.01).Tiles); got != 1 {
		t.Errorf("thickness 0.01: %d tiles, want 1", got)
	}
	if got := len(e.GenerateDoodad("trees", 0, 2, "square", 0).Tiles); got < 5 {
		t.Errorf("brush density 100%%: %d tiles, want many", got)
	}
}

func TestDoodadAreaBySizeAndShape(t *testing.T) {
	for _, testCase := range []struct {
		size  int
		shape string
		want  int
	}{
		{1, "square", 10},
		{2, "square", 26},
		{2, "", 26},
		{1, "circle", 5},
		{2, "circle", 13},
		{3, "circle", 28},
	} {
		if got := doodadArea(testCase.size, testCase.shape); got != testCase.want {
			t.Errorf("doodadArea(%d, %q) = %d, want %d", testCase.size, testCase.shape, got, testCase.want)
		}
	}
}

func TestDoodadScatterCount(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		options       materials.DoodadOptions
		thickness     float64
		least, most   int
		size          int
		shape         string
		wantCountNote string
	}{
		// Size 1 square covers 10 tiles; the count is objects plus up to objects more.
		{"brush density", materials.DoodadOptions{Thickness: 1, ThicknessCeiling: 1}, 0, 10, 20, 1, "square", "10..20"},
		{"thickness overrides brush", materials.DoodadOptions{Thickness: 1, ThicknessCeiling: 1}, 0.5, 5, 10, 1, "square", "5..10"},
		{"ceiling 0 counts as 1", materials.DoodadOptions{Thickness: 1}, 0, 10, 20, 1, "square", "10..20"},
		{"no density gives one object", materials.DoodadOptions{}, 0, 1, 1, 1, "square", "1"},
		{"circle area", materials.DoodadOptions{Thickness: 1, ThicknessCeiling: 1}, 0, 5, 10, 1, "circle", "5..10"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := testEditor(t)
			brush := &DoodadBrush{options: testCase.options}
			for range 20 {
				got := e.doodadScatterCount(brush, testCase.size, testCase.shape, testCase.thickness)
				if got < testCase.least || got > testCase.most {
					t.Fatalf("count = %d, want %s", got, testCase.wantCountNote)
				}
			}
		})
	}
}

func TestDoodadGeneratePicksAlternativeByVariation(t *testing.T) {
	e := doodadEditor(t, &materials.Brush{Name: "two", Type: materials.TypeDoodad, Alternatives: []materials.Alternative{
		doodadSingles(doodadSingleOf(stoneID, 1)), doodadSingles(doodadSingleOf(bagID, 1)),
	}})
	for _, testCase := range []struct {
		variation int
		want      uint16
	}{{0, stoneID}, {1, bagID}, {2, stoneID}, {-1, bagID}} {
		buffer := e.GenerateDoodad("two", testCase.variation, 0, "square", 0)
		if len(buffer.Tiles) != 1 || buffer.Tiles[0].Items[0].ID != testCase.want {
			t.Errorf("variation %d: tiles %+v, want item %d", testCase.variation, buffer.Tiles, testCase.want)
		}
	}
}

func TestDoodadGenerateCopiesItemAttributes(t *testing.T) {
	e := doodadEditor(t, &materials.Brush{Name: "sign", Type: materials.TypeDoodad, Alternatives: []materials.Alternative{
		doodadSingles(materials.Single{Chance: 1, Item: materials.ItemSpec{ID: stoneID, Subtype: 3, ActionID: 5, UniqueID: 6, Text: "hello"}}),
	}})
	buffer := e.GenerateDoodad("sign", 0, 0, "square", 0)
	want := otmap.Item{ID: stoneID, Subtype: 3, Ext: &otmap.ItemExt{ActionID: 5, UniqueID: 6, Text: "hello"}}
	if len(buffer.Tiles) != 1 || !reflect.DeepEqual(buffer.Tiles[0].Items, []otmap.Item{want}) {
		t.Errorf("tiles = %+v, want one item %+v with the spec's attributes", buffer.Tiles, want)
	}
}

func TestDoodadGenerateItemWithoutAttributesHasNoExt(t *testing.T) {
	buffer := testEditor(t).GenerateDoodad("trees", 0, 0, "square", 0)
	if len(buffer.Tiles) != 1 || buffer.Tiles[0].Items[0].Ext != nil {
		t.Errorf("tiles = %+v, want a tree without attributes", buffer.Tiles)
	}
}

func TestDoodadGenerateChoosesBetweenCompositeAndSingle(t *testing.T) {
	composite := func(chance int) materials.Composite {
		return materials.Composite{Chance: chance, Tiles: []materials.CompositeTile{
			{X: 0, Y: 0, Items: []materials.ItemSpec{{ID: bagID}}},
			{X: 0, Y: 1, Items: []materials.ItemSpec{{ID: bagID}}},
		}}
	}
	for _, testCase := range []struct {
		name          string
		alternative   materials.Alternative
		wantComposite bool
	}{
		{"only composites", materials.Alternative{Composites: []materials.Composite{composite(1)}}, true},
		{"only singles", doodadSingles(doodadSingleOf(stoneID, 1)), false},
		{"composite without chance", materials.Alternative{Composites: []materials.Composite{composite(0)}, Singles: []materials.Single{doodadSingleOf(stoneID, 1)}}, false},
		{"single without chance", materials.Alternative{Composites: []materials.Composite{composite(1)}, Singles: []materials.Single{doodadSingleOf(stoneID, 0)}}, true},
		{"no chances at all", materials.Alternative{Composites: []materials.Composite{composite(0)}, Singles: []materials.Single{doodadSingleOf(stoneID, 0)}}, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := doodadEditor(t, &materials.Brush{Name: "mixed", Type: materials.TypeDoodad, Alternatives: []materials.Alternative{testCase.alternative}})
			for range 10 {
				buffer := e.GenerateDoodad("mixed", 0, 0, "square", 0)
				gotComposite := len(buffer.Tiles) == 2
				gotSingle := len(buffer.Tiles) == 1 && buffer.Tiles[0].Items[0].ID == stoneID
				if gotComposite != testCase.wantComposite || gotComposite == gotSingle {
					t.Fatalf("tiles = %+v, want composite %v", buffer.Tiles, testCase.wantComposite)
				}
			}
		})
	}
}

func TestDoodadGeneratePicksSingleByChance(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		singles []materials.Single
		want    uint16
	}{
		{"only the single with a chance", []materials.Single{doodadSingleOf(stoneID, 0), doodadSingleOf(bagID, 3)}, bagID},
		{"first single when no chances", []materials.Single{doodadSingleOf(stoneID, 0), doodadSingleOf(bagID, 0)}, stoneID},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := doodadEditor(t, &materials.Brush{Name: "picks", Type: materials.TypeDoodad, Alternatives: []materials.Alternative{doodadSingles(testCase.singles...)}})
			for range 10 {
				buffer := e.GenerateDoodad("picks", 0, 0, "square", 0)
				if len(buffer.Tiles) != 1 || buffer.Tiles[0].Items[0].ID != testCase.want {
					t.Fatalf("tiles = %+v, want item %d", buffer.Tiles, testCase.want)
				}
			}
		})
	}
}

func TestDoodadGeneratePicksCompositeByChance(t *testing.T) {
	composite := func(chance int, id uint16) materials.Composite {
		return materials.Composite{Chance: chance, Tiles: []materials.CompositeTile{{Items: []materials.ItemSpec{{ID: id}}}}}
	}
	for _, testCase := range []struct {
		name       string
		composites []materials.Composite
		want       uint16
	}{
		{"only the composite with a chance", []materials.Composite{composite(0, stoneID), composite(2, bagID)}, bagID},
		{"first composite when no chances", []materials.Composite{composite(0, stoneID), composite(0, bagID)}, stoneID},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := doodadEditor(t, &materials.Brush{Name: "groups", Type: materials.TypeDoodad, Alternatives: []materials.Alternative{{Composites: testCase.composites}}})
			for range 10 {
				buffer := e.GenerateDoodad("groups", 0, 0, "square", 0)
				if len(buffer.Tiles) != 1 || buffer.Tiles[0].Items[0].ID != testCase.want {
					t.Fatalf("tiles = %+v, want item %d", buffer.Tiles, testCase.want)
				}
			}
		})
	}
}

func TestDoodadGenerateOfEmptyAlternativeHasNoTiles(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		alternative materials.Alternative
	}{
		{"no singles or composites", materials.Alternative{}},
		{"composite without tiles", materials.Alternative{Composites: []materials.Composite{{Chance: 1}}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := doodadEditor(t, &materials.Brush{Name: "hollow", Type: materials.TypeDoodad, Alternatives: []materials.Alternative{testCase.alternative}})
			for _, size := range []int{0, 2} {
				if buffer := e.GenerateDoodad("hollow", 0, size, "square", 0); len(buffer.Tiles) != 0 {
					t.Errorf("size %d: tiles = %+v, want none", size, buffer.Tiles)
				}
			}
		})
	}
}

func TestDoodadGenerateSortsTilesByFloorRowColumn(t *testing.T) {
	stone := []materials.ItemSpec{{ID: stoneID}}
	e := doodadEditor(t, &materials.Brush{Name: "spread", Type: materials.TypeDoodad, Alternatives: []materials.Alternative{
		{Composites: []materials.Composite{{Chance: 1, Tiles: []materials.CompositeTile{
			{X: 1, Y: 1, Items: stone}, {X: 0, Y: 1, Items: stone}, {X: 1, Y: 0, Items: stone},
			{X: 0, Y: 0, Items: stone}, {X: 1, Y: 0, Z: -1, Items: stone},
		}}}},
	}})
	var offsets [][3]int
	for _, tile := range e.GenerateDoodad("spread", 0, 0, "square", 0).Tiles {
		offsets = append(offsets, [3]int{tile.X, tile.Y, tile.Z})
	}
	want := [][3]int{{1, 0, -1}, {0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}}
	if !slices.Equal(offsets, want) {
		t.Errorf("tile order = %v, want %v (floor, then row, then column)", offsets, want)
	}
}

func TestDoodadGenerateIsDeterministicForSeed(t *testing.T) {
	first := testEditor(t).GenerateDoodad("trees", 0, 4, "circle", 0)
	second := testEditor(t).GenerateDoodad("trees", 0, 4, "circle", 0)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("same seed gave different placements:\n%+v\n%+v", first, second)
	}
}

func TestDoodadPlacesTreeOnWalkableGround(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(60, 60)), Size: 2})
	doodadDraw(e, "trees", at(60, 60), false)
	if got := itemIDs(e.Map.Tile(at(60, 60))); !slices.Equal(got, []uint16{treeID}) {
		t.Errorf("items = %v, want [%d]", got, treeID)
	}
}

func TestDoodadSkipsBlockingTileWithoutAlt(t *testing.T) {
	e := testEditor(t)
	doodadPutTile(e, at(60, 60), sandID, treeID)
	doodadDraw(e, "trees", at(60, 60), false)
	if got := itemIDs(e.Map.Tile(at(60, 60))); !slices.Equal(got, []uint16{treeID}) {
		t.Errorf("items = %v, want only the first tree: a tree makes the tile blocking", got)
	}
}

func TestDoodadAltPlacesOnBlockingTile(t *testing.T) {
	e := testEditor(t)
	doodadPutTile(e, at(60, 60), sandID, treeID)
	doodadDraw(e, "trees", at(60, 60), true)
	if got := itemIDs(e.Map.Tile(at(60, 60))); !slices.Equal(got, []uint16{treeID, treeID}) {
		t.Errorf("items = %v, want a second tree with Alt", got)
	}
}

func TestDoodadSkipsTileWithoutGround(t *testing.T) {
	e := testEditor(t)
	doodadDraw(e, "trees", at(90, 90), false)
	if tile := e.Map.Tile(at(90, 90)); tile != nil {
		t.Errorf("tile = %+v, want no tree placed in the void", tile)
	}
}

func TestDoodadSkipsDuplicateItemOnWalkableTile(t *testing.T) {
	e := doodadEditor(t,
		&materials.Brush{Name: "stones", Type: materials.TypeDoodad, Alternatives: []materials.Alternative{doodadSingles(doodadSingleOf(stoneID, 1))}},
		&materials.Brush{Name: "piles", Type: materials.TypeDoodad, Doodad: &materials.DoodadOptions{OnDuplicate: true},
			Alternatives: []materials.Alternative{doodadSingles(doodadSingleOf(stoneID, 1))}},
	)
	for _, testCase := range []struct {
		brush string
		alt   bool
		want  []uint16
	}{
		{"stones", false, []uint16{stoneID}},
		{"stones", true, []uint16{stoneID, stoneID}},
		{"piles", false, []uint16{stoneID, stoneID}},
	} {
		doodadPutTile(e, at(60, 60), sandID, stoneID)
		doodadDraw(e, testCase.brush, at(60, 60), testCase.alt)
		if got := itemIDs(e.Map.Tile(at(60, 60))); !slices.Equal(got, testCase.want) {
			t.Errorf("%s alt=%v: items = %v, want %v", testCase.brush, testCase.alt, got, testCase.want)
		}
	}
}

func TestDoodadFits(t *testing.T) {
	plain := &DoodadBrush{owned: map[uint16]bool{stoneID: true}}
	onBlocking := &DoodadBrush{options: materials.DoodadOptions{OnBlocking: true}, owned: map[uint16]bool{stoneID: true}}
	onDuplicate := &DoodadBrush{options: materials.DoodadOptions{OnDuplicate: true}, owned: map[uint16]bool{stoneID: true}}
	walkable := &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: tileopsItems(bagID)}
	noGround := &otmap.Tile{Items: tileopsItems(bagID)}
	blocked := &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: tileopsItems(treeID)}
	duplicate := &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: tileopsItems(stoneID)}
	for _, testCase := range []struct {
		name         string
		brush        *DoodadBrush
		tile         *otmap.Tile
		replace, alt bool
		want         bool
	}{
		{"walkable tile", plain, walkable, false, false, true},
		{"no tile", plain, nil, false, false, false},
		{"no ground", plain, noGround, false, false, false},
		{"blocking tile", plain, blocked, false, false, false},
		{"blocking tile with alt", plain, blocked, false, true, true},
		{"blocking tile with on blocking brush", onBlocking, blocked, false, false, true},
		{"duplicate item", plain, duplicate, false, false, false},
		{"duplicate item with alt", plain, duplicate, false, true, true},
		{"duplicate item with on duplicate brush", onDuplicate, duplicate, false, false, true},
		{"duplicate item with on blocking brush", onBlocking, duplicate, false, false, false},
		{"replace on no tile", plain, nil, true, false, true},
		{"replace on duplicate item", plain, duplicate, true, false, true},
		{"replace on blocking tile", plain, blocked, true, false, true},
	} {
		if got := testEditor(t).doodadFits(testCase.brush, testCase.tile, testCase.replace, testCase.alt); got != testCase.want {
			t.Errorf("%s: doodadFits = %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

func TestDoodadBigBrushScattersSeveralTrees(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(200, 200)), Size: 5})
	e.Draw(DrawRequest{Kind: KindDoodad, Name: "trees", Points: points(at(200, 200)), Size: 4})
	trees := 0
	e.Map.EachIn(190, 190, 210, 210, 7, 7, func(pos otmap.Position, tile *otmap.Tile) {
		for _, id := range itemIDs(tile) {
			if id == treeID {
				trees++
				if int(pos.X) < 196 || int(pos.X) > 204 || int(pos.Y) < 196 || int(pos.Y) > 204 {
					t.Errorf("tree at %v is outside the size 4 brush", pos)
				}
			}
		}
	})
	if trees < 3 {
		t.Errorf("%d trees, want at least 3 from a size 4 brush", trees)
	}
}

func TestDoodadPlacementGeneratesNextBuffer(t *testing.T) {
	e := testEditor(t)
	doodadPutTile(e, at(60, 60), sandID)
	first := e.GenerateDoodad("trees", 0, 1, "circle", 0.5)
	e.Draw(DrawRequest{Kind: KindDoodad, Name: "trees", Points: points(at(60, 60)), Size: 1, Shape: "circle", Thickness: 0.5})
	next := e.Doodad()
	if next == first || next == nil {
		t.Fatal("placing did not generate a new buffer")
	}
	if next.Brush != "trees" || next.Size != 1 || next.Shape != "circle" || next.Variation != 0 {
		t.Errorf("next buffer = %+v, want the same brush, size, shape and variation", next)
	}
}

func TestPlaceDoodadWithoutBufferOrBrushChangesNothing(t *testing.T) {
	e := testEditor(t)
	doodadPutTile(e, at(60, 60), sandID)
	tx := e.Begin("Doodad", 0)
	e.placeDoodad(tx, at(60, 60), false, 0)
	e.doodad = &DoodadBuffer{Brush: "gone", Tiles: []DoodadTile{{Items: []otmap.Item{{ID: stoneID}}}}}
	e.placeDoodad(tx, at(60, 60), false, 0)
	if changed := tx.Commit(); changed != 0 {
		t.Errorf("%d tiles changed, want 0", changed)
	}
	if got := itemIDs(e.Map.Tile(at(60, 60))); got != nil {
		t.Errorf("items = %v, want none", got)
	}
}

func TestDoodadSkipsTilesOutsideTheMap(t *testing.T) {
	e := doodadEditor(t, &materials.Brush{Name: "edge", Type: materials.TypeDoodad, Alternatives: []materials.Alternative{
		{Composites: []materials.Composite{{Chance: 1, Tiles: []materials.CompositeTile{
			{X: -1, Y: 0, Items: []materials.ItemSpec{{ID: stoneID}}},
			{X: 0, Y: 0, Items: []materials.ItemSpec{{ID: bagID}}},
		}}}},
	}})
	doodadPutTile(e, at(0, 0), sandID)
	doodadDraw(e, "edge", at(0, 0), false)
	if got := itemIDs(e.Map.Tile(at(0, 0))); !slices.Equal(got, []uint16{bagID}) {
		t.Errorf("items = %v, want [%d]: the tile at x -1 is skipped", got, bagID)
	}
}

func TestDoodadStampReplacesTileWithGroundWhole(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(60, 60)), Size: 2})
	doodadDraw(e, "trees", at(60, 60), false)
	doodadDraw(e, "pit", at(60, 60), false)
	// The tree is gone and the border stays above the stone as copied
	// (addItem would have put it at the bottom).
	tile := e.Map.Tile(at(60, 60))
	if doodadGround(tile) != sandID || !slices.Equal(itemIDs(tile), []uint16{stoneID, borderN}) {
		t.Errorf("stamped tile: ground %d, items %v; want ground %d, items [%d %d]",
			doodadGround(tile), itemIDs(tile), sandID, stoneID, borderN)
	}
}

func TestDoodadStampReplacesTileWithoutCheckingBlocking(t *testing.T) {
	e := testEditor(t)
	doodadDraw(e, "pit", at(60, 60), false)
	tile := e.Map.Tile(at(60, 60))
	if doodadGround(tile) != sandID || !slices.Equal(itemIDs(tile), []uint16{stoneID, borderN}) {
		t.Errorf("stamped void tile: ground %d, items %v; want ground %d, items [%d %d]",
			doodadGround(tile), itemIDs(tile), sandID, stoneID, borderN)
	}
}

func TestDoodadStampAddsItemsOfTileWithoutGroundOnTop(t *testing.T) {
	e := testEditor(t)
	doodadPutTile(e, at(61, 60), grassB, bagID)
	doodadDraw(e, "pit", at(60, 60), false)
	tile := e.Map.Tile(at(61, 60))
	if doodadGround(tile) != grassB || !slices.Equal(itemIDs(tile), []uint16{bagID, treeID}) {
		t.Errorf("tile: ground %d, items %v; want the grass kept and the tree added: ground %d, items [%d %d]",
			doodadGround(tile), itemIDs(tile), grassB, bagID, treeID)
	}
}

func TestDoodadStampSkipsTileThatAlreadyHasItsItem(t *testing.T) {
	e := testEditor(t)
	doodadPutTile(e, at(61, 60), grassB, treeID)
	doodadDraw(e, "pit", at(60, 60), false)
	if got := itemIDs(e.Map.Tile(at(61, 60))); !slices.Equal(got, []uint16{treeID}) {
		t.Errorf("items = %v, want one tree: the stamp owns the tree and does not place duplicates", got)
	}
}

func doodadGroundPatch(redoBorders bool) *materials.Brush {
	return &materials.Brush{Name: "patch", Type: materials.TypeDoodad, Doodad: &materials.DoodadOptions{RedoBorders: redoBorders},
		Alternatives: []materials.Alternative{doodadSingles(doodadSingleOf(grassA, 1))}}
}

func doodadNeighborItems(e *Editor, center otmap.Position) [][]uint16 {
	var neighbors [][]uint16
	for _, offset := range neighbors8 {
		pos, _ := center.Add(offset[0], offset[1], 0)
		neighbors = append(neighbors, itemIDs(e.Map.Tile(pos)))
	}
	return neighbors
}

func TestDoodadRedoBordersBordersAroundPlacedGround(t *testing.T) {
	reference := testEditor(t)
	reference.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(60, 60)), Size: 2})
	reference.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(60, 60))})
	want := doodadNeighborItems(reference, at(60, 60))
	if !slices.ContainsFunc(want, func(items []uint16) bool { return len(items) > 0 }) {
		t.Fatalf("reference ground brush placed no borders: %v", want)
	}

	e := doodadEditor(t, doodadGroundPatch(true))
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(60, 60)), Size: 2})
	doodadDraw(e, "patch", at(60, 60), false)
	if got := doodadGround(e.Map.Tile(at(60, 60))); got != grassA {
		t.Fatalf("ground = %d, want the placed grass %d", got, grassA)
	}
	if got := doodadNeighborItems(e, at(60, 60)); !reflect.DeepEqual(got, want) {
		t.Errorf("neighbor items = %v, want the borders the grass brush places %v", got, want)
	}
}

func TestDoodadWithoutRedoBordersLeavesNeighborsAlone(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		redoBorders bool
		automagic   bool
	}{
		{"redo borders off", false, true},
		{"automagic off", true, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := doodadEditor(t, doodadGroundPatch(testCase.redoBorders))
			e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(60, 60)), Size: 2})
			e.Settings.Automagic = testCase.automagic
			doodadDraw(e, "patch", at(60, 60), false)
			if got := doodadGround(e.Map.Tile(at(60, 60))); got != grassA {
				t.Fatalf("ground = %d, want the placed grass %d", got, grassA)
			}
			for i, items := range doodadNeighborItems(e, at(60, 60)) {
				if len(items) != 0 {
					t.Errorf("neighbor %d has items %v, want none", i, items)
				}
			}
		})
	}
}

func TestDoodadWallReplacesWallOnTile(t *testing.T) {
	e := doodadEditor(t, &materials.Brush{Name: "wall piece", Type: materials.TypeDoodad, Doodad: &materials.DoodadOptions{OnBlocking: true},
		Alternatives: []materials.Alternative{doodadSingles(doodadSingleOf(wallV, 1))}})
	doodadPutTile(e, at(60, 60), sandID, borderN, wallH, stoneID)
	doodadDraw(e, "wall piece", at(60, 60), false)
	if got := itemIDs(e.Map.Tile(at(60, 60))); !slices.Equal(got, []uint16{borderN, wallV, stoneID}) {
		t.Errorf("items = %v, want the old wall replaced: [%d %d %d]", got, borderN, wallV, stoneID)
	}
}

func TestDoodadRemoveOptionalBorderClearsTileState(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		remove       bool
		wantOptional bool
	}{
		{"remove optional border", true, false},
		{"keep optional border", false, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := doodadEditor(t, &materials.Brush{Name: "clearer", Type: materials.TypeDoodad, Doodad: &materials.DoodadOptions{RemoveOptionalBorder: testCase.remove},
				Alternatives: []materials.Alternative{doodadSingles(doodadSingleOf(stoneID, 1))}})
			e.Map.SetTile(at(60, 60), &otmap.Tile{Ground: otmap.Item{ID: sandID}, State: otmap.StateOptionalBorder})
			doodadDraw(e, "clearer", at(60, 60), false)
			tile := e.Map.Tile(at(60, 60))
			if !slices.Equal(itemIDs(tile), []uint16{stoneID}) {
				t.Fatalf("items = %v, want [%d]", itemIDs(tile), stoneID)
			}
			if got := tile.State&otmap.StateOptionalBorder != 0; got != testCase.wantOptional {
				t.Errorf("optional border state = %v, want %v", got, testCase.wantOptional)
			}
		})
	}
}

func TestDoodadEraseRemovesTreesInArea(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(200, 200)), Size: 5})
	e.Draw(DrawRequest{Kind: KindDoodad, Name: "trees", Points: points(at(200, 200)), Size: 4})
	e.Draw(DrawRequest{Kind: KindDoodad, Name: "trees", Points: points(at(200, 200)), Size: 6, Erase: true})
	e.Map.EachIn(190, 190, 210, 210, 7, 7, func(pos otmap.Position, tile *otmap.Tile) {
		if slices.Contains(itemIDs(tile), treeID) {
			t.Errorf("tree left at %v after erasing", pos)
		}
		if tile.Ground.ID != grassA && tile.Ground.ID != grassB {
			t.Errorf("ground at %v = %d, want grass kept", pos, tile.Ground.ID)
		}
	})
}

func TestDoodadEraseRemovesItemsOfAnyDoodadOrOnlyLikeBrush(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		eraseLike bool
		brush     string
		want      []uint16
	}{
		// The stone belongs to pit, the tree to trees and pit.
		{"any doodad", false, "trees", []uint16{bagID}},
		{"erase like keeps other brushes' items", true, "trees", []uint16{stoneID, bagID}},
		{"erase like removes the brush's items", true, "pit", []uint16{bagID}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := testEditor(t)
			e.Settings.DoodadEraseLike = testCase.eraseLike
			doodadPutTile(e, at(60, 60), grassA, stoneID, treeID, bagID)
			doodadErase(e, testCase.brush, at(60, 60))
			tile := e.Map.Tile(at(60, 60))
			if doodadGround(tile) != grassA || !slices.Equal(itemIDs(tile), testCase.want) {
				t.Errorf("ground %d, items %v; want ground %d, items %v", doodadGround(tile), itemIDs(tile), grassA, testCase.want)
			}
		})
	}
}

func TestDoodadEraseKeepsItemsWithAttributesWhenLeavingUnique(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		leaveUnique bool
		want        []uint16
	}{
		{"leave unique", true, []uint16{treeID}},
		{"erase unique", false, nil},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := testEditor(t)
			e.Settings.EraserLeaveUnique = testCase.leaveUnique
			e.Map.SetTile(at(60, 60), &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: []otmap.Item{
				{ID: treeID, Ext: &otmap.ItemExt{ActionID: 1000}}, {ID: treeID},
			}})
			doodadErase(e, "trees", at(60, 60))
			if got := itemIDs(e.Map.Tile(at(60, 60))); !slices.Equal(got, testCase.want) {
				t.Errorf("items = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestDoodadEraseRemovesGroundOwnedByDoodad(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		eraseLike  bool
		brush      string
		wantGround uint16
	}{
		// Sand is part of the pit stamp.
		{"any doodad", false, "trees", 0},
		{"erase like keeps ground of another brush", true, "trees", sandID},
		{"erase like removes the brush's ground", true, "pit", 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := testEditor(t)
			e.Settings.DoodadEraseLike = testCase.eraseLike
			doodadPutTile(e, at(60, 60), sandID, bagID)
			doodadErase(e, testCase.brush, at(60, 60))
			tile := e.Map.Tile(at(60, 60))
			if doodadGround(tile) != testCase.wantGround || !slices.Equal(itemIDs(tile), []uint16{bagID}) {
				t.Errorf("ground %d, items %v; want ground %d, items [%d]", doodadGround(tile), itemIDs(tile), testCase.wantGround, bagID)
			}
		})
	}
}

func TestDoodadEraseLeavesTileWithoutDoodadItemsUnchanged(t *testing.T) {
	e := testEditor(t)
	doodadPutTile(e, at(60, 60), grassA, bagID, coinID)
	if changed := doodadErase(e, "trees", at(60, 60)); changed != 0 {
		t.Errorf("%d tiles changed, want 0", changed)
	}
	if got := itemIDs(e.Map.Tile(at(60, 60))); !slices.Equal(got, []uint16{bagID, coinID}) {
		t.Errorf("items = %v, want [%d %d]", got, bagID, coinID)
	}
	if e.CanUndo() {
		t.Error("erasing nothing added an undo step")
	}
}

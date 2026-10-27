package mapedit

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

const (
	tileopsTopA, tileopsTopB = 780, 781 // top items (top order 3)
	tileopsMaterialBorder    = 782      // border by materials, top order 0
	tileopsEquivalentBorder  = 783      // border that brings its own ground
)

// tileopsEditor is testEditor with two top items and two material borders:
// one whose item type has top order 0 and one with a ground equivalent.
func tileopsEditor(t *testing.T) *Editor {
	t.Helper()
	e := brushesTestEditor(t, func(materialsDB *materials.Materials) {
		materialsDB.MetaItems = append(materialsDB.MetaItems, tileopsTopA, tileopsTopB, tileopsMaterialBorder)
		materialsDB.Borders = append(materialsDB.Borders,
			&materials.Border{ID: 2, Edges: map[string]uint16{"n": tileopsMaterialBorder}},
			&materials.Border{ID: 3, GroundEquivalent: sandID, Edges: map[string]uint16{"n": tileopsEquivalentBorder}},
		)
	})
	e.Items.Get(tileopsTopA).TopOrder = 3
	e.Items.Get(tileopsTopB).TopOrder = 3
	return e
}

func tileopsItems(ids ...uint16) []otmap.Item {
	var items []otmap.Item
	for _, id := range ids {
		items = append(items, otmap.Item{ID: id})
	}
	return items
}

func TestTopOrderOfItems(t *testing.T) {
	e := tileopsEditor(t)
	for _, testCase := range []struct {
		name string
		id   uint16
		want uint8
	}{
		{"ground", grassA, 0},
		{"common item", stoneID, 0},
		{"border item", borderN, 1},
		{"foreign border", foreignBorder, 1},
		{"material border with top order 0", tileopsMaterialBorder, 1},
		{"material border without item type", tileopsEquivalentBorder, 1},
		{"wall", wallH, 2},
		{"top item", tileopsTopA, 3},
		{"unknown item", 9999, 0},
	} {
		if got := e.topOrder(testCase.id); got != testCase.want {
			t.Errorf("%s: topOrder(%d) = %d, want %d", testCase.name, testCase.id, got, testCase.want)
		}
	}
}

func TestAddItemStacksLikeTheServer(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		ground     uint16
		items      []uint16
		add        uint16
		wantGround uint16
		wantItems  []uint16
	}{
		{"ground replaces the ground and keeps items", grassA, []uint16{stoneID}, sandID, sandID, []uint16{stoneID}},
		{"common item goes on top", grassA, []uint16{stoneID}, bagID, grassA, []uint16{stoneID, bagID}},
		{"border goes below common items", grassA, []uint16{stoneID}, borderN, grassA, []uint16{borderN, stoneID}},
		{"border goes after borders", grassA, []uint16{borderN, stoneID}, borderE, grassA, []uint16{borderN, borderE, stoneID}},
		{"border goes below walls", grassA, []uint16{wallH}, borderN, grassA, []uint16{borderN, wallH}},
		{"wall goes after borders and below common items", grassA, []uint16{borderN, stoneID}, wallH, grassA, []uint16{borderN, wallH, stoneID}},
		{"top item goes after walls and below common items", grassA, []uint16{wallH, stoneID}, tileopsTopA, grassA, []uint16{wallH, tileopsTopA, stoneID}},
		{"wall goes below top items", grassA, []uint16{tileopsTopA}, wallH, grassA, []uint16{wallH, tileopsTopA}},
		{"top item goes after top items", grassA, []uint16{tileopsTopA}, tileopsTopB, grassA, []uint16{tileopsTopA, tileopsTopB}},
		{"stackable is not merged", grassA, []uint16{coinID}, coinID, grassA, []uint16{coinID, coinID}},
		{"item on empty tile", 0, nil, stoneID, 0, []uint16{stoneID}},
		{"border with ground equivalent sets the ground and goes to the bottom", grassA, []uint16{borderN, stoneID}, tileopsEquivalentBorder, sandID, []uint16{tileopsEquivalentBorder, borderN, stoneID}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := tileopsEditor(t)
			tile := &otmap.Tile{Ground: otmap.Item{ID: testCase.ground}, Items: tileopsItems(testCase.items...)}
			e.addItem(tile, otmap.Item{ID: testCase.add})
			if tile.Ground.ID != testCase.wantGround || !slices.Equal(itemIDs(tile), testCase.wantItems) {
				t.Errorf("after adding %d: ground %d, items %v; want ground %d, items %v",
					testCase.add, tile.Ground.ID, itemIDs(tile), testCase.wantGround, testCase.wantItems)
			}
		})
	}
}

func TestAddBorderItemPutsItAtTheBottom(t *testing.T) {
	tile := &otmap.Tile{Items: tileopsItems(wallH, stoneID)}
	addBorderItem(tile, borderS)
	if want := []uint16{borderS, wallH, stoneID}; !slices.Equal(itemIDs(tile), want) {
		t.Errorf("items = %v, want %v", itemIDs(tile), want)
	}
}

func TestCleanBordersRemovesOnlyBordersAtTheBottom(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		items []uint16
		want  []uint16
	}{
		{"bottom borders removed, border above an item kept", []uint16{borderN, foreignBorder, stoneID, borderS}, []uint16{stoneID, borderS}},
		{"only borders", []uint16{borderN, borderE}, nil},
		{"no borders", []uint16{wallH, stoneID}, []uint16{wallH, stoneID}},
		{"empty tile", nil, nil},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			e := testEditor(t)
			tile := &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: tileopsItems(testCase.items...)}
			e.cleanBorders(tile)
			if !slices.Equal(itemIDs(tile), testCase.want) {
				t.Errorf("items = %v, want %v", itemIDs(tile), testCase.want)
			}
		})
	}
}

func TestTileGroundBrushOf(t *testing.T) {
	e := testEditor(t)
	if e.groundBrushOf(nil) != nil {
		t.Error("ground brush of a nil tile is not nil")
	}
	if e.groundBrushOf(&otmap.Tile{Items: tileopsItems(stoneID)}) != nil {
		t.Error("ground brush of a tile without ground is not nil")
	}
	if got := e.groundBrushOf(&otmap.Tile{Ground: otmap.Item{ID: grassB}}); got != e.Brushes.Ground("grass") {
		t.Errorf("ground brush of a grass tile = %+v, want grass", got)
	}
}

func TestHasOptionalBorder(t *testing.T) {
	e := brushesTestEditor(t, func(materialsDB *materials.Materials) {
		materialsDB.Borders = append(materialsDB.Borders, &materials.Border{ID: 2, Optional: true, Edges: map[string]uint16{"n": 784}})
	})
	for _, testCase := range []struct {
		name string
		tile *otmap.Tile
		want bool
	}{
		{"state flag", &otmap.Tile{State: otmap.StateOptionalBorder}, true},
		{"optional border item", &otmap.Tile{Items: tileopsItems(stoneID, 784)}, true},
		{"plain border item", &otmap.Tile{Items: tileopsItems(borderN)}, false},
		{"no items", &otmap.Tile{}, false},
	} {
		if got := e.hasOptionalBorder(testCase.tile); got != testCase.want {
			t.Errorf("%s: hasOptionalBorder = %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

func TestBlockingTiles(t *testing.T) {
	e := testEditor(t)
	for _, testCase := range []struct {
		name string
		tile *otmap.Tile
		want bool
	}{
		{"no tile", nil, true},
		{"no ground", &otmap.Tile{Items: tileopsItems(stoneID)}, true},
		{"walkable ground", &otmap.Tile{Ground: otmap.Item{ID: grassA}}, false},
		{"ground with moveable items", &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: tileopsItems(stoneID, bagID)}, false},
		{"ground with a tree", &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: tileopsItems(stoneID, treeID)}, true},
		{"ground with a wall", &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: tileopsItems(wallV)}, true},
	} {
		if got := e.IsBlocking(testCase.tile); got != testCase.want {
			t.Errorf("%s: IsBlocking = %v, want %v", testCase.name, got, testCase.want)
		}
		if got := e.blocking(testCase.tile); got != testCase.want {
			t.Errorf("%s: blocking = %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

func TestHasFlagChecksGroundAndItems(t *testing.T) {
	e := testEditor(t)
	tile := &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: tileopsItems(9999, coinID)}
	if !e.hasFlag(tile, otitems.FlagGround) {
		t.Error("hasFlag(ground flag) = false, want true from the ground")
	}
	if !e.hasFlag(tile, otitems.FlagStackable) {
		t.Error("hasFlag(stackable) = false, want true from the coin")
	}
	if e.hasFlag(tile, otitems.FlagContainer) {
		t.Error("hasFlag(container) = true, want false: no container on the tile")
	}
	if e.hasFlag(nil, otitems.FlagGround) {
		t.Error("hasFlag of a nil tile = true, want false")
	}
}

func TestTileHasWallCarpetTable(t *testing.T) {
	e := testEditor(t)
	for _, testCase := range []struct {
		name                            string
		items                           []uint16
		wantWall, wantCarpet, wantTable bool
	}{
		{"wall", []uint16{stoneID, wallH}, true, false, false},
		{"door", []uint16{doorHClosed}, true, false, false},
		{"carpet", []uint16{rugN}, false, true, false},
		{"table", []uint16{tableH}, false, false, true},
		{"plain items", []uint16{stoneID, borderN, treeID}, false, false, false},
		{"empty", nil, false, false, false},
	} {
		tile := &otmap.Tile{Ground: otmap.Item{ID: grassA}, Items: tileopsItems(testCase.items...)}
		if got := e.hasWall(tile); got != testCase.wantWall {
			t.Errorf("%s: hasWall = %v, want %v", testCase.name, got, testCase.wantWall)
		}
		if got := e.hasCarpet(tile); got != testCase.wantCarpet {
			t.Errorf("%s: hasCarpet = %v, want %v", testCase.name, got, testCase.wantCarpet)
		}
		if got := e.hasTable(tile); got != testCase.wantTable {
			t.Errorf("%s: hasTable = %v, want %v", testCase.name, got, testCase.wantTable)
		}
	}
}

func TestPosAtBuildsPosition(t *testing.T) {
	if got, want := posAt(12, 345, 6), (otmap.Position{X: 12, Y: 345, Z: 6}); got != want {
		t.Errorf("posAt = %+v, want %+v", got, want)
	}
}

func TestInRange(t *testing.T) {
	for _, testCase := range []struct {
		x, y int
		want bool
	}{
		{0, 0, true}, {0xFFFF, 0xFFFF, true}, {-1, 0, false}, {0, -1, false}, {0x10000, 0, false}, {0, 0x10000, false},
	} {
		if got := inRange(testCase.x, testCase.y); got != testCase.want {
			t.Errorf("inRange(%d, %d) = %v, want %v", testCase.x, testCase.y, got, testCase.want)
		}
	}
}

func TestNewItemGivesStackablesCountOne(t *testing.T) {
	e := testEditor(t)
	for _, testCase := range []struct {
		id          uint16
		wantSubtype uint16
	}{{coinID, 1}, {stoneID, 0}, {9999, 0}} {
		if got := e.newItem(testCase.id); got.ID != testCase.id || got.Subtype != testCase.wantSubtype {
			t.Errorf("newItem(%d) = %+v, want subtype %d", testCase.id, got, testCase.wantSubtype)
		}
	}
}

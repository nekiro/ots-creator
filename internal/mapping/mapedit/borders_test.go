package mapedit

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Ground items of the custom border materials (bordersEditor).
const (
	bordersGrassItem    = 100
	bordersSandItem     = 110
	bordersStoneItem    = 120
	bordersWaterItem    = 130
	bordersMountainItem = 140
	bordersLavaItem     = 150
	bordersShoreItem    = 160
	bordersDirtItem     = 170
	bordersMossItem     = 180
	bordersClayItem     = 190
)

// First items of the custom border sets: a set holds 12 pieces in edge
// order (n, e, s, w, cnw, cne, csw, cse, dnw, dne, dse, dsw).
const (
	bordersSet1     = 200 // border 1, same ids as the fixture border
	bordersSet2     = 220 // border 2
	bordersSet3     = 240 // border 3
	bordersSet4     = 260 // border 4, optional (mountain)
	bordersSet5     = 280 // border 5, straight sides only
	bordersSet7     = 300 // border 7, in group bordersGroup
	bordersGroup    = 9
	bordersReplaced = 250 // piece put in by specific cases
	bordersAltPiece = 251
)

// bordersPiece is the item of an edge in a border set starting at first.
func bordersPiece(first uint16, edge int) uint16 { return first + uint16(edge-edgeN) }

// bordersFullSet is a material border with all 12 pieces.
func bordersFullSet(id uint32, first uint16) *materials.Border {
	edges := map[string]uint16{}
	for name, edge := range edgeIndex {
		if edge < edgeCenter {
			edges[name] = bordersPiece(first, edge)
		}
	}
	return &materials.Border{ID: id, Edges: edges}
}

// bordersMaterialBorders are the border sets shared by the custom
// materials.
func bordersMaterialBorders() []*materials.Border {
	optional := bordersFullSet(4, bordersSet4)
	optional.Optional = true
	grouped := bordersFullSet(7, bordersSet7)
	grouped.Group = bordersGroup
	straight := &materials.Border{ID: 5, Edges: map[string]uint16{
		"n": bordersPiece(bordersSet5, edgeN), "e": bordersPiece(bordersSet5, edgeE),
		"s": bordersPiece(bordersSet5, edgeS), "w": bordersPiece(bordersSet5, edgeW),
	}}
	return []*materials.Border{
		bordersFullSet(1, bordersSet1), bordersFullSet(2, bordersSet2), bordersFullSet(3, bordersSet3),
		optional, straight, grouped,
	}
}

// bordersGround is a ground brush of one item.
func bordersGround(name string, item uint16, zOrder int, rules ...materials.BorderRule) *materials.Brush {
	return &materials.Brush{Name: name, Type: materials.TypeGround, ZOrder: zOrder,
		Items: []materials.Chance{{ID: item, Chance: 1}}, Borders: rules}
}

// bordersItemDB knows grounds 100-199 and border pieces 200-399 and the
// foreign border.
func bordersItemDB() *otitems.DB {
	var types []*otitems.Type
	for id := uint16(100); id < 200; id++ {
		types = append(types, &otitems.Type{ID: id, ClientID: id, Flags: otitems.FlagGround, Name: "ground"})
	}
	for id := uint16(200); id < 400; id++ {
		types = append(types, &otitems.Type{ID: id, ClientID: id, TopOrder: borderTopOrder, Name: "border"})
	}
	types = append(types, &otitems.Type{ID: foreignBorder, ClientID: foreignBorder, TopOrder: borderTopOrder, Name: "border"})
	return otitems.NewDB(otitems.ModeOTB, types)
}

// bordersBrushes indexes custom ground brushes over the shared border sets.
func bordersBrushes(t *testing.T, grounds ...*materials.Brush) *Brushes {
	t.Helper()
	materialsDB := materials.New()
	materialsDB.Borders = bordersMaterialBorders()
	materialsDB.Brushes = grounds
	brushes := NewBrushes(bordersItemDB(), materialsDB)
	if len(brushes.Warnings) > 0 {
		t.Fatalf("custom materials gave warnings: %v", brushes.Warnings)
	}
	return brushes
}

// bordersEditor is an editor on an empty map with custom ground brushes.
func bordersEditor(t *testing.T, grounds ...*materials.Brush) *Editor {
	t.Helper()
	brushes := bordersBrushes(t, grounds...)
	e := New(otmap.New(1000, 1000), brushes.items, brushes)
	e.Seed(1)
	return e
}

func bordersDraw(t *testing.T, e *Editor, req DrawRequest) {
	t.Helper()
	if _, err := e.Draw(req); err != nil {
		t.Fatalf("draw %s %q: %v", req.Kind, req.Name, err)
	}
}

// bordersPaint paints one tile with a ground brush.
func bordersPaint(t *testing.T, e *Editor, name string, x, y int) {
	t.Helper()
	bordersDraw(t, e, DrawRequest{Kind: KindGround, Name: name, Points: points(at(x, y))})
}

// bordersField paints a ground rectangle.
func bordersField(t *testing.T, e *Editor, name string, x0, y0, x1, y1 int) {
	t.Helper()
	bordersDraw(t, e, DrawRequest{Kind: KindGround, Name: name, Rect: rect(x0, y0, x1, y1)})
}

func bordersErase(t *testing.T, e *Editor, name string, x, y int) {
	t.Helper()
	bordersDraw(t, e, DrawRequest{Kind: KindGround, Name: name, Points: points(at(x, y)), Erase: true})
}

// bordersExpect checks the items (above the ground) of a tile.
func bordersExpect(t *testing.T, e *Editor, x, y int, want []uint16, why string) {
	t.Helper()
	if got := itemIDs(e.Map.Tile(at(x, y))); !slices.Equal(got, want) {
		t.Errorf("tile (%d,%d) items %v, want %v: %s", x, y, got, want, why)
	}
}

func TestBorderPiecesForNeighborMask(t *testing.T) {
	cases := []struct {
		name string
		mask int
		want []int
	}{
		{"no neighbor", 0, nil},
		{"north side", bitN, []int{edgeN}},
		{"east side", bitE, []int{edgeE}},
		{"south side", bitS, []int{edgeS}},
		{"west side", bitW, []int{edgeW}},
		{"north-west corner", bitNW, []int{edgeCNW}},
		{"north-east corner", bitNE, []int{edgeCNE}},
		{"south-west corner", bitSW, []int{edgeCSW}},
		{"south-east corner", bitSE, []int{edgeCSE}},
		{"side hides corners next to it", bitN | bitNW | bitNE, []int{edgeN}},
		{"north and west make a diagonal", bitN | bitW, []int{edgeDNW}},
		{"north and east make a diagonal", bitN | bitE, []int{edgeDNE}},
		{"south and east make a diagonal", bitS | bitE, []int{edgeDSE}},
		{"south and west make a diagonal", bitS | bitW, []int{edgeDSW}},
		{"diagonal hides its corner", bitN | bitW | bitNW, []int{edgeDNW}},
		{"opposite sides stay straight", bitN | bitS, []int{edgeN, edgeS}},
		{"three sides stay straight", bitN | bitW | bitE, []int{edgeN, edgeE, edgeW}},
		{"four sides stay straight", bitN | bitE | bitS | bitW, []int{edgeN, edgeE, edgeS, edgeW}},
		{"all neighbors give four sides", 0xFF, []int{edgeN, edgeE, edgeS, edgeW}},
		{"two opposite corners", bitNE | bitSW, []int{edgeCNE, edgeCSW}},
		{"four corners", bitNW | bitNE | bitSW | bitSE, []int{edgeCNW, edgeCNE, edgeCSW, edgeCSE}},
		{"diagonal with the far corner", bitS | bitE | bitNW, []int{edgeDSE, edgeCNW}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := borderPieces(tc.mask); !slices.Equal(got, tc.want) {
				t.Errorf("borderPieces(%08b) = %v, want %v", tc.mask, got, tc.want)
			}
		})
	}
}

func TestDiagonalSidesSplitsDiagonalsIntoSides(t *testing.T) {
	cases := []struct {
		name          string
		piece         int
		first, second int
	}{
		{"north-west", edgeDNW, edgeW, edgeN},
		{"north-east", edgeDNE, edgeE, edgeN},
		{"south-west", edgeDSW, edgeS, edgeW},
		{"south-east", edgeDSE, edgeS, edgeE},
		{"a side is no diagonal", edgeN, edgeNone, edgeNone},
		{"a corner is no diagonal", edgeCNW, edgeNone, edgeNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first, second := diagonalSides(tc.piece)
			if first != tc.first || second != tc.second {
				t.Errorf("diagonalSides(%d) = %d, %d, want %d, %d", tc.piece, first, second, tc.first, tc.second)
			}
		})
	}
}

// bordersRelationBrushes are grounds with every kind of border rule, for
// the brush relation tests.
func bordersRelationBrushes(t *testing.T) *Brushes {
	t.Helper()
	mountain := bordersGround("mountain", bordersMountainItem, 10)
	mountain.Optional = &materials.BorderRule{Border: 4}
	friendly := bordersGround("friendly", 101, 0)
	friendly.Friends = []string{"plain"}
	allFriend := bordersGround("all friend", 102, 0)
	allFriend.Friends = []string{"all"}
	hater := bordersGround("hater", 103, 0)
	hater.Friends = []string{"plain"}
	hater.HateFriends = true
	return bordersBrushes(t,
		bordersGround("plain", bordersSandItem, 0),
		bordersGround("high", bordersGrassItem, 10, materials.BorderRule{Align: "outer", Border: 1}),
		bordersGround("inner to high", 104, 0, materials.BorderRule{Align: "inner", To: "high", Border: 3}),
		bordersGround("inner to plain", 105, 0, materials.BorderRule{Align: "inner", To: "plain", Border: 3}),
		bordersGround("inner all", 106, 20, materials.BorderRule{Align: "inner", Border: 3}),
		bordersGround("inner zilch", 107, 0, materials.BorderRule{Align: "inner", To: "none", Border: 3}),
		bordersGround("outer zilch", 108, 0, materials.BorderRule{Align: "outer", To: "none", Border: 2}),
		bordersGround("outer to plain", 109, 10, materials.BorderRule{Align: "outer", To: "plain", Border: 2}),
		mountain, friendly, allFriend, hater,
	)
}

func TestGroundBrushHasOuterBorder(t *testing.T) {
	brushes := bordersRelationBrushes(t)
	cases := []struct {
		name string
		want bool
	}{
		{"high", true},           // outer rule toward all
		{"outer to plain", true}, // outer rule toward one ground
		{"mountain", true},       // an optional border counts as outer
		{"plain", false},
		{"inner all", false},
		{"outer zilch", false}, // only toward empty tiles
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := brushes.Ground(tc.name).hasOuterBorder(); got != tc.want {
				t.Errorf("%s hasOuterBorder = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestGroundBrushHasOuterZilchBorder(t *testing.T) {
	brushes := bordersRelationBrushes(t)
	cases := []struct {
		name string
		want bool
	}{
		{"outer zilch", true},
		{"mountain", true}, // an optional border also lies on empty tiles
		{"high", false},
		{"inner zilch", false},
		{"plain", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := brushes.Ground(tc.name).hasOuterZilchBorder(); got != tc.want {
				t.Errorf("%s hasOuterZilchBorder = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestGroundBrushFriendOf(t *testing.T) {
	brushes := bordersRelationBrushes(t)
	cases := []struct {
		name, brush, other string
		want               bool
	}{
		{"listed friend", "friendly", "plain", true},
		{"unlisted ground", "friendly", "high", false},
		{"friend of all", "all friend", "high", true},
		{"hated listed ground", "hater", "plain", false},
		{"unlisted ground of a hater", "hater", "high", true},
		{"no friends", "plain", "friendly", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := brushes.Ground(tc.brush).friendOf(brushes.Ground(tc.other)); got != tc.want {
				t.Errorf("%s.friendOf(%s) = %v, want %v", tc.brush, tc.other, got, tc.want)
			}
		})
	}
}

func TestBrushToPicksBorderBlock(t *testing.T) {
	brushes := bordersRelationBrushes(t)
	ground := func(name string) *GroundBrush {
		if name == "" {
			return nil
		}
		return brushes.Ground(name)
	}
	firstBlockOf := func(name string) *borderBlock { return brushes.Ground(name).borders[0] }
	cases := []struct {
		name          string
		first, second string // "" is an empty tile
		want          *borderBlock
	}{
		{"higher ground puts its outer border on lower", "plain", "high", firstBlockOf("high")},
		{"lower ground puts no border on higher", "high", "plain", nil},
		{"inner border toward the neighbor wins over its outer", "inner to high", "high", firstBlockOf("inner to high")},
		{"inner border toward another ground falls back to outer", "inner to plain", "high", firstBlockOf("high")},
		{"higher ground with inner border borders itself", "inner all", "plain", firstBlockOf("inner all")},
		{"outer border aimed at this ground", "plain", "outer to plain", firstBlockOf("outer to plain")},
		{"outer border aimed at another ground", "inner to high", "outer to plain", nil},
		{"inner zilch border toward empty tile", "inner zilch", "", firstBlockOf("inner zilch")},
		{"no inner zilch border toward empty tile", "plain", "", nil},
		{"outer zilch border on empty tile", "", "outer zilch", firstBlockOf("outer zilch")},
		{"no outer zilch border on empty tile", "", "high", nil},
		{"two empty tiles", "", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := brushTo(ground(tc.first), ground(tc.second)); got != tc.want {
				t.Errorf("brushTo(%q, %q) = %+v, want %+v", tc.first, tc.second, got, tc.want)
			}
		})
	}
}

func TestGrassOuterBorderLiesOnSandAround(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 50)
	ground := e.Map.Tile(at(50, 50)).Ground.ID
	if ground != grassA && ground != grassB {
		t.Fatalf("ground at (50,50) = %d, want a grass item %d or %d", ground, grassA, grassB)
	}
	bordersExpect(t, e, 50, 50, nil, "grass borders lie on the sand, not on the grass")
	want := []struct {
		x, y  int
		piece uint16
	}{
		{50, 49, borderS}, {50, 51, borderN}, {49, 50, borderE}, {51, 50, borderW},
		{49, 49, borderCSE}, {51, 49, borderCSW}, {49, 51, borderCNE}, {51, 51, borderCNW},
	}
	for _, tile := range want {
		bordersExpect(t, e, tile.x, tile.y, []uint16{tile.piece}, "sand next to grass gets the grass outer piece facing it")
	}
	bordersExpect(t, e, 50, 48, nil, "sand two tiles away is not bordered")
}

func TestGrassNeighborsPlaceNoBorderBetweenThem(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 50)
	bordersPaint(t, e, "grass", 51, 50)
	bordersExpect(t, e, 51, 50, nil, "the former border tile is grass now")
	bordersExpect(t, e, 50, 50, nil, "grass next to grass gets no border")
	bordersExpect(t, e, 50, 49, []uint16{borderS}, "the south side hides the south-east corner")
	bordersExpect(t, e, 51, 49, []uint16{borderS}, "the south side hides the south-west corner")
	bordersExpect(t, e, 52, 49, []uint16{borderCSW}, "only the south-west neighbor is grass")
	bordersExpect(t, e, 52, 50, []uint16{borderW}, "grass is west of it")
}

func TestErasingGroundRemovesBordersAround(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 50)
	bordersErase(t, e, "grass", 50, 50)
	if tile := e.Map.Tile(at(50, 50)); tile != nil && tile.Ground.ID != 0 {
		t.Errorf("erased tile keeps ground %d", tile.Ground.ID)
	}
	for y := 49; y <= 51; y++ {
		for x := 49; x <= 51; x++ {
			bordersExpect(t, e, x, y, nil, "no grass is left to border")
		}
	}
}

func TestRepaintingBorderedTileDropsItsBorder(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 50)
	bordersPaint(t, e, "grass", 50, 49)
	bordersExpect(t, e, 50, 49, nil, "the bordered sand tile became grass")
	bordersExpect(t, e, 50, 48, []uint16{borderS}, "the border moved one tile north")
}

func TestSandSurroundedByGrassGetsFourSides(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "grass", 45, 45, 55, 55)
	bordersPaint(t, e, "sand", 50, 50)
	bordersExpect(t, e, 50, 50, []uint16{borderW, borderS, borderE, borderN}, "higher z grass borders the sand on every side")
	bordersExpect(t, e, 50, 49, nil, "lower z sand puts no border on grass")
}

func TestLowerZOrderGroundPutsNoBorderOnHigher(t *testing.T) {
	// The grass keeps its outer border but the sand is above it.
	e := bordersEditor(t,
		bordersGround("grass", bordersGrassItem, 10, materials.BorderRule{Align: "outer", Border: 1}),
		bordersGround("sand", bordersSandItem, 20),
	)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 50)
	for y := 49; y <= 51; y++ {
		for x := 49; x <= 51; x++ {
			bordersExpect(t, e, x, y, nil, "grass is below the sand, so neither borders the other")
		}
	}
}

func TestEqualZOrderGroundsDoNotBorderEachOther(t *testing.T) {
	e := bordersEditor(t,
		bordersGround("grass", bordersGrassItem, 10, materials.BorderRule{Align: "outer", Border: 1}),
		bordersGround("clay", bordersClayItem, 10, materials.BorderRule{Align: "outer", Border: 2}),
	)
	bordersField(t, e, "clay", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 50)
	bordersExpect(t, e, 50, 49, nil, "equal z order: the grass is not above the clay")
	bordersExpect(t, e, 50, 50, nil, "equal z order: the clay is not above the grass")
}

func TestBordersStackLowestZOrderFirst(t *testing.T) {
	e := bordersEditor(t,
		bordersGround("sand", bordersSandItem, 0),
		bordersGround("grass", bordersGrassItem, 10, materials.BorderRule{Align: "outer", Border: 1}),
		bordersGround("stone", bordersStoneItem, 20, materials.BorderRule{Align: "outer", Border: 2}),
	)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	// The higher stone is the first neighbor read, the grass the second.
	bordersPaint(t, e, "stone", 49, 50)
	bordersPaint(t, e, "grass", 51, 50)
	want := []uint16{bordersPiece(bordersSet1, edgeE), bordersPiece(bordersSet2, edgeW)}
	bordersExpect(t, e, 50, 50, want, "the grass border (z 10) goes below the stone border (z 20)")
}

func TestNeighborsSharingABorderMergeIntoOneShape(t *testing.T) {
	e := bordersEditor(t,
		bordersGround("sand", bordersSandItem, 0),
		bordersGround("grass", bordersGrassItem, 10, materials.BorderRule{Align: "outer", Border: 1}),
		bordersGround("moss", bordersMossItem, 12, materials.BorderRule{Align: "outer", Border: 1}),
	)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 49)
	bordersPaint(t, e, "moss", 49, 50)
	bordersExpect(t, e, 50, 50, []uint16{bordersPiece(bordersSet1, edgeDNW)},
		"grass north and moss west use the same border, so they make one diagonal")
}

func TestMissingDiagonalIsBuiltFromItsSides(t *testing.T) {
	e := bordersEditor(t,
		bordersGround("sand", bordersSandItem, 0),
		bordersGround("grass", bordersGrassItem, 10, materials.BorderRule{Align: "outer", Border: 5}),
	)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 49)
	bordersPaint(t, e, "grass", 49, 50)
	want := []uint16{bordersPiece(bordersSet5, edgeN), bordersPiece(bordersSet5, edgeW)}
	bordersExpect(t, e, 50, 50, want, "border 5 has no north-west diagonal, so its north and west sides stand in")
}

func TestInnerBorderLiesOnTheGroundItself(t *testing.T) {
	e := bordersEditor(t,
		bordersGround("sand", bordersSandItem, 5),
		bordersGround("water", bordersWaterItem, 0, materials.BorderRule{Align: "inner", Border: 3}),
	)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "water", 50, 50)
	want := []uint16{
		bordersPiece(bordersSet3, edgeW), bordersPiece(bordersSet3, edgeS),
		bordersPiece(bordersSet3, edgeE), bordersPiece(bordersSet3, edgeN),
	}
	bordersExpect(t, e, 50, 50, want, "the inner border of water lies on the water tile facing the sand")
	bordersExpect(t, e, 50, 49, nil, "an inner border puts nothing on the neighbor")
}

func TestInnerZilchBorderFacesEmptyTiles(t *testing.T) {
	e := bordersEditor(t,
		bordersGround("lava", bordersLavaItem, 0, materials.BorderRule{Align: "inner", To: "none", Border: 3}),
	)
	bordersPaint(t, e, "lava", 50, 50)
	want := []uint16{
		bordersPiece(bordersSet3, edgeW), bordersPiece(bordersSet3, edgeS),
		bordersPiece(bordersSet3, edgeE), bordersPiece(bordersSet3, edgeN),
	}
	bordersExpect(t, e, 50, 50, want, "lava alone on the map is bordered toward the void on all sides")
	if tile := e.Map.Tile(at(50, 49)); tile != nil {
		t.Errorf("empty tile north of lava was created with items %v; an inner border needs nothing there", itemIDs(tile))
	}
}

func TestOuterZilchBorderLiesOnEmptyTiles(t *testing.T) {
	e := bordersEditor(t,
		bordersGround("shore", bordersShoreItem, 0, materials.BorderRule{Align: "outer", To: "none", Border: 2}),
	)
	bordersPaint(t, e, "shore", 50, 50)
	bordersExpect(t, e, 50, 50, nil, "an outer border does not lie on the shore itself")
	bordersExpect(t, e, 50, 49, []uint16{bordersPiece(bordersSet2, edgeS)}, "empty tile north of the shore gets the south piece")
	bordersExpect(t, e, 49, 49, []uint16{bordersPiece(bordersSet2, edgeCSE)}, "empty tile north-west of the shore gets a corner")
	if tile := e.Map.Tile(at(50, 49)); tile == nil || tile.Ground.ID != 0 {
		t.Errorf("the border tile should exist without ground, got %+v", tile)
	}
}

func TestZilchRuleWithoutBorderRunsItsSpecificCases(t *testing.T) {
	// Lava has no border toward the void, only a specific case that swaps
	// the north piece of the stone border.
	e := bordersEditor(t,
		bordersGround("stone", bordersStoneItem, 20, materials.BorderRule{Align: "outer", Border: 2}),
		bordersGround("lava", bordersLavaItem, 0, materials.BorderRule{Align: "inner", To: "none",
			Specific: []materials.SpecificCase{{
				Conditions: []materials.Condition{{Kind: "border", Border: 2, Edge: "n"}},
				Actions:    []materials.Action{{Kind: "replaceBorder", Border: 2, Edge: "n", With: bordersAltPiece}},
			}}}),
	)
	bordersPaint(t, e, "lava", 50, 50)
	bordersPaint(t, e, "stone", 50, 49)
	bordersExpect(t, e, 50, 50, []uint16{bordersAltPiece}, "the void-facing rule replaced the stone north piece")
}

func TestGroundAtMapCornerBordersOnlyExistingNeighbors(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "sand", 0, 0, 3, 3)
	bordersPaint(t, e, "grass", 0, 0)
	bordersExpect(t, e, 1, 0, []uint16{borderW}, "grass is west")
	bordersExpect(t, e, 0, 1, []uint16{borderN}, "grass is north")
	bordersExpect(t, e, 1, 1, []uint16{borderCNW}, "grass is north-west")
}

func TestFriendGroundsPlaceNoBorderBetweenThem(t *testing.T) {
	grass := bordersGround("grass", bordersGrassItem, 10, materials.BorderRule{Align: "outer", Border: 1})
	grass.Friends = []string{"sand"}
	e := bordersEditor(t, grass, bordersGround("sand", bordersSandItem, 0))
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 50)
	bordersExpect(t, e, 50, 49, nil, "grass is a friend of sand")
	bordersExpect(t, e, 49, 49, nil, "grass is a friend of sand")
}

func TestHateFriendsBordersOnlyListedGrounds(t *testing.T) {
	grass := bordersGround("grass", bordersGrassItem, 10, materials.BorderRule{Align: "outer", Border: 1})
	grass.Friends = []string{"sand"}
	grass.HateFriends = true
	e := bordersEditor(t, grass, bordersGround("sand", bordersSandItem, 0), bordersGround("dirt", bordersDirtItem, 0))
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersField(t, e, "dirt", 65, 45, 75, 55)
	bordersPaint(t, e, "grass", 50, 50)
	bordersPaint(t, e, "grass", 70, 50)
	bordersExpect(t, e, 50, 49, []uint16{bordersPiece(bordersSet1, edgeS)}, "sand is listed in the hated friends: bordered")
	bordersExpect(t, e, 70, 49, nil, "dirt is not listed in the hated friends: a friend")
}

func TestSpecificCaseRewritesPlannedBorder(t *testing.T) {
	e := bordersSpecificEditor(t)
	bordersField(t, e, "sand", 40, 40, 60, 60)
	bordersPaint(t, e, "grass", 49, 49)
	bordersPaint(t, e, "grass", 51, 51)
	want := []uint16{bordersPiece(bordersSet1, edgeCSE), bordersReplaced}
	bordersExpect(t, e, 50, 50, want, "both corners are planned, so the north-west corner is replaced")
}

func TestSpecificCaseReplacementIsUndoneWhenConditionsStopMatching(t *testing.T) {
	e := bordersSpecificEditor(t)
	bordersField(t, e, "sand", 40, 40, 60, 60)
	bordersPaint(t, e, "grass", 49, 49)
	bordersPaint(t, e, "grass", 51, 51)
	bordersErase(t, e, "grass", 51, 51)
	bordersExpect(t, e, 50, 50, []uint16{bordersPiece(bordersSet1, edgeCNW)},
		"with one corner left the replacement goes and the plain corner comes back")
}

// bordersSpecificEditor has grass whose outer border replaces its north-west
// corner when the south-east corner is on the same tile.
func bordersSpecificEditor(t *testing.T) *Editor {
	t.Helper()
	return bordersEditor(t,
		bordersGround("sand", bordersSandItem, 0),
		bordersGround("grass", bordersGrassItem, 10, materials.BorderRule{Align: "outer", Border: 1,
			Specific: []materials.SpecificCase{{
				Conditions: []materials.Condition{{Kind: "border", Border: 1, Edge: "cnw"}, {Kind: "border", Border: 1, Edge: "cse"}},
				Actions:    []materials.Action{{Kind: "replaceBorder", Border: 1, Edge: "cnw", With: bordersReplaced}},
			}}}),
	)
}

func TestApplySpecificCase(t *testing.T) {
	e := bordersEditor(t, bordersGround("sand", bordersSandItem, 0))
	north, east := bordersPiece(bordersSet1, edgeN), bordersPiece(bordersSet1, edgeE)
	cornerSE := bordersPiece(bordersSet1, edgeCSE)
	groupNorth, groupSouth := bordersPiece(bordersSet7, edgeN), bordersPiece(bordersSet7, edgeS)
	cases := []struct {
		name     string
		ids      []uint16
		specific specificCase
		want     []uint16
	}{
		{"replaces when all conditions match", []uint16{north, cornerSE},
			specificCase{match: []uint16{north, cornerSE}, replace: north, with: bordersReplaced}, []uint16{bordersReplaced, cornerSE}},
		{"keeps the stack when a condition is missing", []uint16{north},
			specificCase{match: []uint16{north, cornerSE}, replace: north, with: bordersReplaced}, []uint16{north}},
		{"a case without conditions does nothing", []uint16{north},
			specificCase{replace: north, with: bordersReplaced}, []uint16{north}},
		{"replacing an item not in the stack does nothing", []uint16{north},
			specificCase{match: []uint16{north}, replace: east, with: bordersReplaced}, []uint16{north}},
		{"delete borders removes the matched pieces", []uint16{north, east, cornerSE},
			specificCase{match: []uint16{north, cornerSE}, deleteAll: true}, []uint16{east}},
		{"group condition matches a piece of the group and edge", []uint16{groupNorth},
			specificCase{match: []uint16{bordersGroup}, matchGroup: bordersGroup, groupAlign: edgeN, replace: groupNorth, with: bordersReplaced},
			[]uint16{bordersReplaced}},
		{"group condition ignores other edges of the group", []uint16{groupSouth},
			specificCase{match: []uint16{bordersGroup}, matchGroup: bordersGroup, groupAlign: edgeN, replace: groupSouth, with: bordersReplaced},
			[]uint16{groupSouth}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := e.applySpecific(slices.Clone(tc.ids), tc.specific)
			if !slices.Equal(got, tc.want) {
				t.Errorf("applySpecific(%v) = %v, want %v", tc.ids, got, tc.want)
			}
		})
	}
}

func TestRepaintingSameGroundKeepsHandPlacedBorder(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersDraw(t, e, DrawRequest{Kind: KindRaw, Item: borderN, Points: points(at(51, 50))})
	bordersPaint(t, e, "sand", 50, 50)
	bordersExpect(t, e, 51, 50, []uint16{borderN}, "the border plan around did not change, so the hand-placed piece stays")
}

func TestAutomagicKeepsForeignBorderAboveNewPiece(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersDraw(t, e, DrawRequest{Kind: KindRaw, Item: foreignBorder, Points: points(at(50, 49))})
	bordersPaint(t, e, "grass", 50, 50)
	bordersExpect(t, e, 50, 49, []uint16{borderS, foreignBorder}, "the new grass piece goes below the border no material knows")
}

func TestErasingGroundRemovesOnlyItsOwnBorderPiece(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersDraw(t, e, DrawRequest{Kind: KindRaw, Item: foreignBorder, Points: points(at(50, 49))})
	bordersPaint(t, e, "grass", 50, 50)
	bordersErase(t, e, "grass", 50, 50)
	bordersExpect(t, e, 50, 49, []uint16{foreignBorder}, "only the grass piece goes; the foreign border stays")
}

func TestBorderizeSelectionReplacesStrayMaterialBorders(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersDraw(t, e, DrawRequest{Kind: KindRaw, Item: borderN, Points: points(at(51, 50))})
	bordersDraw(t, e, DrawRequest{Kind: KindRaw, Item: foreignBorder, Points: points(at(50, 49))})
	e.SelectRect(Rect{From: at(47, 47), To: at(53, 53)}, FloorsCurrent, false, false, true)
	if changed := e.BorderizeSelection(); changed != 1 {
		t.Errorf("BorderizeSelection changed %d tiles, want 1 (only the stray piece)", changed)
	}
	bordersExpect(t, e, 51, 50, nil, "a material piece no neighbor asks for is removed")
	bordersExpect(t, e, 50, 49, []uint16{foreignBorder}, "a border no material knows is kept")
}

func TestAutomagicOffPlacesNoBorders(t *testing.T) {
	e := testEditor(t)
	e.Settings.Automagic = false
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 50)
	for y := 49; y <= 51; y++ {
		for x := 49; x <= 51; x++ {
			bordersExpect(t, e, x, y, nil, "automagic is off")
		}
	}
}

func TestBorderizeMapAddsBordersMissingAfterAutomagicOff(t *testing.T) {
	e := testEditor(t)
	e.Settings.Automagic = false
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 50)
	e.Settings.Automagic = true
	if changed := e.BorderizeMap(); changed != 8 {
		t.Errorf("BorderizeMap changed %d tiles, want 8 (the ring around the grass)", changed)
	}
	bordersExpect(t, e, 50, 49, []uint16{borderS}, "borderize map adds the missing grass border")
	bordersExpect(t, e, 51, 51, []uint16{borderCNW}, "borderize map adds the missing grass corner")
}

func TestReborderKeepsSelectionOfReusedPiece(t *testing.T) {
	e := testEditor(t)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "grass", 50, 50)
	e.Map.Tile(at(50, 49)).Items[0].Selected = true
	// A grass corner north-east of the tile changes its plan.
	bordersPaint(t, e, "grass", 51, 48)
	tile := e.Map.Tile(at(50, 49))
	bordersExpect(t, e, 50, 49, []uint16{borderCNE, borderS}, "the corner joins the south piece")
	if index := slices.IndexFunc(tile.Items, func(item otmap.Item) bool { return item.ID == borderS }); index < 0 || !tile.Items[index].Selected {
		t.Errorf("the south piece was kept by the new plan, so it should stay selected: %+v", tile.Items)
	}
}

// bordersMountainEditor has sand and a mountain with an outer border (2)
// and an optional border (4).
func bordersMountainEditor(t *testing.T, solo bool, sandFriends ...string) *Editor {
	t.Helper()
	mountain := bordersGround("mountain", bordersMountainItem, 10, materials.BorderRule{Align: "outer", Border: 2})
	mountain.Optional = &materials.BorderRule{Border: 4}
	mountain.SoloOptional = solo
	sand := bordersGround("sand", bordersSandItem, 0)
	sand.Friends = sandFriends
	return bordersEditor(t, sand, mountain)
}

func bordersOptional(t *testing.T, e *Editor, x, y int, erase bool) {
	t.Helper()
	bordersDraw(t, e, DrawRequest{Kind: KindOptionalBorder, Points: points(at(x, y)), Erase: erase})
}

func bordersHasOptionalState(e *Editor, x, y int) bool {
	tile := e.Map.Tile(at(x, y))
	return tile != nil && tile.State&otmap.StateOptionalBorder != 0
}

func TestOptionalBorderToolAddsMountainBorderAbove(t *testing.T) {
	e := bordersMountainEditor(t, false)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "mountain", 50, 50)
	bordersExpect(t, e, 50, 49, []uint16{bordersPiece(bordersSet2, edgeS)}, "without the tool only the outer border lies there")
	bordersOptional(t, e, 50, 49, false)
	if !bordersHasOptionalState(e, 50, 49) {
		t.Errorf("the tool should set StateOptionalBorder on the tile")
	}
	want := []uint16{bordersPiece(bordersSet2, edgeS), bordersPiece(bordersSet4, edgeS)}
	bordersExpect(t, e, 50, 49, want, "the optional border goes above all other borders")
}

func TestOptionalBorderToolSkipsMountainTiles(t *testing.T) {
	e := bordersMountainEditor(t, false)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "mountain", 50, 50)
	bordersOptional(t, e, 50, 50, false)
	if bordersHasOptionalState(e, 50, 50) {
		t.Errorf("a mountain tile must not be marked for its own optional border")
	}
	bordersExpect(t, e, 50, 50, nil, "nothing is put on the mountain")
}

func TestErasingOptionalBorderClearsMark(t *testing.T) {
	e := bordersMountainEditor(t, false)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "mountain", 50, 50)
	bordersOptional(t, e, 50, 49, false)
	bordersOptional(t, e, 50, 49, true)
	if bordersHasOptionalState(e, 50, 49) {
		t.Errorf("erasing should clear StateOptionalBorder")
	}
}

func TestErasingOptionalBorderRemovesMountainPieces(t *testing.T) {
	e := bordersMountainEditor(t, false)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "mountain", 50, 50)
	bordersOptional(t, e, 50, 49, false)
	bordersOptional(t, e, 50, 49, true)
	bordersExpect(t, e, 50, 49, []uint16{bordersPiece(bordersSet2, edgeS)}, "only the outer border is left")
}

func TestErasingOptionalBorderOnUnmarkedTileChangesNothing(t *testing.T) {
	e := bordersMountainEditor(t, false)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "mountain", 50, 50)
	changed, err := e.Draw(DrawRequest{Kind: KindOptionalBorder, Points: points(at(50, 49)), Erase: true})
	if err != nil || changed != 0 {
		t.Errorf("erasing an unmarked tile = %d, %v; want 0 changes", changed, err)
	}
	bordersExpect(t, e, 50, 49, []uint16{bordersPiece(bordersSet2, edgeS)}, "the outer border stays")
}

func TestSoloOptionalMountainPlacesOnlyOptionalBorder(t *testing.T) {
	e := bordersMountainEditor(t, true)
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "mountain", 50, 50)
	bordersExpect(t, e, 50, 51, []uint16{bordersPiece(bordersSet2, edgeN)}, "unmarked tiles still get the outer border")
	bordersOptional(t, e, 50, 49, false)
	bordersExpect(t, e, 50, 49, []uint16{bordersPiece(bordersSet4, edgeS)}, "a solo optional border replaces the outer border")
}

func TestFriendOfMountainGetsOnlyOptionalBorder(t *testing.T) {
	e := bordersMountainEditor(t, false, "mountain")
	bordersField(t, e, "sand", 45, 45, 55, 55)
	bordersPaint(t, e, "mountain", 50, 50)
	bordersExpect(t, e, 50, 51, nil, "sand is a friend of the mountain: no outer border")
	bordersOptional(t, e, 50, 49, false)
	bordersExpect(t, e, 50, 49, []uint16{bordersPiece(bordersSet4, edgeS)}, "friends still get the optional border when marked")
}

func TestOptionalBorderOnEmptyTileNextToMountain(t *testing.T) {
	e := bordersMountainEditor(t, false)
	bordersPaint(t, e, "mountain", 50, 50)
	if tile := e.Map.Tile(at(50, 49)); tile != nil {
		t.Fatalf("empty tile next to the mountain got items %v without the tool", itemIDs(tile))
	}
	bordersOptional(t, e, 50, 49, false)
	if !bordersHasOptionalState(e, 50, 49) {
		t.Errorf("the tool should mark the empty tile")
	}
	bordersExpect(t, e, 50, 49, []uint16{bordersPiece(bordersSet4, edgeS)}, "the optional border also lies on empty tiles")
}

func TestBorderizeDropsOptionalMarkOnEmptyTileNextToGroundWithoutOptionalBorder(t *testing.T) {
	e := bordersEditor(t,
		bordersGround("shore", bordersShoreItem, 0, materials.BorderRule{Align: "outer", To: "none", Border: 2}),
	)
	bordersPaint(t, e, "shore", 50, 50)
	e.Map.Tile(at(50, 49)).State |= otmap.StateOptionalBorder
	e.SelectRect(Rect{From: at(50, 49), To: at(50, 49)}, FloorsCurrent, false, false, true)
	if changed := e.BorderizeSelection(); changed != 1 {
		t.Errorf("BorderizeSelection changed %d tiles, want 1 (the mark is dropped)", changed)
	}
	if bordersHasOptionalState(e, 50, 49) {
		t.Errorf("the shore has no optional border, so the mark should be dropped")
	}
	bordersExpect(t, e, 50, 49, []uint16{bordersPiece(bordersSet2, edgeS)}, "the shore void border stays")
}

func TestOptionalMarkOnEmptyTileIsDroppedNextToGroundWithoutOptionalBorder(t *testing.T) {
	e := bordersEditor(t,
		bordersGround("shore", bordersShoreItem, 0, materials.BorderRule{Align: "outer", To: "none", Border: 2}),
	)
	bordersPaint(t, e, "shore", 50, 50)
	bordersOptional(t, e, 50, 49, false)
	if bordersHasOptionalState(e, 50, 49) {
		t.Errorf("the shore has no optional border, so the mark should be dropped")
	}
	bordersExpect(t, e, 50, 49, []uint16{bordersPiece(bordersSet2, edgeS)}, "the shore void border stays")
}

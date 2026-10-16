package mapedit

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/core/otitems"
)

// brushesTestEditor is testEditor with its materials changed by mutate and
// the brushes rebuilt from them.
func brushesTestEditor(t *testing.T, mutate func(materialsDB *materials.Materials)) *Editor {
	t.Helper()
	e := testEditor(t)
	mutate(e.Brushes.Materials)
	e.SetBrushes(NewBrushes(e.Items, e.Brushes.Materials))
	return e
}

// brushesAddBrush is a mutate function for brushesTestEditor that appends brushes.
func brushesAddBrush(brushes ...*materials.Brush) func(materialsDB *materials.Materials) {
	return func(materialsDB *materials.Materials) {
		materialsDB.Brushes = append(materialsDB.Brushes, brushes...)
	}
}

func TestChanceListSkipsZeroIDsAndCountsNegativeChancesAsZero(t *testing.T) {
	list := newChanceList([]materials.Chance{{ID: 0, Chance: 5}, {ID: grassA, Chance: 3}, {ID: grassB, Chance: -4}, {ID: sandID, Chance: 2}})
	if !slices.Equal(list.ids, []uint16{grassA, grassB, sandID}) {
		t.Errorf("ids = %v, want [%d %d %d] (id 0 skipped)", list.ids, grassA, grassB, sandID)
	}
	if !slices.Equal(list.cum, []int{3, 3, 5}) {
		t.Errorf("cumulative = %v, want [3 3 5] (negative chance counts as 0)", list.cum)
	}
	if list.total != 5 {
		t.Errorf("total = %d, want 5", list.total)
	}
}

func TestChanceListPickOfEmptyListIsZero(t *testing.T) {
	list := newChanceList(nil)
	if got := list.pick(rand.New(rand.NewPCG(1, 1))); got != 0 {
		t.Errorf("pick = %d, want 0 for an empty list", got)
	}
}

func TestChanceListPickWithoutChancesReturnsFirstItem(t *testing.T) {
	list := newChanceList([]materials.Chance{{ID: grassA}, {ID: grassB}})
	rng := rand.New(rand.NewPCG(1, 1))
	for range 20 {
		if got := list.pick(rng); got != grassA {
			t.Fatalf("pick = %d, want the first item %d when all chances are 0", got, grassA)
		}
	}
}

func TestChanceListPickNeverReturnsItemWithZeroChance(t *testing.T) {
	list := newChanceList([]materials.Chance{{ID: grassA, Chance: 0}, {ID: grassB, Chance: 5}})
	rng := rand.New(rand.NewPCG(1, 1))
	for range 50 {
		if got := list.pick(rng); got != grassB {
			t.Fatalf("pick = %d, want %d (the only item with a chance)", got, grassB)
		}
	}
}

func TestChanceListPickFollowsWeights(t *testing.T) {
	list := newChanceList([]materials.Chance{{ID: grassA, Chance: 1}, {ID: grassB, Chance: 9}})
	rng := rand.New(rand.NewPCG(1, 1))
	counts := map[uint16]int{}
	for range 1000 {
		counts[list.pick(rng)]++
	}
	if counts[grassA] == 0 || counts[grassB] <= counts[grassA]*3 {
		t.Errorf("picks = %v, want both items and %d about 9 times as often as %d", counts, grassB, grassA)
	}
	if counts[grassA]+counts[grassB] != 1000 {
		t.Errorf("picks = %v, want only items of the list", counts)
	}
}

func TestChanceListHas(t *testing.T) {
	list := newChanceList([]materials.Chance{{ID: grassA, Chance: 1}, {ID: grassB, Chance: 0}})
	for _, testCase := range []struct {
		id   uint16
		want bool
	}{{grassA, true}, {grassB, true}, {sandID, false}, {0, false}} {
		if got := list.has(testCase.id); got != testCase.want {
			t.Errorf("has(%d) = %v, want %v", testCase.id, got, testCase.want)
		}
	}
}

func TestNewBrushesAcceptsNilMaterials(t *testing.T) {
	brushes := NewBrushes(otitems.NewDB(otitems.ModeOTB, nil), nil)
	if brushes.Materials == nil {
		t.Fatal("Materials is nil, want an empty material set")
	}
	if brushes.Ground("grass") != nil || brushes.Look("grass") != 0 || len(brushes.Warnings) != 0 {
		t.Errorf("brushes from nil materials are not empty: %+v", brushes)
	}
}

func TestNewBrushesBuildsEveryBrushType(t *testing.T) {
	e := testEditor(t)
	brushes := e.Brushes
	for _, testCase := range []struct {
		kind  string
		found bool
		name  string
	}{
		{"ground grass", brushes.Ground("grass") != nil, "grass"},
		{"ground sand", brushes.Ground("sand") != nil, "sand"},
		{"wall", brushes.Wall("stone wall") != nil, "stone wall"},
		{"carpet", brushes.Carpet("rug") != nil, "rug"},
		{"table", brushes.Table("desk") != nil, "desk"},
		{"doodad trees", brushes.Doodad("trees") != nil, "trees"},
		{"doodad pit", brushes.Doodad("pit") != nil, "pit"},
	} {
		if !testCase.found {
			t.Errorf("%s brush %q was not built", testCase.kind, testCase.name)
		}
	}
	if got := brushes.Wall("stone wall").Name; got != "stone wall" {
		t.Errorf("wall name = %q, want the material name", got)
	}
}

func TestBrushLookupsIgnoreCaseAndSpaces(t *testing.T) {
	brushes := testEditor(t).Brushes
	for _, testCase := range []struct {
		kind  string
		found bool
	}{
		{"ground", brushes.Ground("  GRASS ") != nil},
		{"wall", brushes.Wall("Stone Wall") != nil},
		{"carpet", brushes.Carpet("RUG") != nil},
		{"table", brushes.Table(" Desk") != nil},
		{"doodad", brushes.Doodad("TREES ") != nil},
	} {
		if !testCase.found {
			t.Errorf("%s lookup is case or space sensitive", testCase.kind)
		}
	}
}

func TestBrushLookupsOfUnknownNamesReturnNil(t *testing.T) {
	brushes := testEditor(t).Brushes
	if brushes.Ground("lava") != nil || brushes.Wall("lava") != nil || brushes.Carpet("lava") != nil ||
		brushes.Table("lava") != nil || brushes.Doodad("lava") != nil {
		t.Error("a lookup of an unknown brush returned a brush")
	}
	if got := brushes.Look("lava"); got != 0 {
		t.Errorf("Look(unknown) = %d, want 0", got)
	}
}

func TestBrushLookupsDoNotMixKinds(t *testing.T) {
	brushes := testEditor(t).Brushes
	if brushes.Wall("grass") != nil || brushes.Ground("stone wall") != nil || brushes.Carpet("desk") != nil ||
		brushes.Table("rug") != nil || brushes.Doodad("grass") != nil {
		t.Error("a lookup returned a brush of another kind")
	}
}

func TestWallMaterialWithoutWallPartsBuildsNoWallBrush(t *testing.T) {
	e := brushesTestEditor(t, brushesAddBrush(&materials.Brush{Name: "empty wall", Type: materials.TypeWall}))
	if e.Brushes.Wall("empty wall") != nil {
		t.Error("a wall material without parts built a wall brush")
	}
}

func TestBrushWithAlternativesIsAlsoDoodad(t *testing.T) {
	e := brushesTestEditor(t, brushesAddBrush(&materials.Brush{
		Name: "mossy", Type: materials.TypeGround, Items: []materials.Chance{{ID: 720, Chance: 1}},
		Alternatives: []materials.Alternative{{Singles: []materials.Single{{Chance: 1, Item: materials.ItemSpec{ID: 721}}}}},
	}))
	ground, doodad := e.Brushes.Ground("mossy"), e.Brushes.Doodad("mossy")
	if ground == nil || doodad == nil {
		t.Fatalf("ground = %v, doodad = %v, want both brushes for a ground with alternatives", ground, doodad)
	}
	if ground.id == doodad.id {
		t.Errorf("ground and doodad share id %d, want distinct ids", ground.id)
	}
}

func TestNewBrushesMarksMetaItems(t *testing.T) {
	e := brushesTestEditor(t, func(materialsDB *materials.Materials) { materialsDB.MetaItems = []uint16{stoneID, 730} })
	for _, id := range []uint16{stoneID, 730} {
		if itemType := e.Items.Get(id); itemType == nil || !itemType.Flags.Has(otitems.FlagMeta) {
			t.Errorf("item %d is not marked meta: %+v", id, itemType)
		}
	}
}

func TestBrushesGroundOfReturnsGroundBrushOfItem(t *testing.T) {
	brushes := testEditor(t).Brushes
	for _, testCase := range []struct {
		id   uint16
		want string
	}{{grassA, "grass"}, {grassB, "grass"}, {sandID, "sand"}, {borderN, ""}, {bagID, ""}, {9999, ""}} {
		got := ""
		if ground := brushes.GroundOf(testCase.id); ground != nil {
			got = ground.Name
		}
		if got != testCase.want {
			t.Errorf("GroundOf(%d) = %q, want %q", testCase.id, got, testCase.want)
		}
	}
}

func TestBrushOfReturnsOwningBrushAndKind(t *testing.T) {
	e := brushesTestEditor(t, brushesAddBrush(
		&materials.Brush{Name: "vines", Type: materials.TypeDecoration, Walls: map[string]*materials.WallPart{
			"vertical": {Items: []materials.Chance{{ID: 740, Chance: 1}}},
		}},
	))
	for _, testCase := range []struct {
		id         uint16
		name, kind string
	}{
		{grassA, "grass", materials.TypeGround},
		{sandID, "sand", materials.TypeGround},
		{doorHClosed, "stone wall", materials.TypeWall},
		{wallH, "stone wall", materials.TypeWall},
		{740, "vines", materials.TypeDecoration},
		{rugN, "rug", materials.TypeCarpet},
		{tableEID, "desk", materials.TypeTable},
		{stoneID, "pit", materials.TypeDoodad},
		{borderS, "", ""}, // a border item belongs to no brush
		{bagID, "", ""},
		{9999, "", ""},
	} {
		name, kind := e.Brushes.BrushOf(testCase.id)
		if name != testCase.name || kind != testCase.kind {
			t.Errorf("BrushOf(%d) = %q, %q, want %q, %q", testCase.id, name, kind, testCase.name, testCase.kind)
		}
	}
}

func TestLookFallsBackToFirstItemOfBrush(t *testing.T) {
	e := brushesTestEditor(t, brushesAddBrush(
		&materials.Brush{Name: "vertical wall", Type: materials.TypeWall, Walls: map[string]*materials.WallPart{
			"vertical": {Items: []materials.Chance{{ID: 741, Chance: 1}}},
			"pole":     {Items: []materials.Chance{{ID: 742, Chance: 1}}},
		}},
		&materials.Brush{Name: "pole wall", Type: materials.TypeWall, Walls: map[string]*materials.WallPart{
			"pole": {Items: []materials.Chance{{ID: 743, Chance: 1}}},
		}},
	))
	for _, testCase := range []struct {
		name string
		want uint16
	}{
		{"grass", grassA},      // first ground item
		{"stone wall", wallH},  // horizontal first
		{"vertical wall", 741}, // then vertical
		{"pole wall", 743},     // then pole
		{"rug", rugC},          // carpet center
		{"trees", treeID},      // first single
		{"pit", sandID},        // first item of the first composite tile
	} {
		if got := e.Brushes.Look(testCase.name); got != testCase.want {
			t.Errorf("Look(%q) = %d, want %d", testCase.name, got, testCase.want)
		}
	}
}

func TestLookUsesExplicitLookOfMaterial(t *testing.T) {
	e := brushesTestEditor(t, func(materialsDB *materials.Materials) {
		for _, brush := range materialsDB.Brushes {
			brush.Look = 750
		}
	})
	for _, name := range []string{"grass", "stone wall", "rug", "desk", "trees", "pit"} {
		if got := e.Brushes.Look(name); got != 750 {
			t.Errorf("Look(%q) = %d, want the material look 750", name, got)
		}
	}
}

func TestCarpetAndTableRegisterTheirPieces(t *testing.T) {
	e := brushesTestEditor(t, brushesAddBrush(
		&materials.Brush{Name: "mat", Type: materials.TypeCarpet, Parts: map[string][]materials.Chance{
			"nw": {{ID: 760, Chance: 1}}, // not an edge name: skipped
			"s":  {{ID: 761, Chance: 1}},
		}},
		&materials.Brush{Name: "bench", Type: materials.TypeTable, Parts: map[string][]materials.Chance{
			"diagonal": {{ID: 762, Chance: 1}}, // not a table alignment: skipped
			"vertical": {{ID: 763, Chance: 1}},
		}},
	))
	info := e.Brushes.info
	if info[rugN] == nil || info[rugN].carpet != e.Brushes.Carpet("rug") || info[rugN].carpetAlign != edgeN {
		t.Errorf("rug north piece info = %+v, want carpet rug at edge n", info[rugN])
	}
	if info[761] == nil || info[761].carpetAlign != edgeS {
		t.Errorf("mat south piece info = %+v, want edge s", info[761])
	}
	if info[tableWestID] == nil || info[tableWestID].table != e.Brushes.Table("desk") || info[tableWestID].tableAlign != tableWest {
		t.Errorf("desk west piece info = %+v, want table desk aligned west", info[tableWestID])
	}
	if info[763] == nil || info[763].tableAlign != tableVertical {
		t.Errorf("bench vertical piece info = %+v, want aligned vertical", info[763])
	}
	if info[760] != nil || info[762] != nil {
		t.Errorf("pieces of unknown alignments were registered: %+v, %+v", info[760], info[762])
	}
}

func TestDoodadBrushOwnsItemsOfAllAlternatives(t *testing.T) {
	e := testEditor(t)
	pit := e.Brushes.Doodad("pit")
	for _, id := range []uint16{sandID, stoneID, borderN, treeID} {
		if !pit.owns(id) {
			t.Errorf("pit does not own %d, an item of its composite", id)
		}
	}
	if pit.owns(grassA) {
		t.Errorf("pit owns %d, which it never places", grassA)
	}
	if !e.Brushes.Doodad("trees").owns(treeID) {
		t.Errorf("trees does not own its single %d", treeID)
	}
}

func TestDoodadAltWrapsVariation(t *testing.T) {
	e := brushesTestEditor(t, brushesAddBrush(&materials.Brush{Name: "two", Type: materials.TypeDoodad, Alternatives: []materials.Alternative{
		{Singles: []materials.Single{{Chance: 1, Item: materials.ItemSpec{ID: stoneID}}}},
		{Singles: []materials.Single{{Chance: 1, Item: materials.ItemSpec{ID: bagID}}}},
	}}))
	brush := e.Brushes.Doodad("two")
	for _, testCase := range []struct {
		variation int
		want      int
	}{{0, 0}, {1, 1}, {2, 0}, {3, 1}, {-1, 1}, {-2, 0}} {
		if got := brush.alt(testCase.variation); got != brush.alts[testCase.want] {
			t.Errorf("alt(%d) is not alternative %d", testCase.variation, testCase.want)
		}
	}
}

func TestDoodadAltOfBrushWithoutAlternativesIsNil(t *testing.T) {
	if got := (&DoodadBrush{}).alt(0); got != nil {
		t.Errorf("alt = %+v, want nil for a brush without alternatives", got)
	}
}

func TestIsBorder(t *testing.T) {
	e := brushesTestEditor(t, func(materialsDB *materials.Materials) {
		// Item 770 has no item type: it is a border only through the materials.
		materialsDB.Borders = append(materialsDB.Borders, &materials.Border{ID: 2, Edges: map[string]uint16{"n": 770}})
	})
	for _, testCase := range []struct {
		id   uint16
		want bool
	}{
		{borderN, true},       // material border
		{770, true},           // material border without an item type
		{foreignBorder, true}, // border top order, no material
		{stoneID, false},
		{wallH, false},
		{9999, false},
	} {
		if got := e.Brushes.isBorder(testCase.id); got != testCase.want {
			t.Errorf("isBorder(%d) = %v, want %v", testCase.id, got, testCase.want)
		}
	}
}

func TestInfoForCreatesInfoOnce(t *testing.T) {
	brushes := testEditor(t).Brushes
	if brushes.info[9999] != nil {
		t.Fatal("info of 9999 exists before infoFor")
	}
	first := brushes.infoFor(9999)
	if first == nil || brushes.info[9999] != first {
		t.Fatalf("infoFor did not store a new info: %+v", first)
	}
	if again := brushes.infoFor(9999); again != first {
		t.Error("infoFor created a second info for the same item")
	}
}

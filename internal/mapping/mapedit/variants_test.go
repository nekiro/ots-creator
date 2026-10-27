package mapedit

import (
	"reflect"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func TestVariantsListBrushSiblingsFirst(t *testing.T) {
	e := testEditor(t)
	// Every fixture item is named "item", so the grounds of the same name
	// follow the grass of the brush.
	if got, want := e.Variants(grassB), []uint16{grassB, grassA, sandID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Variants(grassB) = %v, want %v", got, want)
	}
}

func TestVariantsKeepDoorState(t *testing.T) {
	e := testEditor(t)
	got := e.Variants(doorHClosed)
	if got[0] != doorHClosed || len(got) < 2 {
		t.Fatalf("Variants(closed door) = %v, want the closed door first", got)
	}
	for _, id := range got[:2] {
		if id == doorHOpn {
			t.Fatalf("Variants(closed door) = %v, the open door is not a brush sibling", got)
		}
	}
}

func TestVariantsFollowRotation(t *testing.T) {
	e := testEditor(t)
	e.Items.Get(tableAloneID).Name = "chair"
	e.Items.Get(tableH).Name = "chair east"
	e.Items.Get(tableAloneID).RotateTo = tableH
	e.Items.Get(tableH).RotateTo = tableAloneID
	if got, want := e.Variants(tableAloneID), []uint16{tableAloneID, tableH}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Variants(rotatable) = %v, want %v", got, want)
	}
}

func TestVariantsOfUnknownItemIsEmpty(t *testing.T) {
	if got := testEditor(t).Variants(4242); got != nil {
		t.Fatalf("Variants(unknown) = %v, want nil", got)
	}
}

func TestItemsOfBrushKinds(t *testing.T) {
	e := testEditor(t)
	cases := []struct {
		name, kind string
		want       []uint16
	}{
		{"grass", "ground", []uint16{grassA, grassB}},
		{"stone wall", "wall", []uint16{wallH, wallV, wallC, wallPoleID}},
		{"rug", "carpet", []uint16{rugC, rugN}},
		{"trees", "doodad", []uint16{treeID}},
		{"grass", "wall", nil},
	}
	for _, test := range cases {
		if got := e.Brushes.ItemsOf(test.name, test.kind); !reflect.DeepEqual(got, test.want) {
			t.Errorf("ItemsOf(%q, %q) = %v, want %v", test.name, test.kind, got, test.want)
		}
	}
}

func TestSwapItemKeepsAttributesAndMergesStroke(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), grassA, otmap.Item{ID: rugC, Ext: &otmap.ItemExt{ActionID: 7}})
	ref := ItemRef{Pos: at(1, 1), Index: 0}
	if err := e.SwapItem(ref, rugN, 5); err != nil {
		t.Fatal(err)
	}
	if err := e.SwapItem(ref, treeID, 5); err != nil {
		t.Fatal(err)
	}
	tile := e.Map.Tile(at(1, 1))
	if tile.Items[0].ID != treeID || tile.Items[0].ActionID() != 7 {
		t.Fatalf("after swaps item = %+v, want the tree with action id 7", tile.Items[0])
	}
	e.Undo()
	if got := e.Map.Tile(at(1, 1)).Items[0].ID; got != rugC {
		t.Fatalf("one undo gives item %d, want the first item %d", got, rugC)
	}
}

func TestSwapItemRefusesItemOnGround(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), grassA)
	if err := e.SwapItem(ItemRef{Pos: at(1, 1), Index: -1}, treeID, 0); err == nil {
		t.Fatal("SwapItem(ground, tree) = nil, want an error")
	}
	if err := e.SwapItem(ItemRef{Pos: at(1, 1), Index: -1}, grassB, 0); err != nil {
		t.Fatalf("SwapItem(ground, grassB) = %v", err)
	}
	if e.Map.Tile(at(1, 1)).Ground.ID != grassB {
		t.Fatal("ground not swapped")
	}
}

func TestAddMixScattersTheItemsWithoutClaimingThem(t *testing.T) {
	e := testEditor(t)
	name, err := e.Brushes.AddMix([]materials.Chance{{ID: stoneID, Chance: 3}, {ID: coinID, Chance: 1}, {ID: 4242, Chance: 5}})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Brushes.ItemsOf(name, "doodad"); !reflect.DeepEqual(got, []uint16{stoneID, coinID}) {
		t.Fatalf("ItemsOf(mix) = %v, want the stone and the coin", got)
	}
	if brush, _ := e.Brushes.BrushOf(coinID); brush != "" {
		t.Fatalf("BrushOf(coin) = %q, the mix must not claim its items", brush)
	}
	toolsPut(e, at(5, 5), grassA)
	if _, err := e.Draw(DrawRequest{Kind: KindDoodad, Name: name, Points: []otmap.Position{at(5, 5)}}); err != nil {
		t.Fatal(err)
	}
	if items := itemIDs(e.Map.Tile(at(5, 5))); len(items) != 1 || (items[0] != stoneID && items[0] != coinID) {
		t.Fatalf("tile after drawing the mix = %v, want a stone or a coin", items)
	}
	if _, err := e.Brushes.AddMix([]materials.Chance{{ID: 4242}}); err == nil {
		t.Fatal("AddMix(unknown items) = nil, want an error")
	}
}

func TestAddItemsIsOneUndoStep(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), grassA)
	toolsPut(e, at(2, 1), grassA)
	if err := e.AddItems([]PlacedItem{{Pos: at(1, 1), Item: treeID}, {Pos: at(2, 1), Item: stoneID}}, 0); err != nil {
		t.Fatal(err)
	}
	if itemIDs(e.Map.Tile(at(1, 1)))[0] != treeID || itemIDs(e.Map.Tile(at(2, 1)))[0] != stoneID {
		t.Fatal("items not placed")
	}
	e.Undo()
	if len(e.Map.Tile(at(1, 1)).Items) != 0 || len(e.Map.Tile(at(2, 1)).Items) != 0 {
		t.Fatal("one undo left items")
	}
}

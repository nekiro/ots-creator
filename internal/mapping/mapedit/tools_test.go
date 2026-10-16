package mapedit

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// toolsPut stores a tile with the given ground (0: none) and items.
func toolsPut(e *Editor, pos otmap.Position, ground uint16, items ...otmap.Item) {
	e.Map.SetTile(pos, &otmap.Tile{Ground: otmap.Item{ID: ground}, Items: items})
}

// toolsBag is a bag holding the given items.
func toolsBag(contents ...otmap.Item) otmap.Item {
	return otmap.Item{ID: bagID, Ext: &otmap.ItemExt{Items: contents}}
}

// toolsWithAction is an item with an action id.
func toolsWithAction(id, actionID uint16) otmap.Item {
	return otmap.Item{ID: id, Ext: &otmap.ItemExt{ActionID: actionID}}
}

// toolsWithUnique is an item with a unique id.
func toolsWithUnique(id, uniqueID uint16) otmap.Item {
	return otmap.Item{ID: id, Ext: &otmap.ItemExt{UniqueID: uniqueID}}
}

// toolsGroundAndItems lists the ground id followed by the item ids of a tile
// (nil when there is no tile).
func toolsGroundAndItems(tile *otmap.Tile) []uint16 {
	if tile == nil {
		return nil
	}
	return append([]uint16{tile.Ground.ID}, itemIDs(tile)...)
}

// toolsSnapshot lists every tile as "{x y z}: [ground items...]".
func toolsSnapshot(e *Editor) []string {
	var lines []string
	e.Map.Each(func(pos otmap.Position, tile *otmap.Tile) {
		lines = append(lines, fmt.Sprintf("%v: %v", pos, toolsGroundAndItems(tile)))
	})
	return lines
}

// --- Find ---

func TestFindByIDsListsMatchingItemsInMapOrder(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID}, otmap.Item{ID: treeID})
	toolsPut(e, at(2, 1), sandID, otmap.Item{ID: coinID})
	toolsPut(e, at(3, 1), sandID, otmap.Item{ID: stoneID})

	found, more := e.Find(FindQuery{IDs: []uint16{stoneID, treeID}})
	want := []FindResult{
		{Pos: at(1, 1), ID: stoneID, Name: "item"},
		{Pos: at(1, 1), ID: treeID, Name: "item"},
		{Pos: at(3, 1), ID: stoneID, Name: "item"},
	}
	if !reflect.DeepEqual(found, want) || more {
		t.Fatalf("Find(ids stone, tree) = %+v more=%v, want the two stones and the tree, more=false", found, more)
	}
}

func TestFindByIDsMatchesGround(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID})

	found, _ := e.Find(FindQuery{IDs: []uint16{sandID}})
	want := []FindResult{{Pos: at(1, 1), ID: sandID, Name: "item"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Find(ids sand) = %+v, want the sand ground %+v", found, want)
	}
}

func TestFindByNameIsCaseInsensitiveTrimmedSubstring(t *testing.T) {
	e := testEditor(t)
	e.Items.Get(stoneID).Name = "Small Stone"
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID}, otmap.Item{ID: coinID})

	found, _ := e.Find(FindQuery{Name: "  STONE "})
	want := []FindResult{{Pos: at(1, 1), ID: stoneID, Name: "Small Stone"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Find(name STONE) = %+v, want only the stone named Small Stone", found)
	}
}

func TestFindByNameSkipsUnknownItems(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), 0, otmap.Item{ID: 9999})

	if found, _ := e.Find(FindQuery{Name: "item"}); len(found) != 0 {
		t.Fatalf("Find(name item) = %+v, want nothing: item 9999 has no type, so no name", found)
	}
}

func TestFindResultNamesUnknownItem(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), 0, otmap.Item{ID: 9999})

	found, _ := e.Find(FindQuery{IDs: []uint16{9999}})
	if len(found) != 1 || found[0].Name != "unknown item 9999" {
		t.Fatalf("Find(ids 9999) = %+v, want one result named %q", found, "unknown item 9999")
	}
}

func TestFindByKind(t *testing.T) {
	longText := strings.Repeat("a", 45)
	cases := []struct {
		name  string
		setup func(e *Editor)
		items []otmap.Item
		kind  string
		want  []FindResult
	}{
		{
			name:  "unique lists items with a unique id",
			items: []otmap.Item{toolsWithUnique(stoneID, 50), {ID: stoneID}},
			kind:  "unique",
			want:  []FindResult{{Pos: at(1, 1), ID: stoneID, Name: "item", Detail: "uid 50"}},
		},
		{
			name:  "action lists items with an action id",
			items: []otmap.Item{{ID: stoneID}, toolsWithAction(coinID, 200)},
			kind:  "action",
			want:  []FindResult{{Pos: at(1, 1), ID: coinID, Name: "item", Detail: "aid 200"}},
		},
		{
			name:  "container lists containers with their content count",
			items: []otmap.Item{{ID: stoneID}, toolsBag(otmap.Item{ID: coinID}, otmap.Item{ID: stoneID}), {ID: bagID}},
			kind:  "container",
			want: []FindResult{
				{Pos: at(1, 1), ID: bagID, Name: "item", Detail: "2 items"},
				{Pos: at(1, 1), ID: bagID, Name: "item", Detail: "0 items"},
			},
		},
		{
			name:  "writeable lists writable items with a truncated quoted text",
			setup: func(e *Editor) { e.Items.Get(rugC).Flags |= otitems.FlagWritable },
			items: []otmap.Item{{ID: stoneID}, {ID: rugC, Ext: &otmap.ItemExt{Text: longText}}, {ID: rugC}},
			kind:  "writeable",
			want: []FindResult{
				{Pos: at(1, 1), ID: rugC, Name: "item", Detail: strconv.Quote(strings.Repeat("a", 40) + "…")},
				{Pos: at(1, 1), ID: rugC, Name: "item"},
			},
		},
		{
			name:  "writeable includes readable items",
			setup: func(e *Editor) { e.Items.Get(rugN).Flags |= otitems.FlagReadable },
			items: []otmap.Item{{ID: rugN, Ext: &otmap.ItemExt{Text: "hi"}}},
			kind:  "writeable",
			want:  []FindResult{{Pos: at(1, 1), ID: rugN, Name: "item", Detail: `"hi"`}},
		},
		{
			name:  "teleport lists items with a destination",
			items: []otmap.Item{{ID: stoneID}, {ID: stoneID, Ext: &otmap.ItemExt{Dest: ptr(posAt(9, 8, 6))}}},
			kind:  "teleport",
			want:  []FindResult{{Pos: at(1, 1), ID: stoneID, Name: "item", Detail: "to 9, 8, 6"}},
		},
		{
			name:  "door lists doors with their door id",
			items: []otmap.Item{{ID: doorHClosed, Ext: &otmap.ItemExt{DoorID: 4}}, {ID: doorHOpn}, {ID: stoneID}},
			kind:  "door",
			want: []FindResult{
				{Pos: at(1, 1), ID: doorHClosed, Name: "item", Detail: "door id 4"},
				{Pos: at(1, 1), ID: doorHOpn, Name: "item"},
			},
		},
		{
			name:  "duplicate lists the second copy of an item on a tile",
			items: []otmap.Item{{ID: stoneID}, {ID: coinID}, {ID: stoneID}, {ID: coinID}},
			kind:  "duplicate",
			want:  []FindResult{{Pos: at(1, 1), ID: stoneID, Name: "item", Detail: "duplicate"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			if tc.setup != nil {
				tc.setup(e)
			}
			toolsPut(e, at(1, 1), sandID, tc.items...)
			toolsPut(e, at(2, 1), sandID) // a plain tile matches no kind
			found, more := e.Find(FindQuery{Kind: tc.kind})
			if !reflect.DeepEqual(found, tc.want) || more {
				t.Fatalf("Find(kind %s) = %+v more=%v, want %+v more=false", tc.kind, found, more, tc.want)
			}
		})
	}
}

func TestFindDuplicateHonorsLimit(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID}, otmap.Item{ID: stoneID})
	toolsPut(e, at(2, 1), sandID, otmap.Item{ID: coinID}, otmap.Item{ID: coinID})

	found, more := e.Find(FindQuery{Kind: "duplicate", Limit: 1})
	want := []FindResult{{Pos: at(1, 1), ID: stoneID, Name: "item", Detail: "duplicate"}}
	if !reflect.DeepEqual(found, want) || !more {
		t.Fatalf("Find(duplicate, limit 1) = %+v more=%v, want only the first tile and more=true", found, more)
	}
}

func TestFindFiltersByActionIDAndShowsIt(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, toolsWithAction(stoneID, 100), toolsWithAction(stoneID, 200), otmap.Item{ID: stoneID})

	found, _ := e.Find(FindQuery{ActionID: 200})
	want := []FindResult{{Pos: at(1, 1), ID: stoneID, Name: "item", Detail: "aid 200"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Find(actionId 200) = %+v, want only the stone with aid 200 and detail %q", found, "aid 200")
	}
}

func TestFindFiltersByUniqueID(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, toolsWithUnique(stoneID, 50), toolsWithUnique(coinID, 51))

	found, _ := e.Find(FindQuery{UniqueID: 51})
	want := []FindResult{{Pos: at(1, 1), ID: coinID, Name: "item"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Find(uniqueId 51) = %+v, want only the coin with uid 51", found)
	}
	found, _ = e.Find(FindQuery{Kind: "unique", UniqueID: 50})
	want = []FindResult{{Pos: at(1, 1), ID: stoneID, Name: "item", Detail: "uid 50"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Find(kind unique, uniqueId 50) = %+v, want only the stone with detail uid 50", found)
	}
}

func TestFindCombinesIDsWithActionID(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, toolsWithAction(stoneID, 100), toolsWithAction(coinID, 100))

	found, _ := e.Find(FindQuery{IDs: []uint16{coinID}, ActionID: 100})
	want := []FindResult{{Pos: at(1, 1), ID: coinID, Name: "item", Detail: "aid 100"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Find(ids coin, aid 100) = %+v, want only the coin", found)
	}
}

func TestFindSearchesContainerContentsOnlyWithContainers(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID, toolsBag(toolsWithAction(coinID, 1234)))

	if found, _ := e.Find(FindQuery{IDs: []uint16{coinID}}); len(found) != 0 {
		t.Fatalf("Find(ids coin) = %+v, want nothing: the coin is inside a bag and Containers is off", found)
	}
	found, _ := e.Find(FindQuery{Kind: "action", Containers: true})
	want := []FindResult{{Pos: at(5, 5), ID: coinID, Name: "item", Detail: "aid 1234"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Find(kind action, containers) = %+v, want the coin inside the bag", found)
	}
}

func TestFindSearchesNestedContainers(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID, toolsBag(toolsBag(otmap.Item{ID: coinID})))

	found, _ := e.Find(FindQuery{IDs: []uint16{coinID}, Containers: true})
	if len(found) != 1 || found[0].ID != coinID {
		t.Fatalf("Find(ids coin, containers) = %+v, want the coin two bags deep", found)
	}
}

func TestFindLimitStopsAndReportsMore(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID}, otmap.Item{ID: stoneID})
	toolsPut(e, at(2, 1), sandID, otmap.Item{ID: stoneID})

	found, more := e.Find(FindQuery{IDs: []uint16{stoneID}, Limit: 2})
	if len(found) != 2 || !more {
		t.Fatalf("Find(limit 2) = %d results more=%v, want 2 results and more=true (3 stones exist)", len(found), more)
	}
	if found[1].Pos != at(1, 1) {
		t.Fatalf("second result at %+v, want the second stone of the first tile", found[1].Pos)
	}
}

func TestFindLimitStopsInsideContainers(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, toolsBag(otmap.Item{ID: coinID}, otmap.Item{ID: coinID}), otmap.Item{ID: coinID})

	found, more := e.Find(FindQuery{IDs: []uint16{coinID}, Containers: true, Limit: 1})
	if len(found) != 1 || !more {
		t.Fatalf("Find(limit 1, containers) = %+v more=%v, want 1 result and more=true", found, more)
	}
}

func TestFindWithoutLimitReturnsAll(t *testing.T) {
	e := testEditor(t)
	for x := 1; x <= 3; x++ {
		toolsPut(e, at(x, 1), sandID, otmap.Item{ID: stoneID})
	}

	found, more := e.Find(FindQuery{IDs: []uint16{stoneID}})
	if len(found) != 3 || more {
		t.Fatalf("Find(no limit) = %d results more=%v, want all 3 stones and more=false", len(found), more)
	}
}

func TestFindInSelectionSearchesOnlySelectedTiles(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID})
	toolsPut(e, at(2, 1), sandID, otmap.Item{ID: stoneID})
	e.SelectTiles([]otmap.Position{at(2, 1)}, false)

	found, _ := e.Find(FindQuery{IDs: []uint16{stoneID}, InSelection: true})
	want := []FindResult{{Pos: at(2, 1), ID: stoneID, Name: "item"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Find(in selection) = %+v, want only the selected stone", found)
	}
}

func TestFindInSelectionSkipsSelectedPositionWithoutTile(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID})
	toolsPut(e, at(2, 1), sandID, otmap.Item{ID: stoneID})
	e.SelectTiles([]otmap.Position{at(1, 1), at(2, 1)}, false)
	// The tile goes away behind the editor's back, so the selection still
	// lists its position.
	e.Map.SetTile(at(1, 1), nil)

	found, _ := e.Find(FindQuery{IDs: []uint16{stoneID}, InSelection: true})
	want := []FindResult{{Pos: at(2, 1), ID: stoneID, Name: "item"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Find(in selection) = %+v, want only the stone of the remaining tile", found)
	}
}

// --- Replace and remove tools ---

func TestReplaceItemsReplacesOnTilesAndInContainersKeepingAttributes(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID, toolsBag(toolsWithAction(coinID, 1234)), otmap.Item{ID: coinID, Subtype: 7})

	if replaced := e.ReplaceItems(map[uint16]uint16{coinID: stoneID}, false); replaced != 2 {
		t.Fatalf("ReplaceItems replaced %d, want 2 (the loose coin and the coin in the bag)", replaced)
	}
	tile := e.Map.Tile(at(5, 5))
	if inner := tile.Items[0].Ext.Items[0]; inner.ID != stoneID || inner.ActionID() != 1234 {
		t.Fatalf("item in bag = %+v, want a stone keeping aid 1234", inner)
	}
	if loose := tile.Items[1]; loose.ID != stoneID || loose.Subtype != 7 {
		t.Fatalf("loose item = %+v, want a stone keeping subtype 7", loose)
	}
}

func TestReplaceItemsReplacesGround(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID)

	if replaced := e.ReplaceItems(map[uint16]uint16{sandID: grassA}, false); replaced != 1 {
		t.Fatalf("ReplaceItems replaced %d, want 1", replaced)
	}
	if got := e.Map.Tile(at(5, 5)).Ground.ID; got != grassA {
		t.Fatalf("ground = %d, want %d", got, grassA)
	}
}

func TestReplaceItemsToZeroRemovesThem(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID, otmap.Item{ID: stoneID}, otmap.Item{ID: coinID})

	if removed := e.ReplaceItems(map[uint16]uint16{stoneID: 0}, false); removed != 1 {
		t.Fatalf("ReplaceItems removed %d, want 1", removed)
	}
	if got := toolsGroundAndItems(e.Map.Tile(at(5, 5))); !slices.Equal(got, []uint16{sandID, coinID}) {
		t.Fatalf("tile = %v, want sand and the coin only", got)
	}
	if undo, _ := e.History(); undo[len(undo)-1].Label != "Remove items" {
		t.Fatalf("undo label = %q, want %q for a single removal pair", undo[len(undo)-1].Label, "Remove items")
	}
}

func TestReplaceItemsToZeroRemovesThemFromContainers(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID, toolsBag(otmap.Item{ID: stoneID}, otmap.Item{ID: coinID}))

	if removed := e.ReplaceItems(map[uint16]uint16{stoneID: 0}, false); removed != 1 {
		t.Fatalf("ReplaceItems removed %d, want 1", removed)
	}
	if contents := e.Map.Tile(at(5, 5)).Items[0].Ext.Items; !reflect.DeepEqual(contents, []otmap.Item{{ID: coinID}}) {
		t.Fatalf("bag contents = %+v, want only the coin", contents)
	}
}

func TestReplaceItemsLabelsReplacement(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID, otmap.Item{ID: stoneID})

	e.ReplaceItems(map[uint16]uint16{stoneID: coinID}, false)
	if undo, _ := e.History(); len(undo) != 1 || undo[0].Label != "Replace items" {
		t.Fatalf("history = %+v, want one %q step", undo, "Replace items")
	}
}

func TestReplaceItemsInSelectionOnly(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID})
	toolsPut(e, at(2, 1), sandID, otmap.Item{ID: stoneID})
	e.SelectTiles([]otmap.Position{at(2, 1), at(3, 1)}, false)

	if replaced := e.ReplaceItems(map[uint16]uint16{stoneID: coinID}, true); replaced != 1 {
		t.Fatalf("ReplaceItems(in selection) replaced %d, want 1", replaced)
	}
	if got := itemIDs(e.Map.Tile(at(1, 1))); !slices.Equal(got, []uint16{stoneID}) {
		t.Fatalf("unselected tile = %v, want the stone kept", got)
	}
	if got := itemIDs(e.Map.Tile(at(2, 1))); !slices.Equal(got, []uint16{coinID}) {
		t.Fatalf("selected tile = %v, want the stone replaced by a coin", got)
	}
}

func TestReplaceItemsWithoutMatchesChangesNothing(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, toolsBag(otmap.Item{ID: stoneID}))

	if replaced := e.ReplaceItems(map[uint16]uint16{treeID: coinID}, false); replaced != 0 {
		t.Fatalf("ReplaceItems replaced %d, want 0", replaced)
	}
	if e.CanUndo() {
		t.Fatal("a replace that changed nothing left an undo step")
	}
}

func TestReplaceItemsIsUndoable(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, toolsBag(otmap.Item{ID: stoneID}))

	e.ReplaceItems(map[uint16]uint16{stoneID: coinID}, false)
	e.Undo()
	if got := e.Map.Tile(at(1, 1)).Items[0].Ext.Items[0].ID; got != stoneID {
		t.Fatalf("after undo the bag holds %d, want the stone back", got)
	}
}

func TestRemoveCorpsesRemovesPlainCorpsesOnly(t *testing.T) {
	e := testEditor(t)
	e.Items.Get(stoneID).Flags |= otitems.FlagCorpse
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID}, toolsWithAction(stoneID, 5), otmap.Item{ID: coinID})

	if removed := e.RemoveCorpses(false); removed != 1 {
		t.Fatalf("RemoveCorpses removed %d, want 1 (the corpse without attributes)", removed)
	}
	tile := e.Map.Tile(at(1, 1))
	if got := itemIDs(tile); !slices.Equal(got, []uint16{stoneID, coinID}) || tile.Items[0].ActionID() != 5 {
		t.Fatalf("tile = %v, want the corpse with an action id kept and the coin", got)
	}
}

func TestRemoveCorpsesCountsCorpsesInContainers(t *testing.T) {
	e := testEditor(t)
	e.Items.Get(stoneID).Flags |= otitems.FlagCorpse
	toolsPut(e, at(2, 1), sandID, toolsBag(otmap.Item{ID: stoneID}, otmap.Item{ID: coinID}))

	if removed := e.RemoveCorpses(false); removed != 1 {
		t.Fatalf("RemoveCorpses removed %d, want 1 (the corpse in the bag)", removed)
	}
}

func TestRemoveCorpsesRemovesCorpsesFromContainers(t *testing.T) {
	e := testEditor(t)
	e.Items.Get(stoneID).Flags |= otitems.FlagCorpse
	toolsPut(e, at(2, 1), sandID, toolsBag(otmap.Item{ID: stoneID}, otmap.Item{ID: coinID}))

	e.RemoveCorpses(false)
	if contents := e.Map.Tile(at(2, 1)).Items[0].Ext.Items; !reflect.DeepEqual(contents, []otmap.Item{{ID: coinID}}) {
		t.Fatalf("bag contents = %+v, want only the coin", contents)
	}
}

func TestRemoveCorpsesInSelection(t *testing.T) {
	e := testEditor(t)
	e.Items.Get(stoneID).Flags |= otitems.FlagCorpse
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID})
	toolsPut(e, at(2, 1), sandID, otmap.Item{ID: stoneID})
	e.SelectTiles([]otmap.Position{at(1, 1)}, false)

	if removed := e.RemoveCorpses(true); removed != 1 {
		t.Fatalf("RemoveCorpses(in selection) removed %d, want 1", removed)
	}
	if got := itemIDs(e.Map.Tile(at(2, 1))); !slices.Equal(got, []uint16{stoneID}) {
		t.Fatalf("unselected tile = %v, want the corpse kept", got)
	}
}

func TestRemoveInvalidRemovesUnknownItemsAndGround(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), 9998, otmap.Item{ID: 9999}, otmap.Item{ID: stoneID}, toolsBag(otmap.Item{ID: 9997}))

	if removed := e.RemoveInvalid(false); removed != 3 {
		t.Fatalf("RemoveInvalid removed %d, want 3 (ground 9998, item 9999, item 9997 in the bag)", removed)
	}
	if got := toolsGroundAndItems(e.Map.Tile(at(1, 1))); !slices.Equal(got, []uint16{0, stoneID, bagID}) {
		t.Fatalf("tile = %v, want no ground, the stone and the bag", got)
	}
}

func TestRemoveInvalidRemovesUnknownItemsFromContainers(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, toolsBag(otmap.Item{ID: 9997}))

	e.RemoveInvalid(false)
	if bag := e.Map.Tile(at(1, 1)).Items[0]; bag.Ext != nil {
		t.Fatalf("bag = %+v (contents %+v), want its invalid content removed and the ext dropped", bag, bag.Ext.Items)
	}
}

func TestRemoveInvalidWithNothingInvalidReturnsZero(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID, otmap.Item{ID: stoneID})

	if removed := e.RemoveInvalid(false); removed != 0 || e.CanUndo() {
		t.Fatalf("RemoveInvalid removed %d canUndo=%v, want 0 and no undo step", removed, e.CanUndo())
	}
}

func TestRemoveUnreachable(t *testing.T) {
	cases := []struct {
		name     string
		walkable otmap.Position
		blocking otmap.Position
		removed  int
	}{
		{"lone blocking tile far away is removed", at(100, 100), at(500, 500), 1},
		{"tile 9 tiles east of a walkable tile stays", at(100, 100), at(109, 100), 0},
		{"tile 10 tiles east of a walkable tile is removed", at(100, 100), at(110, 100), 1},
		{"tile 7 tiles south of a walkable tile stays", at(100, 100), at(100, 107), 0},
		{"tile 8 tiles south of a walkable tile is removed", at(100, 100), at(100, 108), 1},
		{"walkable tile 2 floors up keeps a tile", posAt(100, 100, 5), at(100, 100), 0},
		{"walkable tile 3 floors up does not keep a tile", posAt(100, 100, 4), at(100, 100), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			toolsPut(e, tc.walkable, sandID)
			toolsPut(e, tc.blocking, 0, otmap.Item{ID: treeID})

			if removed := e.RemoveUnreachable(); removed != tc.removed {
				t.Fatalf("RemoveUnreachable = %d, want %d", removed, tc.removed)
			}
			if gone := e.Map.Tile(tc.blocking) == nil; gone != (tc.removed == 1) {
				t.Fatalf("blocking tile removed=%v, want %v", gone, tc.removed == 1)
			}
			if e.Map.Tile(tc.walkable) == nil {
				t.Fatal("the walkable tile was removed")
			}
		})
	}
}

func TestRemoveUnreachableNothingToRemoveLeavesNoUndoStep(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(1, 1), sandID)

	if removed := e.RemoveUnreachable(); removed != 0 || e.CanUndo() {
		t.Fatalf("RemoveUnreachable = %d canUndo=%v, want 0 and no undo step", removed, e.CanUndo())
	}
}

func TestRemoveEmptySpawns(t *testing.T) {
	e := testEditor(t)
	// A monster spawn reaching a creature, an NPC spawn on the same tile
	// without one, and a spawn whose only creature is just out of reach.
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{
		Spawn: &otmap.Spawn{Radius: 2}, SpawnNPC: &otmap.Spawn{Radius: 1},
	}})
	e.Map.SetTile(at(12, 12), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Rat"}}}})
	e.Map.SetTile(at(50, 50), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Spawn: &otmap.Spawn{Radius: 2}}})
	e.Map.SetTile(at(53, 50), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Rat"}}}})
	// A creature on another floor does not count.
	e.Map.SetTile(posAt(50, 50, 6), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Rat"}}}})

	if removed := e.RemoveEmptySpawns(); removed != 2 {
		t.Fatalf("RemoveEmptySpawns = %d, want 2 (the NPC spawn at 10,10 and the spawn at 50,50)", removed)
	}
	kept := e.Map.Tile(at(10, 10))
	if kept.Spawn() == nil || kept.SpawnNPC() != nil {
		t.Fatalf("tile 10,10 spawn=%v npc spawn=%v, want the monster spawn kept and the NPC spawn removed", kept.Spawn(), kept.SpawnNPC())
	}
	if emptied := e.Map.Tile(at(50, 50)); emptied.Ext != nil {
		t.Fatalf("tile 50,50 ext = %+v, want it dropped with the spawn", emptied.Ext)
	}
}

func TestRemoveEmptySpawnsWithNoneEmptyReturnsZero(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{
		Spawn: &otmap.Spawn{Radius: 1}, Creatures: []otmap.Creature{{Name: "Rat"}},
	}})

	if removed := e.RemoveEmptySpawns(); removed != 0 || e.CanUndo() {
		t.Fatalf("RemoveEmptySpawns = %d canUndo=%v, want 0 and no undo step", removed, e.CanUndo())
	}
}

func TestClearInvalidHousesClearsTilesOfMissingHouses(t *testing.T) {
	e := testEditor(t)
	e.Map.Houses[1] = &otmap.House{ID: 1, Name: "Kept"}
	e.Map.SetTile(at(1, 1), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 1})
	e.Map.SetTile(at(2, 1), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 5})
	e.Map.SetTile(at(3, 1), &otmap.Tile{Ground: otmap.Item{ID: sandID}, HouseID: 6})

	if cleared := e.ClearInvalidHouses(); cleared != 2 {
		t.Fatalf("ClearInvalidHouses = %d, want 2", cleared)
	}
	for _, check := range []struct {
		pos   otmap.Position
		house uint32
	}{{at(1, 1), 1}, {at(2, 1), 0}, {at(3, 1), 0}} {
		if got := e.Map.Tile(check.pos).HouseID; got != check.house {
			t.Errorf("house of %+v = %d, want %d", check.pos, got, check.house)
		}
	}
}

func TestClearModifiedClearsStateWithoutUndoStep(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindRaw, Item: stoneID, Points: points(at(1, 1), at(40, 40))})
	e.TakeDirty()
	undoBefore, _ := e.History()

	e.ClearModified()
	for _, pos := range []otmap.Position{at(1, 1), at(40, 40)} {
		if e.Map.Tile(pos).State&otmap.StateModified != 0 {
			t.Errorf("tile %+v still modified", pos)
		}
	}
	chunks, _ := e.TakeDirty()
	slices.Sort(chunks)
	wantChunks := []otmap.ChunkKey{otmap.KeyOf(at(1, 1)), otmap.KeyOf(at(40, 40))}
	slices.Sort(wantChunks)
	if !slices.Equal(chunks, wantChunks) {
		t.Errorf("dirty chunks = %v, want every chunk %v redrawn", chunks, wantChunks)
	}
	if undoAfter, _ := e.History(); len(undoAfter) != len(undoBefore) {
		t.Errorf("history grew from %d to %d steps, want ClearModified not undoable", len(undoBefore), len(undoAfter))
	}
}

func TestBorderizeMapRedoesGroundBorders(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Rect: rect(20, 20, 24, 24)})
	e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(22, 22))})
	drawn := toolsSnapshot(e)
	// Strip every border, as an import from another editor might.
	e.Map.Each(func(pos otmap.Position, tile *otmap.Tile) {
		stripped := tile.Clone()
		stripped.Items = slices.DeleteFunc(stripped.Items, func(item otmap.Item) bool { return item.ID >= borderN && item.ID <= borderDSW })
		defer e.Map.SetTile(pos, stripped)
	})

	if changed := e.BorderizeMap(); changed != 8 {
		t.Fatalf("BorderizeMap changed %d tiles, want the 8 tiles around the grass", changed)
	}
	if got := toolsSnapshot(e); !slices.Equal(got, drawn) {
		t.Fatalf("after BorderizeMap:\n%v\nwant the borders the brush drew:\n%v", got, drawn)
	}
}

func TestBorderizeMapRedoesWalls(t *testing.T) {
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(40, 40, 44, 40)})
	e.Draw(DrawRequest{Kind: KindWall, Name: "stone wall", Rect: rect(40, 41, 40, 44)})
	drawn := toolsSnapshot(e)
	e.Map.Each(func(pos otmap.Position, tile *otmap.Tile) {
		flattened := tile.Clone()
		for i := range flattened.Items {
			flattened.Items[i].ID = wallH
		}
		defer e.Map.SetTile(pos, flattened)
	})

	if changed := e.BorderizeMap(); changed == 0 {
		t.Fatal("BorderizeMap changed nothing, want the flattened walls reshaped")
	}
	if got := toolsSnapshot(e); !slices.Equal(got, drawn) {
		t.Fatalf("after BorderizeMap:\n%v\nwant the walls the brush drew:\n%v", got, drawn)
	}
}

func TestRandomizeMapRerollsGroundsOfRandomBrushes(t *testing.T) {
	e := testEditor(t)
	for x := 0; x < 20; x++ {
		toolsPut(e, at(x, 1), grassA)
		toolsPut(e, at(x, 2), sandID)
	}

	changed := e.RandomizeMap()
	if changed == 0 {
		t.Fatal("RandomizeMap changed nothing, want some grass rerolled to the second grass item")
	}
	rerolled := 0
	for x := 0; x < 20; x++ {
		switch ground := e.Map.Tile(at(x, 1)).Ground.ID; ground {
		case grassB:
			rerolled++
		case grassA:
		default:
			t.Fatalf("grass tile %d has ground %d, want a grass item", x, ground)
		}
		if ground := e.Map.Tile(at(x, 2)).Ground.ID; ground != sandID {
			t.Fatalf("sand tile %d has ground %d, want sand kept (one item only)", x, ground)
		}
	}
	if rerolled != changed {
		t.Fatalf("RandomizeMap returned %d, but %d tiles changed", changed, rerolled)
	}
}

func TestStatsCountsEverything(t *testing.T) {
	e := testEditor(t)
	e.Map.Houses[1] = &otmap.House{ID: 1, Name: "House"}
	e.Map.Towns[1] = &otmap.Town{ID: 1, Name: "Town"}
	e.Map.Waypoints = append(e.Map.Waypoints, &otmap.Waypoint{Name: "start"})
	// A walkable PZ house tile with a bag (aid) holding a coin (uid).
	e.Map.SetTile(at(10, 10), &otmap.Tile{
		Ground: otmap.Item{ID: sandID}, Flags: otmap.FlagPZ, HouseID: 1,
		Items: []otmap.Item{{ID: bagID, Ext: &otmap.ItemExt{ActionID: 5, Items: []otmap.Item{toolsWithUnique(coinID, 7)}}}},
	})
	// A blocking tile with both spawns, a monster and an NPC.
	e.Map.SetTile(at(11, 12), &otmap.Tile{
		Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: treeID}},
		Ext: &otmap.TileExt{Spawn: &otmap.Spawn{Radius: 1}, SpawnNPC: &otmap.Spawn{Radius: 1},
			Creatures: []otmap.Creature{{Name: "Rat"}, {Name: "Tom", NPC: true}}},
	})
	// A teleport without ground on floor 6.
	e.Map.SetTile(posAt(13, 11, 6), &otmap.Tile{Items: []otmap.Item{{ID: stoneID, Ext: &otmap.ItemExt{Dest: ptr(at(1, 1))}}}})

	floors := make([]int, otmap.MaxFloor+1)
	floors[6], floors[7] = 1, 2
	want := Statistics{
		Tiles: 3, Items: 6, Walkable: 1, Blocking: 2, PZ: 1, HouseTiles: 1, Houses: 1, Towns: 1,
		Spawns: 2, Monsters: 1, NPCs: 1, Containers: 1, ContainerItems: 1, ActionIDs: 1, UniqueIDs: 1,
		Teleports: 1, Waypoints: 1, Floors: floors, MinX: 10, MinY: 10, MaxX: 13, MaxY: 12,
	}
	if got := e.Stats(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Stats() =\n%+v\nwant\n%+v", got, want)
	}
}

// --- Item inspector ---

// inspectorEditor has sand at 5,5 with a bag (index 0) holding a coin.
func inspectorEditor(t *testing.T) *Editor {
	t.Helper()
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID, toolsBag(otmap.Item{ID: coinID, Subtype: 1}))
	return e
}

// inspectorItem resolves a reference on the current map (read only).
func inspectorItem(t *testing.T, e *Editor, ref ItemRef) otmap.Item {
	t.Helper()
	item, _, _, err := itemAt(e.Map.Tile(ref.Pos), ref)
	if err != nil {
		t.Fatalf("item %+v: %v", ref, err)
	}
	return *item
}

var inspectorCoinRef = ItemRef{Pos: at(5, 5), Index: 0, Path: []int{0}}

func TestUpdateItemAppliesEachPatchField(t *testing.T) {
	attrs := []otmap.Attr{{Key: "level", Type: otmap.AttrInteger, Int: 3}}
	cases := []struct {
		name  string
		ref   ItemRef
		patch ItemPatch
		want  otmap.Item
	}{
		{"id", inspectorCoinRef, ItemPatch{ID: ptr[uint16](stoneID)}, otmap.Item{ID: stoneID, Subtype: 1}},
		{"subtype", inspectorCoinRef, ItemPatch{Subtype: ptr[uint16](50)}, otmap.Item{ID: coinID, Subtype: 50}},
		{"action id", inspectorCoinRef, ItemPatch{ActionID: ptr[uint16](1000)}, otmap.Item{ID: coinID, Subtype: 1, Ext: &otmap.ItemExt{ActionID: 1000}}},
		{"unique id", inspectorCoinRef, ItemPatch{UniqueID: ptr[uint16](2000)}, otmap.Item{ID: coinID, Subtype: 1, Ext: &otmap.ItemExt{UniqueID: 2000}}},
		{"text", inspectorCoinRef, ItemPatch{Text: ptr("hello")}, otmap.Item{ID: coinID, Subtype: 1, Ext: &otmap.ItemExt{Text: "hello"}}},
		{"description", inspectorCoinRef, ItemPatch{Description: ptr("shiny")}, otmap.Item{ID: coinID, Subtype: 1, Ext: &otmap.ItemExt{Description: "shiny"}}},
		{"dest", inspectorCoinRef, ItemPatch{Dest: ptr(at(9, 9))}, otmap.Item{ID: coinID, Subtype: 1, Ext: &otmap.ItemExt{Dest: ptr(at(9, 9))}}},
		{"clear dest wins over dest", inspectorCoinRef, ItemPatch{Dest: ptr(at(9, 9)), ClearDest: true}, otmap.Item{ID: coinID, Subtype: 1}},
		{"depot id", inspectorCoinRef, ItemPatch{DepotID: ptr[uint16](3)}, otmap.Item{ID: coinID, Subtype: 1, Ext: &otmap.ItemExt{DepotID: 3}}},
		{"door id", inspectorCoinRef, ItemPatch{DoorID: ptr[uint8](4)}, otmap.Item{ID: coinID, Subtype: 1, Ext: &otmap.ItemExt{DoorID: 4}}},
		{"attrs", inspectorCoinRef, ItemPatch{Attrs: &attrs}, otmap.Item{ID: coinID, Subtype: 1, Ext: &otmap.ItemExt{Attrs: attrs}}},
		{"ground through index -1", ItemRef{Pos: at(5, 5), Index: -1}, ItemPatch{ActionID: ptr[uint16](7)}, otmap.Item{ID: sandID, Ext: &otmap.ItemExt{ActionID: 7}}},
		{"container keeps its contents", ItemRef{Pos: at(5, 5), Index: 0}, ItemPatch{ActionID: ptr[uint16](8)},
			otmap.Item{ID: bagID, Ext: &otmap.ItemExt{ActionID: 8, Items: []otmap.Item{{ID: coinID, Subtype: 1}}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := inspectorEditor(t)
			if err := e.UpdateItem(tc.ref, tc.patch); err != nil {
				t.Fatalf("UpdateItem: %v", err)
			}
			if got := inspectorItem(t, e, tc.ref); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("item = %+v (ext %+v), want %+v (ext %+v)", got, got.Ext, tc.want, tc.want.Ext)
			}
		})
	}
}

func TestUpdateItemCopiesAttrs(t *testing.T) {
	e := inspectorEditor(t)
	attrs := []otmap.Attr{{Key: "level", Type: otmap.AttrInteger, Int: 3}}
	if err := e.UpdateItem(inspectorCoinRef, ItemPatch{Attrs: &attrs}); err != nil {
		t.Fatal(err)
	}
	attrs[0].Int = 99
	if got := inspectorItem(t, e, inspectorCoinRef).Ext.Attrs[0].Int; got != 3 {
		t.Fatalf("stored attr = %d after changing the patch slice, want 3 (the item keeps its own copy)", got)
	}
}

func TestUpdateItemDropsEmptyAttributes(t *testing.T) {
	e := inspectorEditor(t)
	if err := e.UpdateItem(inspectorCoinRef, ItemPatch{ActionID: ptr[uint16](5)}); err != nil {
		t.Fatal(err)
	}
	if err := e.UpdateItem(inspectorCoinRef, ItemPatch{ActionID: ptr[uint16](0)}); err != nil {
		t.Fatal(err)
	}
	if got := inspectorItem(t, e, inspectorCoinRef); got.Ext != nil {
		t.Fatalf("ext = %+v, want nil once the only attribute is cleared", got.Ext)
	}
}

func TestUpdateItemErrors(t *testing.T) {
	cases := []struct {
		name  string
		ref   ItemRef
		patch ItemPatch
		want  any
	}{
		{"no tile", ItemRef{Pos: at(6, 6)}, ItemPatch{}, ErrNoItem.Error()},
		{"index past the items", ItemRef{Pos: at(5, 5), Index: 9}, ItemPatch{}, ErrNoItem.Error()},
		{"path past the contents", ItemRef{Pos: at(5, 5), Index: 0, Path: []int{1}}, ItemPatch{}, ErrNoItem.Error()},
		{"negative path index", ItemRef{Pos: at(5, 5), Index: 0, Path: []int{-1}}, ItemPatch{}, ErrNoItem.Error()},
		{"path into an item without contents", ItemRef{Pos: at(5, 5), Index: 0, Path: []int{0, 0}}, ItemPatch{}, ErrNoItem.Error()},
		{"unknown new id", inspectorCoinRef, ItemPatch{ID: ptr[uint16](9999), ActionID: ptr[uint16](1)}, "unknown item 9999"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := inspectorEditor(t)
			if got := errText(e.UpdateItem(tc.ref, tc.patch)); got != tc.want {
				t.Fatalf("UpdateItem error = %v, want %v", got, tc.want)
			}
			want := otmap.Item{ID: coinID, Subtype: 1}
			if got := inspectorItem(t, e, inspectorCoinRef); !reflect.DeepEqual(got, want) {
				t.Fatalf("coin = %+v after a failed update, want it unchanged", got)
			}
		})
	}
}

func TestUpdateItemOnTileWithoutGroundFailsForGround(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), 0, otmap.Item{ID: stoneID})

	if err := e.UpdateItem(ItemRef{Pos: at(5, 5), Index: -1}, ItemPatch{ActionID: ptr[uint16](1)}); !errors.Is(err, ErrNoItem) {
		t.Fatalf("UpdateItem(ground of a tile without ground) = %v, want ErrNoItem", err)
	}
}

func TestUpdateItemIsOneUndoStep(t *testing.T) {
	e := inspectorEditor(t)
	if err := e.UpdateItem(inspectorCoinRef, ItemPatch{ActionID: ptr[uint16](5), Text: ptr("x")}); err != nil {
		t.Fatal(err)
	}
	if label := e.Undo(); label != "Edit item" {
		t.Fatalf("undo label = %q, want %q", label, "Edit item")
	}
	if got := inspectorItem(t, e, inspectorCoinRef); got.Ext != nil {
		t.Fatalf("coin after undo = %+v, want its attributes gone", got)
	}
}

func TestRemoveItemRemovesTopLevelItem(t *testing.T) {
	e := inspectorEditor(t)
	if err := e.RemoveItem(ItemRef{Pos: at(5, 5), Index: 0}); err != nil {
		t.Fatal(err)
	}
	if got := toolsGroundAndItems(e.Map.Tile(at(5, 5))); !slices.Equal(got, []uint16{sandID}) {
		t.Fatalf("tile = %v, want only the sand ground left", got)
	}
}

func TestRemoveItemRemovesGround(t *testing.T) {
	e := inspectorEditor(t)
	if err := e.RemoveItem(ItemRef{Pos: at(5, 5), Index: -1}); err != nil {
		t.Fatal(err)
	}
	if got := toolsGroundAndItems(e.Map.Tile(at(5, 5))); !slices.Equal(got, []uint16{0, bagID}) {
		t.Fatalf("tile = %v, want no ground and the bag", got)
	}
}

func TestRemoveItemRemovesContainerContent(t *testing.T) {
	e := inspectorEditor(t)
	if err := e.RemoveItem(inspectorCoinRef); err != nil {
		t.Fatal(err)
	}
	if bag := e.Map.Tile(at(5, 5)).Items[0]; bag.ID != bagID || bag.Ext != nil {
		t.Fatalf("bag = %+v, want an empty bag without ext", bag)
	}
}

func TestRemoveItemErrors(t *testing.T) {
	cases := []struct {
		name string
		ref  ItemRef
	}{
		{"no tile", ItemRef{Pos: at(6, 6)}},
		{"index past the items", ItemRef{Pos: at(5, 5), Index: 3}},
		{"content index past the contents", ItemRef{Pos: at(5, 5), Index: 0, Path: []int{4}}},
		{"content of an item without contents", ItemRef{Pos: at(5, 5), Index: 0, Path: []int{0, 0}}},
		{"parent path past the contents", ItemRef{Pos: at(5, 5), Index: 0, Path: []int{2, 0}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := inspectorEditor(t)
			if err := e.RemoveItem(tc.ref); !errors.Is(err, ErrNoItem) {
				t.Fatalf("RemoveItem(%+v) = %v, want ErrNoItem", tc.ref, err)
			}
			if got := inspectorItem(t, e, inspectorCoinRef); got.ID != coinID {
				t.Fatalf("coin = %+v, want it untouched", got)
			}
		})
	}
}

func TestAddToContainerAppendsNewItem(t *testing.T) {
	e := inspectorEditor(t)
	if err := e.AddToContainer(ItemRef{Pos: at(5, 5), Index: 0}, stoneID); err != nil {
		t.Fatal(err)
	}
	if err := e.AddToContainer(ItemRef{Pos: at(5, 5), Index: 0}, coinID); err != nil {
		t.Fatal(err)
	}
	want := []otmap.Item{{ID: coinID, Subtype: 1}, {ID: stoneID}, {ID: coinID, Subtype: 1}}
	if got := e.Map.Tile(at(5, 5)).Items[0].Ext.Items; !reflect.DeepEqual(got, want) {
		t.Fatalf("bag contents = %+v, want %+v (stackables get a count of one)", got, want)
	}
}

func TestAddToContainerIntoNestedContainer(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID, toolsBag(otmap.Item{ID: bagID}))

	if err := e.AddToContainer(ItemRef{Pos: at(5, 5), Index: 0, Path: []int{0}}, stoneID); err != nil {
		t.Fatal(err)
	}
	if got := e.Map.Tile(at(5, 5)).Items[0].Ext.Items[0].Ext.Items; len(got) != 1 || got[0].ID != stoneID {
		t.Fatalf("inner bag contents = %+v, want the stone", got)
	}
}

func TestAddToContainerErrors(t *testing.T) {
	cases := []struct {
		name string
		ref  ItemRef
		id   uint16
		want any
	}{
		{"unknown item", ItemRef{Pos: at(5, 5), Index: 0}, 9999, "unknown item 9999"},
		{"no tile", ItemRef{Pos: at(6, 6), Index: 0}, stoneID, ErrNoItem.Error()},
		{"missing item", ItemRef{Pos: at(5, 5), Index: 4}, stoneID, ErrNoItem.Error()},
		{"not a container", inspectorCoinRef, stoneID, "the item is not a container"},
		{"ground is not a container", ItemRef{Pos: at(5, 5), Index: -1}, stoneID, "the item is not a container"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := inspectorEditor(t)
			if got := errText(e.AddToContainer(tc.ref, tc.id)); got != tc.want {
				t.Fatalf("AddToContainer error = %v, want %v", got, tc.want)
			}
			if got := e.Map.Tile(at(5, 5)).Items[0].Ext.Items; len(got) != 1 {
				t.Fatalf("bag contents = %+v, want only the coin", got)
			}
		})
	}
}

func TestMoveItemReordersStack(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID, otmap.Item{ID: stoneID}, otmap.Item{ID: coinID}, otmap.Item{ID: bagID})

	if err := e.MoveItem(at(5, 5), 2, 0); err != nil {
		t.Fatal(err)
	}
	if got := itemIDs(e.Map.Tile(at(5, 5))); !slices.Equal(got, []uint16{bagID, stoneID, coinID}) {
		t.Fatalf("stack = %v, want the bag moved to the bottom", got)
	}
	if label := e.Undo(); label != "Reorder items" {
		t.Fatalf("undo label = %q, want %q", label, "Reorder items")
	}
}

func TestMoveItemIgnoresInvalidMoves(t *testing.T) {
	cases := []struct {
		name     string
		pos      otmap.Position
		from, to int
	}{
		{"no tile", at(6, 6), 0, 1},
		{"same index", at(5, 5), 1, 1},
		{"negative from", at(5, 5), -1, 0},
		{"from past the stack", at(5, 5), 2, 0},
		{"negative to", at(5, 5), 0, -1},
		{"to past the stack", at(5, 5), 0, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			toolsPut(e, at(5, 5), sandID, otmap.Item{ID: stoneID}, otmap.Item{ID: coinID})
			if err := e.MoveItem(tc.pos, tc.from, tc.to); err != nil {
				t.Fatalf("MoveItem = %v, want nil (invalid moves are ignored)", err)
			}
			if got := itemIDs(e.Map.Tile(at(5, 5))); !slices.Equal(got, []uint16{stoneID, coinID}) || e.CanUndo() {
				t.Fatalf("stack = %v canUndo=%v, want unchanged and no undo step", got, e.CanUndo())
			}
		})
	}
}

func TestAddItemStacksLikeRawBrush(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID, otmap.Item{ID: stoneID})

	if err := e.AddItem(at(5, 5), borderN); err != nil {
		t.Fatal(err)
	}
	if err := e.AddItem(at(5, 5), coinID); err != nil {
		t.Fatal(err)
	}
	tile := e.Map.Tile(at(5, 5))
	if got := itemIDs(tile); !slices.Equal(got, []uint16{borderN, stoneID, coinID}) {
		t.Fatalf("stack = %v, want the border below the stone and the coin on top", got)
	}
	if tile.Items[2].Subtype != 1 {
		t.Fatalf("coin subtype = %d, want 1 (new stackables count one)", tile.Items[2].Subtype)
	}
}

func TestAddItemCreatesTileAndSetsGround(t *testing.T) {
	e := testEditor(t)
	if err := e.AddItem(at(7, 7), grassA); err != nil {
		t.Fatal(err)
	}
	if got := toolsGroundAndItems(e.Map.Tile(at(7, 7))); !slices.Equal(got, []uint16{grassA}) {
		t.Fatalf("tile = %v, want a new tile with grass ground", got)
	}
}

func TestAddItemRejectsUnknownItem(t *testing.T) {
	e := testEditor(t)
	if got := errText(e.AddItem(at(7, 7), 9999)); got != "unknown item 9999" {
		t.Fatalf("AddItem(9999) error = %v, want %q", got, "unknown item 9999")
	}
	if e.Map.Tile(at(7, 7)) != nil {
		t.Fatal("AddItem of an unknown item created a tile")
	}
}

func TestRotateItemTurnsIntoRotateTo(t *testing.T) {
	e := inspectorEditor(t)
	e.Items.Get(bagID).RotateTo = stoneID

	if err := e.RotateItem(ItemRef{Pos: at(5, 5), Index: 0}); err != nil {
		t.Fatal(err)
	}
	if got := e.Map.Tile(at(5, 5)).Items[0]; got.ID != stoneID || len(got.Ext.Items) != 1 {
		t.Fatalf("rotated item = %+v, want id %d keeping its contents", got, stoneID)
	}
}

func TestRotateItemErrors(t *testing.T) {
	cases := []struct {
		name string
		ref  ItemRef
		want any
	}{
		{"no tile", ItemRef{Pos: at(6, 6)}, ErrNoItem.Error()},
		{"missing item", ItemRef{Pos: at(5, 5), Index: 5}, ErrNoItem.Error()},
		{"item without rotation", ItemRef{Pos: at(5, 5), Index: 0}, "the item cannot be rotated"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := inspectorEditor(t)
			if got := errText(e.RotateItem(tc.ref)); got != tc.want {
				t.Fatalf("RotateItem error = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUpdateTileSetsFlagsHouseAndZones(t *testing.T) {
	e := testEditor(t)
	toolsPut(e, at(5, 5), sandID)

	e.UpdateTile(at(5, 5), TilePatch{Flags: ptr(otmap.FlagNoPVP), HouseID: ptr[uint32](3), Zones: &[]uint16{1, 2}})
	tile := e.Map.Tile(at(5, 5))
	if tile.Flags != otmap.FlagNoPVP || tile.HouseID != 3 || !slices.Equal(tile.Zones(), []uint16{1, 2}) {
		t.Fatalf("tile flags=%d house=%d zones=%v, want %d, 3, [1 2]", tile.Flags, tile.HouseID, tile.Zones(), otmap.FlagNoPVP)
	}
}

func TestUpdateTileLeavesNilFieldsAndDropsEmptyZones(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(5, 5), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Flags: otmap.FlagPZ, HouseID: 2, Ext: &otmap.TileExt{Zones: []uint16{4}}})

	e.UpdateTile(at(5, 5), TilePatch{Zones: &[]uint16{}})
	tile := e.Map.Tile(at(5, 5))
	if tile.Flags != otmap.FlagPZ || tile.HouseID != 2 || tile.Ext != nil {
		t.Fatalf("tile flags=%d house=%d ext=%+v, want flags and house kept and the empty ext dropped", tile.Flags, tile.HouseID, tile.Ext)
	}
}

// creatureEditor has a spawn at 5,5 with a rat and a monster spawn.
func creatureEditor(t *testing.T) *Editor {
	t.Helper()
	e := testEditor(t)
	e.Map.SetTile(at(5, 5), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{
		Spawn:     &otmap.Spawn{Radius: 3},
		Creatures: []otmap.Creature{{Name: "Rat", SpawnTime: 60}, {Name: "Tom", NPC: true}},
	}})
	return e
}

func TestUpdateCreatureChangesFields(t *testing.T) {
	e := creatureEditor(t)
	err := e.UpdateCreature(at(5, 5), 0, CreaturePatch{SpawnTime: ptr[int32](90), Direction: ptr[uint8](6), Weight: ptr[uint8](3), Name: ptr("Cave Rat")})
	if err != nil {
		t.Fatal(err)
	}
	want := otmap.Creature{Name: "Cave Rat", SpawnTime: 90, Direction: 2, Weight: 3}
	if got := e.Map.Tile(at(5, 5)).Creatures()[0]; got != want {
		t.Fatalf("creature = %+v, want %+v (direction wraps at 4)", got, want)
	}
}

func TestUpdateCreatureIgnoresEmptyName(t *testing.T) {
	e := creatureEditor(t)
	if err := e.UpdateCreature(at(5, 5), 0, CreaturePatch{Name: ptr("")}); err != nil {
		t.Fatal(err)
	}
	if got := e.Map.Tile(at(5, 5)).Creatures()[0].Name; got != "Rat" {
		t.Fatalf("name = %q, want %q kept", got, "Rat")
	}
}

func TestUpdateCreatureRemove(t *testing.T) {
	e := creatureEditor(t)
	if err := e.UpdateCreature(at(5, 5), 0, CreaturePatch{Remove: true}); err != nil {
		t.Fatal(err)
	}
	if got := e.Map.Tile(at(5, 5)).Creatures(); len(got) != 1 || got[0].Name != "Tom" {
		t.Fatalf("creatures = %+v, want only Tom left", got)
	}
}

func TestUpdateCreatureRemoveLastDropsExt(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(5, 5), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Rat"}}}})

	if err := e.UpdateCreature(at(5, 5), 0, CreaturePatch{Remove: true}); err != nil {
		t.Fatal(err)
	}
	if ext := e.Map.Tile(at(5, 5)).Ext; ext != nil {
		t.Fatalf("ext = %+v, want nil once the last creature is gone", ext)
	}
}

func TestUpdateCreatureNotFound(t *testing.T) {
	cases := []struct {
		name  string
		pos   otmap.Position
		index int
	}{
		{"no tile", at(6, 6), 0},
		{"negative index", at(5, 5), -1},
		{"index past the creatures", at(5, 5), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := creatureEditor(t)
			if got := errText(e.UpdateCreature(tc.pos, tc.index, CreaturePatch{Remove: true})); got != "creature not found" {
				t.Fatalf("UpdateCreature error = %v, want %q", got, "creature not found")
			}
		})
	}
}

func TestUpdateSpawn(t *testing.T) {
	cases := []struct {
		name       string
		npc        bool
		radius     int32
		wantRadius int32 // 0: spawn removed
	}{
		{"sets the radius", false, 5, 5},
		{"caps the radius", false, 100, maxSpawnRadius},
		{"zero removes the spawn", false, 0, 0},
		{"negative removes the spawn", false, -1, 0},
		{"sets the npc spawn radius", true, 4, 4},
		{"zero removes the npc spawn", true, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Map.SetTile(at(5, 5), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{
				Spawn: &otmap.Spawn{Radius: 3}, SpawnNPC: &otmap.Spawn{Radius: 2},
			}})
			if err := e.UpdateSpawn(at(5, 5), tc.npc, tc.radius); err != nil {
				t.Fatal(err)
			}
			tile := e.Map.Tile(at(5, 5))
			changed, other := tile.Spawn(), tile.SpawnNPC()
			otherRadius := int32(2)
			if tc.npc {
				changed, other = other, changed
				otherRadius = 3
			}
			switch {
			case tc.wantRadius == 0 && changed != nil:
				t.Fatalf("spawn = %+v, want it removed", changed)
			case tc.wantRadius != 0 && (changed == nil || changed.Radius != tc.wantRadius):
				t.Fatalf("spawn = %+v, want radius %d", changed, tc.wantRadius)
			case other == nil || other.Radius != otherRadius:
				t.Fatalf("other spawn = %+v, want radius %d untouched", other, otherRadius)
			}
		})
	}
}

func TestUpdateSpawnRemoveLastDropsExt(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(5, 5), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Spawn: &otmap.Spawn{Radius: 3}}})

	if err := e.UpdateSpawn(at(5, 5), false, 0); err != nil {
		t.Fatal(err)
	}
	if ext := e.Map.Tile(at(5, 5)).Ext; ext != nil {
		t.Fatalf("ext = %+v, want nil once the only spawn is removed", ext)
	}
}

func TestUpdateSpawnNotFound(t *testing.T) {
	cases := []struct {
		name string
		pos  otmap.Position
		npc  bool
	}{
		{"no tile", at(6, 6), false},
		{"no monster spawn", at(5, 5), false},
		{"no npc spawn", at(7, 7), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Map.SetTile(at(5, 5), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{SpawnNPC: &otmap.Spawn{Radius: 2}}})
			e.Map.SetTile(at(7, 7), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Ext: &otmap.TileExt{Spawn: &otmap.Spawn{Radius: 2}}})
			if got := errText(e.UpdateSpawn(tc.pos, tc.npc, 4)); got != "spawn not found" {
				t.Fatalf("UpdateSpawn error = %v, want %q", got, "spawn not found")
			}
		})
	}
}

// --- Moving selections next to hand-placed borders ---

// moveNextToBorderEditor has sand around 70,70 with a hand-placed north
// border at 70,69 and a selected stone at 70,70.
func moveNextToBorderEditor(t *testing.T) *Editor {
	t.Helper()
	e := testEditor(t)
	e.Draw(DrawRequest{Kind: KindGround, Name: "sand", Points: points(at(70, 70)), Size: 2})
	// A hand-placed border the brushes would not make.
	e.Draw(DrawRequest{Kind: KindRaw, Item: borderN, Points: points(at(70, 69))})
	e.Draw(DrawRequest{Kind: KindRaw, Item: stoneID, Points: points(at(70, 70))})
	e.SelectAt(at(70, 70), false, true, true)
	return e
}

func TestMoveSelectionMovesSelectedItem(t *testing.T) {
	e := moveNextToBorderEditor(t)
	if err := e.MoveSelection(1, 0, 0); err != nil {
		t.Fatal(err)
	}
	if got := itemIDs(e.Map.Tile(at(71, 70))); !slices.Equal(got, []uint16{stoneID}) {
		t.Fatalf("items at 71,70 = %v, want the moved stone", got)
	}
}

func TestMoveSelectionKeepsHandPlacedBorderNextToIt(t *testing.T) {
	e := moveNextToBorderEditor(t)
	if err := e.MoveSelection(1, 0, 0); err != nil {
		t.Fatal(err)
	}
	if got := itemIDs(e.Map.Tile(at(70, 69))); !slices.Equal(got, []uint16{borderN}) {
		t.Fatalf("items at 70,69 = %v, want the hand-placed border kept (moving items must not reborder)", got)
	}
}

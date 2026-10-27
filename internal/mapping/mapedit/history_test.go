package mapedit

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func historyPutStone(e *Editor, label string, pos otmap.Position, stroke int64) int {
	tx := e.Begin(label, stroke)
	tile := tx.Edit(pos)
	tile.Items = append(tile.Items, otmap.Item{ID: stoneID})
	return tx.Commit()
}

func historyLabels(entries []ActionInfo) []string {
	var labels []string
	for _, entry := range entries {
		labels = append(labels, entry.Label)
	}
	return labels
}

func historyDrawStroke(e *Editor, stroke int64) {
	for i := range 3 {
		e.Draw(DrawRequest{Kind: KindRaw, Item: stoneID, Points: points(at(10+i, 10)), Stroke: stroke})
	}
}

func TestStrokeRequestsMergeIntoOneUndoStep(t *testing.T) {
	e := testEditor(t)
	historyDrawStroke(e, 7)
	if undo, _ := e.History(); len(undo) != 1 || undo[0].Tiles != 3 {
		t.Fatalf("history = %+v, want one step of 3 tiles", undo)
	}
}

func TestUndoRemovesWholeStroke(t *testing.T) {
	e := testEditor(t)
	historyDrawStroke(e, 7)
	e.Undo()
	for i := range 3 {
		if tile := e.Map.Tile(at(10+i, 10)); tile != nil {
			t.Fatalf("tile %d,10 = %+v after undo, want none", 10+i, tile)
		}
	}
}

func TestRedoReappliesWholeStroke(t *testing.T) {
	e := testEditor(t)
	historyDrawStroke(e, 7)
	e.Undo()
	e.Redo()
	for i := range 3 {
		if got := itemIDs(e.Map.Tile(at(10+i, 10))); !slices.Equal(got, []uint16{stoneID}) {
			t.Fatalf("tile %d,10 items %v after redo, want [%d]", 10+i, got, stoneID)
		}
	}
}

func TestStrokeMergeKeepsOldestTileState(t *testing.T) {
	e := testEditor(t)
	historyPutStone(e, "Draw", at(10, 10), 3)
	historyPutStone(e, "Draw", at(10, 10), 3)
	if undo, _ := e.History(); len(undo) != 1 || undo[0].Tiles != 1 {
		t.Fatalf("history = %+v, want one step of 1 tile", undo)
	}
	e.Undo()
	if tile := e.Map.Tile(at(10, 10)); tile != nil {
		t.Fatalf("tile after undo %+v, want the empty state from before the stroke", tile)
	}
	e.Redo()
	if got := itemIDs(e.Map.Tile(at(10, 10))); !slices.Equal(got, []uint16{stoneID, stoneID}) {
		t.Fatalf("items after redo %v, want both stones of the stroke", got)
	}
}

func TestActionsOfDifferentOrNoStrokeStaySeparate(t *testing.T) {
	cases := []struct {
		name          string
		first, second int64
	}{
		{"different strokes", 1, 2},
		{"no stroke", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			historyPutStone(e, "A", at(10, 10), tc.first)
			historyPutStone(e, "B", at(11, 10), tc.second)
			if undo, _ := e.History(); len(undo) != 2 {
				t.Fatalf("history = %+v, want 2 steps", undo)
			}
		})
	}
}

func TestUndoAndRedoReturnLabels(t *testing.T) {
	e := testEditor(t)
	historyPutStone(e, "Place stone", at(10, 10), 0)
	if label := e.Undo(); label != "Place stone" {
		t.Fatalf("Undo() = %q, want \"Place stone\"", label)
	}
	if label := e.Redo(); label != "Place stone" {
		t.Fatalf("Redo() = %q, want \"Place stone\"", label)
	}
}

func TestUndoAndRedoWithEmptyHistoryReturnEmptyLabel(t *testing.T) {
	e := testEditor(t)
	if label := e.Undo(); label != "" {
		t.Fatalf("Undo() = %q, want \"\"", label)
	}
	if label := e.Redo(); label != "" {
		t.Fatalf("Redo() = %q, want \"\"", label)
	}
}

func TestCanUndoAndCanRedoFollowStacks(t *testing.T) {
	e := testEditor(t)
	check := func(step string, wantUndo, wantRedo bool) {
		t.Helper()
		if e.CanUndo() != wantUndo || e.CanRedo() != wantRedo {
			t.Fatalf("%s: CanUndo %v CanRedo %v, want %v and %v", step, e.CanUndo(), e.CanRedo(), wantUndo, wantRedo)
		}
	}
	check("new editor", false, false)
	historyPutStone(e, "A", at(10, 10), 0)
	check("after an action", true, false)
	e.Undo()
	check("after undo", false, true)
	e.Redo()
	check("after redo", true, false)
}

func TestNewActionClearsRedo(t *testing.T) {
	e := testEditor(t)
	historyPutStone(e, "A", at(10, 10), 0)
	e.Undo()
	historyPutStone(e, "B", at(11, 10), 0)
	if e.CanRedo() {
		t.Fatal("CanRedo() after a new action, want the redo stack dropped")
	}
	if label := e.Redo(); label != "" {
		t.Fatalf("Redo() = %q, want nothing to redo", label)
	}
}

func TestHistoryListsUndoOldestFirstAndRedoNextFirst(t *testing.T) {
	e := testEditor(t)
	historyPutStone(e, "A", at(10, 10), 0)
	historyPutStone(e, "B", at(11, 10), 0)
	tx := e.Begin("C", 0)
	tx.Edit(at(12, 10)).Ground = otmap.Item{ID: sandID}
	tx.Edit(at(13, 10)).Ground = otmap.Item{ID: sandID}
	tx.Commit()
	e.Undo()
	e.Undo()
	undo, redo := e.History()
	if want := []ActionInfo{{"A", 1}}; !slices.Equal(undo, want) {
		t.Fatalf("undo = %+v, want %+v", undo, want)
	}
	if want := []ActionInfo{{"B", 1}, {"C", 2}}; !slices.Equal(redo, want) {
		t.Fatalf("redo = %+v, want %+v (next redo first, with tile counts)", redo, want)
	}
}

func TestHistoryKeepsAtMostMaxHistorySteps(t *testing.T) {
	e := testEditor(t)
	for i := range maxHistory + 1 {
		historyPutStone(e, fmt.Sprintf("step %d", i), at(10+i, 10), 0)
	}
	undo, _ := e.History()
	if len(undo) != maxHistory {
		t.Fatalf("history length %d, want %d", len(undo), maxHistory)
	}
	if undo[0].Label != "step 1" || undo[len(undo)-1].Label != fmt.Sprintf("step %d", maxHistory) {
		t.Fatalf("history runs from %q to %q, want the oldest step dropped", undo[0].Label, undo[len(undo)-1].Label)
	}
}

func TestTrimmedHistoryMovesSavedMark(t *testing.T) {
	e := testEditor(t)
	for i := range maxHistory {
		historyPutStone(e, "step", at(10+i, 10), 0)
	}
	e.MarkSaved()
	historyPutStone(e, "after save", at(10, 11), 0)
	e.Undo()
	if e.Changed() {
		t.Fatal("Changed() after undoing back to the saved state, want false: trimming must shift the saved mark")
	}
}

func TestTrimmedHistoryDropsUnreachableSavedMark(t *testing.T) {
	e := testEditor(t)
	e.MarkSaved()
	for i := range maxHistory + 1 {
		historyPutStone(e, "step", at(10+i, 10), 0)
	}
	for e.CanUndo() {
		e.Undo()
	}
	if !e.Changed() {
		t.Fatal("Changed() = false after undoing everything, want true: the saved state fell out of the history")
	}
}

func TestChangedFollowsSavedState(t *testing.T) {
	e := testEditor(t)
	check := func(step string, want bool) {
		t.Helper()
		if e.Changed() != want {
			t.Fatalf("%s: Changed() = %v, want %v", step, e.Changed(), want)
		}
	}
	check("new editor", false)
	historyPutStone(e, "A", at(10, 10), 0)
	check("after an action", true)
	e.Undo()
	check("after undoing to the start", false)
	e.Redo()
	check("after redo", true)
	e.MarkSaved()
	check("after save", false)
	e.Undo()
	check("after undoing past the save", true)
	e.Redo()
	check("after redoing to the save", false)
}

func TestMarkUnsavedCountsAsChanged(t *testing.T) {
	e := testEditor(t)
	e.MarkUnsaved()
	if !e.Changed() {
		t.Fatal("Changed() after MarkUnsaved = false, want true")
	}
	e.MarkSaved()
	if e.Changed() {
		t.Fatal("Changed() after MarkSaved = true, want false")
	}
}

func TestSavedStateUndoneAndReplacedIsChanged(t *testing.T) {
	cases := []struct {
		name  string
		saved int // actions before the save
		undos int
	}{
		{"two undone", 2, 2},
		{"one undone", 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			for i := range tc.saved {
				historyPutStone(e, "A", at(10+i, 10), 0)
			}
			e.MarkSaved()
			for range tc.undos {
				e.Undo()
			}
			historyPutStone(e, "B", at(20, 20), 0)
			if !e.Changed() {
				t.Fatal("Changed() = false, want true: the saved state was undone and replaced by another action")
			}
		})
	}
}

func TestStrokeContinuedAfterSaveIsChanged(t *testing.T) {
	e := testEditor(t)
	historyPutStone(e, "Paint", at(10, 10), 5)
	e.MarkSaved()
	historyPutStone(e, "Paint", at(11, 10), 5)
	if len(e.undo) != 1 {
		t.Fatalf("undo depth %d, want 1: the stroke should merge", len(e.undo))
	}
	if !e.Changed() {
		t.Fatal("Changed() = false, want true: the stroke changed the saved step")
	}
}

func TestTxLabelIsTheBeginLabel(t *testing.T) {
	e := testEditor(t)
	if label := e.Begin("Paint", 0).Label(); label != "Paint" {
		t.Fatalf("Label() = %q, want \"Paint\"", label)
	}
}

func TestTxEditCreatesTileVisibleInTheMap(t *testing.T) {
	e := testEditor(t)
	tx := e.Begin("Paint", 0)
	tile := tx.Edit(at(10, 10))
	if tile == nil || e.Map.Tile(at(10, 10)) != tile || tx.Get(at(10, 10)) != tile {
		t.Fatal("Edit on an empty position should store a new tile in the map right away")
	}
	if tile.State&otmap.StateModified == 0 {
		t.Fatalf("tile state %08b, want modified", tile.State)
	}
	if again := tx.Edit(at(10, 10)); again != tile {
		t.Fatal("a second Edit of the same tile should return the same working copy")
	}
}

func TestTxPriorReturnsTileFromBeforeTheAction(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}})
	tx := e.Begin("Paint", 0)
	if prior := tx.Prior(at(10, 10)); prior == nil || prior.Ground.ID != sandID {
		t.Fatalf("Prior before any edit = %+v, want the current sand tile", prior)
	}
	tx.Edit(at(10, 10)).Ground = otmap.Item{ID: grassA}
	if prior := tx.Prior(at(10, 10)); prior == nil || prior.Ground.ID != sandID {
		t.Fatalf("Prior after edit = %+v, want the sand from before", prior)
	}
	if current := tx.Get(at(10, 10)); current.Ground.ID != grassA {
		t.Fatalf("Get after edit = %+v, want the grass", current)
	}
	tx.Edit(at(11, 10))
	if prior := tx.Prior(at(11, 10)); prior != nil {
		t.Fatalf("Prior of a tile created in the action = %+v, want nil", prior)
	}
}

func TestCommitReturnsChangedTileCount(t *testing.T) {
	e := testEditor(t)
	tx := e.Begin("Paint", 0)
	tx.Edit(at(10, 10)).Ground = otmap.Item{ID: sandID}
	tx.Edit(at(11, 10)).Ground = otmap.Item{ID: sandID}
	tx.Edit(at(10, 10)).Items = []otmap.Item{{ID: stoneID}}
	if n := tx.Commit(); n != 2 {
		t.Fatalf("Commit() = %d, want 2 tiles", n)
	}
}

func TestCommitWithoutChangesRecordsNothing(t *testing.T) {
	e := testEditor(t)
	tx := e.Begin("Paint", 0)
	tx.Edit(at(10, 10))
	if n := tx.Commit(); n != 0 {
		t.Fatalf("Commit() = %d, want 0", n)
	}
	if e.CanUndo() {
		t.Fatal("an action without changes was recorded")
	}
	if e.Map.Tile(at(10, 10)) != nil {
		t.Fatal("the empty working tile was not pruned")
	}
}

func TestUndoRedoTileChangesInPlace(t *testing.T) {
	e := testEditor(t)
	e.Map.SetTile(at(10, 10), &otmap.Tile{Ground: otmap.Item{ID: sandID}, Items: []otmap.Item{{ID: coinID}}})
	tx := e.Begin("Replace", 0)
	tile := tx.Edit(at(10, 10))
	tile.Ground = otmap.Item{ID: grassA}
	tile.Items = nil
	tx.Commit()
	// Later edits of the live tile must not leak into the history.
	e.Map.Tile(at(10, 10)).Ground.ID = grassB
	e.Undo()
	if tile := e.Map.Tile(at(10, 10)); tile.Ground.ID != sandID || !slices.Equal(itemIDs(tile), []uint16{coinID}) {
		t.Fatalf("tile after undo %+v, want sand with a coin", tile)
	}
	e.Redo()
	if tile := e.Map.Tile(at(10, 10)); tile.Ground.ID != grassA || len(tile.Items) != 0 {
		t.Fatalf("tile after redo %+v, want plain grass %d", tile, grassA)
	}
}

func historyChangeDescription(e *Editor, description string, stroke int64) {
	tx := e.Begin("Map properties", stroke)
	tx.Meta()
	e.Map.Description = description
	tx.Commit()
}

func TestUndoRestoresMapDataOutsideTiles(t *testing.T) {
	e := testEditor(t)
	e.Map.Towns[1] = &otmap.Town{ID: 1, Name: "Old"}
	tx := e.Begin("Edit town", 0)
	tx.Meta()
	e.Map.Description = "new description"
	e.Map.Towns[1].Name = "New"
	e.Map.Houses[3] = &otmap.House{ID: 3}
	e.Map.Waypoints = append(e.Map.Waypoints, &otmap.Waypoint{Name: "temple"})
	e.Map.Zones = append(e.Map.Zones, otmap.Zone{ID: 2, Name: "arena"})
	if n := tx.Commit(); n != 0 {
		t.Fatalf("Commit() = %d, want 0 tiles", n)
	}
	if !e.CanUndo() {
		t.Fatal("a map data change without tile changes was not recorded")
	}
	e.Undo()
	if e.Map.Description != "" || e.Map.Towns[1].Name != "Old" || len(e.Map.Houses) != 0 || len(e.Map.Waypoints) != 0 || len(e.Map.Zones) != 0 {
		t.Fatalf("after undo: description %q town %q houses %d waypoints %d zones %d, want the old data",
			e.Map.Description, e.Map.Towns[1].Name, len(e.Map.Houses), len(e.Map.Waypoints), len(e.Map.Zones))
	}
	e.Redo()
	if e.Map.Description != "new description" || e.Map.Towns[1].Name != "New" || e.Map.Houses[3] == nil || len(e.Map.Waypoints) != 1 || len(e.Map.Zones) != 1 {
		t.Fatalf("after redo: description %q town %q houses %d waypoints %d zones %d, want the new data",
			e.Map.Description, e.Map.Towns[1].Name, len(e.Map.Houses), len(e.Map.Waypoints), len(e.Map.Zones))
	}
}

func TestStrokeMergeKeepsFirstMapDataState(t *testing.T) {
	cases := []struct {
		name          string
		firstHasMeta  bool
		wantAfterRedo string
	}{
		{"both requests change map data", true, "second"},
		{"only the later request changes map data", false, "second"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t)
			e.Map.Description = "original"
			if tc.firstHasMeta {
				historyChangeDescription(e, "first", 4)
			} else {
				historyPutStone(e, "Map properties", at(10, 10), 4)
			}
			historyChangeDescription(e, "second", 4)
			if undo, _ := e.History(); len(undo) != 1 {
				t.Fatalf("history = %+v, want one merged step", undo)
			}
			e.Undo()
			if e.Map.Description != "original" {
				t.Fatalf("description after undo %q, want \"original\"", e.Map.Description)
			}
			e.Redo()
			if e.Map.Description != tc.wantAfterRedo {
				t.Fatalf("description after redo %q, want %q", e.Map.Description, tc.wantAfterRedo)
			}
		})
	}
}

func TestTakeDirtyReturnsChangedChunksOnce(t *testing.T) {
	e := testEditor(t)
	historyPutStone(e, "A", at(10, 10), 0)
	historyPutStone(e, "B", at(300, 300), 0)
	chunks, meta := e.TakeDirty()
	slices.SortFunc(chunks, func(a, b otmap.ChunkKey) int { return cmp.Compare(a, b) })
	want := []otmap.ChunkKey{otmap.KeyOf(at(10, 10)), otmap.KeyOf(at(300, 300))}
	slices.SortFunc(want, func(a, b otmap.ChunkKey) int { return cmp.Compare(a, b) })
	if !slices.Equal(chunks, want) || meta {
		t.Fatalf("TakeDirty() = %v %v, want %v false", chunks, meta, want)
	}
	if chunks, meta := e.TakeDirty(); len(chunks) != 0 || meta {
		t.Fatalf("second TakeDirty() = %v %v, want nothing", chunks, meta)
	}
}

func TestTakeDirtyReportsMapDataChange(t *testing.T) {
	e := testEditor(t)
	historyChangeDescription(e, "new", 0)
	if chunks, meta := e.TakeDirty(); len(chunks) != 0 || !meta {
		t.Fatalf("TakeDirty() = %v %v, want no chunks and meta true", chunks, meta)
	}
	e.Undo()
	if _, meta := e.TakeDirty(); !meta {
		t.Fatal("TakeDirty() after undo of a map data change: meta false, want true")
	}
	e.Redo()
	if _, meta := e.TakeDirty(); !meta {
		t.Fatal("TakeDirty() after redo of a map data change: meta false, want true")
	}
}

func TestUndoAndRedoMarkChunksDirty(t *testing.T) {
	e := testEditor(t)
	historyPutStone(e, "A", at(10, 10), 0)
	e.TakeDirty()
	want := []otmap.ChunkKey{otmap.KeyOf(at(10, 10))}
	e.Undo()
	if chunks, _ := e.TakeDirty(); !slices.Equal(chunks, want) {
		t.Fatalf("dirty after undo %v, want %v", chunks, want)
	}
	e.Redo()
	if chunks, _ := e.TakeDirty(); !slices.Equal(chunks, want) {
		t.Fatalf("dirty after redo %v, want %v", chunks, want)
	}
}

func TestSetBrushesReplacesBrushesAndDropsDoodadPreview(t *testing.T) {
	e := testEditor(t)
	e.doodad = &DoodadBuffer{Brush: "trees"}
	replacement := NewBrushes(e.Items, materials.New())
	e.SetBrushes(replacement)
	if e.Brushes != replacement {
		t.Fatal("SetBrushes did not install the new brushes")
	}
	if e.doodad != nil {
		t.Fatalf("doodad preview = %+v, want nil: it was made with the old brushes", e.doodad)
	}
}

func TestSeedMakesRandomChoicesRepeatable(t *testing.T) {
	paintGrass := func(seed uint64) []uint16 {
		e := testEditor(t)
		e.Seed(seed)
		e.Settings.Automagic = false
		e.Draw(DrawRequest{Kind: KindGround, Name: "grass", Points: points(at(50, 50)), Size: 3})
		var grounds []uint16
		for y := 47; y <= 53; y++ {
			for x := 47; x <= 53; x++ {
				if tile := e.Map.Tile(at(x, y)); tile != nil {
					grounds = append(grounds, tile.Ground.ID)
				}
			}
		}
		return grounds
	}
	first, second := paintGrass(5), paintGrass(5)
	if len(first) == 0 || !slices.Equal(first, second) {
		t.Fatalf("grounds with the same seed differ:\n%v\n%v", first, second)
	}
	if !slices.Contains(first, grassA) || !slices.Contains(first, grassB) {
		t.Fatalf("grounds %v, want both grass items picked at random", first)
	}
}

func TestSettingsFormatFollowsCountU16(t *testing.T) {
	for _, countU16 := range []bool{false, true} {
		if got := (Settings{CountU16: countU16}).Format(); got != (otmap.Format{CountU16: countU16}) {
			t.Fatalf("Format() with CountU16 %v = %+v", countU16, got)
		}
	}
}

func TestDefaultSettings(t *testing.T) {
	want := Settings{Automagic: true, EraserLeaveUnique: true, HouseRemoveItems: true, AutoDoorID: true, DefaultSpawnTime: 60, AutoCreateSpawn: true}
	if got := DefaultSettings(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultSettings() = %+v, want %+v", got, want)
	}
}

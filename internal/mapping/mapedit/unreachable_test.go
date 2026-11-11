package mapedit

import (
	"encoding/json"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// seenByScan reports whether a walkable tile is within the client view of
// pos, by looking at every position in range: the slow reference for
// RemoveUnreachable.
func seenByScan(e *Editor, pos otmap.Position) bool {
	x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
	for floor := max(z-e.Settings.ViewRangeFloors, 0); floor <= min(z+e.Settings.ViewRangeFloors, otmap.MaxFloor); floor++ {
		for dy := -e.Settings.ViewRangeY; dy <= e.Settings.ViewRangeY; dy++ {
			for dx := -e.Settings.ViewRangeX; dx <= e.Settings.ViewRangeX; dx++ {
				if tile := e.Map.At(x+dx, y+dy, floor); tile != nil && !e.blocking(tile) {
					return true
				}
			}
		}
	}
	return false
}

func TestRemoveUnreachableMatchesScanOnRandomMap(t *testing.T) {
	e := testEditor(t)
	random := rand.New(rand.NewPCG(1, 2))
	for range 3000 {
		pos := posAt(random.IntN(200), random.IntN(200), 5+random.IntN(5))
		if random.IntN(20) == 0 {
			toolsPut(e, pos, sandID)
		} else {
			toolsPut(e, pos, 0, otmap.Item{ID: treeID})
		}
	}
	want := map[otmap.Position]bool{}
	e.Map.Each(func(pos otmap.Position, _ *otmap.Tile) {
		want[pos] = seenByScan(e, pos)
	})

	e.RemoveUnreachable()

	for pos, seen := range want {
		if kept := e.Map.Tile(pos) != nil; kept != seen {
			t.Fatalf("tile %v kept=%v, want %v", pos, kept, seen)
		}
	}
}

func TestRemoveUnreachableReportsProgress(t *testing.T) {
	e := testEditor(t)
	for x := range 100 {
		toolsPut(e, at(x, 0), sandID)
	}
	lastDone, lastTotal := 0, 0
	e.Progress = func(done, total int) { lastDone, lastTotal = done, total }

	e.RemoveUnreachable()

	if lastTotal != 200 || lastDone != lastTotal {
		t.Fatalf("last progress %d/%d, want 200/200", lastDone, lastTotal)
	}
}

func TestRemoveUnreachableOnLargeMapIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("large map")
	}
	e := testEditor(t)
	// 1000x1000 blocking tiles on one floor with a walkable tile every 50.
	for y := range 1000 {
		for x := range 1000 {
			if x%50 == 0 && y%50 == 0 {
				toolsPut(e, at(x, y), sandID)
			} else {
				toolsPut(e, at(x, y), 0, otmap.Item{ID: treeID})
			}
		}
	}
	start := time.Now()
	removed := e.RemoveUnreachable()
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("RemoveUnreachable took %v on a million tiles", elapsed)
	}
	// Each walkable tile keeps a 19x15 area; the ones at x 0 and y 0 keep
	// only the part inside the map (10 columns, 8 rows).
	keptColumns, keptRows := 10+19*19, 8+19*15
	if want := 1000*1000 - keptColumns*keptRows; removed != want {
		t.Fatalf("removed %d, want %d", removed, want)
	}
}

func TestRemoveUnreachableFollowsViewRangeSettings(t *testing.T) {
	e := testEditor(t)
	e.Settings.ViewRangeX, e.Settings.ViewRangeY, e.Settings.ViewRangeFloors = 3, 2, 0
	toolsPut(e, at(100, 100), sandID)
	seen := []otmap.Position{at(103, 100), at(97, 102)}
	unseen := []otmap.Position{at(104, 100), at(100, 103), posAt(100, 100, 6)}
	for _, pos := range append(append([]otmap.Position{}, seen...), unseen...) {
		toolsPut(e, pos, 0, otmap.Item{ID: treeID})
	}

	e.RemoveUnreachable()

	for _, pos := range seen {
		if e.Map.Tile(pos) == nil {
			t.Fatalf("tile %v in view was removed", pos)
		}
	}
	for _, pos := range unseen {
		if e.Map.Tile(pos) != nil {
			t.Fatalf("tile %v out of view was kept", pos)
		}
	}
}

func TestSettingsSavedBeforeViewRangeGetDefaults(t *testing.T) {
	var settings Settings
	if err := json.Unmarshal([]byte(`{"automagic":false,"defaultSpawnTime":30}`), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Automagic || settings.DefaultSpawnTime != 30 {
		t.Fatalf("saved fields lost: %+v", settings)
	}
	if settings.ViewRangeX != 9 || settings.ViewRangeY != 7 || settings.ViewRangeFloors != 2 {
		t.Fatalf("view range = %d x %d, %d floors, want defaults 9 x 7, 2 floors", settings.ViewRangeX, settings.ViewRangeY, settings.ViewRangeFloors)
	}
}

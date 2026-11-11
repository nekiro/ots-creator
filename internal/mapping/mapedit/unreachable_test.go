package mapedit

import (
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
	for floor := max(z-clientViewFloorsRange, 0); floor <= min(z+clientViewFloorsRange, otmap.MaxFloor); floor++ {
		for dy := -clientViewRangeY; dy <= clientViewRangeY; dy++ {
			for dx := -clientViewRangeX; dx <= clientViewRangeX; dx++ {
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

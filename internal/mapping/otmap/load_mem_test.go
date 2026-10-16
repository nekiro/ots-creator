package otmap

import (
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/nekiro/ots-creator/internal/core/otitems"
)

// TestMapMemory measures loading a real map (skipped unless OTS_MAP is set):
//
//	OTS_MAP=path/map.otbm [OTS_HEAP=heap.prof] [OTS_ROUNDTRIP=1] go test -run TestMapMemory -v ./internal/otmap
//
// OTS_ROUNDTRIP also saves the map in memory, reads it back and compares.
//
// The item database is fake (no ground flags), so grounds count as items.
func TestMapMemory(t *testing.T) {
	path := os.Getenv("OTS_MAP")
	if path == "" {
		t.Skip("OTS_MAP not set")
	}
	types := make([]*otitems.Type, 0, 60000)
	for id := uint16(100); id < 60000; id++ {
		types = append(types, &otitems.Type{ID: id, ClientID: id})
	}
	items := otitems.NewDB(otitems.ModeClient, types)
	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return ms.HeapAlloc
	}
	base := heap()
	start := time.Now()
	var peak uint64
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(50 * time.Millisecond):
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				if ms.HeapAlloc > peak {
					peak = ms.HeapAlloc
				}
			}
		}
	}()
	m, warns, err := Load(path, items, Format{})
	close(done)
	if err != nil {
		t.Fatal(err)
	}
	el := time.Since(start)
	after := heap()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	items0, ext, exti, single := 0, 0, 0, 0
	m.Each(func(p Position, tl *Tile) {
		items0 += len(tl.Items)
		if len(tl.Items)+min(int(tl.Ground.ID), 1) == 1 && tl.Ext == nil && tl.Flags == 0 && tl.HouseID == 0 {
			single++
		}
		if tl.Ext != nil {
			ext++
		}
		for i := range tl.Items {
			if tl.Items[i].Ext != nil {
				exti++
			}
		}
	})
	t.Logf("tiles with one item and nothing else: %d", single)
	t.Logf("load %v, warns %d, tiles %d, items %d, tileExt %d, itemExt %d, chunks %d", el, len(warns), m.TileCount(), items0, ext, exti, len(m.Chunks()))
	t.Logf("heap retained %.0f MB, peak sampled %.0f MB, sys %.0f MB", float64(after-base)/1e6, float64(peak)/1e6, float64(ms.Sys)/1e6)
	if os.Getenv("OTS_ROUNDTRIP") != "" {
		start := time.Now()
		data, err := EncodeOTBM(m, items, Format{})
		if err != nil {
			t.Fatal(err)
		}
		enc := time.Since(start)
		got, _, err := ReadOTBM(data, items, Format{})
		if err != nil {
			t.Fatal(err)
		}
		gotItems := 0
		got.Each(func(_ Position, tl *Tile) { gotItems += len(tl.Items) })
		// Tiles with only creatures or spawns go to the spawn file, not the OTBM.
		spawnOnly := 0
		m.Each(func(_ Position, tl *Tile) {
			if tl.Ground.ID == 0 && len(tl.Items) == 0 && tl.HouseID == 0 && tl.Flags == 0 && len(tl.Zones()) == 0 {
				spawnOnly++
			}
		})
		if got.TileCount() != m.TileCount()-spawnOnly || gotItems != items0 {
			t.Fatalf("round trip: tiles %d/%d items %d/%d", got.TileCount(), m.TileCount(), gotItems, items0)
		}
		t.Logf("round trip ok: encode %v, %d MB", enc, len(data)>>20)
	}
	if out := os.Getenv("OTS_HEAP"); out != "" {
		f, _ := os.Create(out)
		_ = pprof.WriteHeapProfile(f)
		f.Close()
	}
	runtime.KeepAlive(m)
}

package suggest

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// TestIndexMemory measures counting a real map:
//
//	OTS_MAP=path/map.otbm [OTS_ITEMS=path/items.otb | OTS_CLIENT=path/client] go test -run TestIndexMemory -v ./internal/mapping/suggest
//
// OTS_CLIENT reads the items of a client with assets (client ids), as the
// editor does without items.otb. Without either the item database is fake
// (no ground flags), so grounds count as items.
func TestIndexMemory(t *testing.T) {
	gameMap, _, ignore := loadEnvMap(t)
	heap := func() uint64 {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return stats.HeapAlloc
	}
	base := heap()
	start := time.Now()
	index := New()
	index.Refresh(gameMap, ignore, 1<<30)
	counted := time.Since(start)
	t.Logf("%d tiles, %d blocks: counted in %v, index %d MB", gameMap.TileCount(), len(index.blocks), counted, (heap()-base)>>20)
	start = time.Now()
	index.Refresh(gameMap, ignore, 1<<30)
	t.Logf("refresh without changes: %v", time.Since(start))
	var place Context
	gameMap.Each(func(pos otmap.Position, tile *otmap.Tile) {
		if place.Ground == 0 && pos.Z == 7 && tile.Ground.ID != 0 && len(tile.Items) > 0 {
			place = Context{Pos: pos, Ground: tile.Ground.ID, Items: []uint16{tile.Items[0].ID}}
		}
	})
	start = time.Now()
	here := index.Here(place, 24)
	t.Logf("here at %v: %v (%d)", place.Pos, time.Since(start), len(here))
	start = time.Now()
	substitutes := index.Substitutes(place.Items[0], 8)
	t.Logf("substitutes: %v (%d)", time.Since(start), len(substitutes))
	start = time.Now()
	completions := index.Completions(place.Items[0])
	t.Logf("completions: %v (%d)", time.Since(start), len(completions))
	runtime.KeepAlive(index)
}

// loadEnvMap loads the map of OTS_MAP with the items of OTS_ITEMS and
// returns it with the items and a filter of borders; it skips the test
// without OTS_MAP.
func loadEnvMap(t *testing.T) (*otmap.Map, *otitems.DB, func(id uint16) bool) {
	t.Helper()
	path := os.Getenv("OTS_MAP")
	if path == "" {
		t.Skip("OTS_MAP not set")
	}
	types := make([]*otitems.Type, 0, 60000)
	for id := uint16(100); id < 60000; id++ {
		types = append(types, &otitems.Type{ID: id, ClientID: id})
	}
	items := otitems.NewDB(otitems.ModeClient, types)
	if clientPath := os.Getenv("OTS_CLIENT"); clientPath != "" {
		// Client ids: the items and their flags come from the client.
		client, err := project.OpenAssets(clientPath)
		if err != nil {
			t.Fatal(err)
		}
		items = otitems.FromClient(client, client.Info().Counts.Items, client.Names(thing.CategoryItem))
	} else if itemsPath := os.Getenv("OTS_ITEMS"); itemsPath != "" {
		data, err := os.ReadFile(itemsPath)
		if err != nil {
			t.Fatal(err)
		}
		if items, err = otitems.ParseOTB(data); err != nil {
			t.Fatal(err)
		}
	}
	ignore := func(id uint16) bool {
		itemType := items.Get(id)
		return itemType == nil || itemType.IsBorder()
	}
	gameMap, _, err := otmap.Load(path, items, otmap.Format{})
	if err != nil {
		t.Fatal(err)
	}
	return gameMap, items, ignore
}

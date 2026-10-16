package project

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// TestOpenAssetsMemory measures opening a real client (skipped unless
// OTS_ASSETS is set to a folder with catalog-content.json):
//
//	OTS_ASSETS=path/client/data/things go test -run TestOpenAssetsMemory -v ./internal/project
func TestOpenAssetsMemory(t *testing.T) {
	dir := os.Getenv("OTS_ASSETS")
	if dir == "" {
		t.Skip("OTS_ASSETS not set")
	}
	heap := func() uint64 {
		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return ms.HeapAlloc
	}
	base := heap()
	start := time.Now()
	project, err := OpenAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("open %v, heap retained %.0f MB", time.Since(start), float64(heap()-base)/1e6)
	runtime.KeepAlive(project)
}

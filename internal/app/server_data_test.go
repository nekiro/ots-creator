package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectServerData(t *testing.T) {
	server := t.TempDir()
	items := filepath.Join(server, "data", "items")
	world := filepath.Join(server, "data", "world")
	for _, dir := range []string{items, world} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"items.otb", "items.xml"} {
		os.WriteFile(filepath.Join(items, name), nil, 0o644)
	}
	mapPath := filepath.Join(world, "map.otbm")
	os.WriteFile(mapPath, nil, 0o644)
	config := detectServerData(mapPath)
	if config.OTBPath != filepath.Join(items, "items.otb") || config.XMLPath != filepath.Join(items, "items.xml") {
		t.Fatalf("config = %+v", config)
	}
	if config.ServerDir != filepath.Join(server, "data") || config.ItemsMode != itemsAuto {
		t.Fatalf("server dir %q, mode %q", config.ServerDir, config.ItemsMode)
	}
	if empty := detectServerData(""); empty.ItemsMode != "" {
		t.Fatalf("empty start = %+v", empty)
	}
	if none := detectServerData(t.TempDir()); none.XMLPath != "" || none.ServerDir != "" {
		t.Fatalf("nothing to find = %+v", none)
	}
}

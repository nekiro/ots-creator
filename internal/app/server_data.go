package app

import (
	"path/filepath"

	"github.com/nekiro/ots-creator/internal/fsutil"
)

const serverDataSearchDepth = 5

var serverDataFolders = []string{"items", filepath.Join("data", "items"), ""}

// detectServerData looks for items.otb and items.xml in a server data
// folder, a map folder or their parents (data/items, data/world/..).
func detectServerData(start string) MapDataConfig {
	var config MapDataConfig
	if start == "" {
		return config
	}
	dir := start
	if fsutil.FileExists(dir) {
		dir = filepath.Dir(dir)
	}
	for level := 0; level < serverDataSearchDepth && dir != "" && dir != filepath.Dir(dir); level++ {
		for _, folder := range serverDataFolders {
			base := filepath.Join(dir, folder)
			if config.OTBPath == "" && fsutil.FileExists(filepath.Join(base, "items.otb")) {
				config.OTBPath = filepath.Join(base, "items.otb")
			}
			if config.XMLPath == "" && fsutil.FileExists(filepath.Join(base, "items.xml")) {
				config.XMLPath = filepath.Join(base, "items.xml")
			}
		}
		if config.XMLPath != "" {
			config.ServerDir = dir
			if filepath.Base(dir) != "data" && fsutil.DirExists(filepath.Join(dir, "data")) {
				config.ServerDir = filepath.Join(dir, "data")
			}
			break
		}
		dir = filepath.Dir(dir)
	}
	config.ItemsMode = itemsAuto
	return config
}

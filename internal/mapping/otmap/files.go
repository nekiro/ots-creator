package otmap

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nekiro/ots-creator/internal/fsutil"
)

type Stage int

const (
	StageRead   Stage = iota // reading the file
	StageTiles               // reading tiles (done/total in bytes of the file)
	StageFiles               // reading houses, spawns and zones (done/total in files)
	StageEncode              // saving: encoding tiles (done/total in chunks)
	StageWrite               // saving: writing the files (done/total in files)
)

type Progress func(stage Stage, done, total int)

func Load(path string, items Items, format Format) (*Map, []string, error) {
	return LoadProgress(path, items, format, nil)
}

func LoadProgress(path string, items Items, format Format, progress Progress) (*Map, []string, error) {
	if progress == nil {
		progress = func(Stage, int, int) {}
	}
	progress(StageRead, 0, 1)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	// Server attributes must be known before the items are read.
	var warnings []string
	var customSizes map[byte]int
	attrsName := base + "-attributes.json"
	if attrsData, err := os.ReadFile(filepath.Join(dir, attrsName)); err == nil {
		sizes, attrsWarnings, err := ReadCustomAttrs(attrsData)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", attrsName, err))
		}
		for _, warning := range attrsWarnings {
			warnings = append(warnings, fmt.Sprintf("%s: %s", attrsName, warning))
		}
		customSizes = sizes
	} else if !os.IsNotExist(err) {
		warnings = append(warnings, fmt.Sprintf("%s: %v", attrsName, err))
	}
	gameMap, mapWarnings, err := readOTBM(data, items, format, customSizes, progress)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, mapWarnings...)
	progress(StageFiles, 0, 1)
	files := 0
	readAux := func(name, fallback string, fn func([]byte) ([]string, error)) string {
		files++
		progress(StageFiles, files, 4)
		if name == "" {
			name = fallback
		}
		data, err := os.ReadFile(filepath.Join(dir, baseName(name)))
		if err != nil {
			if !os.IsNotExist(err) {
				warnings = append(warnings, fmt.Sprintf("%s: %v", name, err))
			}
			return name
		}
		fileWarnings, err := fn(data)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", name, err))
		}
		warnings = append(warnings, fileWarnings...)
		return name
	}
	gameMap.HouseFile = readAux(gameMap.HouseFile, base+"-house.xml", func(fileData []byte) ([]string, error) { return ReadHouses(gameMap, fileData) })
	if gameMap.SpawnFormat == SpawnsSeparate {
		gameMap.SpawnFile = readAux(gameMap.SpawnFile, base+"-monster.xml", func(fileData []byte) ([]string, error) { return ReadSpawns(gameMap, fileData, false) })
		gameMap.NpcFile = readAux(gameMap.NpcFile, base+"-npc.xml", func(fileData []byte) ([]string, error) { return ReadSpawns(gameMap, fileData, true) })
	} else {
		gameMap.SpawnFile = readAux(gameMap.SpawnFile, base+"-spawn.xml", func(fileData []byte) ([]string, error) { return ReadSpawns(gameMap, fileData, false) })
	}
	if gameMap.ZoneFile != "" {
		gameMap.ZoneFile = readAux(gameMap.ZoneFile, base+"-zones.xml", func(fileData []byte) ([]string, error) { return nil, ReadZones(gameMap, fileData) })
	}
	// Loading spawns sets tiles; a freshly loaded map has no changes.
	gameMap.ClearModified()
	return gameMap, warnings, nil
}

func (m *Map) ClearModified() {
	for _, chunk := range m.chunks {
		// Packed tiles have no state.
		chunk.eachPtr(func(tile *Tile) { tile.State &^= StateModified })
	}
}

// Save writes the map and its auxiliary files next to it. File names come
// from the map (defaults derive from the map name). It returns warnings.
func Save(path string, gameMap *Map, items Items, format Format) ([]string, error) {
	return SaveProgress(path, gameMap, items, format, nil)
}

func SaveProgress(path string, gameMap *Map, items Items, format Format, progress Progress) ([]string, error) {
	if progress == nil {
		progress = func(Stage, int, int) {}
	}
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	defaultName := func(name *string, suffix string) {
		if *name == "" {
			*name = base + suffix
		}
		*name = baseName(*name)
	}
	defaultName(&gameMap.HouseFile, "-house.xml")
	if gameMap.SpawnFormat == SpawnsSeparate {
		defaultName(&gameMap.SpawnFile, "-monster.xml")
		defaultName(&gameMap.NpcFile, "-npc.xml")
	} else {
		defaultName(&gameMap.SpawnFile, "-spawn.xml")
	}
	if len(gameMap.Zones) > 0 {
		defaultName(&gameMap.ZoneFile, "-zones.xml")
	}
	var buf bytes.Buffer
	if err := writeOTBM(&buf, gameMap, items, format, func(done, total int) { progress(StageEncode, done, total) }); err != nil {
		return nil, err
	}
	var warnings []string
	progress(StageWrite, 0, 4)
	if err := fsutil.WriteAtomic(path, buf.Bytes()); err != nil {
		return nil, err
	}
	progress(StageWrite, 1, 4)
	if err := fsutil.WriteAtomic(filepath.Join(dir, gameMap.HouseFile), WriteHouses(gameMap)); err != nil {
		return nil, err
	}
	progress(StageWrite, 2, 4)
	monsters, npcs, unsaved := WriteSpawns(gameMap)
	if err := fsutil.WriteAtomic(filepath.Join(dir, gameMap.SpawnFile), monsters); err != nil {
		return nil, err
	}
	if npcs != nil {
		if err := fsutil.WriteAtomic(filepath.Join(dir, gameMap.NpcFile), npcs); err != nil {
			return nil, err
		}
	}
	progress(StageWrite, 3, 4)
	if gameMap.ZoneFile != "" {
		if err := fsutil.WriteAtomic(filepath.Join(dir, gameMap.ZoneFile), WriteZones(gameMap)); err != nil {
			return nil, err
		}
	}
	progress(StageWrite, 4, 4)
	if unsaved > 0 {
		warnings = append(warnings, fmt.Sprintf("%d creatures are outside every spawn and were not saved", unsaved))
	}
	gameMap.ClearModified()
	return warnings, nil
}

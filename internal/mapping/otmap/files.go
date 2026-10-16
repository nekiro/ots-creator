package otmap

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nekiro/ots-creator/internal/fsutil"
)

// Stage is a step of loading a map.
type Stage int

const (
	StageRead   Stage = iota // reading the file
	StageTiles               // reading tiles (done/total in bytes of the file)
	StageFiles               // reading houses, spawns and zones (done/total in files)
	StageEncode              // saving: encoding tiles (done/total in chunks)
	StageWrite               // saving: writing the files (done/total in files)
)

// Progress is called while a map loads.
type Progress func(stage Stage, done, total int)

// Load reads a map and its auxiliary files from disk.
func Load(path string, items Items, format Format) (*Map, []string, error) {
	return LoadProgress(path, items, format, nil)
}

// LoadProgress is Load that reports its progress (progress may be nil).
func LoadProgress(path string, items Items, format Format, progress Progress) (*Map, []string, error) {
	if progress == nil {
		progress = func(Stage, int, int) {}
	}
	progress(StageRead, 0, 1)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	m, warnings, err := readOTBM(data, items, format, progress)
	if err != nil {
		return nil, nil, err
	}
	progress(StageFiles, 0, 1)
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
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
	m.HouseFile = readAux(m.HouseFile, base+"-house.xml", func(fileData []byte) ([]string, error) { return ReadHouses(m, fileData) })
	if m.SpawnFormat == SpawnsSeparate {
		m.SpawnFile = readAux(m.SpawnFile, base+"-monster.xml", func(fileData []byte) ([]string, error) { return ReadSpawns(m, fileData, false) })
		m.NpcFile = readAux(m.NpcFile, base+"-npc.xml", func(fileData []byte) ([]string, error) { return ReadSpawns(m, fileData, true) })
	} else {
		m.SpawnFile = readAux(m.SpawnFile, base+"-spawn.xml", func(fileData []byte) ([]string, error) { return ReadSpawns(m, fileData, false) })
	}
	if m.ZoneFile != "" {
		m.ZoneFile = readAux(m.ZoneFile, base+"-zones.xml", func(fileData []byte) ([]string, error) { return nil, ReadZones(m, fileData) })
	}
	// Loading spawns sets tiles; a freshly loaded map has no changes.
	m.ClearModified()
	return m, warnings, nil
}

// ClearModified clears the modified state of every tile.
func (m *Map) ClearModified() {
	for _, chunk := range m.chunks {
		// Packed tiles have no state.
		chunk.eachPtr(func(tile *Tile) { tile.State &^= StateModified })
	}
}

// Save writes the map and its auxiliary files next to it. File names come
// from the map (defaults derive from the map name). It returns warnings.
func Save(path string, m *Map, items Items, format Format) ([]string, error) {
	return SaveProgress(path, m, items, format, nil)
}

// SaveProgress is Save that reports its progress (progress may be nil).
func SaveProgress(path string, m *Map, items Items, format Format, progress Progress) ([]string, error) {
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
	defaultName(&m.HouseFile, "-house.xml")
	if m.SpawnFormat == SpawnsSeparate {
		defaultName(&m.SpawnFile, "-monster.xml")
		defaultName(&m.NpcFile, "-npc.xml")
	} else {
		defaultName(&m.SpawnFile, "-spawn.xml")
	}
	if len(m.Zones) > 0 {
		defaultName(&m.ZoneFile, "-zones.xml")
	}
	var buf bytes.Buffer
	if err := writeOTBM(&buf, m, items, format, func(done, total int) { progress(StageEncode, done, total) }); err != nil {
		return nil, err
	}
	var warnings []string
	progress(StageWrite, 0, 4)
	if err := fsutil.WriteAtomic(path, buf.Bytes()); err != nil {
		return nil, err
	}
	progress(StageWrite, 1, 4)
	if err := fsutil.WriteAtomic(filepath.Join(dir, m.HouseFile), WriteHouses(m)); err != nil {
		return nil, err
	}
	progress(StageWrite, 2, 4)
	monsters, npcs, unsaved := WriteSpawns(m)
	if err := fsutil.WriteAtomic(filepath.Join(dir, m.SpawnFile), monsters); err != nil {
		return nil, err
	}
	if npcs != nil {
		if err := fsutil.WriteAtomic(filepath.Join(dir, m.NpcFile), npcs); err != nil {
			return nil, err
		}
	}
	progress(StageWrite, 3, 4)
	if m.ZoneFile != "" {
		if err := fsutil.WriteAtomic(filepath.Join(dir, m.ZoneFile), WriteZones(m)); err != nil {
			return nil, err
		}
	}
	progress(StageWrite, 4, 4)
	if unsaved > 0 {
		warnings = append(warnings, fmt.Sprintf("%d creatures are outside every spawn and were not saved", unsaved))
	}
	m.ClearModified()
	return warnings, nil
}

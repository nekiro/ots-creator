package app

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

const (
	// minMapSide is the smallest width and height of a new map.
	minMapSide = 32
	// maxFillTiles is the largest new map that gets its ground filled.
	maxFillTiles = 4_000_000
	// maxMinimapPixels is the largest floor ExportMinimap draws.
	maxMinimapPixels = 64_000_000
	// minimapGray is the minimap color of tiles whose items have none.
	minimapGray = 60
	// progressTotal is the Total of map progress events: Done counts
	// thousandths of the whole load or save.
	progressTotal = 1000
)

// EventMapProgress carries a Progress while a map opens or saves: Label is
// the step, Done/Total the whole load in thousandths.
const EventMapProgress = "map:progress"

// progressStep is a step of opening or saving a map: its label and the
// share of the whole work it takes, in thousandths.
type progressStep struct {
	label      string
	start, end int
}

// loadSteps are the steps of opening a map.
var loadSteps = map[otmap.Stage]progressStep{
	otmap.StageRead:  {"Reading the file", 50, 120},
	otmap.StageTiles: {"Reading tiles", 120, 900},
	otmap.StageFiles: {"Reading houses and spawns", 900, 960},
}

// saveSteps are the steps of saving a map.
var saveSteps = map[otmap.Stage]progressStep{
	otmap.StageEncode: {"Writing tiles", 0, 800},
	otmap.StageWrite:  {"Writing the files", 800, progressTotal},
}

// Progress points of opening a map outside its steps.
const (
	progressLoadingItems = 0
	progressPreparing    = 960
)

// mapProgress returns a progress callback that sends EventMapProgress at
// most once per progressEvery, and always when the step changes.
func (ms *MapService) mapProgress() func(label string, permille int) {
	var last time.Time
	lastLabel := ""
	return func(label string, permille int) {
		if label == lastLabel && time.Since(last) < progressEvery {
			return
		}
		last, lastLabel = time.Now(), label
		ms.session.notify(EventMapProgress, Progress{Label: label, Done: permille, Total: progressTotal, Percent: true})
	}
}

// stageReporter turns otmap stage progress into report calls along steps.
func stageReporter(steps map[otmap.Stage]progressStep, report func(label string, permille int)) func(otmap.Stage, int, int) {
	return func(stage otmap.Stage, done, total int) {
		step := steps[stage]
		permille := step.start
		if total > 0 {
			permille += (step.end - step.start) * min(done, total) / total
		}
		report(step.label, permille)
	}
}

func (ms *MapService) add(gameMap *otmap.Map, path string, data *mapData) *openMap {
	ms.lastID++
	editor := mapedit.New(gameMap, data.items, data.brushes)
	editor.Settings = ms.store.EditorSettings()
	editor.RebuildSelection()
	opened := &openMap{id: ms.lastID, path: path, editor: editor}
	ms.maps[opened.id] = opened
	return opened
}

// NewMapOptions configure a new map.
type NewMapOptions struct {
	Width       uint16            `json:"width"`
	Height      uint16            `json:"height"`
	SpawnFormat otmap.SpawnFormat `json:"spawnFormat"`
	Description string            `json:"description"`
	// Fill paints a ground brush over the whole area (empty: no ground).
	Fill string `json:"fill"`
}

// NewMap creates an empty map.
func (ms *MapService) NewMap(options NewMapOptions) (MapInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	data, err := ms.ensure("")
	if err != nil {
		return MapInfo{}, err
	}
	gameMap := otmap.New(max(options.Width, minMapSide), max(options.Height, minMapSide))
	if options.SpawnFormat == otmap.SpawnsSeparate {
		gameMap.SpawnFormat = otmap.SpawnsSeparate
	}
	gameMap.Description = options.Description
	gameMap.ItemsMajor, gameMap.ItemsMinor = data.items.MajorVersion, data.items.MinorVersion
	opened := ms.add(gameMap, "", data)
	if options.Fill != "" && int(gameMap.Width)*int(gameMap.Height) <= maxFillTiles {
		opened.editor.Draw(mapedit.DrawRequest{Kind: mapedit.KindGround, Name: options.Fill, Rect: &mapedit.Rect{
			From: otmap.Position{Z: otmap.GroundFloor}, To: otmap.Position{X: gameMap.Width - 1, Y: gameMap.Height - 1, Z: otmap.GroundFloor},
		}})
	}
	opened.editor.MarkUnsaved()
	opened.editor.TakeDirty()
	return ms.info(opened, true), nil
}

// OpenResult is an opened map with load warnings.
type OpenResult struct {
	Info     MapInfo  `json:"info"`
	Warnings []string `json:"warnings"`
}

// OpenMap loads an OTBM map (and its houses, spawns and zones).
func (ms *MapService) OpenMap(path string) (OpenResult, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, opened := range ms.maps {
		if fsutil.SamePath(opened.path, path) {
			return OpenResult{Info: ms.info(opened, true)}, nil
		}
	}
	report := ms.mapProgress()
	report("Loading items", progressLoadingItems)
	data, err := ms.ensure(path)
	if err != nil {
		return OpenResult{}, err
	}
	gameMap, warnings, err := otmap.LoadProgress(path, data.items, ms.store.EditorSettings().Format(), stageReporter(loadSteps, report))
	if err != nil {
		return OpenResult{}, err
	}
	items := data.items
	if items.Mode == otitems.ModeOTB && gameMap.ItemsMinor > items.MinorVersion && items.MinorVersion != 0 {
		newer := fmt.Sprintf("the map was saved with a newer items.otb (client version %d, loaded %d)", gameMap.ItemsMinor, items.MinorVersion)
		warnings = append([]string{newer}, warnings...)
	}
	report("Preparing the editor", progressPreparing)
	opened := ms.add(gameMap, path, data)
	// Loading leaves as much garbage as the map itself: give it back to the
	// system instead of keeping it for later allocations.
	go debug.FreeOSMemory()
	opened.editor.MarkSaved()
	_ = ms.store.AddRecent(RecentMap{Path: path, Client: data.client})
	return OpenResult{Info: ms.info(opened, true), Warnings: warnings}, nil
}

// SaveMap writes a map to its file (path "" keeps the current file).
func (ms *MapService) SaveMap(id int, path string) (MapChange, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	opened, data, err := ms.get(id)
	if err != nil {
		return MapChange{}, err
	}
	if path == "" {
		path = opened.path
	}
	if path == "" {
		return MapChange{}, errors.New("choose where to save the map")
	}
	path = fsutil.EnsureExt(path, ".otbm")
	if path != opened.path && opened.path != "" {
		opened.editor.Map.ForgetAuxFileNames()
	}
	if data.items.Mode == otitems.ModeOTB {
		opened.editor.Map.ItemsMajor, opened.editor.Map.ItemsMinor = data.items.MajorVersion, data.items.MinorVersion
	}
	progress := stageReporter(saveSteps, ms.mapProgress())
	warnings, err := otmap.SaveProgress(path, opened.editor.Map, data.items, opened.editor.Settings.Format(), progress)
	if err != nil {
		return MapChange{}, err
	}
	opened.path = path
	opened.editor.MarkSaved()
	_ = ms.store.AddRecent(RecentMap{Path: path, Client: data.client})
	change := ms.change(opened)
	// Saving clears the modified state of every tile.
	change.All = true
	change.Warnings = warnings
	return change, nil
}

// CloseMap closes a map without saving.
func (ms *MapService) CloseMap(id int) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.maps, id)
	go debug.FreeOSMemory()
}

// AnyChanged reports whether an open map has unsaved changes.
func (ms *MapService) AnyChanged() bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, opened := range ms.maps {
		if opened.editor.Changed() {
			return true
		}
	}
	return false
}

// RemoveRecent forgets a recent map ("" clears the list).
func (ms *MapService) RemoveRecent(path string) error { return ms.store.RemoveRecent(path) }

// ImportMap merges another map file into a map.
func (ms *MapService) ImportMap(id int, path string, options mapedit.ImportOptions) (MapChange, error) {
	var items *otitems.DB
	if err := ms.withMap(id, func(_ *openMap, data *mapData) error { items = data.items; return nil }); err != nil {
		return MapChange{}, err
	}
	// The file loads without the lock: it can take long.
	imported, warnings, err := otmap.Load(path, items, ms.store.EditorSettings().Format())
	if err != nil {
		return MapChange{}, err
	}
	change, err := ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) { return editor.Import(imported, options) })
	change.All = true
	change.Warnings = append(warnings, change.Warnings...)
	return change, err
}

// SaveImage writes a PNG made by the frontend (screenshots).
func (ms *MapService) SaveImage(path string, data []byte) error {
	return os.WriteFile(fsutil.EnsureExt(path, ".png"), data, fsutil.FileMode)
}

// minimapImage draws tiles of one floor, one pixel per tile.
func minimapImage(editor *mapedit.Editor, area image.Rectangle, floor int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, area.Dx(), area.Dy()))
	editor.Map.EachIn(area.Min.X, area.Min.Y, area.Max.X-1, area.Max.Y-1, floor, floor, func(pos otmap.Position, tile *otmap.Tile) {
		minimapColor := editor.MinimapColor(tile)
		if minimapColor == 0 && tile.Ground.ID == 0 && len(tile.Items) == 0 {
			return
		}
		red, green, blue := mapedit.MinimapRGB(minimapColor)
		if minimapColor == 0 {
			// Items without a minimap color still show where the map is.
			red, green, blue = minimapGray, minimapGray, minimapGray
		}
		img.SetNRGBA(int(pos.X)-area.Min.X, int(pos.Y)-area.Min.Y, color.NRGBA{red, green, blue, 255})
	})
	return img
}

// ExportMinimap writes one PNG per floor (1 pixel per tile) of the used
// area into dir. It returns the written files.
func (ms *MapService) ExportMinimap(id int, dir string, floors []int) (written []string, err error) {
	err = ms.withMap(id, func(opened *openMap, _ *mapData) error {
		minX, minY, maxX, maxY, ok := opened.editor.Map.Bounds()
		if !ok {
			return errors.New("the map is empty")
		}
		area := image.Rect(minX, minY, maxX+1, maxY+1)
		if area.Dx()*area.Dy() > maxMinimapPixels {
			return errors.New("the map is too big for one image")
		}
		base := fsutil.TrimExt(opened.name())
		for _, floor := range floors {
			if floor < 0 || floor > otmap.MaxFloor {
				continue
			}
			data, err := imaging.EncodePNG(minimapImage(opened.editor, area, floor))
			if err != nil {
				return err
			}
			path := filepath.Join(dir, fmt.Sprintf("%s-floor-%02d.png", base, floor))
			if err := os.WriteFile(path, data, fsutil.FileMode); err != nil {
				return err
			}
			written = append(written, path)
		}
		return nil
	})
	return written, err
}

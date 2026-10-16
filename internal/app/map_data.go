package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
)

// Item data sources of MapDataConfig.ItemsMode.
const (
	itemsAuto   = "auto"
	itemsOTB    = "otb"
	itemsClient = "client"
)

// mapData is the item database and brushes of the open client.
type mapData struct {
	project       *project.Project
	client        string
	config        MapDataConfig
	items         *otitems.DB
	materials     *materials.Materials
	materialsPath string
	brushes       *mapedit.Brushes
	// creatures holds every known creature by lower case name; the
	// materials file wins over the creatures scanned from the server.
	creatures map[string]*materials.CreatureType
	scanned   []*materials.CreatureType
	warnings  []string
}

// projectSource reads client items for the item database.
type projectSource struct{ things map[uint32]*thing.Thing }

func (source projectSource) Thing(_ thing.Category, id uint32) (*thing.Thing, error) {
	if found, ok := source.things[id]; ok {
		return found, nil
	}
	return nil, errors.New("missing")
}

func clientKey(proj *project.Project) string {
	info := proj.Info()
	if info.DatPath == "" {
		return "unsaved client " + info.Version.Name
	}
	return info.DatPath
}

// dataConfig returns config, or when nil the stored profile of the client,
// completed with server data found near hint (a map path).
func (ms *MapService) dataConfig(config *MapDataConfig, client, hint string) MapDataConfig {
	var resolved MapDataConfig
	if config != nil {
		resolved = *config
	} else {
		if stored, ok := ms.store.Profile(client); ok {
			resolved = stored
		}
		if resolved.XMLPath == "" && resolved.OTBPath == "" && hint != "" {
			detected := detectServerData(hint)
			resolved.OTBPath, resolved.XMLPath = detected.OTBPath, detected.XMLPath
			if resolved.ServerDir == "" {
				resolved.ServerDir = detected.ServerDir
			}
		}
	}
	if resolved.ItemsMode == "" {
		resolved.ItemsMode = itemsAuto
	}
	return resolved
}

// loadData builds the item database and brushes for the open client.
func (ms *MapService) loadData(config *MapDataConfig, hint string) (*mapData, error) {
	proj, err := ms.session.Project()
	if err != nil {
		return nil, errNoClient
	}
	client := clientKey(proj)
	data := &mapData{project: proj, client: client, config: ms.dataConfig(config, client, hint)}
	if err := data.loadItems(); err != nil {
		return nil, err
	}
	data.loadMaterials(ms.store.Dir())
	if dir := data.config.ServerDir; dir != "" {
		if creatures, err := materials.ScanCreatures(dir); err == nil {
			data.scanned = creatures
		}
	}
	data.index()
	return data, nil
}

// loadItems reads the item database: from items.otb when the config uses
// one, else from the client; then names from items.xml.
func (data *mapData) loadItems() error {
	config, info := data.config, data.project.Info()
	source := projectSource{things: map[uint32]*thing.Thing{}}
	for _, item := range data.project.ThingRange(thing.CategoryItem, thing.CategoryItem.MinID(), info.Counts.Items) {
		source.things[item.ID] = item
	}
	useOTB := config.ItemsMode == itemsOTB ||
		(config.ItemsMode == itemsAuto && config.OTBPath != "" && fsutil.FileExists(config.OTBPath))
	if useOTB {
		otb, err := os.ReadFile(config.OTBPath)
		if err != nil {
			return fmt.Errorf("items.otb: %w", err)
		}
		if data.items, err = otitems.ParseOTB(otb); err != nil {
			return err
		}
		data.items.ApplyClient(source)
	} else {
		data.items = otitems.FromClient(source, info.Counts.Items, data.project.Names(thing.CategoryItem))
	}
	if xml, err := fsutil.ReadOptional(config.XMLPath); err != nil {
		data.warnings = append(data.warnings, "items.xml: "+err.Error())
	} else if xml != nil {
		if err := data.items.ApplyXML(xml); err != nil {
			data.warnings = append(data.warnings, err.Error())
		}
	}
	return nil
}

// loadMaterials reads the brush file: the configured one, else one per
// client version and item mode in storeDir.
func (data *mapData) loadMaterials(storeDir string) {
	data.materialsPath = data.config.MaterialsPath
	if data.materialsPath == "" && storeDir != "" {
		name := fmt.Sprintf("%d-%s.json", data.project.Info().Version.Value, data.items.Mode)
		data.materialsPath = filepath.Join(storeDir, "materials", name)
	}
	data.materials = materials.New()
	if data.materialsPath == "" {
		return
	}
	if loaded, err := materials.Load(data.materialsPath); err == nil {
		data.materials = loaded
	} else if !os.IsNotExist(err) {
		data.warnings = append(data.warnings, err.Error())
	}
}

// index rebuilds what derives from the items and materials.
func (data *mapData) index() {
	data.brushes = mapedit.NewBrushes(data.items, data.materials)
	data.creatures = map[string]*materials.CreatureType{}
	for _, creature := range data.allCreatures() {
		data.creatures[strings.ToLower(creature.Name)] = creature
	}
}

// allCreatures lists the creatures of the materials file and the scanned
// ones it does not override, sorted by name.
func (data *mapData) allCreatures() []*materials.CreatureType {
	var all []*materials.CreatureType
	defined := map[string]bool{}
	for _, creature := range data.materials.Creatures {
		defined[strings.ToLower(creature.Name)] = true
		all = append(all, creature)
	}
	for _, creature := range data.scanned {
		if !defined[strings.ToLower(creature.Name)] {
			all = append(all, creature)
		}
	}
	slices.SortFunc(all, func(first, second *materials.CreatureType) int {
		return strings.Compare(strings.ToLower(first.Name), strings.ToLower(second.Name))
	})
	return all
}

func (data *mapData) outfit(creature string) (materials.Outfit, bool) {
	if found := data.creatures[strings.ToLower(creature)]; found != nil {
		return found.Outfit, true
	}
	return materials.Outfit{}, false
}

// ensure returns the data of the open client, building it when the client
// changed.
func (ms *MapService) ensure(hint string) (*mapData, error) {
	proj, err := ms.session.Project()
	if err != nil {
		return nil, errNoClient
	}
	if ms.data != nil && ms.data.project == proj {
		return ms.data, nil
	}
	data, err := ms.loadData(nil, hint)
	if err != nil {
		return nil, err
	}
	ms.setData(data)
	return data, nil
}

func (ms *MapService) setData(data *mapData) {
	ms.data = data
	for _, opened := range ms.maps {
		opened.editor.Items = data.items
		opened.editor.SetBrushes(data.brushes)
	}
}

// DataInfo describes the item and brush data of the map editor.
type DataInfo struct {
	Ready      bool          `json:"ready"`
	Client     string        `json:"client"`
	Config     MapDataConfig `json:"config"`
	Mode       otitems.Mode  `json:"mode"`
	Items      int           `json:"items"`
	OTBVersion string        `json:"otbVersion"`
	Materials  string        `json:"materials"`
	Brushes    int           `json:"brushes"`
	Tilesets   int           `json:"tilesets"`
	Creatures  int           `json:"creatures"`
	Warnings   []string      `json:"warnings"`
}

func (data *mapData) info() DataInfo {
	info := DataInfo{
		Ready: true, Client: data.client, Config: data.config, Mode: data.items.Mode, Items: data.items.Count(),
		Materials: data.materialsPath, Brushes: len(data.materials.Brushes), Tilesets: len(data.materials.Tilesets),
		Creatures: len(data.creatures), Warnings: slices.Concat(data.warnings, data.brushes.Warnings),
	}
	if data.items.Mode == otitems.ModeOTB {
		info.OTBVersion = fmt.Sprintf("%d.%d (build %d)", data.items.MajorVersion, data.items.MinorVersion, data.items.BuildNumber)
	}
	return info
}

// MapStatus is the state of the map editor.
type MapStatus struct {
	Data     DataInfo         `json:"data"`
	Maps     []MapInfo        `json:"maps"`
	Recent   []RecentMap      `json:"recent"`
	Settings mapedit.Settings `json:"settings"`
	// Keys are the keyboard shortcuts changed by the user.
	Keys map[string][]string `json:"keys"`
}

// Status returns the data info, the open maps and the recent maps.
func (ms *MapService) Status() MapStatus {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	status := MapStatus{Recent: ms.store.Recent(), Settings: ms.store.EditorSettings(), Keys: ms.store.Keys()}
	if data, err := ms.ensure(""); err == nil {
		status.Data = data.info()
	}
	ids := make([]int, 0, len(ms.maps))
	for id := range ms.maps {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		status.Maps = append(status.Maps, ms.info(ms.maps[id], true))
	}
	return status
}

// DetectData looks for items.otb and items.xml from a folder or map path.
func (ms *MapService) DetectData(path string) MapDataConfig { return detectServerData(path) }

// SetData changes where the item data and brushes come from and reloads
// them.
func (ms *MapService) SetData(config MapDataConfig) (DataInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	data, err := ms.loadData(&config, "")
	if err != nil {
		return DataInfo{}, err
	}
	if err := ms.store.SetProfile(data.client, config); err != nil {
		data.warnings = append(data.warnings, "settings: "+err.Error())
	}
	ms.setData(data)
	return data.info(), nil
}

// Reload rebuilds the item data from the client (after object edits).
func (ms *MapService) Reload() (DataInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	config := MapDataConfig{}
	if ms.data != nil {
		config = ms.data.config
	}
	data, err := ms.loadData(&config, "")
	if err != nil {
		return DataInfo{}, err
	}
	ms.setData(data)
	return data.info(), nil
}

// SetEditorSettings changes brush behavior settings.
func (ms *MapService) SetEditorSettings(settings mapedit.Settings) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, opened := range ms.maps {
		opened.editor.Settings = settings
	}
	return ms.store.SetEditorSettings(settings)
}

// SetKeys stores the keyboard shortcuts changed by the user (action id ->
// key combinations); nil restores the defaults.
func (ms *MapService) SetKeys(keys map[string][]string) error {
	return ms.store.SetKeys(keys)
}

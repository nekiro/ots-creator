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

const (
	itemsAuto   = "auto"
	itemsOTB    = "otb"
	itemsClient = "client"
)

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
	creatures  map[string]*materials.CreatureType
	scanned    []*materials.CreatureType
	warnings   []string
	clientData *mapData
}

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

func (data *mapData) index() {
	data.brushes = mapedit.NewBrushes(data.items, data.materials)
	data.creatures = map[string]*materials.CreatureType{}
	for _, creature := range data.allCreatures() {
		data.creatures[strings.ToLower(creature.Name)] = creature
	}
}

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
		if opened.clientData != nil {
			opened.clientData = data.asClientIDs(ms.store.Dir())
		}
		resolved := ms.dataFor(opened)
		opened.editor.Items = resolved.items
		opened.editor.SetBrushes(resolved.brushes)
	}
}

// dataFor resolves each map independently of the selected tab.
func (ms *MapService) dataFor(opened *openMap) *mapData {
	if opened.clientData != nil && opened.editor.Items.Mode == otitems.ModeClient {
		return opened.clientData
	}
	return ms.data
}

// asClientIDs builds the client palette without applying server-ID XML or brushes.
func (data *mapData) asClientIDs(storeDir string) *mapData {
	if data.items.Mode == otitems.ModeClient {
		return data
	}
	if data.clientData != nil {
		return data.clientData
	}
	info := data.project.Info()
	source := projectSource{things: map[uint32]*thing.Thing{}}
	for _, item := range data.project.ThingRange(thing.CategoryItem, thing.CategoryItem.MinID(), info.Counts.Items) {
		source.things[item.ID] = item
	}
	converted := &mapData{
		project: data.project, client: data.client, scanned: data.scanned,
		config: MapDataConfig{ItemsMode: itemsClient, ServerDir: data.config.ServerDir},
		items:  otitems.FromClient(source, info.Counts.Items, data.project.Names(thing.CategoryItem)),
	}
	converted.loadMaterials(storeDir)
	converted.index()
	data.clientData = converted
	return converted
}

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

type MapStatus struct {
	Data     DataInfo            `json:"data"`
	Maps     []MapInfo           `json:"maps"`
	Recent   []RecentMap         `json:"recent"`
	Settings mapedit.Settings    `json:"settings"`
	Keys     map[string][]string `json:"keys"`
}

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

func (ms *MapService) DetectData(path string) MapDataConfig { return detectServerData(path) }

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

type EditorPrefs struct {
	Settings mapedit.Settings    `json:"settings"`
	Keys     map[string][]string `json:"keys"`
}

// EditorPrefs gives the map editor settings and keys without loading the
// server data, for the settings dialog of the object editor.
func (ms *MapService) EditorPrefs() EditorPrefs {
	return EditorPrefs{Settings: ms.store.EditorSettings(), Keys: ms.store.Keys()}
}

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

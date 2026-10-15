package app

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/mapedit"
	"github.com/nekiro/ots-creator/internal/materials"
	"github.com/nekiro/ots-creator/internal/otitems"
	"github.com/nekiro/ots-creator/internal/otmap"
	"github.com/nekiro/ots-creator/internal/thing"
)

// TileItem is one item of the tile inspector.
type TileItem struct {
	ID        uint16         `json:"id"`
	ClientID  uint16         `json:"clientId"`
	Name      string         `json:"name"`
	Subtype   uint16         `json:"subtype"`
	Selected  bool           `json:"selected"`
	Ext       *otmap.ItemExt `json:"ext"`
	Kind      string         `json:"kind"`
	Flags     otitems.Flags  `json:"flags"`
	TopOrder  uint8          `json:"topOrder"`
	Brush     string         `json:"brush"`
	BrushKind string         `json:"brushKind"`
	RotateTo  uint16         `json:"rotateTo"`
	Contents  []TileItem     `json:"contents"`
}

// TileCreature is a creature with its look.
type TileCreature struct {
	otmap.Creature
	Outfit  materials.Outfit `json:"outfit"`
	Unknown bool             `json:"unknown"`
}

// TileInfo is everything on a tile, for the inspector.
type TileInfo struct {
	Pos       otmap.Position `json:"pos"`
	Exists    bool           `json:"exists"`
	Ground    *TileItem      `json:"ground"`
	Items     []TileItem     `json:"items"`
	Flags     uint32         `json:"flags"`
	HouseID   uint32         `json:"houseId"`
	HouseName string         `json:"houseName"`
	Creatures []TileCreature `json:"creatures"`
	Spawn     *otmap.Spawn   `json:"spawn"`
	SpawnNPC  *otmap.Spawn   `json:"spawnNpc"`
	Zones     []uint16       `json:"zones"`
	Modified  bool           `json:"modified"`
	// HouseExit names the house whose exit is here.
	HouseExit string `json:"houseExit"`
	// Waypoints lists waypoints on the tile; Temple the town whose temple
	// is here.
	Waypoints []string `json:"waypoints"`
	Temple    string   `json:"temple"`
}

func (d *mapData) tileItem(it otmap.Item) TileItem {
	ti := TileItem{ID: it.ID, Subtype: it.Subtype, Selected: it.Selected, Ext: it.Ext}
	if t := d.items.Get(it.ID); t != nil {
		ti.ClientID, ti.Name, ti.Flags, ti.TopOrder, ti.RotateTo = t.ClientID, t.DisplayName(), t.Flags, t.TopOrder, t.RotateTo
		ti.Kind = t.Kind.String()
		switch {
		case ti.Kind != "":
		case t.IsContainer():
			ti.Kind = "container"
		case t.IsDoor():
			ti.Kind = "door"
		case t.IsTeleport():
			ti.Kind = "teleport"
		}
		if ti.Kind == "" && t.Flags&(otitems.FlagSplash|otitems.FlagFluidContainer) != 0 {
			ti.Kind = "fluid"
		}
		if ti.Kind == "" && t.Flags.Has(otitems.FlagStackable) {
			ti.Kind = "stackable"
		}
		if ti.Kind == "" && t.Flags.Has(otitems.FlagWritable) {
			ti.Kind = "writeable"
		}
		if ti.Kind == "" && t.Flags.Has(otitems.FlagCharges) {
			ti.Kind = "charges"
		}
	} else {
		ti.Name = fmt.Sprintf("unknown item %d", it.ID)
		ti.Kind = "invalid"
	}
	ti.Brush, ti.BrushKind = d.brushes.BrushOf(it.ID)
	if it.Ext != nil {
		for _, c := range it.Ext.Items {
			ti.Contents = append(ti.Contents, d.tileItem(c))
		}
	}
	return ti
}

func (d *mapData) outfit(name string) (materials.Outfit, bool) {
	if c := d.creatures[strings.ToLower(name)]; c != nil {
		return c.Outfit, true
	}
	return materials.Outfit{}, false
}

// Tile returns the inspector data of a tile.
func (ms *MapService) Tile(id int, p otmap.Position) (TileInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, d, err := ms.get(id)
	if err != nil {
		return TileInfo{}, err
	}
	info := TileInfo{Pos: p}
	for _, h := range m.ed.Map.Houses {
		if h.Exit == p {
			info.HouseExit = h.Name
		}
	}
	for _, w := range m.ed.Map.Waypoints {
		if w.Pos == p {
			info.Waypoints = append(info.Waypoints, w.Name)
		}
	}
	for _, t := range m.ed.Map.Towns {
		if t.Temple == p {
			info.Temple = t.Name
		}
	}
	t := m.ed.Map.Tile(p)
	if t == nil {
		return info, nil
	}
	info.Exists = true
	if t.Ground.ID != 0 {
		g := d.tileItem(t.Ground)
		info.Ground = &g
	}
	for _, it := range t.Items {
		info.Items = append(info.Items, d.tileItem(it))
	}
	info.Flags, info.HouseID, info.Zones = t.Flags, t.HouseID, t.Zones()
	if h := m.ed.Map.Houses[t.HouseID]; h != nil {
		info.HouseName = h.Name
	}
	for _, c := range t.Creatures() {
		o, ok := d.outfit(c.Name)
		info.Creatures = append(info.Creatures, TileCreature{Creature: c, Outfit: o, Unknown: !ok})
	}
	info.Spawn, info.SpawnNPC = t.Spawn(), t.SpawnNPC()
	info.Modified = t.State&otmap.StateModified != 0
	return info, nil
}

// UpdateItem changes an item's attributes.
func (ms *MapService) UpdateItem(id int, r mapedit.ItemRef, p mapedit.ItemPatch) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.UpdateItem(r, p) })
}

// RemoveItem deletes an item.
func (ms *MapService) RemoveItem(id int, r mapedit.ItemRef) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.RemoveItem(r) })
}

// AddItem puts an item on a tile.
func (ms *MapService) AddItem(id int, p otmap.Position, item uint16) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.AddItem(p, item) })
}

// AddToContainer puts an item into a container.
func (ms *MapService) AddToContainer(id int, r mapedit.ItemRef, item uint16) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.AddToContainer(r, item) })
}

// MoveItem reorders an item of a tile.
func (ms *MapService) MoveItem(id int, p otmap.Position, from, to int) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.MoveItem(p, from, to) })
}

// RotateItem rotates an item.
func (ms *MapService) RotateItem(id int, r mapedit.ItemRef) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.RotateItem(r) })
}

// SwitchDoor opens or closes a door.
func (ms *MapService) SwitchDoor(id int, p otmap.Position, index int) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.SwitchDoor(p, index) })
}

// UpdateTile changes tile flags, house or zones.
func (ms *MapService) UpdateTile(id int, p otmap.Position, patch mapedit.TilePatch) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { e.UpdateTile(p, patch); return 0, nil })
}

// UpdateCreature changes or removes a creature.
func (ms *MapService) UpdateCreature(id int, p otmap.Position, index int, patch mapedit.CreaturePatch) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.UpdateCreature(p, index, patch) })
}

// UpdateSpawn changes the radius of a spawn (0 removes it).
func (ms *MapService) UpdateSpawn(id int, p otmap.Position, npc bool, radius int32) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.UpdateSpawn(p, npc, radius) })
}

// SetProperties changes the map header.
func (ms *MapService) SetProperties(id int, p mapedit.Properties) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.SetProperties(p) })
}

// SaveHouse creates (ID 0) or updates a house.
func (ms *MapService) SaveHouse(id int, h otmap.House) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		hid, err := e.SaveHouse(h)
		return int(hid), err
	})
}

// RemoveHouse deletes a house.
func (ms *MapService) RemoveHouse(id int, house uint32) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { e.RemoveHouse(house); return 0, nil })
}

// HouseTiles returns the tiles of a house.
func (ms *MapService) HouseTiles(id int, house uint32) ([]otmap.Position, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, _, err := ms.get(id)
	if err != nil {
		return nil, err
	}
	return m.ed.HouseTiles(house), nil
}

// SaveTown creates (ID 0) or updates a town.
func (ms *MapService) SaveTown(id int, t otmap.Town) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		tid, err := e.SaveTown(t)
		return int(tid), err
	})
}

// RemoveTown deletes a town.
func (ms *MapService) RemoveTown(id int, town uint32) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { e.RemoveTown(town); return 0, nil })
}

// SaveWaypoint creates or changes a waypoint (old "" creates).
func (ms *MapService) SaveWaypoint(id int, old string, w otmap.Waypoint) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.SaveWaypoint(old, w) })
}

// RemoveWaypoint deletes a waypoint.
func (ms *MapService) RemoveWaypoint(id int, name string) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { e.RemoveWaypoint(name); return 0, nil })
}

// SaveZone creates (ID 0) or renames a zone.
func (ms *MapService) SaveZone(id int, z otmap.Zone) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		zid, err := e.SaveZone(z)
		return int(zid), err
	})
}

// RemoveZone deletes a zone.
func (ms *MapService) RemoveZone(id int, zone uint16) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { e.RemoveZone(zone); return 0, nil })
}

// ImportMap merges another map file into a map.
func (ms *MapService) ImportMap(id int, path string, o mapedit.ImportOptions) (MapChange, error) {
	ms.mu.Lock()
	m, d, err := ms.get(id)
	ms.mu.Unlock()
	if err != nil {
		return MapChange{}, err
	}
	src, warns, err := otmap.Load(path, d.items, ms.store.EditorSettings().Format())
	if err != nil {
		return MapChange{}, err
	}
	ch, err := ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return e.Import(src, o) })
	_ = m
	ch.All = true
	ch.Warnings = append(warns, ch.Warnings...)
	return ch, err
}

// minimapImage draws tiles of one floor, one pixel per tile.
func minimapImage(e *mapedit.Editor, x0, y0, w, h, z int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	e.Map.EachIn(x0, y0, x0+w-1, y0+h-1, z, z, func(p otmap.Position, t *otmap.Tile) {
		c := e.MinimapColor(t)
		if c == 0 && t.Ground.ID == 0 && len(t.Items) == 0 {
			return
		}
		r, g, b := mapedit.MinimapRGB(c)
		if c == 0 {
			// Items without a minimap color still show where the map is.
			r, g, b = 60, 60, 60
		}
		img.SetNRGBA(int(p.X)-x0, int(p.Y)-y0, color.NRGBA{r, g, b, 255})
	})
	return img
}

// ExportMinimap writes one PNG per floor (1 pixel per tile) of the used
// area into dir. It returns the written files.
func (ms *MapService) ExportMinimap(id int, dir string, floors []int) ([]string, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, _, err := ms.get(id)
	if err != nil {
		return nil, err
	}
	x0, y0, x1, y1, ok := m.ed.Map.Bounds()
	if !ok {
		return nil, errors.New("the map is empty")
	}
	w, h := x1-x0+1, y1-y0+1
	if w*h > 64_000_000 {
		return nil, errors.New("the map is too big for one image")
	}
	base := strings.TrimSuffix(m.name(), filepath.Ext(m.name()))
	var out []string
	for _, z := range floors {
		if z < 0 || z > otmap.MaxFloor {
			continue
		}
		data, err := imaging.EncodePNG(minimapImage(m.ed, x0, y0, w, h, z))
		if err != nil {
			return out, err
		}
		p := filepath.Join(dir, fmt.Sprintf("%s-floor-%02d.png", base, z))
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return out, err
		}
		out = append(out, p)
	}
	return out, nil
}

// ItemTable lists every item type in parallel arrays (compact for the
// palette, search and drawing).
type ItemTable struct {
	IDs      []uint16          `json:"ids"`
	Clients  []uint16          `json:"clients"`
	Flags    []uint32          `json:"flags"`
	Tops     []uint16          `json:"tops"`
	Minimap  []uint16          `json:"minimap"`
	Names    []string          `json:"names"`
	Kinds    []uint16          `json:"kinds"`
	Mode     string            `json:"mode"`
	MaxID    uint16            `json:"maxId"`
	Flagbits map[string]uint32 `json:"flagbits"`
}

// Items returns the item table.
func (ms *MapService) Items() (ItemTable, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return ItemTable{}, err
	}
	t := ItemTable{Mode: string(d.items.Mode), MaxID: d.items.MaxID(), Flagbits: map[string]uint32{
		"ground": uint32(otitems.FlagGround), "unpassable": uint32(otitems.FlagUnpassable), "blockMissile": uint32(otitems.FlagBlockMissile),
		"blockPath": uint32(otitems.FlagBlockPath), "pickupable": uint32(otitems.FlagPickupable), "moveable": uint32(otitems.FlagMoveable),
		"stackable": uint32(otitems.FlagStackable), "fluidContainer": uint32(otitems.FlagFluidContainer), "splash": uint32(otitems.FlagSplash),
		"hangable": uint32(otitems.FlagHangable), "hookSouth": uint32(otitems.FlagHookSouth), "hookEast": uint32(otitems.FlagHookEast),
		"rotatable": uint32(otitems.FlagRotatable), "readable": uint32(otitems.FlagReadable), "writable": uint32(otitems.FlagWritable),
		"container": uint32(otitems.FlagContainer), "floorChange": uint32(otitems.FlagFloorChange), "elevation": uint32(otitems.FlagElevation),
		"fullGround": uint32(otitems.FlagFullGround), "charges": uint32(otitems.FlagCharges), "corpse": uint32(otitems.FlagCorpse),
		"meta": uint32(otitems.FlagMeta), "light": uint32(otitems.FlagLight),
	}}
	d.items.Each(func(it *otitems.Type) {
		t.IDs = append(t.IDs, it.ID)
		t.Clients = append(t.Clients, it.ClientID)
		t.Flags = append(t.Flags, uint32(it.Flags))
		t.Tops = append(t.Tops, uint16(it.TopOrder))
		t.Minimap = append(t.Minimap, it.MinimapColor)
		t.Names = append(t.Names, it.Name)
		t.Kinds = append(t.Kinds, uint16(it.Kind))
	})
	return t, nil
}

// Appearance is what the map renderer needs to draw a client thing.
type Appearance struct {
	ID        uint32   `json:"id"`
	Width     uint8    `json:"w"`
	Height    uint8    `json:"h"`
	Layers    uint8    `json:"l"`
	PatternX  uint8    `json:"px"`
	PatternY  uint8    `json:"py"`
	PatternZ  uint8    `json:"pz"`
	Frames    uint8    `json:"f"`
	Sprites   []uint32 `json:"s"`
	Durations []uint32 `json:"d,omitempty"`
	// Walk is the walking frame group of outfits (for nothing but idle
	// fallbacks it is nil).
	OffsetX   int16  `json:"ox,omitempty"`
	OffsetY   int16  `json:"oy,omitempty"`
	Elevation uint16 `json:"el,omitempty"`
	// Flags: 1 animate always, 2 hangable, 4 hook south, 8 hook east,
	// 16 lying object, 32 translucent, 64 top, 128 bottom, 256 border,
	// 512 ground, 1024 dont hide, 2048 full ground.
	Flags      uint32 `json:"fl"`
	LightLevel uint16 `json:"ll,omitempty"`
	LightColor uint16 `json:"lc,omitempty"`
}

// Appearances returns render data of client things (missing ids are left
// out).
func (ms *MapService) Appearances(c thing.Category, ids []uint32) ([]Appearance, error) {
	p, err := ms.s.Project()
	if err != nil {
		return nil, errNoClient
	}
	out := make([]Appearance, 0, len(ids))
	for _, id := range ids {
		t, err := p.Thing(c, id)
		if err != nil || len(t.FrameGroups) == 0 {
			continue
		}
		g := t.FrameGroups[0]
		a := Appearance{ID: id, Width: g.Width, Height: g.Height, Layers: g.Layers, PatternX: g.PatternX, PatternY: g.PatternY,
			PatternZ: g.PatternZ, Frames: g.Frames, Sprites: g.Sprites}
		if g.Frames > 1 {
			for _, d := range g.Durations {
				a.Durations = append(a.Durations, (d.Min+d.Max)/2)
			}
		}
		pr := &t.Props
		if pr.HasOffset {
			a.OffsetX, a.OffsetY = pr.OffsetX, pr.OffsetY
		}
		if pr.HasElevation {
			a.Elevation = pr.Elevation
		}
		if pr.HasLight {
			a.LightLevel, a.LightColor = pr.LightLevel, pr.LightColor
		}
		for _, f := range []struct {
			on  bool
			bit uint32
		}{
			{pr.AnimateAlways, 1}, {pr.Hangable, 2}, {pr.HookSouth, 4}, {pr.HookEast, 8}, {pr.LyingObject, 16},
			{pr.Translucent, 32}, {pr.OnTop, 64}, {pr.OnBottom, 128}, {pr.GroundBorder, 256}, {pr.Ground, 512},
			{pr.DontHide, 1024}, {pr.FullGround, 2048},
		} {
			if f.on {
				a.Flags |= f.bit
			}
		}
		out = append(out, a)
	}
	return out, nil
}

// PaletteBrush is a brush shown in the palette.
type PaletteBrush struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Look is the server id of the item shown.
	Look uint16 `json:"look"`
	// Preview shows a brush that places a group of tiles (a stamp made from
	// a selection) as the whole group; nil for one-tile brushes.
	Preview *BrushPreview `json:"preview,omitempty"`
}

// BrushPreview is the ground floor of the first group of a doodad brush,
// at most previewMax tiles a side: W x H tiles, items bottom to top.
type BrushPreview struct {
	W     int           `json:"w"`
	H     int           `json:"h"`
	Tiles []PreviewTile `json:"tiles"`
}

// PreviewTile is one tile of a BrushPreview.
type PreviewTile struct {
	X     int      `json:"x"`
	Y     int      `json:"y"`
	Items []uint16 `json:"items"`
}

const previewMax = 12

func brushPreview(b *materials.Brush) *BrushPreview {
	if len(b.Alternatives) == 0 || len(b.Alternatives[0].Composites) == 0 {
		return nil
	}
	tiles := b.Alternatives[0].Composites[0].Tiles
	minX, minY, maxX, maxY := 1<<30, 1<<30, -(1 << 30), -(1 << 30)
	for _, t := range tiles {
		if t.Z == 0 {
			minX, minY, maxX, maxY = min(minX, t.X), min(minY, t.Y), max(maxX, t.X), max(maxY, t.Y)
		}
	}
	if minX > maxX || (minX == maxX && minY == maxY) {
		return nil
	}
	p := &BrushPreview{W: min(maxX-minX+1, previewMax), H: min(maxY-minY+1, previewMax)}
	for _, t := range tiles {
		x, y := t.X-minX, t.Y-minY
		if t.Z != 0 || x >= p.W || y >= p.H {
			continue
		}
		pt := PreviewTile{X: x, Y: y}
		for _, it := range t.Items {
			pt.Items = append(pt.Items, it.ID)
		}
		p.Tiles = append(p.Tiles, pt)
	}
	return p
}

// PaletteTileset is a tileset with resolved brushes.
type PaletteTileset struct {
	Name    string         `json:"name"`
	Auto    bool           `json:"auto"`
	Terrain []PaletteBrush `json:"terrain"`
	Doodads []PaletteBrush `json:"doodads"`
	Items   []uint16       `json:"items"`
	Raw     []uint16       `json:"raw"`
}

// Palette is everything the brush palette shows.
type Palette struct {
	Tilesets  []PaletteTileset          `json:"tilesets"`
	Brushes   []PaletteBrush            `json:"brushes"`
	Creatures []*materials.CreatureType `json:"creatures"`
	DoorTypes []string                  `json:"doorTypes"`
}

// Palette returns tilesets, brushes and creatures.
func (ms *MapService) Palette() (Palette, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return Palette{}, err
	}
	pal := Palette{DoorTypes: materials.DoorTypes}
	brush := func(name string) (PaletteBrush, bool) {
		b := d.mats.Brush(name)
		if b == nil {
			return PaletteBrush{}, false
		}
		kind := b.Type
		if b.Type == materials.TypeWall && len(b.Walls) == 0 && len(b.Alternatives) > 0 {
			kind = materials.TypeDoodad
		}
		look := d.brushes.Look(b.Name)
		if look == 0 {
			look = b.Look
		}
		return PaletteBrush{Name: b.Name, Kind: kind, Look: look, Preview: brushPreview(b)}, true
	}
	for _, b := range d.mats.Brushes {
		if pb, ok := brush(b.Name); ok {
			pal.Brushes = append(pal.Brushes, pb)
		}
	}
	sets := slices.Clone(d.mats.Tilesets)
	sets = append(sets, materials.AutoTilesets(d.items)...)
	for _, ts := range sets {
		pt := PaletteTileset{Name: ts.Name, Auto: ts.Auto, Items: ts.Items, Raw: ts.Raw}
		for _, n := range ts.Terrain {
			if pb, ok := brush(n); ok {
				pt.Terrain = append(pt.Terrain, pb)
			}
		}
		for _, n := range ts.Doodads {
			if pb, ok := brush(n); ok {
				pt.Doodads = append(pt.Doodads, pb)
			}
		}
		pal.Tilesets = append(pal.Tilesets, pt)
	}
	seen := map[string]bool{}
	for _, c := range d.mats.Creatures {
		seen[strings.ToLower(c.Name)] = true
		pal.Creatures = append(pal.Creatures, c)
	}
	for _, c := range d.scanned {
		if !seen[strings.ToLower(c.Name)] {
			pal.Creatures = append(pal.Creatures, c)
		}
	}
	slices.SortFunc(pal.Creatures, func(a, b *materials.CreatureType) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return pal, nil
}

// saveMaterials writes the brush file and reindexes brushes.
func (ms *MapService) saveMaterials(d *mapData) error {
	d.index()
	ms.setData(d)
	if d.matsPath == "" {
		return errors.New("no materials file")
	}
	if err := os.MkdirAll(filepath.Dir(d.matsPath), 0o755); err != nil {
		return err
	}
	return d.mats.Save(d.matsPath)
}

// MaterialsResult reports a materials change.
type MaterialsResult struct {
	Data     DataInfo `json:"data"`
	Warnings []string `json:"warnings"`
}

// ImportRME merges brushes from a Remere's Map Editor data folder.
func (ms *MapService) ImportRME(dir string) (MaterialsResult, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return MaterialsResult{}, err
	}
	found, err := materials.FindRMEData(dir, int(d.project.Info().Version.Value))
	if err != nil {
		return MaterialsResult{}, err
	}
	m, warns, err := materials.ImportRME(found, func(c uint16) (uint16, bool) { return d.items.FromClient(c) })
	if err != nil {
		return MaterialsResult{}, err
	}
	d.mats.Merge(m)
	if err := ms.saveMaterials(d); err != nil {
		return MaterialsResult{}, err
	}
	return MaterialsResult{Data: ms.dataInfo(d), Warnings: warns}, nil
}

// ImportMaterials merges a materials JSON file.
func (ms *MapService) ImportMaterials(path string) (MaterialsResult, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return MaterialsResult{}, err
	}
	m, err := materials.Load(path)
	if err != nil {
		return MaterialsResult{}, err
	}
	d.mats.Merge(m)
	if err := ms.saveMaterials(d); err != nil {
		return MaterialsResult{}, err
	}
	return MaterialsResult{Data: ms.dataInfo(d)}, nil
}

// ExportMaterials writes the materials to a JSON file.
func (ms *MapService) ExportMaterials(path string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return err
	}
	return d.mats.Save(path)
}

// Brush returns a brush definition for the brush editor.
func (ms *MapService) Brush(name string) (*materials.Brush, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return nil, err
	}
	b := d.mats.Brush(name)
	if b == nil {
		return nil, fmt.Errorf("no brush %q", name)
	}
	return b, nil
}

// Borders returns the border definitions.
func (ms *MapService) Borders() ([]*materials.Border, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return nil, err
	}
	return d.mats.Borders, nil
}

// SaveBorder creates (ID 0) or replaces a border and returns its id.
func (ms *MapService) SaveBorder(b materials.Border) (uint32, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return 0, err
	}
	if b.ID == 0 {
		b.ID = d.mats.NextBorderID()
	}
	if cur := d.mats.Border(b.ID); cur != nil {
		*cur = b
	} else {
		d.mats.Borders = append(d.mats.Borders, &b)
	}
	return b.ID, ms.saveMaterials(d)
}

// SaveBrush creates or replaces a brush; old is its current name ("" for
// a new brush). tilesets, when not nil, are the collections the brush is in
// afterwards: missing ones are created, others no longer list it.
func (ms *MapService) SaveBrush(old string, b materials.Brush, tilesets []string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return err
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		return errors.New("the brush needs a name")
	}
	if !strings.EqualFold(old, b.Name) && d.mats.Brush(b.Name) != nil {
		return fmt.Errorf("a brush named %q exists", b.Name)
	}
	if old != "" && d.mats.Brush(old) != nil {
		if !strings.EqualFold(old, b.Name) {
			d.mats.RenameBrush(old, b.Name)
		}
		*d.mats.Brush(b.Name) = b
	} else {
		cp := b
		d.mats.Brushes = append(d.mats.Brushes, &cp)
	}
	if tilesets != nil {
		same := func(s string) bool { return strings.EqualFold(s, b.Name) }
		want := map[string]bool{}
		for _, n := range tilesets {
			if n = strings.TrimSpace(n); n != "" {
				want[strings.ToLower(n)] = true
				if d.mats.Tileset(n) == nil {
					d.mats.Tilesets = append(d.mats.Tilesets, &materials.Tileset{Name: n})
				}
			}
		}
		for _, ts := range d.mats.Tilesets {
			ts.Terrain = slices.DeleteFunc(ts.Terrain, same)
			ts.Doodads = slices.DeleteFunc(ts.Doodads, same)
			if !want[strings.ToLower(ts.Name)] {
				continue
			}
			if b.Type == materials.TypeDoodad {
				ts.Doodads = append(ts.Doodads, b.Name)
			} else {
				ts.Terrain = append(ts.Terrain, b.Name)
			}
		}
	}
	return ms.saveMaterials(d)
}

// RemoveBrush deletes a brush.
func (ms *MapService) RemoveBrush(name string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return err
	}
	d.mats.RemoveBrush(name)
	return ms.saveMaterials(d)
}

// SaveTileset creates or replaces a tileset (old "" creates).
func (ms *MapService) SaveTileset(old string, ts materials.Tileset) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return err
	}
	ts.Name = strings.TrimSpace(ts.Name)
	if ts.Name == "" {
		return errors.New("the tileset needs a name")
	}
	ts.Auto = false
	if cur := d.mats.Tileset(old); old != "" && cur != nil {
		*cur = ts
	} else if d.mats.Tileset(ts.Name) != nil {
		return fmt.Errorf("a tileset named %q exists", ts.Name)
	} else {
		cp := ts
		d.mats.Tilesets = append(d.mats.Tilesets, &cp)
	}
	return ms.saveMaterials(d)
}

// RemoveTileset deletes a tileset (its brushes stay).
func (ms *MapService) RemoveTileset(name string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return err
	}
	d.mats.Tilesets = slices.DeleteFunc(d.mats.Tilesets, func(t *materials.Tileset) bool { return strings.EqualFold(t.Name, name) })
	return ms.saveMaterials(d)
}

// AddToTileset adds items (raw palette) to a tileset, creating it.
func (ms *MapService) AddToTileset(name string, items []uint16) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return err
	}
	ts := d.mats.Tileset(name)
	if ts == nil {
		ts = &materials.Tileset{Name: name}
		d.mats.Tilesets = append(d.mats.Tilesets, ts)
	}
	for _, id := range items {
		if !slices.Contains(ts.Raw, id) {
			ts.Raw = append(ts.Raw, id)
		}
	}
	return ms.saveMaterials(d)
}

// SaveCreature adds or replaces a creature type.
func (ms *MapService) SaveCreature(c materials.CreatureType) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return err
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("the creature needs a name")
	}
	if cur := d.mats.Creature(c.Name); cur != nil {
		*cur = c
	} else {
		cp := c
		d.mats.Creatures = append(d.mats.Creatures, &cp)
	}
	return ms.saveMaterials(d)
}

// DoodadFromSelection creates a doodad brush from the selected items, as
// one composite relative to the selection's top-left tile.
func (ms *MapService) DoodadFromSelection(id int, name string, tilesets []string) error {
	ms.mu.Lock()
	m, _, err := ms.get(id)
	if err != nil {
		ms.mu.Unlock()
		return err
	}
	clip := m.ed.Copy()
	ms.mu.Unlock()
	if clip == nil {
		return errors.New("select the items of the doodad first")
	}
	var comp materials.Composite
	comp.Chance = 1
	for _, t := range clip.Tiles {
		ct := materials.CompositeTile{X: t.DX, Y: t.DY, Z: t.DZ}
		if t.Ground != nil {
			ct.Items = append(ct.Items, materials.ItemSpec{ID: t.Ground.ID, Subtype: t.Ground.Subtype})
		}
		for _, it := range t.Items {
			ct.Items = append(ct.Items, materials.ItemSpec{ID: it.ID, Subtype: it.Subtype, ActionID: it.ActionID(), UniqueID: it.UniqueID()})
		}
		if len(ct.Items) > 0 {
			comp.Tiles = append(comp.Tiles, ct)
		}
	}
	if len(comp.Tiles) == 0 {
		return errors.New("the selection holds no items")
	}
	b := materials.Brush{Name: name, Type: materials.TypeDoodad, Doodad: &materials.DoodadOptions{OnBlocking: true, OneSize: true},
		Alternatives: []materials.Alternative{{Composites: []materials.Composite{comp}}}}
	b.Look = comp.Tiles[0].Items[len(comp.Tiles[0].Items)-1].ID
	return ms.SaveBrush("", b, tilesets)
}

// SaveImage writes a PNG made by the frontend (screenshots).
func (ms *MapService) SaveImage(path string, data []byte) error {
	if !strings.EqualFold(filepath.Ext(path), ".png") {
		path += ".png"
	}
	return os.WriteFile(path, data, 0o644)
}

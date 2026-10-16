// Package mapedit is the map editing core: brushes and their automatic
// borders, walls, carpets and tables, selection, clipboard, undo/redo and the
// map tools (find, replace, cleanup, statistics).
package mapedit

import (
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/nekiro/ots-creator/internal/materials"
	"github.com/nekiro/ots-creator/internal/otitems"
)

type chanceList struct {
	ids   []uint16
	cum   []int // cumulative chances
	total int
}

func newChanceList(chances []materials.Chance) chanceList {
	var l chanceList
	for _, chance := range chances {
		if chance.ID == 0 {
			continue
		}
		l.total += max(chance.Chance, 0)
		l.ids = append(l.ids, chance.ID)
		l.cum = append(l.cum, l.total)
	}
	return l
}

func (l *chanceList) pick(rng *rand.Rand) uint16 {
	if len(l.ids) == 0 {
		return 0
	}
	if l.total <= 0 {
		return l.ids[0]
	}
	roll := rng.IntN(l.total) + 1
	for i, cumulative := range l.cum {
		if roll <= cumulative {
			return l.ids[i]
		}
	}
	return l.ids[0]
}

func (l *chanceList) has(id uint16) bool { return slices.Contains(l.ids, id) }

// autoBorder is a runtime border: items by edge.
type autoBorder struct {
	id               uint32
	tiles            [edgeCount]uint16
	group            uint16
	groundEquivalent uint16
}

type specificCase struct {
	match      []uint16 // item ids to match (group conditions use group)
	matchGroup uint16
	groupAlign int
	replace    uint16
	with       uint16
	deleteAll  bool
}

type borderBlock struct {
	outer    bool
	to       int // brush id, toAll or toNone
	super    bool
	border   *autoBorder
	specific []specificCase
}

// GroundBrush paints ground with automatic borders.
type GroundBrush struct {
	id            int
	Name          string
	look          uint16
	items         chanceList
	zOrder        int
	randomize     bool
	soloOptional  bool
	optional      *autoBorder
	borders       []*borderBlock
	friends       []int
	hateFriends   bool
	hasOuter      bool
	hasInner      bool
	hasOuterZilch bool
	hasInnerZilch bool
}

func (g *GroundBrush) hasOuterBorder() bool      { return g.hasOuter || g.optional != nil }
func (g *GroundBrush) hasOuterZilchBorder() bool { return g.hasOuterZilch || g.optional != nil }

// friendOf reports whether other is a friend (or not an enemy).
func friendOf(friends []int, hate bool, other int) bool {
	found := slices.Contains(friends, other) || slices.Contains(friends, toAll)
	if hate {
		return !found
	}
	return found
}

func (g *GroundBrush) friendOf(other *GroundBrush) bool {
	return friendOf(g.friends, g.hateFriends, other.id)
}

type wallNode struct {
	items chanceList
	doors []wallDoor
}

type wallDoor struct {
	id       uint16
	doorType int
	open     bool
	hate     bool
}

// WallBrush paints walls that connect to their neighbors.
type WallBrush struct {
	id          int
	Name        string
	look        uint16
	nodes       [wallAlignmentMax]wallNode
	redirect    *WallBrush
	friends     []int
	decoration  bool
	redirectsTo string
}

func (w *WallBrush) friendOf(other *WallBrush) bool { return friendOf(w.friends, false, other.id) }

// pick returns a random item of an alignment, following the redirect chain.
func (w *WallBrush) pick(align int, rng *rand.Rand) uint16 {
	for wall, depth := w, 0; wall != nil && depth < maxWallRedirects; wall, depth = wall.redirect, depth+1 {
		if id := wall.nodes[align].items.pick(rng); id != 0 {
			return id
		}
	}
	return 0
}

func (w *WallBrush) hasWall(id uint16, align int) bool {
	for wall, depth := w, 0; wall != nil && depth < maxWallRedirects; wall, depth = wall.redirect, depth+1 {
		if wall.nodes[align].items.has(id) {
			return true
		}
		for _, door := range wall.nodes[align].doors {
			if door.id == id {
				return true
			}
		}
	}
	return false
}

// CarpetBrush paints carpets that join with their neighbors.
type CarpetBrush struct {
	id    int
	Name  string
	look  uint16
	parts [edgeCount]chanceList
}

// TableBrush paints tables that join with their neighbors.
type TableBrush struct {
	id    int
	Name  string
	look  uint16
	parts [tableCount]chanceList
}

type doodadSingle struct {
	chance int
	item   materials.ItemSpec
}

type doodadComposite struct {
	chance int
	tiles  []materials.CompositeTile
}

type doodadAlt struct {
	singles         []doodadSingle
	singleTotal     int
	composites      []doodadComposite
	compositeTotal  int
	ownedItems      map[uint16]bool
	hasComposite    bool
	hasSingleObject bool
}

// DoodadBrush places random decoration.
type DoodadBrush struct {
	id    int
	Name  string
	look  uint16
	opts  materials.DoodadOptions
	alts  []*doodadAlt
	owned map[uint16]bool
}

func (d *DoodadBrush) alt(variation int) *doodadAlt {
	if len(d.alts) == 0 {
		return nil
	}
	if variation < 0 {
		variation = -variation
	}
	return d.alts[variation%len(d.alts)]
}

func (d *DoodadBrush) owns(id uint16) bool { return d.owned[id] }

// itemInfo is what the brushes know about one item id.
type itemInfo struct {
	ground           *GroundBrush
	border           bool
	borderAlign      int
	borderGroup      uint16
	optionalBorder   bool
	groundEquivalent uint16
	wall             *WallBrush
	wallAlign        int
	door             bool
	doorType         int
	doorOpen         bool
	wallHate         bool
	carpet           *CarpetBrush
	carpetAlign      int
	table            *TableBrush
	tableAlign       int
	doodad           *DoodadBrush
}

// Brushes indexes materials for drawing.
type Brushes struct {
	items     *otitems.DB
	Materials *materials.Materials
	grounds   map[string]*GroundBrush
	walls     map[string]*WallBrush
	carpets   map[string]*CarpetBrush
	tables    map[string]*TableBrush
	doodads   map[string]*DoodadBrush
	info      map[uint16]*itemInfo
	nextID    int
	// Warnings collected while indexing.
	Warnings []string
}

func lower(text string) string { return strings.ToLower(strings.TrimSpace(text)) }

// NewBrushes indexes materials. Unknown item ids are skipped with a warning.
func NewBrushes(items *otitems.DB, materialsDB *materials.Materials) *Brushes {
	if materialsDB == nil {
		materialsDB = materials.New()
	}
	b := &Brushes{
		items: items, Materials: materialsDB,
		grounds: map[string]*GroundBrush{}, walls: map[string]*WallBrush{}, carpets: map[string]*CarpetBrush{},
		tables: map[string]*TableBrush{}, doodads: map[string]*DoodadBrush{}, info: map[uint16]*itemInfo{},
	}
	items.MarkMeta(materialsDB.MetaItems)
	borders := b.indexBorders(materialsDB.Borders)
	// First pass: create brushes so rules can reference any brush by name.
	b.createBrushes(materialsDB.Brushes)
	b.buildBrushes(materialsDB.Brushes, borders)
	b.resolveWallRedirects()
	return b
}

// indexBorders creates and registers the shared borders, keyed by border id.
func (b *Brushes) indexBorders(materialBorders []*materials.Border) map[uint32]*autoBorder {
	borders := map[uint32]*autoBorder{}
	for _, materialBorder := range materialBorders {
		border := b.autoBorder(materialBorder)
		borders[materialBorder.ID] = border
		b.registerBorder(border, materialBorder.Optional)
	}
	return borders
}

// createBrushes creates an empty brush of the right kind for every material
// brush and gives each one an id.
func (b *Brushes) createBrushes(materialBrushes []*materials.Brush) {
	for _, material := range materialBrushes {
		key := lower(material.Name)
		b.nextID++
		switch material.Type {
		case materials.TypeGround:
			b.grounds[key] = &GroundBrush{id: b.nextID, Name: material.Name, look: material.Look}
		case materials.TypeWall, materials.TypeDecoration:
			if len(material.Walls) > 0 {
				b.walls[key] = &WallBrush{id: b.nextID, Name: material.Name, look: material.Look, decoration: material.Type == materials.TypeDecoration}
			}
		case materials.TypeCarpet:
			b.carpets[key] = &CarpetBrush{id: b.nextID, Name: material.Name, look: material.Look}
		case materials.TypeTable:
			b.tables[key] = &TableBrush{id: b.nextID, Name: material.Name, look: material.Look}
		}
		if len(material.Alternatives) > 0 {
			b.nextID++
			b.doodads[key] = &DoodadBrush{id: b.nextID, Name: material.Name, look: material.Look, owned: map[uint16]bool{}}
		}
	}
}

// resolveBrushID returns the id of the ground or wall brush named in a rule,
// or toAll, toNone or toUnknown.
func (b *Brushes) resolveBrushID(name string) int {
	switch lower(name) {
	case "all":
		return toAll
	case "none", "":
		return toNone
	}
	key := lower(name)
	if ground := b.grounds[key]; ground != nil {
		return ground.id
	}
	if wall := b.walls[key]; wall != nil {
		return wall.id
	}
	return toUnknown
}

// buildBrushes fills the brushes made by createBrushes from their materials.
func (b *Brushes) buildBrushes(materialBrushes []*materials.Brush, borders map[uint32]*autoBorder) {
	for _, material := range materialBrushes {
		key := lower(material.Name)
		if ground := b.grounds[key]; ground != nil {
			b.buildGround(ground, material, borders, b.resolveBrushID)
		}
		if wall := b.walls[key]; wall != nil {
			b.buildWall(wall, material, b.resolveBrushID)
		}
		if carpet := b.carpets[key]; carpet != nil {
			b.buildCarpet(carpet, material)
		}
		if table := b.tables[key]; table != nil {
			b.buildTable(table, material)
		}
		if doodad := b.doodads[key]; doodad != nil {
			b.buildDoodad(doodad, material)
		}
	}
}

// resolveWallRedirects links every wall to the wall it redirects to.
func (b *Brushes) resolveWallRedirects() {
	for _, wall := range b.walls {
		if wall.redirectsTo != "" {
			wall.redirect = b.walls[lower(wall.redirectsTo)]
		}
	}
}

func (b *Brushes) infoFor(id uint16) *itemInfo {
	info := b.info[id]
	if info == nil {
		info = &itemInfo{}
		b.info[id] = info
	}
	return info
}

func (b *Brushes) autoBorder(materialBorder *materials.Border) *autoBorder {
	border := &autoBorder{id: materialBorder.ID, group: materialBorder.Group, groundEquivalent: materialBorder.GroundEquivalent}
	for name, id := range materialBorder.Edges {
		if edge, ok := edgeIndex[name]; ok && edge < edgeCenter {
			border.tiles[edge] = id
		}
	}
	return border
}

func (b *Brushes) registerBorder(border *autoBorder, optional bool) {
	for edge, id := range border.tiles {
		if id == 0 {
			continue
		}
		info := b.infoFor(id)
		info.border = true
		if info.borderAlign == edgeNone {
			info.borderAlign = edge
		}
		if border.group != 0 && info.borderGroup == 0 {
			info.borderGroup = border.group
		}
		if optional {
			info.optionalBorder = true
		}
		if border.groundEquivalent != 0 {
			info.groundEquivalent = border.groundEquivalent
		}
	}
}

func (b *Brushes) buildGround(ground *GroundBrush, material *materials.Brush, borders map[uint32]*autoBorder, idOf func(string) int) {
	ground.items = newChanceList(material.Items)
	ground.zOrder = material.ZOrder
	ground.randomize = !material.NoRandomize && ground.items.total > 0
	ground.soloOptional = material.SoloOptional
	for _, chance := range material.Items {
		b.infoFor(chance.ID).ground = ground
	}
	if ground.look == 0 && len(ground.items.ids) > 0 {
		ground.look = ground.items.ids[0]
	}
	if material.Optional != nil {
		ground.optional = b.resolveBorderRule(ground, *material.Optional, borders)
		if ground.optional != nil {
			for _, id := range ground.optional.tiles {
				if id != 0 {
					b.infoFor(id).optionalBorder = true
				}
			}
		}
	}
	for _, rule := range material.Borders {
		ground.borders = append(ground.borders, b.buildBorderBlock(ground, rule, borders, idOf))
	}
	for _, friend := range material.Friends {
		if id := idOf(friend); id != toNone {
			ground.friends = append(ground.friends, id)
		}
	}
	ground.hateFriends = material.HateFriends
}

// resolveBorderRule returns the border of a ground rule: an inline border
// (registered here) or a shared one by id. It is nil when the rule has none.
func (b *Brushes) resolveBorderRule(ground *GroundBrush, rule materials.BorderRule, borders map[uint32]*autoBorder) *autoBorder {
	if rule.Inline != nil {
		border := b.autoBorder(rule.Inline)
		b.registerBorder(border, false)
		if border.groundEquivalent != 0 {
			b.infoFor(border.groundEquivalent).ground = ground
		}
		return border
	}
	if rule.Border == 0 {
		return nil
	}
	border := borders[rule.Border]
	if border == nil {
		b.Warnings = append(b.Warnings, "brush "+ground.Name+": unknown border")
	}
	return border
}

// buildBorderBlock builds one border rule of a ground and records on the
// ground which kinds of borders it has.
func (b *Brushes) buildBorderBlock(ground *GroundBrush, rule materials.BorderRule, borders map[uint32]*autoBorder, idOf func(string) int) *borderBlock {
	block := &borderBlock{outer: rule.Align != "inner", super: rule.Super, border: b.resolveBorderRule(ground, rule, borders)}
	block.to = idOf(rule.To)
	if rule.To == "" {
		block.to = toAll
	}
	for _, specific := range rule.Specific {
		block.specific = append(block.specific, b.specificCase(specific, borders))
	}
	if block.outer {
		if block.to == toNone {
			ground.hasOuterZilch = true
		} else {
			ground.hasOuter = true
		}
	} else {
		if block.to == toNone {
			ground.hasInnerZilch = true
		} else {
			ground.hasInner = true
		}
	}
	return block
}

func (b *Brushes) specificCase(specific materials.SpecificCase, borders map[uint32]*autoBorder) specificCase {
	var out specificCase
	edgeItem := func(borderID uint32, edge string) uint16 {
		border := borders[borderID]
		if border == nil {
			return 0
		}
		return border.tiles[edgeIndex[edge]]
	}
	for _, condition := range specific.Conditions {
		switch condition.Kind {
		case "border":
			out.match = append(out.match, edgeItem(condition.Border, condition.Edge))
		case "group":
			out.matchGroup = condition.Group
			out.groupAlign = edgeIndex[condition.Edge]
			out.match = append(out.match, condition.Group)
		case "item":
			out.match = append(out.match, condition.Item)
		}
	}
	for _, action := range specific.Actions {
		switch action.Kind {
		case "replaceBorder":
			out.replace, out.with = edgeItem(action.Border, action.Edge), action.With
		case "replaceItem":
			out.replace, out.with = action.Item, action.With
		case "deleteBorders":
			out.deleteAll = true
		}
		if out.with != 0 {
			b.infoFor(out.with).border = true
		}
	}
	return out
}

func (b *Brushes) buildWall(wall *WallBrush, material *materials.Brush, idOf func(string) int) {
	for alignName, part := range material.Walls {
		align := slices.Index(materials.WallAlignments, alignName)
		if align < 0 {
			continue
		}
		wall.nodes[align].items = newChanceList(part.Items)
		for _, chance := range part.Items {
			info := b.infoFor(chance.ID)
			info.wall, info.wallAlign = wall, align
		}
		for _, door := range part.Doors {
			doorType := slices.Index(materials.DoorTypes, door.Type) + 1
			wall.nodes[align].doors = append(wall.nodes[align].doors, wallDoor{id: door.ID, doorType: doorType, open: door.Open, hate: door.Hate})
			info := b.infoFor(door.ID)
			info.wall, info.wallAlign, info.door, info.doorOpen, info.wallHate = wall, align, true, door.Open, door.Hate
			if info.doorType == doorUndefined {
				info.doorType = doorType
			}
		}
	}
	for _, friend := range material.Friends {
		if id := idOf(friend); id > 0 {
			wall.friends = append(wall.friends, id)
		}
	}
	wall.redirectsTo = material.Redirect
	if wall.look == 0 {
		for _, align := range []int{wallHorizontal, wallVertical, wallPole} {
			if len(wall.nodes[align].items.ids) > 0 {
				wall.look = wall.nodes[align].items.ids[0]
				break
			}
		}
	}
}

// buildCarpet fills the carpet pieces and registers their items.
func (b *Brushes) buildCarpet(carpet *CarpetBrush, material *materials.Brush) {
	for alignName, chances := range material.Parts {
		if edge, ok := edgeIndex[alignName]; ok {
			carpet.parts[edge] = newChanceList(chances)
			for _, chance := range chances {
				info := b.infoFor(chance.ID)
				info.carpet, info.carpetAlign = carpet, edge
			}
		}
	}
	if carpet.look == 0 && len(carpet.parts[edgeCenter].ids) > 0 {
		carpet.look = carpet.parts[edgeCenter].ids[0]
	}
}

// buildTable fills the table pieces and registers their items.
func (b *Brushes) buildTable(table *TableBrush, material *materials.Brush) {
	for alignName, chances := range material.Parts {
		align := slices.Index(materials.TableAlignments, alignName)
		if align < 0 {
			continue
		}
		table.parts[align] = newChanceList(chances)
		for _, chance := range chances {
			info := b.infoFor(chance.ID)
			info.table, info.tableAlign = table, align
		}
	}
}

func (b *Brushes) buildDoodad(doodad *DoodadBrush, material *materials.Brush) {
	if material.Doodad != nil {
		doodad.opts = *material.Doodad
	}
	for _, alternative := range material.Alternatives {
		alt := &doodadAlt{ownedItems: map[uint16]bool{}}
		for _, single := range alternative.Singles {
			alt.singles = append(alt.singles, doodadSingle{chance: single.Chance, item: single.Item})
			alt.singleTotal += max(single.Chance, 0)
			alt.ownedItems[single.Item.ID] = true
		}
		for _, composite := range alternative.Composites {
			alt.compositeTotal += max(composite.Chance, 0)
			alt.composites = append(alt.composites, doodadComposite{chance: alt.compositeTotal, tiles: composite.Tiles})
			for _, tile := range composite.Tiles {
				for _, item := range tile.Items {
					alt.ownedItems[item.ID] = true
				}
			}
		}
		alt.hasComposite = len(alt.composites) > 0
		alt.hasSingleObject = len(alt.singles) > 0
		for id := range alt.ownedItems {
			doodad.owned[id] = true
			b.infoFor(id).doodad = doodad
		}
		doodad.alts = append(doodad.alts, alt)
	}
	if doodad.look == 0 && len(doodad.alts) > 0 {
		first := doodad.alts[0]
		if len(first.singles) > 0 {
			doodad.look = first.singles[0].item.ID
		} else if len(first.composites) > 0 && len(first.composites[0].tiles) > 0 {
			doodad.look = first.composites[0].tiles[0].Items[0].ID
		}
	}
}

// Ground returns a ground brush by name.
func (b *Brushes) Ground(name string) *GroundBrush { return b.grounds[lower(name)] }

// Wall returns a wall brush by name.
func (b *Brushes) Wall(name string) *WallBrush { return b.walls[lower(name)] }

// Carpet returns a carpet brush by name.
func (b *Brushes) Carpet(name string) *CarpetBrush { return b.carpets[lower(name)] }

// Table returns a table brush by name.
func (b *Brushes) Table(name string) *TableBrush { return b.tables[lower(name)] }

// Doodad returns a doodad brush by name.
func (b *Brushes) Doodad(name string) *DoodadBrush { return b.doodads[lower(name)] }

// GroundOf returns the ground brush of an item id.
func (b *Brushes) GroundOf(id uint16) *GroundBrush {
	if info := b.info[id]; info != nil {
		return info.ground
	}
	return nil
}

// BrushOf returns the name and kind of the brush that owns an item, for
// "pick brush" (empty when none).
func (b *Brushes) BrushOf(id uint16) (name, kind string) {
	info := b.info[id]
	switch {
	case info == nil:
		return "", ""
	case info.ground != nil:
		return info.ground.Name, materials.TypeGround
	case info.door && info.wall != nil:
		return info.wall.Name, materials.TypeWall
	case info.wall != nil:
		if info.wall.decoration {
			return info.wall.Name, materials.TypeDecoration
		}
		return info.wall.Name, materials.TypeWall
	case info.carpet != nil:
		return info.carpet.Name, materials.TypeCarpet
	case info.table != nil:
		return info.table.Name, materials.TypeTable
	case info.doodad != nil:
		return info.doodad.Name, materials.TypeDoodad
	}
	return "", ""
}

// Look returns the palette item of a brush.
func (b *Brushes) Look(name string) uint16 {
	key := lower(name)
	switch {
	case b.grounds[key] != nil:
		return b.grounds[key].look
	case b.walls[key] != nil:
		return b.walls[key].look
	case b.carpets[key] != nil:
		return b.carpets[key].look
	case b.tables[key] != nil:
		return b.tables[key].look
	case b.doodads[key] != nil:
		return b.doodads[key].look
	}
	return 0
}

func (b *Brushes) isBorder(id uint16) bool {
	if info := b.info[id]; info != nil && info.border {
		return true
	}
	itemType := b.items.Get(id)
	return itemType != nil && itemType.TopOrder == borderTopOrder
}

func (b *Brushes) wallOf(id uint16) (*WallBrush, *itemInfo) {
	info := b.info[id]
	if info == nil || info.wall == nil {
		return nil, nil
	}
	return info.wall, info
}

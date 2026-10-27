package app

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
	"github.com/nekiro/ots-creator/internal/mapping/suggest"
)

const (
	// suggestBudget is the number of changed blocks a request counts
	// itself; more are left to a background goroutine.
	suggestBudget = 64
	// indexBatch is the number of blocks the goroutine counts per lock.
	indexBatch = 256
	// suggestLimit is the number of suggestions per list.
	suggestLimit = 12
	// maxBrushItems limits the items of a brush used as context.
	maxBrushItems = 32
	// maxFamily limits a variant family with the items the map and the
	// sprites add to it.
	maxFamily = 64
	// substituteLimit and alikeLimit are the items the map (taking the same
	// places) and the sprites (looking alike) add to a variant family.
	substituteLimit = 8
	alikeLimit      = 6
	// alikeWindow is how far, in ids, look-alikes are searched: clients
	// keep similar objects close together.
	alikeWindow = 400
	// maxAlikeDistance is the largest difference of two looks: the distance
	// of their average colors (0-441) plus 100 times the difference of the
	// share of the sprite they cover.
	maxAlikeDistance = 40
)

// SuggestBrush is the brush being drawn with: a palette brush by kind and
// name, or a raw item.
type SuggestBrush struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Item uint16 `json:"item"`
}

// Suggestion is an item to draw: a raw item, or the brush that owns it.
type Suggestion struct {
	Item uint16 `json:"item"`
	// Brush and Kind name the palette brush of the item (empty: raw item).
	Brush  string  `json:"brush,omitempty"`
	Kind   string  `json:"kind,omitempty"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

type Suggestions struct {
	// Here fits the place, With goes with the brush, Grounds border the
	// grounds of the place.
	Here    []Suggestion `json:"here"`
	With    []Suggestion `json:"with"`
	Grounds []Suggestion `json:"grounds"`
	// Pending is the number of map blocks not counted yet.
	Pending int `json:"pending"`
}

// Suggest ranks what to draw at a position, from what the map shows in the
// same places (the area around the position weighs more). When the map
// knows nothing alike, the tilesets of the ground give the suggestions.
func (ms *MapService) Suggest(id int, pos otmap.Position, brush SuggestBrush) (suggestions Suggestions, err error) {
	opened, data, unlock, err := ms.read(id)
	if err != nil {
		return suggestions, err
	}
	defer unlock()
	editor := opened.editor
	ignore := suggestIgnore(editor)
	suggestions.Pending = ms.refreshSuggestions(opened)
	place := placeAt(editor, pos, ignore)
	fits := placeable(editor, pos)
	namer := suggestionNamer{items: data.items, brushes: editor.Brushes}
	suggestions.Here = namer.list(keep(opened.suggestions.Here(place, suggestLimit*3), fits), false)
	if len(suggestions.Here) == 0 {
		suggestions.Here = namer.fromTilesets(place.Ground, fits)
	}
	suggestions.Grounds = namer.list(opened.suggestions.Grounds(place, suggestLimit*2), true)
	if ids := brushItems(editor, brush); len(ids) > 0 {
		notGround := func(item uint16) bool {
			itemType := data.items.Get(item)
			return itemType != nil && !itemType.IsGround()
		}
		suggestions.With = namer.list(keep(opened.suggestions.With(ids, pos, suggestLimit*3), notGround), false)
	}
	return suggestions, nil
}

// refreshSuggestions counts the changed blocks a request may count and
// leaves the rest to the background; it returns the blocks left.
func (ms *MapService) refreshSuggestions(opened *openMap) int {
	pending := opened.suggestions.Refresh(opened.editor.Map, suggestIgnore(opened.editor), suggestBudget)
	if pending > 0 {
		ms.indexInBackground(opened)
	}
	return pending
}

// indexInBackground counts the changed blocks of a map, a batch per read
// lock, so edits are not held up.
func (ms *MapService) indexInBackground(opened *openMap) {
	if !opened.indexing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer opened.indexing.Store(false)
		for {
			current, _, unlock, err := ms.read(opened.id)
			if err != nil || current != opened {
				if err == nil {
					unlock()
				}
				return
			}
			pending := opened.suggestions.Refresh(opened.editor.Map, suggestIgnore(opened.editor), indexBatch)
			unlock()
			if pending == 0 {
				return
			}
		}
	}()
}

// suggestIgnore leaves out borders (ground brushes place them) and editor
// items.
func suggestIgnore(editor *mapedit.Editor) func(id uint16) bool {
	return func(id uint16) bool {
		itemType := editor.Items.Get(id)
		return itemType == nil || itemType.Flags.Has(otitems.FlagMeta) || editor.Brushes.IsBorder(id)
	}
}

func placeAt(editor *mapedit.Editor, pos otmap.Position, ignore func(id uint16) bool) suggest.Context {
	place := suggest.Context{Pos: pos}
	var scratch otmap.Tile
	buffer := make([]otmap.Item, 0, 32)
	collect := func(tile *otmap.Tile) []uint16 {
		var ids []uint16
		for _, item := range tile.Items {
			if !ignore(item.ID) {
				ids = append(ids, item.ID)
			}
		}
		return ids
	}
	if tile := editor.Map.PeekAt(int(pos.X), int(pos.Y), int(pos.Z), &scratch, buffer); tile != nil {
		place.Ground = tile.Ground.ID
		place.Items = collect(tile)
	}
	for direction, offset := range suggest.Offsets {
		if tile := editor.Map.PeekAt(int(pos.X)+offset[0], int(pos.Y)+offset[1], int(pos.Z), &scratch, buffer); tile != nil {
			place.Neighbors[direction] = suggest.Cell{Ground: tile.Ground.ID, Items: collect(tile)}
		}
	}
	return place
}

// placeable tells which items can be put on the tile at a position: no
// grounds, nothing blocking on a blocking tile, hangables only on a wall
// with a hook.
func placeable(editor *mapedit.Editor, pos otmap.Position) func(id uint16) bool {
	var scratch otmap.Tile
	var blocked, hooks bool
	if tile := editor.Map.PeekAt(int(pos.X), int(pos.Y), int(pos.Z), &scratch, nil); tile != nil {
		for _, item := range tile.Items {
			if itemType := editor.Items.Get(item.ID); itemType != nil {
				blocked = blocked || itemType.Flags.Has(otitems.FlagUnpassable)
				hooks = hooks || itemType.Flags&(otitems.FlagHookSouth|otitems.FlagHookEast) != 0
			}
		}
	}
	return func(id uint16) bool {
		itemType := editor.Items.Get(id)
		switch {
		case itemType == nil || itemType.IsGround():
			return false
		case blocked && itemType.Flags.Has(otitems.FlagUnpassable):
			return false
		case itemType.Flags.Has(otitems.FlagHangable) && !hooks:
			return false
		}
		return true
	}
}

func keep(ranked []suggest.Ranked, fits func(id uint16) bool) []suggest.Ranked {
	return slices.DeleteFunc(ranked, func(entry suggest.Ranked) bool { return !fits(entry.ID) })
}

func brushItems(editor *mapedit.Editor, brush SuggestBrush) []uint16 {
	if brush.Kind == "raw" {
		if brush.Item == 0 {
			return nil
		}
		return []uint16{brush.Item}
	}
	if brush.Name == "" {
		return nil
	}
	ids := editor.Brushes.ItemsOf(brush.Name, brush.Kind)
	return ids[:min(len(ids), maxBrushItems)]
}

// suggestionNamer turns ranked ids into suggestions: an item of a palette
// brush becomes that brush, once.
type suggestionNamer struct {
	items   *otitems.DB
	brushes *mapedit.Brushes
}

func (namer suggestionNamer) list(ranked []suggest.Ranked, grounds bool) []Suggestion {
	var out []Suggestion
	seen := map[string]bool{}
	for _, entry := range ranked {
		suggestion, key := namer.suggestion(entry.ID, namer.reason(entry, grounds))
		suggestion.Score = entry.Score
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, suggestion)
		if len(out) == suggestLimit {
			break
		}
	}
	return out
}

// suggestion makes the suggestion of an item and its key for removing
// duplicates: the brush of the item when it has one.
func (namer suggestionNamer) suggestion(id uint16, reason string) (Suggestion, string) {
	suggestion := Suggestion{Item: id, Reason: reason}
	if name, kind := namer.brushes.BrushOf(id); name != "" {
		suggestion.Brush, suggestion.Kind = name, kind
		if look := namer.brushes.Look(name); look != 0 {
			suggestion.Item = look
		}
		return suggestion, kind + ":" + name
	}
	return suggestion, fmt.Sprintf("item:%d", id)
}

// fromTilesets suggests the doodads and items of the tilesets that hold
// the brush of a ground, for a map that has no place alike yet.
func (namer suggestionNamer) fromTilesets(ground uint16, fits func(id uint16) bool) []Suggestion {
	groundBrush, kind := namer.brushes.BrushOf(ground)
	if groundBrush == "" || kind != materials.TypeGround || namer.brushes.Materials == nil {
		return nil
	}
	var out []Suggestion
	seen := map[string]bool{}
	add := func(suggestion Suggestion, key string) bool {
		if !seen[key] {
			seen[key] = true
			out = append(out, suggestion)
		}
		return len(out) == suggestLimit
	}
	for _, tileset := range namer.brushes.Materials.Tilesets {
		if !slices.ContainsFunc(tileset.Terrain, func(name string) bool { return strings.EqualFold(name, groundBrush) }) {
			continue
		}
		reason := "in the tileset " + tileset.Name
		for _, name := range tileset.Doodads {
			if look := namer.brushes.Look(name); look != 0 && add(Suggestion{Item: look, Brush: name, Kind: materials.TypeDoodad, Reason: reason}, "doodad:"+strings.ToLower(name)) {
				return out
			}
		}
		for _, id := range slices.Concat(tileset.Items, tileset.Raw) {
			if fits(id) && add(namer.suggestion(id, reason)) {
				return out
			}
		}
	}
	return out
}

func (namer suggestionNamer) name(id uint16) string {
	if itemType := namer.items.Get(id); itemType != nil && itemType.Name != "" {
		return itemType.Name
	}
	return fmt.Sprintf("item %d", id)
}

func (namer suggestionNamer) reason(entry suggest.Ranked, grounds bool) string {
	via := namer.name(entry.Via)
	var reason string
	switch {
	case grounds:
		reason = "borders " + via
	case entry.Reason == suggest.ReasonGround:
		reason = "lies on " + via
	case entry.Reason == suggest.ReasonTile:
		reason = "lies with " + via
	case entry.Reason == suggest.ReasonNear:
		reason = "lies next to " + via
	default:
		reason = "goes with " + via
	}
	if entry.Local {
		reason += " around here"
	}
	return reason
}

// Variants returns the items an item can be swapped for, the item first:
// its family (see mapedit.Editor.Variants), then the items that take the
// same places in the map, then the items that look alike.
func (ms *MapService) Variants(id int, item uint16) (variants []uint16, err error) {
	opened, data, unlock, err := ms.read(id)
	if err != nil {
		return nil, err
	}
	defer unlock()
	editor := opened.editor
	family := editor.Variants(item)
	if len(family) == 0 {
		return nil, nil
	}
	add := func(candidate uint16) {
		if len(family) < maxFamily && !slices.Contains(family, candidate) && sameClass(editor.Items, item, candidate) {
			family = append(family, candidate)
		}
	}
	ms.refreshSuggestions(opened)
	for _, substitute := range opened.suggestions.Substitutes(item, substituteLimit) {
		add(substitute.ID)
	}
	for _, alike := range ms.looks.alike(data, editor.Items, item) {
		add(alike)
	}
	return family, nil
}

// sameClass reports whether two items can take each other's place: both
// grounds or not, the same stack order, both blocking or not, both
// hanging or not.
func sameClass(items *otitems.DB, first, second uint16) bool {
	left, right := items.Get(first), items.Get(second)
	const flags = otitems.FlagGround | otitems.FlagUnpassable | otitems.FlagHangable
	return left != nil && right != nil && left.TopOrder == right.TopOrder && left.Flags&flags == right.Flags&flags && !right.Flags.Has(otitems.FlagMeta)
}

// spriteLook is the size of an item in sprites, the average color of its
// first texture and the share of the texture it covers.
type spriteLook struct {
	ok               bool
	width, height    int
	red, green, blue float64
	cover            float64
}

// lookCache keeps the looks of the items of one client.
type lookCache struct {
	mu      sync.Mutex
	project *project.Project
	looks   map[uint16]spriteLook
}

func (cache *lookCache) look(data *mapData, items *otitems.DB, id uint16) spriteLook {
	if found, ok := cache.looks[id]; ok {
		return found
	}
	var look spriteLook
	if itemType := items.Get(id); itemType != nil && data.project != nil {
		if object, err := data.project.Thing(thing.CategoryItem, uint32(itemType.ClientID)); err == nil {
			if texture, err := data.project.Render(thing.CategoryItem, uint32(itemType.ClientID), project.ThumbnailPos(object)); err == nil {
				look = averageLook(texture.Pix, texture.Rect.Dx(), texture.Rect.Dy(), data.project.SpriteSize())
			}
		}
	}
	cache.looks[id] = look
	return look
}

func averageLook(pixels []byte, width, height, spriteSize int) spriteLook {
	look := spriteLook{width: width / max(spriteSize, 1), height: height / max(spriteSize, 1)}
	var opaque float64
	for i := 0; i+3 < len(pixels); i += 4 {
		if pixels[i+3] < 128 {
			continue
		}
		opaque++
		look.red += float64(pixels[i])
		look.green += float64(pixels[i+1])
		look.blue += float64(pixels[i+2])
	}
	if opaque == 0 {
		return look
	}
	look.ok = true
	look.red, look.green, look.blue = look.red/opaque, look.green/opaque, look.blue/opaque
	look.cover = opaque / float64(max(width*height, 1))
	return look
}

func lookDistance(left, right spriteLook) float64 {
	return math.Sqrt((left.red-right.red)*(left.red-right.red)+(left.green-right.green)*(left.green-right.green)+(left.blue-right.blue)*(left.blue-right.blue)) +
		100*math.Abs(left.cover-right.cover)
}

// alike returns the items near an item's id of its class and size that
// look most like it, closest first.
func (cache *lookCache) alike(data *mapData, items *otitems.DB, id uint16) []uint16 {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.project != data.project || cache.looks == nil {
		cache.project, cache.looks = data.project, map[uint16]spriteLook{}
	}
	base := cache.look(data, items, id)
	if !base.ok {
		return nil
	}
	type scored struct {
		id       uint16
		distance float64
	}
	var found []scored
	for candidate := max(int(id)-alikeWindow, 1); candidate <= min(int(id)+alikeWindow, int(items.MaxID())); candidate++ {
		if candidate == int(id) || !sameClass(items, id, uint16(candidate)) {
			continue
		}
		look := cache.look(data, items, uint16(candidate))
		if !look.ok || look.width != base.width || look.height != base.height {
			continue
		}
		if distance := lookDistance(base, look); distance <= maxAlikeDistance {
			found = append(found, scored{uint16(candidate), distance})
		}
	}
	slices.SortFunc(found, func(left, right scored) int {
		if left.distance != right.distance {
			return int(math.Copysign(1, left.distance-right.distance))
		}
		return int(left.id) - int(right.id)
	})
	out := make([]uint16, 0, alikeLimit)
	for _, entry := range found[:min(alikeLimit, len(found))] {
		out = append(out, entry.id)
	}
	return out
}

// SwapItem replaces an item by another one; swaps with the same non-zero
// stroke are one undo step.
func (ms *MapService) SwapItem(id int, ref mapedit.ItemRef, item uint16, stroke int64) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.SwapItem(ref, item, stroke) })
}

// Completions returns the parts missing around an item at a position: the
// other parts of the object it belongs to, as the map shows them.
func (ms *MapService) Completions(id int, pos otmap.Position, item uint16) (parts []mapedit.PlacedItem, err error) {
	opened, _, unlock, err := ms.read(id)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ms.refreshSuggestions(opened)
	var scratch otmap.Tile
	for _, part := range opened.suggestions.Completions(item) {
		offset := suggest.Offsets[part.Direction]
		x, y := int(pos.X)+offset[0], int(pos.Y)+offset[1]
		if x < 0 || y < 0 || x > 0xFFFF || y > 0xFFFF {
			continue
		}
		tile := opened.editor.Map.PeekAt(x, y, int(pos.Z), &scratch, nil)
		if tile != nil && slices.ContainsFunc(tile.Items, func(placed otmap.Item) bool { return placed.ID == part.ID }) {
			continue
		}
		parts = append(parts, mapedit.PlacedItem{Pos: otmap.Position{X: uint16(x), Y: uint16(y), Z: pos.Z}, Item: part.ID})
	}
	return parts, nil
}

// PlaceItems puts items on tiles in one undo step (joining the last step
// with the same non-zero stroke).
func (ms *MapService) PlaceItems(id int, placed []mapedit.PlacedItem, stroke int64) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.AddItems(placed, stroke) })
}

// MixItem is an item of a mix with its weight.
type MixItem struct {
	Item   uint16 `json:"item"`
	Weight int    `json:"weight"`
}

// MakeMix makes a doodad brush that scatters the given items, for this
// session, and returns its name.
func (ms *MapService) MakeMix(id int, mix []MixItem) (name string, err error) {
	err = ms.withMap(id, func(opened *openMap, _ *mapData) error {
		chances := make([]materials.Chance, len(mix))
		for i, entry := range mix {
			chances[i] = materials.Chance{ID: entry.Item, Chance: entry.Weight}
		}
		name, err = opened.editor.Brushes.AddMix(chances)
		return err
	})
	return name, err
}

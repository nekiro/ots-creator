package otmap

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/workpool"
)

type sectorKey struct{ x, y uint16 }

type textWriter struct {
	items     Items
	attrKeys  map[byte]string
	attrTypes map[byte]string
}

func newTextWriter(gameMap *Map, items Items) *textWriter {
	types := make(map[byte]string, len(gameMap.Attributes))
	for _, attr := range gameMap.Attributes {
		types[attr.Code] = attr.Type
	}
	return &textWriter{items: items, attrKeys: attrKeys(gameMap.Attributes), attrTypes: types}
}

// saveOTMAP writes the world file at path and the sector files in the
// sectors directory next to it. Sector files whose content did not change
// are not rewritten; files of sectors that became empty are deleted.
func saveOTMAP(path string, gameMap *Map, items Items, progress Progress) ([]string, error) {
	sectorsDir := filepath.Join(filepath.Dir(path), sectorsDirName)
	if err := os.MkdirAll(sectorsDir, fsutil.DirMode); err != nil {
		return nil, err
	}
	w := newTextWriter(gameMap, items)
	keys := gameMap.sectorKeys()
	written := make([]string, len(keys))
	var progressMu sync.Mutex
	done := 0
	err := workpool.Run(len(keys), func(index int) error {
		data, err := w.sector(gameMap, keys[index])
		if err != nil {
			return err
		}
		if data != nil {
			name := sectorFileName(keys[index].x, keys[index].y)
			if err := writeIfChanged(filepath.Join(sectorsDir, name), data); err != nil {
				return err
			}
			written[index] = name
		}
		progressMu.Lock()
		done++
		if done%progressEveryChunks == 0 || done == len(keys) {
			progress(StageEncode, done, len(keys))
		}
		progressMu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}
	progress(StageWrite, 0, 2)
	if err := removeStaleSectors(sectorsDir, written); err != nil {
		return nil, err
	}
	world, err := w.world(gameMap)
	if err != nil {
		return nil, err
	}
	if err := writeIfChanged(path, world); err != nil {
		return nil, err
	}
	progress(StageWrite, 2, 2)
	gameMap.ClearModified()
	return nil, nil
}

func writeIfChanged(path string, data []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	return fsutil.WriteAtomic(path, data)
}

func removeStaleSectors(sectorsDir string, written []string) error {
	keep := make(map[string]bool, len(written))
	for _, name := range written {
		if name != "" {
			keep[name] = true
		}
	}
	entries, err := os.ReadDir(sectorsDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, _, ok := parseSectorFileName(name); !ok || keep[name] || entry.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(sectorsDir, name)); err != nil {
			return err
		}
	}
	return nil
}

// EncodeOTMAP returns the world file and the sector files by name, the way
// saveOTMAP writes them.
func EncodeOTMAP(gameMap *Map, items Items) (world []byte, sectors map[string][]byte, err error) {
	w := newTextWriter(gameMap, items)
	sectors = map[string][]byte{}
	for _, key := range gameMap.sectorKeys() {
		data, err := w.sector(gameMap, key)
		if err != nil {
			return nil, nil, err
		}
		if data != nil {
			sectors[sectorFileName(key.x, key.y)] = data
		}
	}
	world, err = w.world(gameMap)
	return world, sectors, err
}

func (m *Map) sectorKeys() []sectorKey {
	seen := map[sectorKey]bool{}
	keys := make([]sectorKey, 0, len(m.chunks)/4+1)
	for _, chunk := range m.chunks {
		key := sectorKey{chunk.X, chunk.Y}
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	slices.SortFunc(keys, func(left, right sectorKey) int {
		return cmp.Or(cmp.Compare(left.y, right.y), cmp.Compare(left.x, right.x))
	})
	return keys
}

func (w *textWriter) world(gameMap *Map) ([]byte, error) {
	out := append([]byte(worldHeader), '\n')
	out = append(out, "items "...)
	if db, ok := w.items.(*otitems.DB); ok && db.Mode == otitems.ModeClient {
		out = append(out, itemsClient...)
	} else {
		out = append(out, itemsServer...)
	}
	out = append(out, '\n')
	if gameMap.ItemsMajor != 0 || gameMap.ItemsMinor != 0 {
		out = fmt.Appendf(out, "otb %d %d\n", gameMap.ItemsMajor, gameMap.ItemsMinor)
	}
	out = fmt.Appendf(out, "size %d %d\n", gameMap.Width, gameMap.Height)
	if gameMap.Description != "" {
		out = append(out, "description "...)
		out = append(appendQuoted(out, gameMap.Description), '\n')
	}
	if gameMap.SpawnFormat == SpawnsSeparate {
		out = append(out, "spawnformat separate\n"...)
	}
	attrs := slices.Clone(gameMap.Attributes)
	slices.SortFunc(attrs, func(left, right CustomAttr) int { return cmp.Compare(left.Code, right.Code) })
	for _, attr := range attrs {
		key := w.attrKeys[attr.Code]
		out = fmt.Appendf(out, "attribute %d %s %s", attr.Code, key, attr.Type)
		if attr.Name != key {
			out = appendQuoted(append(out, ' '), attr.Name)
		}
		out = append(out, '\n')
	}
	for _, town := range gameMap.SortedTowns() {
		out = fmt.Appendf(out, "town %d ", town.ID)
		out = append(appendQuoted(out, town.Name), " temple="...)
		out = append(appendPosition(out, town.Temple), '\n')
	}
	for _, house := range gameMap.SortedHouses() {
		out = fmt.Appendf(out, "house %d ", house.ID)
		out = appendQuoted(out, house.Name)
		out = fmt.Appendf(out, " town=%d exit=", house.TownID)
		out = appendPosition(out, house.Exit)
		if house.Rent != 0 {
			out = fmt.Appendf(out, " rent=%d", house.Rent)
		}
		if house.Guildhall {
			out = append(out, " guildhall"...)
		}
		if house.Beds != 0 {
			out = fmt.Appendf(out, " beds=%d", house.Beds)
		}
		if house.ClientID != 0 {
			out = fmt.Appendf(out, " clientid=%d", house.ClientID)
		}
		out = append(out, '\n')
	}
	zones := slices.Clone(gameMap.Zones)
	slices.SortStableFunc(zones, func(left, right Zone) int { return cmp.Compare(left.ID, right.ID) })
	for _, zone := range zones {
		out = fmt.Appendf(out, "zone %d ", zone.ID)
		out = append(appendQuoted(out, zone.Name), '\n')
	}
	waypoints := slices.Clone(gameMap.Waypoints)
	slices.SortStableFunc(waypoints, func(left, right *Waypoint) int {
		return cmp.Or(cmp.Compare(left.Name, right.Name), ComparePositions(left.Pos, right.Pos))
	})
	for _, waypoint := range waypoints {
		out = appendQuoted(append(out, "waypoint "...), waypoint.Name)
		out = append(appendPosition(append(out, ' '), waypoint.Pos), '\n')
	}
	return out, nil
}

// sector encodes one sector, nil when it has nothing to write.
func (w *textWriter) sector(gameMap *Map, key sectorKey) ([]byte, error) {
	var chunks [MaxFloor + 1]*Chunk
	for z := range chunks {
		chunks[z] = gameMap.chunks[MakeKey(key.x, key.y, uint8(z))]
	}
	out := append([]byte(sectorHeader), '\n')
	headerLen := len(out)
	var err error
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		chunk.Each(func(pos Position, tile *Tile) {
			if err == nil {
				out, err = w.tile(out, pos, tile)
			}
		})
	}
	if err != nil {
		return nil, err
	}
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		chunk.eachStored(func(pos Position, tile *Tile) {
			if spawn := tile.Spawn(); spawn != nil {
				out = appendArea(out, "spawn", pos, spawn)
			}
			if spawn := tile.SpawnNPC(); spawn != nil {
				out = appendArea(out, "npcspawn", pos, spawn)
			}
		})
	}
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		chunk.eachStored(func(pos Position, tile *Tile) {
			for _, creature := range tile.Creatures() {
				out = appendCreature(out, pos, creature)
			}
		})
	}
	if len(out) == headerLen {
		return nil, nil
	}
	return out, nil
}

func appendCoords(out []byte, pos Position) []byte {
	out = strconv.AppendUint(out, uint64(pos.X), 10)
	out = append(out, ' ')
	out = strconv.AppendUint(out, uint64(pos.Y), 10)
	out = append(out, ' ')
	return strconv.AppendUint(out, uint64(pos.Z), 10)
}

func appendArea(out []byte, kind string, pos Position, spawn *Spawn) []byte {
	out = append(out, kind...)
	out = appendCoords(append(out, ' '), pos)
	out = append(out, " radius="...)
	out = strconv.AppendInt(out, int64(max(spawn.Radius, 1)), 10)
	return append(out, '\n')
}

func appendCreature(out []byte, pos Position, creature Creature) []byte {
	if creature.NPC {
		out = append(out, "npc "...)
	} else {
		out = append(out, "monster "...)
	}
	out = appendCoords(out, pos)
	out = appendQuoted(append(out, ' '), creature.Name)
	if creature.SpawnTime != 0 {
		out = append(out, " spawntime="...)
		out = strconv.AppendInt(out, int64(creature.SpawnTime), 10)
	}
	if direction := min(int(creature.Direction), len(directionNames)-1); direction != 0 {
		out = append(append(out, " dir="...), directionNames[direction]...)
	}
	if creature.Weight != 0 {
		out = append(out, " weight="...)
		out = strconv.AppendUint(out, uint64(creature.Weight), 10)
	}
	return append(out, '\n')
}

func (w *textWriter) meta(id uint16) bool {
	itemType := w.items.Get(id)
	return itemType != nil && itemType.Flags.Has(otitems.FlagMeta)
}

func (w *textWriter) tile(out []byte, pos Position, tile *Tile) ([]byte, error) {
	ground := tile.Ground.ID != 0 && !w.meta(tile.Ground.ID)
	items := false
	for i := range tile.Items {
		if !w.meta(tile.Items[i].ID) {
			items = true
			break
		}
	}
	zones := tile.Zones()
	if !ground && !items && tile.Flags == 0 && tile.HouseID == 0 && len(zones) == 0 {
		return out, nil // creature or spawn only
	}
	out = appendCoords(out, pos)
	if tile.Flags != 0 || tile.HouseID != 0 || len(zones) > 0 {
		out = append(out, " ["...)
		start := len(out)
		separate := func() {
			if len(out) > start {
				out = append(out, ' ')
			}
		}
		for _, named := range tileFlagNames {
			if tile.Flags&named.flag != 0 {
				separate()
				out = append(out, named.name...)
			}
		}
		if other := tile.Flags &^ namedTileFlags; other != 0 {
			separate()
			out = strconv.AppendUint(append(out, "flags="...), uint64(other), 10)
		}
		if tile.HouseID != 0 {
			separate()
			out = strconv.AppendUint(append(out, "house="...), uint64(tile.HouseID), 10)
		}
		if len(zones) > 0 {
			separate()
			out = append(out, "zone="...)
			for i, zone := range zones {
				if i > 0 {
					out = append(out, ',')
				}
				out = strconv.AppendUint(out, uint64(zone), 10)
			}
		}
		out = append(out, ']')
	}
	out = append(out, ' ')
	var err error
	if ground {
		if out, err = w.item(out, &tile.Ground); err != nil {
			return nil, fmt.Errorf("%d,%d,%d: %w", pos.X, pos.Y, pos.Z, err)
		}
	} else {
		out = append(out, '-')
	}
	for i := range tile.Items {
		if w.meta(tile.Items[i].ID) {
			continue
		}
		if out, err = w.item(append(out, ' '), &tile.Items[i]); err != nil {
			return nil, fmt.Errorf("%d,%d,%d: %w", pos.X, pos.Y, pos.Z, err)
		}
	}
	return append(out, '\n'), nil
}

func (w *textWriter) item(out []byte, item *Item) ([]byte, error) {
	out = strconv.AppendUint(out, uint64(item.ID), 10)
	if item.Subtype != 0 {
		out = strconv.AppendUint(append(out, ':'), uint64(item.Subtype), 10)
	}
	ext := item.Ext
	if ext == nil {
		return out, nil
	}
	var err error
	if out, err = w.itemAttrs(out, item.ID, ext); err != nil {
		return nil, err
	}
	start := len(out)
	out = append(out, '{')
	for i := range ext.Items {
		if w.meta(ext.Items[i].ID) {
			continue
		}
		if len(out) > start+1 {
			out = append(out, ' ')
		}
		if out, err = w.item(out, &ext.Items[i]); err != nil {
			return nil, err
		}
	}
	if len(out) == start+1 {
		return out[:start], nil
	}
	return append(out, '}'), nil
}

func (w *textWriter) itemAttrs(out []byte, id uint16, ext *ItemExt) ([]byte, error) {
	start := len(out)
	out = append(out, '(')
	key := func(name string) {
		if len(out) > start+1 {
			out = append(out, ' ')
		}
		out = append(append(out, name...), '=')
	}
	if ext.ActionID != 0 {
		key("aid")
		out = strconv.AppendUint(out, uint64(ext.ActionID), 10)
	}
	if ext.UniqueID != 0 {
		key("uid")
		out = strconv.AppendUint(out, uint64(ext.UniqueID), 10)
	}
	if ext.DepotID != 0 {
		key("depot")
		out = strconv.AppendUint(out, uint64(ext.DepotID), 10)
	}
	if ext.DoorID != 0 {
		key("door")
		out = strconv.AppendUint(out, uint64(ext.DoorID), 10)
	}
	if ext.Dest != nil {
		key("dest")
		out = appendPosition(out, *ext.Dest)
	}
	if ext.Text != "" {
		key("text")
		out = appendQuoted(out, ext.Text)
	}
	if ext.Description != "" {
		key("desc")
		out = appendQuoted(out, ext.Description)
	}
	if len(ext.Raw) > 0 {
		raw := map[byte][]byte{}
		var declared []byte
		for _, attr := range ext.Raw {
			if _, seen := raw[attr.Code]; !seen && w.attrTypes[attr.Code] != "" {
				declared = append(declared, attr.Code)
			}
			raw[attr.Code] = attr.Data
		}
		var err error
		for _, standard := range rawAttrNames {
			if data, ok := raw[standard.code]; ok {
				key(standard.name)
				if out, err = appendRawValue(out, standard.valueType, data); err != nil {
					return nil, fmt.Errorf("item %d: %s: %w", id, standard.name, err)
				}
				delete(raw, standard.code)
			}
		}
		slices.Sort(declared)
		for _, code := range declared {
			if _, ok := raw[code]; !ok {
				continue // also a standard code
			}
			key(w.attrKeys[code])
			if out, err = appendRawValue(out, w.attrTypes[code], raw[code]); err != nil {
				return nil, fmt.Errorf("item %d: attribute %d: %w", id, code, err)
			}
			delete(raw, code)
		}
		for code := range raw {
			return nil, fmt.Errorf("item %d: attribute %d is not declared", id, code)
		}
	}
	if len(ext.Attrs) > 0 {
		attrs := slices.Clone(ext.Attrs)
		slices.SortStableFunc(attrs, func(left, right Attr) int { return cmp.Compare(left.Key, right.Key) })
		for _, attr := range attrs {
			if len(out) > start+1 {
				out = append(out, ' ')
			}
			out = append(out, '@')
			if validName(attr.Key) {
				out = append(out, attr.Key...)
			} else {
				out = appendQuoted(out, attr.Key)
			}
			out = append(out, '=')
			switch attr.Type {
			case AttrString:
				out = appendQuoted(out, attr.Str)
			case AttrInteger:
				out = strconv.AppendInt(out, int64(attr.Int), 10)
			case AttrFloat:
				out = append(appendFloat(out, attr.Num, 32), 'f')
			case AttrDouble:
				out = appendFloat(out, attr.Num, 64)
			case AttrBoolean:
				out = strconv.AppendBool(out, attr.Value)
			default:
				return nil, fmt.Errorf("item %d: attribute %q has unknown type %d", id, attr.Key, attr.Type)
			}
		}
	}
	if len(out) == start+1 {
		return out[:start], nil
	}
	return append(out, ')'), nil
}

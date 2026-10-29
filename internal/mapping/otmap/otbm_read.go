package otmap

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/formats/otbnode"
)

var otbmIdentifiers = []string{"OTBM", "\x00\x00\x00\x00"}

// streamDepth is the depth of the nodes readOTBM streams: the children of
// the map data node.
const streamDepth = 2

type reader struct {
	gameMap *Map
	items   Items
	format  Format
	// customSizes are the server attributes from the map's attributes file.
	customSizes map[byte]int
	version     uint32
	warnings    []string
	warned      map[string]int
}

func (r *reader) warn(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if len(r.warnings) < maxWarnings {
		r.warnings = append(r.warnings, message)
		return
	}
	problem := message
	if i := strings.LastIndex(message, ": "); i >= 0 {
		problem = message[i+2:]
	}
	r.warned[problem]++
}

func ReadOTBM(data []byte, items Items, format Format) (*Map, []string, error) {
	return readOTBM(data, items, format, nil, nil)
}

func readOTBM(data []byte, items Items, format Format, customSizes map[byte]int, progress Progress) (*Map, []string, error) {
	if len(data) >= 4 {
		if identifier := string(data[:4]); !slices.Contains(otbmIdentifiers, identifier) {
			return nil, nil, fmt.Errorf("%w: identifier %q", ErrNotOTBM, identifier)
		}
	}
	// The map data children (tile areas, towns, waypoints) are streamed: a
	// whole node tree of a big map takes several times the file size. The
	// reader is made from the header when the first child arrives.
	var r *reader
	reportTiles := func(done, total int) {
		if progress != nil {
			progress(StageTiles, done, total)
		}
	}
	root, _, err := otbnode.Stream(data, streamDepth, reportTiles, func(stack []*otbnode.Node, node *otbnode.Node) error {
		if r == nil {
			var err error
			if r, err = newReader(stack[0], stack[1], items, format, customSizes); err != nil {
				return err
			}
		}
		switch node.Type {
		case nodeTileArea:
			r.area(node)
		case nodeTowns:
			r.towns(node)
		case nodeWaypoints:
			r.waypoints(node)
		default:
			r.warn("unknown map node type %d", node.Type)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNotOTBM) {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("%w: %v", ErrNotOTBM, err)
	}
	if r == nil {
		var mapData *otbnode.Node
		if len(root.Children) > 0 {
			mapData = root.Children[0]
		}
		if r, err = newReader(root, mapData, items, format, customSizes); err != nil {
			return nil, nil, err
		}
	}
	for _, chunk := range r.gameMap.chunks {
		chunk.clip()
	}
	for problem, count := range r.warned {
		r.warnings = append(r.warnings, fmt.Sprintf("%d more not listed: %s", count, problem))
	}
	return r.gameMap, r.warnings, nil
}

func newReader(root, mapData *otbnode.Node, items Items, format Format, customSizes map[byte]int) (*reader, error) {
	header := otbnode.NewReader(root.Props)
	version := header.U32()
	width, height := header.U16(), header.U16()
	itemsMajor, itemsMinor := header.U32(), header.U32()
	if header.Err() != nil {
		return nil, fmt.Errorf("%w: header: %v", ErrNotOTBM, header.Err())
	}
	gameMap := New(width, height)
	gameMap.ItemsMajor, gameMap.ItemsMinor = itemsMajor, itemsMinor
	r := &reader{gameMap: gameMap, items: items, format: format, customSizes: customSizes, version: version, warned: map[string]int{}}
	if version > otbmNewest {
		r.warn("OTBM version %d is newer than the editor knows; the map may load wrong", version)
	}
	if mapData == nil || mapData.Type != nodeMapData {
		return nil, fmt.Errorf("%w: no map data", ErrNotOTBM)
	}
	props := otbnode.NewReader(mapData.Props)
	var descriptions []string
	for props.More() {
		switch attr := props.U8(); attr {
		case attrDescription:
			descriptions = append(descriptions, props.String())
		case attrSpawnFile:
			gameMap.SpawnFile = props.String()
		case attrHouseFile:
			gameMap.HouseFile = props.String()
		case attrSpawnNpcFile:
			gameMap.NpcFile = props.String()
			gameMap.SpawnFormat = SpawnsSeparate
		case attrZoneFile:
			gameMap.ZoneFile = props.String()
		default:
			r.warn("unknown map header attribute %d", attr)
			props.Skip(props.Remaining())
		}
	}
	gameMap.Description = pickDescription(descriptions)
	return r, nil
}

// pickDescription drops the "Saved with ..." lines editors add.
func pickDescription(descriptions []string) string {
	var keep []string
	for _, description := range descriptions {
		if !strings.HasPrefix(description, "Saved with ") {
			keep = append(keep, description)
		}
	}
	return strings.Join(keep, "\n")
}

func (r *reader) area(node *otbnode.Node) {
	props := otbnode.NewReader(node.Props)
	baseX, baseY, floor := props.U16(), props.U16(), props.U8()
	if props.Err() != nil || floor > MaxFloor {
		r.warn("tile area without a valid base position")
		return
	}
	for _, tileNode := range node.Children {
		if tileNode.Type != nodeTile && tileNode.Type != nodeHouseTile {
			r.warn("unknown tile node type %d", tileNode.Type)
			continue
		}
		tileProps := otbnode.NewReader(tileNode.Props)
		pos := Position{baseX + uint16(tileProps.U8()), baseY + uint16(tileProps.U8()), floor}
		tile := &Tile{}
		if tileNode.Type == nodeHouseTile {
			tile.HouseID = tileProps.U32()
		}
		if tileProps.Err() != nil {
			r.warn("tile without position in area %d,%d,%d", baseX, baseY, floor)
			continue
		}
		if r.gameMap.Has(pos) {
			r.warn("duplicate tile at %d,%d,%d discarded", pos.X, pos.Y, pos.Z)
			continue
		}
		r.tile(tile, tileNode, tileProps, pos)
		if tile.HouseID != 0 {
			if _, ok := r.gameMap.Houses[tile.HouseID]; !ok {
				r.gameMap.Houses[tile.HouseID] = &House{ID: tile.HouseID, Name: fmt.Sprintf("House #%d", tile.HouseID)}
			}
		}
		if len(tile.Items) == 0 {
			tile.Items = nil
		}
		// Keep empty house/flag tiles: SetTile drops truly empty ones.
		r.gameMap.SetTile(pos, tile)
	}
}

func (r *reader) tile(tile *Tile, node *otbnode.Node, props *otbnode.Reader, pos Position) {
	// Sized up front: grown by append, item slices keep up to twice the room.
	if children := len(node.Children); children > 0 {
		tile.Items = make([]Item, 0, children)
	}
attrs:
	for props.More() {
		switch attr := props.U8(); attr {
		case attrTileFlags:
			tile.Flags = props.U32()
		case attrItem:
			item, ok := r.compactItem(props)
			if !ok {
				r.warn("broken item at %d,%d,%d", pos.X, pos.Y, pos.Z)
				break attrs
			}
			r.place(tile, item)
		default:
			r.warn("unknown tile attribute %d at %d,%d,%d", attr, pos.X, pos.Y, pos.Z)
			break attrs
		}
	}
	for _, child := range node.Children {
		switch child.Type {
		case nodeItem:
			if item, ok := r.itemNode(child, pos); ok {
				r.place(tile, item)
			}
		case nodeTileZone:
			zones := otbnode.NewReader(child.Props)
			count := int(zones.U16())
			for i := 0; i < count && zones.Err() == nil; i++ {
				tile.EnsureExt().Zones = append(tile.EnsureExt().Zones, zones.U16())
			}
		default:
			r.warn("unknown tile child node %d at %d,%d,%d", child.Type, pos.X, pos.Y, pos.Z)
		}
	}
}

// place puts an item on a tile: the first ground item becomes the ground,
// everything else keeps the file order.
func (r *reader) place(tile *Tile, item Item) {
	if itemType := r.items.Get(item.ID); itemType != nil && itemType.IsGround() && tile.Ground.ID == 0 && len(tile.Items) == 0 {
		tile.Ground = item
		return
	}
	tile.Items = append(tile.Items, item)
}

// hasCountByte reports whether a compact item has a count byte: OTBM 1
// stores one for stackables and fluids.
func (r *reader) hasCountByte(id uint16) bool {
	if r.version > 1 {
		return false
	}
	itemType := r.items.Get(id)
	return itemType != nil && itemType.Flags&(otitems.FlagStackable|otitems.FlagSplash|otitems.FlagFluidContainer) != 0
}

func (r *reader) compactItem(props *otbnode.Reader) (Item, bool) {
	item := Item{ID: props.U16()}
	if r.hasCountByte(item.ID) {
		item.Subtype = uint16(props.U8())
	}
	return item, props.Err() == nil && item.ID != 0
}

func (r *reader) itemNode(node *otbnode.Node, pos Position) (Item, bool) {
	props := otbnode.NewReader(node.Props)
	item, ok := r.compactItem(props)
	if !ok {
		r.warn("item without id at %d,%d,%d", pos.X, pos.Y, pos.Z)
		return item, false
	}
	if err := readItemAttrs(props, &item, r.format, r.customSizes); err != nil {
		r.warn("item %d at %d,%d,%d: %v", item.ID, pos.X, pos.Y, pos.Z, err)
	}
	for _, child := range node.Children {
		if child.Type != nodeItem {
			r.warn("unknown container child node %d at %d,%d,%d", child.Type, pos.X, pos.Y, pos.Z)
			continue
		}
		if content, ok := r.itemNode(child, pos); ok {
			item.Ensure().Items = append(item.Ensure().Items, content)
		}
	}
	item.Compact()
	return item, true
}

func readItemAttrs(props *otbnode.Reader, item *Item, format Format, customSizes map[byte]int) error {
	for props.More() {
		attr := props.U8()
		switch attr {
		case attrCount:
			if format.CountU16 {
				item.Subtype = props.U16()
			} else {
				item.Subtype = uint16(props.U8())
			}
		case attrRuneCharges:
			item.Subtype = uint16(props.U8())
		case attrCharges:
			item.Subtype = props.U16()
		case attrActionID:
			item.Ensure().ActionID = props.U16()
		case attrUniqueID:
			item.Ensure().UniqueID = props.U16()
		case attrText:
			item.Ensure().Text = props.String()
		case attrDesc:
			item.Ensure().Description = props.String()
		case attrTeleDest:
			dest := Position{props.U16(), props.U16(), props.U8()}
			item.Ensure().Dest = &dest
		case attrDepotID:
			item.Ensure().DepotID = props.U16()
		case attrHouseDoorID:
			item.Ensure().DoorID = props.U8()
		case attrAttributeMap:
			if err := readAttributeMap(props, item); err != nil {
				return err
			}
		default:
			if err := readRawAttr(props, item, attr, customSizes); err != nil {
				return err
			}
		}
	}
	return props.Err()
}

func readAttributeMap(props *otbnode.Reader, item *Item) error {
	count := int(props.U16())
	for i := 0; i < count && props.Err() == nil; i++ {
		custom := Attr{Key: props.String(), Type: props.U8()}
		switch custom.Type {
		case AttrString:
			custom.Str = props.LongString()
		case AttrInteger:
			custom.Int = int32(props.U32())
		case AttrFloat:
			custom.Num = float64(math.Float32frombits(props.U32()))
		case AttrDouble:
			custom.Num = math.Float64frombits(props.U64())
		case AttrBoolean:
			custom.Value = props.U8() != 0
		default:
			return fmt.Errorf("unknown attribute map type %d", custom.Type)
		}
		item.Ensure().Attrs = append(item.Ensure().Attrs, custom)
	}
	return nil
}

// readRawAttr keeps an attribute the editor does not use verbatim (see
// rawSizes and the map's attributes file).
func readRawAttr(props *otbnode.Reader, item *Item, attr byte, customSizes map[byte]int) error {
	size, ok := rawSizes[attr]
	if !ok {
		size, ok = customSizes[attr]
	}
	if !ok {
		return fmt.Errorf("unknown attribute %d", attr)
	}
	var data []byte
	if size < 0 {
		text := props.String()
		data = append([]byte{byte(len(text)), byte(len(text) >> 8)}, text...)
	} else {
		data = append([]byte(nil), props.Bytes(size)...)
	}
	item.Ensure().Raw = append(item.Ensure().Raw, RawAttr{Code: attr, Data: data})
	return nil
}

func (r *reader) towns(node *otbnode.Node) {
	for _, townNode := range node.Children {
		if townNode.Type != nodeTown {
			r.warn("unknown town node %d", townNode.Type)
			continue
		}
		props := otbnode.NewReader(townNode.Props)
		town := &Town{ID: props.U32(), Name: props.String(), Temple: Position{props.U16(), props.U16(), props.U8()}}
		if props.Err() != nil {
			r.warn("broken town node")
			continue
		}
		if _, duplicate := r.gameMap.Towns[town.ID]; duplicate {
			r.warn("duplicate town id %d discarded", town.ID)
			continue
		}
		r.gameMap.Towns[town.ID] = town
	}
}

func (r *reader) waypoints(node *otbnode.Node) {
	for _, waypointNode := range node.Children {
		if waypointNode.Type != nodeWaypoint {
			continue
		}
		props := otbnode.NewReader(waypointNode.Props)
		waypoint := &Waypoint{Name: props.String(), Pos: Position{props.U16(), props.U16(), props.U8()}}
		if props.Err() != nil {
			r.warn("broken waypoint node")
			continue
		}
		r.gameMap.Waypoints = append(r.gameMap.Waypoints, waypoint)
	}
}

package otmap

import (
	"bytes"
	"io"
	"math"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/formats/otbnode"
)

const (
	// areaMask keeps the base of the 256 x 256 tile area of a coordinate;
	// tiles store their offset in it as a byte.
	areaMask            = 0xFF00
	progressEveryChunks = 64
)

// WriteOTBM writes the map. Meta items (editor-only) are left out.
func WriteOTBM(out io.Writer, gameMap *Map, items Items, format Format) error {
	return writeOTBM(out, gameMap, items, format, nil)
}

func writeOTBM(out io.Writer, gameMap *Map, items Items, format Format, progress func(done, total int)) error {
	nodes := otbnode.NewWriter(out, [4]byte{'O', 'T', 'B', 'M'})
	nodes.Start(nodeRoot)
	nodes.U32(OTBMVersion)
	nodes.U16(gameMap.Width)
	nodes.U16(gameMap.Height)
	nodes.U32(gameMap.ItemsMajor)
	nodes.U32(gameMap.ItemsMinor)
	nodes.Start(nodeMapData)
	writeMapHeader(nodes, gameMap)
	w := &writer{nodes: nodes, items: items, format: format}
	w.tiles(gameMap, progress)
	nodes.Start(nodeTowns)
	for _, town := range gameMap.SortedTowns() {
		nodes.Start(nodeTown)
		nodes.U32(town.ID)
		nodes.String(town.Name)
		w.position(town.Temple)
		nodes.End()
	}
	nodes.End()
	nodes.Start(nodeWaypoints)
	for _, waypoint := range gameMap.Waypoints {
		nodes.Start(nodeWaypoint)
		nodes.String(waypoint.Name)
		w.position(waypoint.Pos)
		nodes.End()
	}
	nodes.End()
	nodes.End() // map data
	nodes.End() // root
	return nodes.Flush()
}

func writeMapHeader(nodes *otbnode.Writer, gameMap *Map) {
	nodes.U8(attrDescription)
	nodes.String("Saved with " + Generator)
	if gameMap.Description != "" {
		nodes.U8(attrDescription)
		nodes.String(gameMap.Description)
	}
	nodes.U8(attrSpawnFile)
	nodes.String(baseName(gameMap.SpawnFile))
	nodes.U8(attrHouseFile)
	nodes.String(baseName(gameMap.HouseFile))
	if gameMap.SpawnFormat == SpawnsSeparate {
		nodes.U8(attrSpawnNpcFile)
		nodes.String(baseName(gameMap.NpcFile))
	}
	if gameMap.ZoneFile != "" || len(gameMap.Zones) > 0 {
		nodes.U8(attrZoneFile)
		nodes.String(baseName(gameMap.ZoneFile))
	}
}

func baseName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

type writer struct {
	nodes  *otbnode.Writer
	items  Items
	format Format
}

func (w *writer) position(position Position) {
	w.nodes.U16(position.X)
	w.nodes.U16(position.Y)
	w.nodes.U8(position.Z)
}

func (w *writer) tiles(gameMap *Map, progress func(done, total int)) {
	area := struct {
		base Position
		open bool
	}{}
	chunks := gameMap.Chunks()
	for i, chunk := range chunks {
		if progress != nil && i%progressEveryChunks == 0 {
			progress(i, len(chunks))
		}
		chunk.Each(func(position Position, tile *Tile) {
			if tile.Ground.ID == 0 && len(tile.Items) == 0 && tile.HouseID == 0 && tile.Flags == 0 && len(tile.Zones()) == 0 {
				return // creature or spawn only: saved in the spawn file
			}
			base := Position{X: position.X & areaMask, Y: position.Y & areaMask, Z: position.Z}
			if !area.open || base != area.base {
				if area.open {
					w.nodes.End()
				}
				w.nodes.Start(nodeTileArea)
				w.position(base)
				area.base, area.open = base, true
			}
			w.tile(position, tile)
		})
	}
	if area.open {
		w.nodes.End()
	}
}

func (w *writer) meta(id uint16) bool {
	itemType := w.items.Get(id)
	return itemType != nil && itemType.Flags.Has(otitems.FlagMeta)
}

func (w *writer) tile(position Position, tile *Tile) {
	if tile.HouseID != 0 {
		w.nodes.Start(nodeHouseTile)
	} else {
		w.nodes.Start(nodeTile)
	}
	w.nodes.U8(uint8(position.X &^ areaMask))
	w.nodes.U8(uint8(position.Y &^ areaMask))
	if tile.HouseID != 0 {
		w.nodes.U32(tile.HouseID)
	}
	if tile.Flags != 0 {
		w.nodes.U8(attrTileFlags)
		w.nodes.U32(tile.Flags)
	}
	if ground := tile.Ground; ground.ID != 0 && !w.meta(ground.ID) {
		if ground.Complex() || ground.Subtype != 0 {
			w.item(&ground)
		} else {
			w.nodes.U8(attrItem)
			w.nodes.U16(ground.ID)
		}
	}
	for i := range tile.Items {
		if !w.meta(tile.Items[i].ID) {
			w.item(&tile.Items[i])
		}
	}
	if zones := tile.Zones(); len(zones) > 0 {
		w.nodes.Start(nodeTileZone)
		w.nodes.U16(uint16(len(zones)))
		for _, id := range zones {
			w.nodes.U16(id)
		}
		w.nodes.End()
	}
	w.nodes.End()
}

func (w *writer) item(item *Item) {
	w.nodes.Start(nodeItem)
	w.nodes.U16(item.ID)
	if item.Subtype != 0 {
		w.subtype(item)
	}
	if ext := item.Ext; ext != nil {
		w.itemExt(ext)
		for i := range ext.Items {
			if !w.meta(ext.Items[i].ID) {
				w.item(&ext.Items[i])
			}
		}
	}
	w.nodes.End()
}

func (w *writer) subtype(item *Item) {
	itemType := w.items.Get(item.ID)
	if itemType != nil && itemType.Flags.Has(otitems.FlagCharges) && !itemType.Flags.Has(otitems.FlagStackable) {
		w.nodes.U8(attrCharges)
		w.nodes.U16(item.Subtype)
		return
	}
	w.nodes.U8(attrCount)
	if w.format.CountU16 {
		w.nodes.U16(item.Subtype)
	} else {
		w.nodes.U8(uint8(min(item.Subtype, math.MaxUint8)))
	}
}

func (w *writer) itemExt(ext *ItemExt) {
	if ext.ActionID != 0 {
		w.nodes.U8(attrActionID)
		w.nodes.U16(ext.ActionID)
	}
	if ext.UniqueID != 0 {
		w.nodes.U8(attrUniqueID)
		w.nodes.U16(ext.UniqueID)
	}
	if ext.Text != "" {
		w.nodes.U8(attrText)
		w.nodes.String(ext.Text)
	}
	if ext.Description != "" {
		w.nodes.U8(attrDesc)
		w.nodes.String(ext.Description)
	}
	if ext.Dest != nil {
		w.nodes.U8(attrTeleDest)
		w.position(*ext.Dest)
	}
	if ext.DepotID != 0 {
		w.nodes.U8(attrDepotID)
		w.nodes.U16(ext.DepotID)
	}
	if ext.DoorID != 0 {
		w.nodes.U8(attrHouseDoorID)
		w.nodes.U8(ext.DoorID)
	}
	for _, raw := range ext.Raw {
		w.nodes.U8(raw.Code)
		w.nodes.Bytes(raw.Data)
	}
	if len(ext.Attrs) > 0 {
		w.attributeMap(ext.Attrs)
	}
}

func (w *writer) attributeMap(attrs []Attr) {
	w.nodes.U8(attrAttributeMap)
	w.nodes.U16(uint16(min(len(attrs), math.MaxUint16)))
	for _, custom := range attrs {
		w.nodes.String(custom.Key)
		w.nodes.U8(custom.Type)
		switch custom.Type {
		case AttrString:
			w.nodes.LongString(custom.Str)
		case AttrInteger:
			w.nodes.U32(uint32(custom.Int))
		case AttrFloat:
			w.nodes.U32(math.Float32bits(float32(custom.Num)))
		case AttrDouble:
			w.nodes.U64(math.Float64bits(custom.Num))
		case AttrBoolean:
			if custom.Value {
				w.nodes.U8(1)
			} else {
				w.nodes.U8(0)
			}
		}
	}
}

func EncodeOTBM(gameMap *Map, items Items, format Format) ([]byte, error) {
	var buf bytes.Buffer
	err := WriteOTBM(&buf, gameMap, items, format)
	return buf.Bytes(), err
}

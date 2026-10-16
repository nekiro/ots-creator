package otmap

import (
	"bytes"
	"io"
	"math"
	"strings"

	"github.com/nekiro/ots-creator/internal/otbnode"
	"github.com/nekiro/ots-creator/internal/otitems"
)

const (
	// areaMask keeps the base of the 256 x 256 tile area of a coordinate;
	// tiles store their offset in it as a byte.
	areaMask = 0xFF00
	// progressEveryChunks is how often writeOTBM reports progress.
	progressEveryChunks = 64
)

// WriteOTBM writes the map. Meta items (editor-only) are left out.
func WriteOTBM(out io.Writer, m *Map, items Items, f Format) error {
	return writeOTBM(out, m, items, f, nil)
}

// writeOTBM is WriteOTBM that reports the chunks written (progress may be
// nil).
func writeOTBM(out io.Writer, m *Map, items Items, f Format, progress func(done, total int)) error {
	nodes := otbnode.NewWriter(out, [4]byte{'O', 'T', 'B', 'M'})
	nodes.Start(nodeRoot)
	nodes.U32(OTBMVersion)
	nodes.U16(m.Width)
	nodes.U16(m.Height)
	nodes.U32(m.ItemsMajor)
	nodes.U32(m.ItemsMinor)
	nodes.Start(nodeMapData)
	writeMapHeader(nodes, m)
	w := &writer{nodes: nodes, items: items, format: f}
	w.tiles(m, progress)
	nodes.Start(nodeTowns)
	for _, town := range m.SortedTowns() {
		nodes.Start(nodeTown)
		nodes.U32(town.ID)
		nodes.String(town.Name)
		w.position(town.Temple)
		nodes.End()
	}
	nodes.End()
	nodes.Start(nodeWaypoints)
	for _, waypoint := range m.Waypoints {
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

// writeMapHeader writes the attributes of the map data node.
func writeMapHeader(nodes *otbnode.Writer, m *Map) {
	nodes.U8(attrDescription)
	nodes.String("Saved with " + Generator)
	if m.Description != "" {
		nodes.U8(attrDescription)
		nodes.String(m.Description)
	}
	nodes.U8(attrSpawnFile)
	nodes.String(baseName(m.SpawnFile))
	nodes.U8(attrHouseFile)
	nodes.String(baseName(m.HouseFile))
	if m.SpawnFormat == SpawnsSeparate {
		nodes.U8(attrSpawnNpcFile)
		nodes.String(baseName(m.NpcFile))
	}
	if m.ZoneFile != "" || len(m.Zones) > 0 {
		nodes.U8(attrZoneFile)
		nodes.String(baseName(m.ZoneFile))
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

func (w *writer) position(p Position) {
	w.nodes.U16(p.X)
	w.nodes.U16(p.Y)
	w.nodes.U8(p.Z)
}

// tiles writes the tiles in tile area nodes.
func (w *writer) tiles(m *Map, progress func(done, total int)) {
	area := struct {
		base Position
		open bool
	}{}
	chunks := m.Chunks()
	for i, chunk := range chunks {
		if progress != nil && i%progressEveryChunks == 0 {
			progress(i, len(chunks))
		}
		chunk.Each(func(p Position, tile *Tile) {
			if tile.Ground.ID == 0 && len(tile.Items) == 0 && tile.HouseID == 0 && tile.Flags == 0 && len(tile.Zones()) == 0 {
				return // creature or spawn only: saved in the spawn file
			}
			base := Position{X: p.X & areaMask, Y: p.Y & areaMask, Z: p.Z}
			if !area.open || base != area.base {
				if area.open {
					w.nodes.End()
				}
				w.nodes.Start(nodeTileArea)
				w.position(base)
				area.base, area.open = base, true
			}
			w.tile(p, tile)
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

func (w *writer) tile(p Position, tile *Tile) {
	if tile.HouseID != 0 {
		w.nodes.Start(nodeHouseTile)
	} else {
		w.nodes.Start(nodeTile)
	}
	w.nodes.U8(uint8(p.X &^ areaMask))
	w.nodes.U8(uint8(p.Y &^ areaMask))
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

// subtype writes the count, or the charges of items with charges.
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

// attributeMap writes the custom attributes of newer servers.
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

// EncodeOTBM returns the map as OTBM bytes.
func EncodeOTBM(m *Map, items Items, f Format) ([]byte, error) {
	var buf bytes.Buffer
	err := WriteOTBM(&buf, m, items, f)
	return buf.Bytes(), err
}

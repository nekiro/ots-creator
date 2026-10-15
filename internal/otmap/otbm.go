package otmap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/nekiro/ots-creator/internal/otbnode"
	"github.com/nekiro/ots-creator/internal/otitems"
)

// OTBM node types.
const (
	nodeRoot      = 0
	nodeMapData   = 2
	nodeTileArea  = 4
	nodeTile      = 5
	nodeItem      = 6
	nodeTowns     = 12
	nodeTown      = 13
	nodeHouseTile = 14
	nodeWaypoints = 15
	nodeWaypoint  = 16
	nodeTileZone  = 19
)

// OTBM attributes.
const (
	attrDescription   = 1
	attrTileFlags     = 3
	attrActionID      = 4
	attrUniqueID      = 5
	attrText          = 6
	attrDesc          = 7
	attrTeleDest      = 8
	attrItem          = 9
	attrDepotID       = 10
	attrSpawnFile     = 11
	attrRuneCharges   = 12
	attrHouseFile     = 13
	attrHouseDoorID   = 14
	attrCount         = 15
	attrDuration      = 16
	attrDecayingState = 17
	attrWrittenDate   = 18
	attrWrittenBy     = 19
	attrSleeperGUID   = 20
	attrSleepStart    = 21
	attrCharges       = 22
	attrSpawnNpcFile  = 23
	attrZoneFile      = 24
	attrAttributeMap  = 128
)

// Generator is written into saved maps as the first description.
var Generator = "OTS Creator"

// ErrNotOTBM is returned for files that are not OTBM maps.
var ErrNotOTBM = errors.New("not an OTBM map")

// Items is what the codec needs to know about item types.
type Items interface {
	Get(id uint16) *otitems.Type
}

// Format holds OTBM variants of server forks. The zero value is the
// standard format.
type Format struct {
	// CountU16 stores the count attribute as uint16 instead of uint8.
	CountU16 bool `json:"countU16"`
}

type reader struct {
	m        *Map
	items    Items
	format   Format
	version  uint32
	warnings []string
	warned   map[string]int
}

// maxWarnings bounds the memory of a badly broken map; past it warnings are
// only counted, per problem.
const maxWarnings = 100_000

// warn records a warning, "where: problem".
func (r *reader) warn(format string, args ...any) {
	if len(r.warnings) < maxWarnings {
		r.warnings = append(r.warnings, fmt.Sprintf(format, args...))
		return
	}
	msg := fmt.Sprintf(format, args...)
	key := msg
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		key = msg[i+2:]
	}
	r.warned[key]++
}

// ReadOTBM parses OTBM data. Unknown or broken parts are skipped and
// reported as warnings.
func ReadOTBM(data []byte, items Items, f Format) (*Map, []string, error) {
	return readOTBM(data, items, f, nil)
}

func readOTBM(data []byte, items Items, f Format, progress Progress) (*Map, []string, error) {
	report := func(stage Stage, done, total int) {
		if progress != nil {
			progress(stage, done, total)
		}
	}
	// The map data children (tile areas, towns, waypoints) are streamed: a
	// whole node tree of a big map takes several times the file size.
	var r *reader
	header := func(root, md *otbnode.Node) error {
		hr := otbnode.NewReader(root.Props)
		version := hr.U32()
		w, h := hr.U16(), hr.U16()
		major, minor := hr.U32(), hr.U32()
		if hr.Err() != nil {
			return fmt.Errorf("%w: header: %v", ErrNotOTBM, hr.Err())
		}
		m := New(w, h)
		m.ItemsMajor, m.ItemsMinor = major, minor
		r = &reader{m: m, items: items, format: f, version: version, warned: map[string]int{}}
		if version > otbmNewest {
			r.warn("OTBM version %d is newer than the editor knows; the map may load wrong", version)
		}
		if md == nil || md.Type != nodeMapData {
			return fmt.Errorf("%w: no map data", ErrNotOTBM)
		}
		mr := otbnode.NewReader(md.Props)
		var descs []string
		for mr.More() {
			switch a := mr.U8(); a {
			case attrDescription:
				descs = append(descs, mr.String())
			case attrSpawnFile:
				m.SpawnFile = mr.String()
			case attrHouseFile:
				m.HouseFile = mr.String()
			case attrSpawnNpcFile:
				m.NpcFile = mr.String()
				m.SpawnFormat = SpawnsSeparate
			case attrZoneFile:
				m.ZoneFile = mr.String()
			default:
				r.warn("unknown map header attribute %d", a)
				mr.Skip(mr.Remaining())
			}
		}
		m.Description = pickDescription(descs)
		return nil
	}
	if len(data) >= 4 {
		if s := string(data[:4]); s != "OTBM" && s != "\x00\x00\x00\x00" {
			return nil, nil, fmt.Errorf("%w: identifier %q", ErrNotOTBM, s)
		}
	}
	progressFn := func(done, total int) { report(StageTiles, done, total) }
	root, _, err := otbnode.Stream(data, 2, progressFn, func(stack []*otbnode.Node, n *otbnode.Node) error {
		if r == nil {
			if err := header(stack[0], stack[1]); err != nil {
				return err
			}
		}
		switch n.Type {
		case nodeTileArea:
			r.area(n)
		case nodeTowns:
			r.towns(n)
		case nodeWaypoints:
			r.waypoints(n)
		default:
			r.warn("unknown map node type %d", n.Type)
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
		// A map without tile areas: nothing was streamed.
		var md *otbnode.Node
		if len(root.Children) > 0 {
			md = root.Children[0]
		}
		if err := header(root, md); err != nil {
			return nil, nil, err
		}
	}
	m := r.m
	for _, c := range m.chunks {
		c.clip()
	}
	for k, n := range r.warned {
		r.warnings = append(r.warnings, fmt.Sprintf("%d more not listed: %s", n, k))
	}
	return m, r.warnings, nil
}

// pickDescription drops the "Saved with ..." lines editors add.
func pickDescription(descs []string) string {
	var keep []string
	for _, d := range descs {
		if strings.HasPrefix(d, "Saved with ") {
			continue
		}
		keep = append(keep, d)
	}
	return strings.Join(keep, "\n")
}

func (r *reader) area(n *otbnode.Node) {
	ar := otbnode.NewReader(n.Props)
	bx, by, bz := ar.U16(), ar.U16(), ar.U8()
	if ar.Err() != nil || bz > MaxFloor {
		r.warn("tile area without a valid base position")
		return
	}
	for _, tn := range n.Children {
		if tn.Type != nodeTile && tn.Type != nodeHouseTile {
			r.warn("unknown tile node type %d", tn.Type)
			continue
		}
		tr := otbnode.NewReader(tn.Props)
		pos := Position{bx + uint16(tr.U8()), by + uint16(tr.U8()), bz}
		t := &Tile{}
		if tn.Type == nodeHouseTile {
			t.HouseID = tr.U32()
		}
		if tr.Err() != nil {
			r.warn("tile without position in area %d,%d,%d", bx, by, bz)
			continue
		}
		if r.m.Has(pos) {
			r.warn("duplicate tile at %d,%d,%d discarded", pos.X, pos.Y, pos.Z)
			continue
		}
		// Sized up front: grown by append, item slices keep up to twice the room.
		if k := len(tn.Children); k > 0 {
			t.Items = make([]Item, 0, k)
		}
	attrs:
		for tr.More() {
			switch a := tr.U8(); a {
			case attrTileFlags:
				t.Flags = tr.U32()
			case attrItem:
				it, ok := r.compactItem(tr)
				if !ok {
					r.warn("broken item at %d,%d,%d", pos.X, pos.Y, pos.Z)
					break attrs
				}
				r.place(t, it)
			default:
				r.warn("unknown tile attribute %d at %d,%d,%d", a, pos.X, pos.Y, pos.Z)
				break attrs
			}
		}
		for _, in := range tn.Children {
			switch in.Type {
			case nodeItem:
				it, ok := r.itemNode(in, pos)
				if ok {
					r.place(t, it)
				}
			case nodeTileZone:
				zr := otbnode.NewReader(in.Props)
				cnt := int(zr.U16())
				for i := 0; i < cnt && zr.Err() == nil; i++ {
					t.EnsureExt().Zones = append(t.EnsureExt().Zones, zr.U16())
				}
			default:
				r.warn("unknown tile child node %d at %d,%d,%d", in.Type, pos.X, pos.Y, pos.Z)
			}
		}
		if t.HouseID != 0 {
			if _, ok := r.m.Houses[t.HouseID]; !ok {
				r.m.Houses[t.HouseID] = &House{ID: t.HouseID, Name: fmt.Sprintf("House #%d", t.HouseID)}
			}
		}
		if len(t.Items) == 0 {
			t.Items = nil
		}
		// Keep empty house/flag tiles: SetTile drops truly empty ones.
		r.m.SetTile(pos, t)
	}
}

// place puts an item on a tile: the first ground item becomes the ground,
// everything else keeps the file order.
func (r *reader) place(t *Tile, it Item) {
	if typ := r.items.Get(it.ID); typ != nil && typ.IsGround() && t.Ground.ID == 0 && len(t.Items) == 0 {
		t.Ground = it
		return
	}
	t.Items = append(t.Items, it)
}

func (r *reader) hasCountByte(id uint16) bool {
	if r.version > 1 {
		return false
	}
	typ := r.items.Get(id)
	return typ != nil && typ.Flags&(otitems.FlagStackable|otitems.FlagSplash|otitems.FlagFluidContainer) != 0
}

func (r *reader) compactItem(tr *otbnode.Reader) (Item, bool) {
	it := Item{ID: tr.U16()}
	if r.hasCountByte(it.ID) {
		it.Subtype = uint16(tr.U8())
	}
	return it, tr.Err() == nil && it.ID != 0
}

func (r *reader) itemNode(n *otbnode.Node, pos Position) (Item, bool) {
	ir := otbnode.NewReader(n.Props)
	it, ok := r.compactItem(ir)
	if !ok {
		r.warn("item without id at %d,%d,%d", pos.X, pos.Y, pos.Z)
		return it, false
	}
	if err := readItemAttrs(ir, &it, r.format); err != nil {
		r.warn("item %d at %d,%d,%d: %v", it.ID, pos.X, pos.Y, pos.Z, err)
	}
	for _, cn := range n.Children {
		if cn.Type != nodeItem {
			r.warn("unknown container child node %d at %d,%d,%d", cn.Type, pos.X, pos.Y, pos.Z)
			continue
		}
		if child, ok := r.itemNode(cn, pos); ok {
			it.Ensure().Items = append(it.Ensure().Items, child)
		}
	}
	it.Compact()
	return it, true
}

// rawSizes are attributes stored verbatim, by their payload size; -1 is a
// uint16-prefixed string.
var rawSizes = map[byte]int{
	attrDuration: 4, attrDecayingState: 1, attrWrittenDate: 4, attrWrittenBy: -1,
	attrSleeperGUID: 4, attrSleepStart: 4,
}

func readItemAttrs(r *otbnode.Reader, it *Item, f Format) error {
	for r.More() {
		a := r.U8()
		switch a {
		case attrCount:
			if f.CountU16 {
				it.Subtype = r.U16()
			} else {
				it.Subtype = uint16(r.U8())
			}
		case attrRuneCharges:
			it.Subtype = uint16(r.U8())
		case attrCharges:
			it.Subtype = r.U16()
		case attrActionID:
			it.Ensure().ActionID = r.U16()
		case attrUniqueID:
			it.Ensure().UniqueID = r.U16()
		case attrText:
			it.Ensure().Text = r.String()
		case attrDesc:
			it.Ensure().Description = r.String()
		case attrTeleDest:
			d := Position{r.U16(), r.U16(), r.U8()}
			it.Ensure().Dest = &d
		case attrDepotID:
			it.Ensure().DepotID = r.U16()
		case attrHouseDoorID:
			it.Ensure().DoorID = r.U8()
		case attrAttributeMap:
			n := int(r.U16())
			for i := 0; i < n && r.Err() == nil; i++ {
				at := Attr{Key: r.String(), Type: r.U8()}
				switch at.Type {
				case AttrString:
					at.Str = r.LongString()
				case AttrInteger:
					at.Int = int32(r.U32())
				case AttrFloat:
					at.Num = float64(math.Float32frombits(r.U32()))
				case AttrDouble:
					at.Num = math.Float64frombits(r.U64())
				case AttrBoolean:
					at.Value = r.U8() != 0
				default:
					return fmt.Errorf("unknown attribute map type %d", at.Type)
				}
				it.Ensure().Attrs = append(it.Ensure().Attrs, at)
			}
		default:
			size, ok := rawSizes[a]
			if !ok {
				return fmt.Errorf("unknown attribute %d", a)
			}
			var data []byte
			if size < 0 {
				s := r.String()
				data = append([]byte{byte(len(s)), byte(len(s) >> 8)}, s...)
			} else {
				data = append([]byte(nil), r.Bytes(size)...)
			}
			it.Ensure().Raw = append(it.Ensure().Raw, RawAttr{Code: a, Data: data})
		}
	}
	return r.Err()
}

func (r *reader) towns(n *otbnode.Node) {
	for _, tn := range n.Children {
		if tn.Type != nodeTown {
			r.warn("unknown town node %d", tn.Type)
			continue
		}
		tr := otbnode.NewReader(tn.Props)
		t := &Town{ID: tr.U32(), Name: tr.String(), Temple: Position{tr.U16(), tr.U16(), tr.U8()}}
		if tr.Err() != nil {
			r.warn("broken town node")
			continue
		}
		if _, dup := r.m.Towns[t.ID]; dup {
			r.warn("duplicate town id %d discarded", t.ID)
			continue
		}
		r.m.Towns[t.ID] = t
	}
}

func (r *reader) waypoints(n *otbnode.Node) {
	for _, wn := range n.Children {
		if wn.Type != nodeWaypoint {
			continue
		}
		wr := otbnode.NewReader(wn.Props)
		w := &Waypoint{Name: wr.String(), Pos: Position{wr.U16(), wr.U16(), wr.U8()}}
		if wr.Err() != nil {
			r.warn("broken waypoint node")
			continue
		}
		r.m.Waypoints = append(r.m.Waypoints, w)
	}
}

// WriteOTBM writes the map. Meta items (editor-only) are left out.
func WriteOTBM(out io.Writer, m *Map, items Items, f Format) error {
	w := otbnode.NewWriter(out, [4]byte{'O', 'T', 'B', 'M'})
	w.Start(nodeRoot)
	w.U32(OTBMVersion)
	w.U16(m.Width)
	w.U16(m.Height)
	w.U32(m.ItemsMajor)
	w.U32(m.ItemsMinor)
	w.Start(nodeMapData)
	w.U8(attrDescription)
	w.String("Saved with " + Generator)
	if m.Description != "" {
		w.U8(attrDescription)
		w.String(m.Description)
	}
	w.U8(attrSpawnFile)
	w.String(baseName(m.SpawnFile))
	w.U8(attrHouseFile)
	w.String(baseName(m.HouseFile))
	if m.SpawnFormat == SpawnsSeparate {
		w.U8(attrSpawnNpcFile)
		w.String(baseName(m.NpcFile))
	}
	if m.ZoneFile != "" || len(m.Zones) > 0 {
		w.U8(attrZoneFile)
		w.String(baseName(m.ZoneFile))
	}
	ww := &writer{w: w, items: items, format: f}
	area := struct {
		x, y uint16
		z    uint8
		open bool
	}{}
	for _, c := range m.Chunks() {
		c.Each(func(p Position, t *Tile) {
			if t.Ground.ID == 0 && len(t.Items) == 0 && t.HouseID == 0 && t.Flags == 0 && len(t.Zones()) == 0 {
				return // creature or spawn only: saved in the spawn file
			}
			ax, ay := p.X&0xFF00, p.Y&0xFF00
			if !area.open || ax != area.x || ay != area.y || p.Z != area.z {
				if area.open {
					w.End()
				}
				w.Start(nodeTileArea)
				w.U16(ax)
				w.U16(ay)
				w.U8(p.Z)
				area.x, area.y, area.z, area.open = ax, ay, p.Z, true
			}
			ww.tile(p, t)
		})
	}
	if area.open {
		w.End()
	}
	w.Start(nodeTowns)
	for _, t := range m.SortedTowns() {
		w.Start(nodeTown)
		w.U32(t.ID)
		w.String(t.Name)
		w.U16(t.Temple.X)
		w.U16(t.Temple.Y)
		w.U8(t.Temple.Z)
		w.End()
	}
	w.End()
	w.Start(nodeWaypoints)
	for _, wp := range m.Waypoints {
		w.Start(nodeWaypoint)
		w.String(wp.Name)
		w.U16(wp.Pos.X)
		w.U16(wp.Pos.Y)
		w.U8(wp.Pos.Z)
		w.End()
	}
	w.End()
	w.End() // map data
	w.End() // root
	return w.Flush()
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

type writer struct {
	w      *otbnode.Writer
	items  Items
	format Format
}

func (w *writer) meta(id uint16) bool {
	t := w.items.Get(id)
	return t != nil && t.Flags.Has(otitems.FlagMeta)
}

func (w *writer) tile(p Position, t *Tile) {
	if t.HouseID != 0 {
		w.w.Start(nodeHouseTile)
	} else {
		w.w.Start(nodeTile)
	}
	w.w.U8(uint8(p.X & 0xFF))
	w.w.U8(uint8(p.Y & 0xFF))
	if t.HouseID != 0 {
		w.w.U32(t.HouseID)
	}
	if t.Flags != 0 {
		w.w.U8(attrTileFlags)
		w.w.U32(t.Flags)
	}
	if g := t.Ground; g.ID != 0 && !w.meta(g.ID) {
		if g.Complex() || g.Subtype != 0 {
			w.item(&g)
		} else {
			w.w.U8(attrItem)
			w.w.U16(g.ID)
		}
	}
	for i := range t.Items {
		if !w.meta(t.Items[i].ID) {
			w.item(&t.Items[i])
		}
	}
	if z := t.Zones(); len(z) > 0 {
		w.w.Start(nodeTileZone)
		w.w.U16(uint16(len(z)))
		for _, id := range z {
			w.w.U16(id)
		}
		w.w.End()
	}
	w.w.End()
}

func (w *writer) item(it *Item) {
	w.w.Start(nodeItem)
	w.w.U16(it.ID)
	if it.Subtype != 0 {
		t := w.items.Get(it.ID)
		if t != nil && t.Flags.Has(otitems.FlagCharges) && !t.Flags.Has(otitems.FlagStackable) {
			w.w.U8(attrCharges)
			w.w.U16(it.Subtype)
		} else {
			w.w.U8(attrCount)
			if w.format.CountU16 {
				w.w.U16(it.Subtype)
			} else {
				w.w.U8(uint8(min(it.Subtype, 255)))
			}
		}
	}
	if e := it.Ext; e != nil {
		if e.ActionID != 0 {
			w.w.U8(attrActionID)
			w.w.U16(e.ActionID)
		}
		if e.UniqueID != 0 {
			w.w.U8(attrUniqueID)
			w.w.U16(e.UniqueID)
		}
		if e.Text != "" {
			w.w.U8(attrText)
			w.w.String(e.Text)
		}
		if e.Description != "" {
			w.w.U8(attrDesc)
			w.w.String(e.Description)
		}
		if e.Dest != nil {
			w.w.U8(attrTeleDest)
			w.w.U16(e.Dest.X)
			w.w.U16(e.Dest.Y)
			w.w.U8(e.Dest.Z)
		}
		if e.DepotID != 0 {
			w.w.U8(attrDepotID)
			w.w.U16(e.DepotID)
		}
		if e.DoorID != 0 {
			w.w.U8(attrHouseDoorID)
			w.w.U8(e.DoorID)
		}
		for _, r := range e.Raw {
			w.w.U8(r.Code)
			w.w.Bytes(r.Data)
		}
		if len(e.Attrs) > 0 {
			w.w.U8(attrAttributeMap)
			w.w.U16(uint16(min(len(e.Attrs), math.MaxUint16)))
			for _, a := range e.Attrs {
				w.w.String(a.Key)
				w.w.U8(a.Type)
				switch a.Type {
				case AttrString:
					w.w.LongString(a.Str)
				case AttrInteger:
					w.w.U32(uint32(a.Int))
				case AttrFloat:
					w.w.U32(math.Float32bits(float32(a.Num)))
				case AttrDouble:
					w.w.U64(math.Float64bits(a.Num))
				case AttrBoolean:
					if a.Value {
						w.w.U8(1)
					} else {
						w.w.U8(0)
					}
				}
			}
		}
		for i := range e.Items {
			if !w.meta(e.Items[i].ID) {
				w.item(&e.Items[i])
			}
		}
	}
	w.w.End()
}

// EncodeOTBM returns the map as OTBM bytes.
func EncodeOTBM(m *Map, items Items, f Format) ([]byte, error) {
	var buf bytes.Buffer
	err := WriteOTBM(&buf, m, items, f)
	return buf.Bytes(), err
}

package app

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"math"
	"runtime"
	"sync"

	"github.com/nekiro/ots-creator/internal/otitems"
	"github.com/nekiro/ots-creator/internal/otmap"
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

// View options the chunk geometry depends on (bits of the o parameter;
// see GEO in frontend/src/lib/map/geo.ts).
const (
	geoPathing = 1 << iota
	geoHouses
	geoSpecial
	geoZones
	geoSpawns
	geoHighlight
	geoGhostItems
	geoCreatures
	geoOnlyModified
	geoAnimation
	// geoFar leaves out what does not shape the map seen from far away:
	// movable and pickupable items.
	geoFar
)

// Sprite values of the geometry besides sprite ids (quads of sprites are
// one sprite size a side): geoHidden hides the quad (an animation frame
// without a sprite), geoComposed marks a colored outfit piece (the low
// bits index the chunk's composed table) and geoRect a colored rectangle
// (width in bits 15-29, height in bits 0-14).
const (
	geoHidden   = 0xFFFFFFFF
	geoComposed = 0x80000000
	geoRect     = 0x40000000
)

// GeoOptions is what chunk geometry is built for besides the chunk.
type GeoOptions struct {
	Opts  uint32
	House uint32
	// Block makes every key stand for Block x Block chunks from it, built
	// into one geometry (fewer draws far out). 0 or 1: single chunks.
	Block int
}

// geoThing is the drawing data of one client thing.
type geoThing struct {
	g        *thing.FrameGroup
	ox, oy   int
	el       int
	hangable bool
}

// geoBuilder builds the quads of chunks. It is not safe for concurrent
// use: every worker has its own (and its own thing cache).
type geoBuilder struct {
	p       *project.Project
	items   *otitems.DB
	ed      interface{ IsBlocking(*otmap.Tile) bool }
	outfit  func(name string) (lookType, mount, lookItem uint16, head, body, legs, feet, addons uint8)
	o       GeoOptions
	S       int
	things  map[uint32]*geoThing // item client ids, outfits as 1<<20|id
	b       []byte
	n       int
	anim    []byte
	nAnim   int
	comp    []byte
	nComp   int
	sprites map[uint32]struct{}
}

// One quad, 12 bytes: int16 x, y (world pixels from the chunk's corner),
// uint32 sprite (see geoRect), rgba8 color.
func (g *geoBuilder) quad(x, y int, sprite uint32, r, gr, b, a float64) {
	g.b = binary.LittleEndian.AppendUint16(g.b, uint16(int16(x)))
	g.b = binary.LittleEndian.AppendUint16(g.b, uint16(int16(y)))
	g.b = binary.LittleEndian.AppendUint32(g.b, sprite)
	g.b = append(g.b, unit8(r), unit8(gr), unit8(b), unit8(a))
	g.n++
}

func unit8(v float64) uint8 {
	return uint8(math.Round(min(max(v, 0), 1) * 255))
}

func (g *geoBuilder) rect(x, y, w, h int, c [4]float64) {
	size := uint32(min(max(w, 0), 0x7FFF))<<15 | uint32(min(max(h, 0), 0x7FFF))
	g.quad(x, y, geoRect|size, c[0], c[1], c[2], c[3])
}

func (g *geoBuilder) thingOf(c thing.Category, id uint32) *geoThing {
	k := id
	if c == thing.CategoryOutfit {
		k |= 1 << 20
	}
	if th, ok := g.things[k]; ok {
		return th
	}
	var th *geoThing
	if t, err := g.p.Thing(c, id); err == nil && len(t.FrameGroups) > 0 {
		fg := t.FrameGroups[0]
		if fg.PatternX > 0 && fg.PatternY > 0 && fg.PatternZ > 0 && fg.Width > 0 && fg.Height > 0 && fg.Layers > 0 {
			th = &geoThing{g: fg, hangable: t.Props.Hangable}
			if t.Props.HasOffset {
				th.ox, th.oy = int(t.Props.OffsetX), int(t.Props.OffsetY)
			}
			if t.Props.HasElevation {
				th.el = int(t.Props.Elevation)
			}
		}
	}
	g.things[k] = th
	return th
}

// look emits the pieces of one texture with its bottom-right tile at
// (ax, ay). animated records the pieces of items for frame patching;
// colors makes composed outfit pieces.
func (g *geoBuilder) look(th *geoThing, cid uint32, px, py, pz, ax, ay int, r, gr, b, a float64, animated bool, colors *[4]uint8) {
	fg := th.g
	S := g.S
	layers := int(fg.Layers)
	if colors != nil {
		layers = 1
	}
	rec := animated && colors == nil && fg.Frames > 1
	ax -= th.ox
	ay -= th.oy
	for l := 0; l < layers; l++ {
		for cy := 0; cy < int(fg.Height); cy++ {
			for cx := 0; cx < int(fg.Width); cx++ {
				idx := fg.SpriteIndex(cx, cy, l, px, py, pz, 0)
				if idx >= len(fg.Sprites) {
					continue
				}
				sid := fg.Sprites[idx]
				x, y := ax-cx*S, ay-cy*S
				if rec {
					g.anim = binary.LittleEndian.AppendUint32(g.anim, uint32(g.n))
					g.anim = binary.LittleEndian.AppendUint32(g.anim, cid)
					g.anim = binary.LittleEndian.AppendUint32(g.anim, uint32(idx))
					g.nAnim++
				}
				if sid == 0 {
					if rec {
						g.quad(x, y, geoHidden, r, gr, b, a)
					}
					continue
				}
				if colors != nil {
					tpl := uint32(0)
					if t := idx + int(fg.Height)*int(fg.Width); t < len(fg.Sprites) {
						tpl = fg.Sprites[t]
					}
					g.comp = binary.LittleEndian.AppendUint32(g.comp, uint32(g.n))
					g.comp = binary.LittleEndian.AppendUint32(g.comp, sid)
					g.comp = binary.LittleEndian.AppendUint32(g.comp, tpl)
					g.comp = append(g.comp, colors[0], colors[1], colors[2], colors[3])
					g.quad(x, y, geoComposed|uint32(g.nComp), r, gr, b, a)
					g.nComp++
					continue
				}
				g.sprites[sid] = struct{}{}
				g.quad(x, y, sid, r, gr, b, a)
			}
		}
	}
}

// item emits one item and returns its elevation.
func (g *geoBuilder) item(it otmap.Item, x, y, z, ax, ay int, r, gr, b, a float64, hook int) int {
	S := g.S
	typ := g.items.Get(it.ID)
	if typ == nil {
		g.rect(ax, ay, S, S, [4]float64{0.8, 0, 0, 0.6 * a})
		return 0
	}
	if typ.Flags.Has(otitems.FlagMeta) {
		return 0
	}
	th := g.thingOf(thing.CategoryItem, uint32(typ.ClientID))
	if th == nil {
		g.rect(ax, ay, S, S, [4]float64{0.8, 0, 0, 0.5 * a})
		return 0
	}
	fg := th.g
	var px, py int
	switch {
	case typ.Flags&(otitems.FlagSplash|otitems.FlagFluidContainer) != 0:
		c := int(it.Subtype % 8)
		px, py = c%4%int(fg.PatternX), c/4%int(fg.PatternY)
	case th.hangable && hook != 0:
		px = min(hook, int(fg.PatternX)-1)
	case typ.Flags.Has(otitems.FlagStackable) && fg.PatternX == 4 && fg.PatternY == 2:
		switch n := int(it.Subtype); {
		case n <= 0:
		case n < 5:
			px = n - 1
		case n < 10:
			py = 1
		case n < 25:
			px, py = 1, 1
		case n < 50:
			px, py = 2, 1
		default:
			px, py = 3, 1
		}
	default:
		px, py = x%int(fg.PatternX), y%int(fg.PatternY)
	}
	g.look(th, uint32(typ.ClientID), px, py, z%int(fg.PatternZ), ax, ay, r, gr, b, a, g.o.Opts&geoAnimation != 0, nil)
	return th.el
}

// creature emits a creature as the client draws it standing still.
func (g *geoBuilder) creature(c otmap.Creature, x, y, z, ax, ay int) {
	S := g.S
	dark := 1.0
	if c.Selected {
		dark = 0.5
	}
	lookType, mount, lookItem, head, body, legs, feet, addons := g.outfit(c.Name)
	if lookItem != 0 {
		g.item(otmap.Item{ID: lookItem}, x, y, z, ax, ay, dark, dark, dark, 1, 0)
		return
	}
	if lookType == 0 {
		// Unknown creature: a marker so it is not invisible.
		col := [4]float64{1, 0.5, 0.2, 0.8}
		if c.NPC {
			col = [4]float64{0.3, 0.6, 1, 0.8}
		}
		g.rect(ax+S/4, ay+S/4, S/2, S/2, col)
		return
	}
	th := g.thingOf(thing.CategoryOutfit, uint32(lookType))
	if th == nil {
		return
	}
	fg := th.g
	dir := min(int(c.Direction&3), int(fg.PatternX)-1)
	mounted := 0
	if mount != 0 && fg.PatternZ > 1 {
		if m := g.thingOf(thing.CategoryOutfit, uint32(mount)); m != nil {
			g.look(m, 0, min(dir, int(m.g.PatternX)-1), 0, 0, ax, ay, dark, dark, dark, 1, false, nil)
			mounted = 1
		}
	}
	var colors *[4]uint8
	if fg.Layers > 1 {
		colors = &[4]uint8{head, body, legs, feet}
	}
	g.look(th, 0, dir, 0, mounted, ax, ay, dark, dark, dark, 1, false, colors)
	for _, a := range [2][2]int{{1, 1}, {2, 2}} {
		if int(addons)&a[0] != 0 && int(fg.PatternY) > a[1] {
			g.look(th, 0, dir, a[1], mounted, ax, ay, dark, dark, dark, 1, false, colors)
		}
	}
}

// tile emits one tile, stacked as the client does: ground, borders,
// bottom items, common items, creatures, top items. Mirrors drawChunk of
// the old frontend builder.
func (g *geoBuilder) tile(t *otmap.Tile, x, y, z, ax0, ay0 int) {
	o := g.o.Opts
	S := g.S
	r, gr, b := 1.0, 1.0, 1.0
	n := len(t.Items)
	if t.Ground.ID != 0 {
		n++
	}
	if o&geoPathing != 0 && n > 0 && g.ed.IsBlocking(t) {
		gr *= 2.0 / 3
		b *= 2.0 / 3
	}
	if o&geoHouses != 0 && t.HouseID != 0 {
		r *= 0.5
		if t.HouseID != g.o.House {
			gr *= 0.5
		}
	} else if o&geoSpecial != 0 && t.Flags&otmap.FlagPZ != 0 {
		r *= 0.5
		b *= 0.5
	}
	if o&geoSpecial != 0 && t.Flags&otmap.FlagPVPZone != 0 {
		gr = r / 4
		b = b * 2 / 3
	}
	if o&geoSpecial != 0 && t.Flags&otmap.FlagNoLogout != 0 {
		b /= 2
	}
	if o&geoSpecial != 0 && t.Flags&otmap.FlagNoPVP != 0 {
		gr /= 2
	}
	if o&geoZones != 0 && len(t.Zones()) > 0 {
		r *= 0.85
		gr *= 0.7
	}
	if o&geoSpawns != 0 && (t.Spawn() != nil || t.SpawnNPC() != nil) {
		gr *= 0.7
		b *= 0.7
	}
	if o&geoHighlight != 0 {
		if k := len(t.Items); k > 0 {
			f := [5]float64{0.75, 0.6, 0.48, 0.4, 0.33}[min(k, 5)-1]
			r *= f
			gr *= f
		}
	}
	hook := 0
	for _, it := range t.Items {
		if typ := g.items.Get(it.ID); typ != nil {
			if typ.Flags.Has(otitems.FlagHookSouth) {
				hook = 1
			} else if typ.Flags.Has(otitems.FlagHookEast) {
				hook = 2
			}
		}
	}
	sel := func(it otmap.Item) float64 {
		if it.Selected {
			return 0.5
		}
		return 1
	}
	elev := 0
	if t.Ground.ID != 0 {
		d := sel(t.Ground)
		elev += g.item(t.Ground, x, y, z, ax0, ay0, r*d, gr*d, b*d, 1, hook)
	}
	var tops []otmap.Item
	for _, pass := range [3]uint8{1, 2, 0} {
		for _, it := range t.Items {
			typ := g.items.Get(it.ID)
			top := uint8(0)
			if typ != nil {
				top = typ.TopOrder
			}
			if top == 3 {
				if pass == 0 {
					tops = append(tops, it)
				}
				continue
			}
			if top != pass || g.skip(typ) {
				continue
			}
			d := sel(it)
			a := 1.0
			if o&geoGhostItems != 0 && top == 0 && typ != nil && typ.Flags.Has(otitems.FlagPickupable) {
				a = 0.5
			}
			tr, tg, tb := 1.0, 1.0, 1.0
			if top == 1 || top == 2 {
				tr, tg, tb = r, gr, b
			}
			elev += g.item(it, x, y, z, ax0-elev, ay0-elev, tr*d, tg*d, tb*d, a, hook)
			elev = min(elev, 24)
		}
	}
	creatures := t.Creatures()
	if o&geoCreatures != 0 {
		for _, c := range creatures {
			g.creature(c, x, y, z, ax0-elev, ay0-elev)
		}
	}
	for _, it := range tops {
		if g.skip(g.items.Get(it.ID)) {
			continue
		}
		d := sel(it)
		g.item(it, x, y, z, ax0-elev, ay0-elev, d, d, d, 1, hook)
	}
	if n == 0 && o&(geoSpecial|geoHouses) != 0 && (t.Flags&otmap.FlagPZ != 0 || t.HouseID != 0) {
		g.rect(ax0, ay0, S, S, [4]float64{r, gr, b, 0.3})
	}
	if t.Selected() && n == 0 && len(creatures) == 0 && (t.Spawn() != nil || t.SpawnNPC() != nil) {
		g.rect(ax0, ay0, S, S, [4]float64{0.4, 0.6, 1, 0.3})
	}
}

// skip reports whether an item (not a ground) is left out of far geometry.
func (g *geoBuilder) skip(typ *otitems.Type) bool {
	return g.o.Opts&geoFar != 0 && typ != nil && typ.Flags&(otitems.FlagMoveable|otitems.FlagPickupable) != 0
}

// chunk appends the geometry of one chunk to out:
//
//	u32 key, u32 quads, u32 animated, u32 composed, u32 sprites, then
//	quads (16 bytes each, see quad), animated pieces (u32 quad, u32 client
//	item id, u32 frame 0 sprite index), composed pieces (u32 quad, u32
//	sprite, u32 template sprite, u8 head, body, legs, feet), sprite ids
//	(u32 each, every plain sprite the quads use).
func (g *geoBuilder) chunk(m *otmap.Map, k otmap.ChunkKey, out []byte) []byte {
	g.b, g.n, g.anim, g.nAnim, g.comp, g.nComp = g.b[:0], 0, g.anim[:0], 0, g.comp[:0], 0
	clear(g.sprites)
	cx0, cy0, z8 := k.Split()
	ox, oy := int(cx0)<<otmap.ChunkBits, int(cy0)<<otmap.ChunkBits
	z := int(z8)
	n := max(g.o.Block, 1)
	// Chunks row by row: sprites only reach up and left of their tile, into
	// tiles drawn before.
	for dy := 0; dy < n; dy++ {
		for dx := 0; dx < n; dx++ {
			c := m.Chunk(otmap.MakeKey(cx0+uint16(dx), cy0+uint16(dy), z8))
			if c == nil {
				continue
			}
			c.Each(func(p otmap.Position, t *otmap.Tile) {
				if g.o.Opts&geoOnlyModified != 0 && t.State&otmap.StateModified == 0 {
					return
				}
				x, y := int(p.X), int(p.Y)
				g.tile(t, x, y, z, (x-ox)*g.S, (y-oy)*g.S)
			})
		}
	}
	out = binary.LittleEndian.AppendUint32(out, uint32(k))
	out = binary.LittleEndian.AppendUint32(out, uint32(g.n))
	out = binary.LittleEndian.AppendUint32(out, uint32(g.nAnim))
	out = binary.LittleEndian.AppendUint32(out, uint32(g.nComp))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(g.sprites)))
	out = append(out, g.b...)
	out = append(out, g.anim...)
	out = append(out, g.comp...)
	for sid := range g.sprites {
		out = binary.LittleEndian.AppendUint32(out, sid)
	}
	return out
}

// buildGeo builds the geometry of chunks on all cores, in the order of
// keys, as one raw deflate stream (see buildGeoParts). It also returns the
// size before compression.
//
// On Windows the webview gets a response in one copy on the UI thread, so
// its size is what a load waits for: geometry compresses about 4 times.
func (ms *MapService) buildGeo(m *openMap, d *mapData, p *project.Project, keys []otmap.ChunkKey, o GeoOptions) ([]byte, int) {
	parts, raw := ms.buildGeoParts(m, d, p, keys, o, true)
	size := 0
	for _, b := range parts {
		size += len(b)
	}
	out := make([]byte, 0, size+2)
	for _, b := range parts {
		out = append(out, b...)
	}
	// Every part ends in a sync flush (no back references cross parts, so
	// they join into one stream); an empty final block ends the stream.
	return append(out, 0x03, 0x00), raw
}

// buildGeoParts builds the geometry of each key on all cores, each part
// compressed (raw deflate, sync flushed) when pack is set. It also returns
// the size before compression.
func (ms *MapService) buildGeoParts(m *openMap, d *mapData, p *project.Project, keys []otmap.ChunkKey, o GeoOptions, pack bool) ([][]byte, int) {
	outfit := func(name string) (uint16, uint16, uint16, uint8, uint8, uint8, uint8, uint8) {
		f, _ := d.outfit(name)
		return f.LookType, f.Mount, f.LookItem, f.Head, f.Body, f.Legs, f.Feet, f.Addons
	}
	parts := make([][]byte, len(keys))
	workers := min(runtime.NumCPU(), len(keys))
	var next int
	var raw int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g := &geoBuilder{p: p, items: m.ed.Items, ed: m.ed, outfit: outfit, o: o, S: p.SpriteSize(),
				things: map[uint32]*geoThing{}, sprites: map[uint32]struct{}{}}
			var buf []byte
			var packed bytes.Buffer
			fw, _ := flate.NewWriter(&packed, flate.BestSpeed)
			for {
				mu.Lock()
				i := next
				next++
				mu.Unlock()
				if i >= len(keys) {
					return
				}
				buf = g.chunk(m.ed.Map, keys[i], buf[:0])
				mu.Lock()
				raw += len(buf)
				mu.Unlock()
				if !pack {
					parts[i] = bytes.Clone(buf)
					continue
				}
				packed.Reset()
				fw.Reset(&packed)
				fw.Write(buf)
				fw.Flush()
				parts[i] = bytes.Clone(packed.Bytes())
			}
		}()
	}
	wg.Wait()
	return parts, raw
}

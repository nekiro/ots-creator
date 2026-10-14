package app

import (
	"encoding/binary"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/otmap"
)

// Tile flags of the chunk format.
const (
	chunkSelected = 1 << iota
	chunkModified
	chunkHouse
	chunkPZ
	chunkNoPVP
	chunkNoLogout
	chunkPVPZone
	chunkCreatures
	chunkSpawn
	chunkSpawnNPC
	chunkZones
	chunkOptional
	chunkBlocking
)

// Item flags of the chunk format.
const (
	itemSelected = 1 << iota
	itemIDs      // action or unique id
	itemText
	itemTeleport
	itemContents
	itemDoorID
)

// maxChunksPerRequest limits one chunk request.
const maxChunksPerRequest = 512

type chunkWriter struct{ b []byte }

func (w *chunkWriter) u8(v uint8)   { w.b = append(w.b, v) }
func (w *chunkWriter) u16(v uint16) { w.b = binary.LittleEndian.AppendUint16(w.b, v) }
func (w *chunkWriter) u32(v uint32) { w.b = binary.LittleEndian.AppendUint32(w.b, v) }

func (w *chunkWriter) item(it otmap.Item) {
	w.u16(it.ID)
	w.u16(it.Subtype)
	var f uint8
	if it.Selected {
		f |= itemSelected
	}
	if e := it.Ext; e != nil {
		if e.ActionID != 0 || e.UniqueID != 0 {
			f |= itemIDs
		}
		if e.Text != "" || e.Description != "" {
			f |= itemText
		}
		if e.Dest != nil {
			f |= itemTeleport
		}
		if len(e.Items) > 0 {
			f |= itemContents
		}
		if e.DoorID != 0 {
			f |= itemDoorID
		}
	}
	w.u8(f)
}

// serveMap handles /res/map/{id}/chunks?k=key,key and
// /res/map/{id}/minimap?x=&y=&z=&w=&h= and
// /res/map/{id}/geo?k=key,key&o=&h=&b= (chunk quads, see buildGeo).
//
// Requests only read the map: they run side by side (read lock).
//
// Chunk format (little endian), per requested chunk:
//
//	u32 key, u16 tile count, then per tile:
//	u16 local index (y*32+x), u16 flags, [u32 house id],
//	u8 item count (ground first when the tile has one; flag bit 0x4000 of
//	   the tile marks a ground), items: u16 id, u16 subtype, u8 flags,
//	[creatures: u8 count, per creature: u8 flags (1 npc, 2 selected,
//	   dir<<2), u16 look type, u8 head, body, legs, feet, addons,
//	   u16 mount, u16 look item, u8 name length, name],
//	[spawn: u16 radius, u8 selected], [npc spawn: same],
//	[zones: u8 count, u16 ids]
func (ms *MapService) serveMap(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "bad map id", http.StatusBadRequest)
		return
	}
	m, d, unlock, err := ms.read(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer unlock()
	w.Header().Set("Cache-Control", "no-store")
	switch parts[1] {
	case "chunks":
		keys := strings.Split(r.URL.Query().Get("k"), ",")
		if len(keys) > maxChunksPerRequest {
			http.Error(w, "too many chunks", http.StatusBadRequest)
			return
		}
		cw := &chunkWriter{b: make([]byte, 0, 64<<10)}
		for _, ks := range keys {
			k, err := strconv.ParseUint(ks, 10, 32)
			if err != nil {
				continue
			}
			ms.writeChunk(cw, m, d, otmap.ChunkKey(k))
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(cw.b)
	case "geo":
		q := r.URL.Query()
		keys := strings.Split(q.Get("k"), ",")
		if len(keys) > maxChunksPerRequest {
			http.Error(w, "too many chunks", http.StatusBadRequest)
			return
		}
		p, err := ms.s.Project()
		if err != nil {
			http.Error(w, errNoClient.Error(), http.StatusNotFound)
			return
		}
		ks := make([]otmap.ChunkKey, 0, len(keys))
		for _, s := range keys {
			if k, err := strconv.ParseUint(s, 10, 32); err == nil {
				ks = append(ks, otmap.ChunkKey(k))
			}
		}
		num := func(k string) int { n, _ := strconv.Atoi(q.Get(k)); return n }
		o := GeoOptions{Opts: uint32(num("o")), House: uint32(num("h")), Block: min(max(num("b"), 1), 8)}
		start := time.Now()
		data, raw := ms.buildGeo(m, d, p, ks, o)
		// Raw deflate: the page inflates it (DecompressionStream "deflate-raw").
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Build-Ms", strconv.FormatFloat(float64(time.Since(start).Microseconds())/1000, 'f', 2, 64))
		w.Header().Set("X-Raw-Bytes", strconv.Itoa(raw))
		w.Header().Set("X-Packed-Bytes", strconv.Itoa(len(data)))
		w.Write(data)
	case "minimap":
		q := r.URL.Query()
		num := func(k string) int { n, _ := strconv.Atoi(q.Get(k)); return n }
		x, y, z, ww, hh := num("x"), num("y"), num("z"), num("w"), num("h")
		if ww <= 0 || hh <= 0 || ww > 2048 || hh > 2048 || z < 0 || z > otmap.MaxFloor {
			http.Error(w, "bad area", http.StatusBadRequest)
			return
		}
		data, err := imaging.EncodePNG(minimapImage(m.ed, x, y, ww, hh, z))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	default:
		http.NotFound(w, r)
	}
}

func (ms *MapService) writeChunk(cw *chunkWriter, m *openMap, d *mapData, k otmap.ChunkKey) {
	cw.u32(uint32(k))
	countAt := len(cw.b)
	cw.u16(0)
	c := m.ed.Map.Chunk(k)
	if c == nil {
		return
	}
	n := 0
	c.Each(func(p otmap.Position, t *otmap.Tile) {
		n++
		cw.u16(uint16(int(p.Y&(otmap.ChunkSize-1))<<otmap.ChunkBits | int(p.X&(otmap.ChunkSize-1))))
		var f uint16
		if t.Selected() {
			f |= chunkSelected
		}
		if t.State&otmap.StateModified != 0 {
			f |= chunkModified
		}
		if t.HouseID != 0 {
			f |= chunkHouse
		}
		if t.Flags&otmap.FlagPZ != 0 {
			f |= chunkPZ
		}
		if t.Flags&otmap.FlagNoPVP != 0 {
			f |= chunkNoPVP
		}
		if t.Flags&otmap.FlagNoLogout != 0 {
			f |= chunkNoLogout
		}
		if t.Flags&otmap.FlagPVPZone != 0 {
			f |= chunkPVPZone
		}
		creatures := t.Creatures()
		if len(creatures) > 0 {
			f |= chunkCreatures
		}
		if t.Spawn() != nil {
			f |= chunkSpawn
		}
		if t.SpawnNPC() != nil {
			f |= chunkSpawnNPC
		}
		if len(t.Zones()) > 0 {
			f |= chunkZones
		}
		if t.State&otmap.StateOptionalBorder != 0 {
			f |= chunkOptional
		}
		if m.ed.IsBlocking(t) {
			f |= chunkBlocking
		}
		if t.Ground.ID != 0 {
			f |= 0x4000
		}
		cw.u16(f)
		if t.HouseID != 0 {
			cw.u32(t.HouseID)
		}
		items := t.Items[:min(len(t.Items), 254)]
		count := len(items)
		if t.Ground.ID != 0 {
			count++
		}
		cw.u8(uint8(count))
		if t.Ground.ID != 0 {
			cw.item(t.Ground)
		}
		for _, it := range items {
			cw.item(it)
		}
		if len(creatures) > 0 {
			cw.u8(uint8(min(len(creatures), 255)))
			for _, cr := range creatures[:min(len(creatures), 255)] {
				var cf uint8
				if cr.NPC {
					cf |= 1
				}
				if cr.Selected {
					cf |= 2
				}
				cf |= (cr.Direction & 3) << 2
				o, _ := d.outfit(cr.Name)
				cw.u8(cf)
				cw.u16(o.LookType)
				cw.u8(o.Head)
				cw.u8(o.Body)
				cw.u8(o.Legs)
				cw.u8(o.Feet)
				cw.u8(o.Addons)
				cw.u16(o.Mount)
				cw.u16(o.LookItem)
				name := cr.Name
				if len(name) > 255 {
					name = name[:255]
				}
				cw.u8(uint8(len(name)))
				cw.b = append(cw.b, name...)
			}
		}
		for _, s := range []*otmap.Spawn{t.Spawn(), t.SpawnNPC()} {
			if s == nil {
				continue
			}
			cw.u16(uint16(min(max(s.Radius, 0), 0xFFFF)))
			if s.Selected {
				cw.u8(1)
			} else {
				cw.u8(0)
			}
		}
		if z := t.Zones(); len(z) > 0 {
			cw.u8(uint8(min(len(z), 255)))
			for _, id := range z[:min(len(z), 255)] {
				cw.u16(id)
			}
		}
	})
	binary.LittleEndian.PutUint16(cw.b[countAt:], uint16(n))
}

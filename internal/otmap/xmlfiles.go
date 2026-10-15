package otmap

import (
	"bytes"
	"cmp"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// xmlDoc is a generic element tree, enough for the flat auxiliary files.
type xmlDoc struct {
	Name     string
	Attrs    map[string]string
	Children []*xmlDoc
}

func parseXML(data []byte) (*xmlDoc, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	var stack []*xmlDoc
	var root *xmlDoc
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &xmlDoc{Name: strings.ToLower(t.Name.Local), Attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.Attrs[strings.ToLower(a.Name.Local)] = a.Value
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.Children = append(p.Children, n)
			} else if root == nil {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("empty document")
	}
	return root, nil
}

func (n *xmlDoc) int(key string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(n.Attrs[key]))
	return v
}

func (n *xmlDoc) has(key string) bool {
	_, ok := n.Attrs[key]
	return ok
}

// xmlWriter writes simple indented XML.
type xmlWriter struct {
	b     bytes.Buffer
	depth int
}

func newXMLWriter() *xmlWriter {
	w := &xmlWriter{}
	w.b.WriteString("<?xml version=\"1.0\"?>\n")
	return w
}

type kv struct {
	k string
	v string
}

func (w *xmlWriter) elem(name string, attrs []kv, selfClose bool) {
	w.b.WriteString(strings.Repeat("\t", w.depth))
	w.b.WriteString("<" + name)
	for _, a := range attrs {
		w.b.WriteString(" " + a.k + "=\"")
		xml.EscapeText(&w.b, []byte(a.v))
		w.b.WriteString("\"")
	}
	if selfClose {
		w.b.WriteString(" />\n")
		return
	}
	w.b.WriteString(">\n")
	w.depth++
}

func (w *xmlWriter) end(name string) {
	w.depth--
	w.b.WriteString(strings.Repeat("\t", w.depth) + "</" + name + ">\n")
}

func itoa[T ~int | ~int32 | ~uint32 | ~uint16 | ~uint8](v T) string { return strconv.Itoa(int(v)) }

// ReadHouses applies a houses.xml to the map.
func ReadHouses(m *Map, data []byte) ([]string, error) {
	doc, err := parseXML(data)
	if err != nil {
		return nil, err
	}
	var warns []string
	for _, n := range doc.Children {
		if n.Name != "house" {
			continue
		}
		id := uint32(n.int("houseid"))
		if id == 0 {
			warns = append(warns, "house without an id discarded")
			continue
		}
		h := m.Houses[id]
		if h == nil {
			h = &House{ID: id}
			m.Houses[id] = h
		}
		h.Name = n.Attrs["name"]
		if h.Name == "" {
			h.Name = fmt.Sprintf("House #%d", id)
		}
		h.Exit = Position{uint16(n.int("entryx")), uint16(n.int("entryy")), uint8(n.int("entryz"))}
		h.Rent = int32(n.int("rent"))
		h.TownID = uint32(n.int("townid"))
		h.Guildhall = strings.EqualFold(n.Attrs["guildhall"], "true") || n.Attrs["guildhall"] == "1"
		h.ClientID = uint32(n.int("clientid"))
		h.Beds = int32(n.int("beds"))
	}
	return warns, nil
}

// HouseSizes counts the tiles of every house.
func HouseSizes(m *Map) map[uint32]int {
	sizes := map[uint32]int{}
	for _, c := range m.chunks {
		// House tiles are never packed.
		c.eachPtr(func(t *Tile) {
			if t.HouseID != 0 {
				sizes[t.HouseID]++
			}
		})
	}
	return sizes
}

// WriteHouses returns a houses.xml.
func WriteHouses(m *Map) []byte {
	w := newXMLWriter()
	w.elem("houses", nil, false)
	sizes := HouseSizes(m)
	for _, h := range m.SortedHouses() {
		attrs := []kv{
			{"name", h.Name}, {"houseid", itoa(h.ID)},
			{"entryx", itoa(h.Exit.X)}, {"entryy", itoa(h.Exit.Y)}, {"entryz", itoa(h.Exit.Z)},
			{"rent", itoa(h.Rent)},
		}
		if h.Guildhall {
			attrs = append(attrs, kv{"guildhall", "true"})
		}
		attrs = append(attrs, kv{"townid", itoa(h.TownID)}, kv{"size", strconv.Itoa(sizes[h.ID])})
		if h.ClientID != 0 {
			attrs = append(attrs, kv{"clientid", itoa(h.ClientID)})
		}
		if h.Beds != 0 {
			attrs = append(attrs, kv{"beds", itoa(h.Beds)})
		}
		w.elem("house", attrs, true)
	}
	w.end("houses")
	return w.b.Bytes()
}

// ReadSpawns applies a spawn file. npcFile marks a separate NPC spawn file
// (its areas become NPC spawns); a TFS file holds monsters and NPCs.
func ReadSpawns(m *Map, data []byte, npcFile bool) ([]string, error) {
	doc, err := parseXML(data)
	if err != nil {
		return nil, err
	}
	var warns []string
	for _, sn := range doc.Children {
		if !sn.has("centerx") {
			continue
		}
		cx, cy, cz := sn.int("centerx"), sn.int("centery"), sn.int("centerz")
		radius := sn.int("radius")
		if cx <= 0 || cy <= 0 || cz < 0 || cz > MaxFloor || cx > 0xFFFF || cy > 0xFFFF {
			warns = append(warns, "spawn with a bad position discarded")
			continue
		}
		if radius < 1 {
			radius = 1
		}
		center := Position{uint16(cx), uint16(cy), uint8(cz)}
		t := m.Tile(center).Clone()
		if t == nil {
			t = &Tile{}
		}
		if npcFile {
			if t.SpawnNPC() != nil {
				warns = append(warns, fmt.Sprintf("duplicate NPC spawn at %d,%d,%d discarded", cx, cy, cz))
				continue
			}
			t.EnsureExt().SpawnNPC = &Spawn{Radius: int32(radius)}
		} else {
			if t.Spawn() != nil {
				warns = append(warns, fmt.Sprintf("duplicate spawn at %d,%d,%d discarded", cx, cy, cz))
				continue
			}
			t.EnsureExt().Spawn = &Spawn{Radius: int32(radius)}
		}
		m.SetTile(center, t)
		for _, cn := range sn.Children {
			if cn.Name != "monster" && cn.Name != "npc" {
				continue
			}
			name := cn.Attrs["name"]
			if name == "" || !cn.has("x") || !cn.has("y") {
				warns = append(warns, fmt.Sprintf("creature without a name or position in spawn %d,%d,%d discarded", cx, cy, cz))
				continue
			}
			p, ok := center.Add(cn.int("x"), cn.int("y"), 0)
			if !ok {
				continue
			}
			if cn.has("z") {
				if z := cn.int("z"); z >= 0 && z <= MaxFloor {
					p.Z = uint8(z)
				}
			}
			cr := Creature{Name: name, NPC: cn.Name == "npc" || npcFile, SpawnTime: int32(cn.int("spawntime")), Weight: uint8(cn.int("weight"))}
			if cn.has("direction") {
				cr.Direction = uint8(min(max(cn.int("direction"), 0), 3))
			}
			ct := m.Tile(p).Clone()
			if ct == nil {
				ct = &Tile{}
			}
			ct.EnsureExt().Creatures = append(ct.EnsureExt().Creatures, cr)
			m.SetTile(p, ct)
		}
	}
	return warns, nil
}

type spawnArea struct {
	pos    Position
	radius int32
}

func spawnAreas(m *Map, npc bool) []spawnArea {
	var out []spawnArea
	for _, c := range m.chunks {
		c.Each(func(p Position, t *Tile) {
			s := t.Spawn()
			if npc {
				s = t.SpawnNPC()
			}
			if s != nil {
				out = append(out, spawnArea{p, max(s.Radius, 1)})
			}
		})
	}
	slices.SortFunc(out, func(a, b spawnArea) int {
		return cmp.Or(cmp.Compare(a.pos.Z, b.pos.Z), cmp.Compare(a.pos.Y, b.pos.Y), cmp.Compare(a.pos.X, b.pos.X))
	})
	return out
}

type creatureRef struct {
	pos Position
	i   int
}

// WriteSpawns returns the spawn files. Maps with single spawns get one file
// (npcs is nil); maps with separate spawns get a monster file and an NPC
// file. unsaved counts creatures outside every matching spawn area.
func WriteSpawns(m *Map) (monsters, npcs []byte, unsaved int) {
	saved := map[creatureRef]bool{}
	separate := m.SpawnFormat == SpawnsSeparate
	write := func(root, area, kind string, areas []spawnArea, want func(Creature) bool) []byte {
		w := newXMLWriter()
		w.elem(root, nil, false)
		for _, a := range areas {
			w.elem(area, []kv{{"centerx", itoa(a.pos.X)}, {"centery", itoa(a.pos.Y)}, {"centerz", itoa(a.pos.Z)}, {"radius", itoa(a.radius)}}, false)
			r := int(a.radius)
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					p, ok := a.pos.Add(dx, dy, 0)
					if !ok {
						continue
					}
					t := m.Tile(p)
					if t == nil {
						continue
					}
					for i, c := range t.Creatures() {
						ref := creatureRef{p, i}
						if saved[ref] || !want(c) {
							continue
						}
						saved[ref] = true
						tag := kind
						if tag == "" {
							tag = "monster"
							if c.NPC {
								tag = "npc"
							}
						}
						attrs := []kv{{"name", c.Name}, {"x", strconv.Itoa(dx)}, {"y", strconv.Itoa(dy)}, {"z", itoa(a.pos.Z)}, {"spawntime", itoa(c.SpawnTime)}}
						if c.Direction != 0 {
							attrs = append(attrs, kv{"direction", itoa(c.Direction)})
						}
						if separate && c.Weight != 0 && !c.NPC {
							attrs = append(attrs, kv{"weight", itoa(c.Weight)})
						}
						w.elem(tag, attrs, true)
					}
				}
			}
			w.end(area)
		}
		w.end(root)
		return w.b.Bytes()
	}
	if separate {
		monsters = write("monsters", "monster", "monster", spawnAreas(m, false), func(c Creature) bool { return !c.NPC })
		npcs = write("npcs", "npc", "npc", spawnAreas(m, true), func(c Creature) bool { return c.NPC })
	} else {
		areas := append(spawnAreas(m, false), spawnAreas(m, true)...)
		monsters = write("spawns", "spawn", "", areas, func(Creature) bool { return true })
	}
	for _, c := range m.chunks {
		c.Each(func(p Position, t *Tile) {
			for i := range t.Creatures() {
				if !saved[creatureRef{p, i}] {
					unsaved++
				}
			}
		})
	}
	return monsters, npcs, unsaved
}

// ReadZones applies a zones.xml.
func ReadZones(m *Map, data []byte) error {
	doc, err := parseXML(data)
	if err != nil {
		return err
	}
	for _, n := range doc.Children {
		if n.Name == "zone" {
			m.Zones = append(m.Zones, Zone{ID: uint16(n.int("zoneid")), Name: n.Attrs["name"]})
		}
	}
	return nil
}

// WriteZones returns a zones.xml.
func WriteZones(m *Map) []byte {
	w := newXMLWriter()
	w.elem("zones", nil, false)
	for _, z := range m.Zones {
		w.elem("zone", []kv{{"name", z.Name}, {"zoneid", itoa(z.ID)}}, true)
	}
	w.end("zones")
	return w.b.Bytes()
}

// Stage is a step of loading a map.
type Stage int

const (
	StageRead  Stage = iota // reading the file
	StageTiles              // reading tiles (done/total in bytes of the file)
	StageFiles              // reading houses, spawns and zones (done/total in files)
)

// Progress is called while a map loads.
type Progress func(stage Stage, done, total int)

// Load reads a map and its auxiliary files from disk.
func Load(path string, items Items, f Format) (*Map, []string, error) {
	return LoadProgress(path, items, f, nil)
}

// LoadProgress is Load that reports its progress (progress may be nil).
func LoadProgress(path string, items Items, f Format, progress Progress) (*Map, []string, error) {
	if progress == nil {
		progress = func(Stage, int, int) {}
	}
	progress(StageRead, 0, 1)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	m, warns, err := readOTBM(data, items, f, progress)
	if err != nil {
		return nil, nil, err
	}
	progress(StageFiles, 0, 1)
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	files := 0
	aux := func(name, fallback string, fn func([]byte) ([]string, error)) string {
		files++
		progress(StageFiles, files, 4)
		if name == "" {
			name = fallback
		}
		data, err := os.ReadFile(filepath.Join(dir, baseName(name)))
		if err != nil {
			if !os.IsNotExist(err) {
				warns = append(warns, fmt.Sprintf("%s: %v", name, err))
			}
			return name
		}
		w, err := fn(data)
		if err != nil {
			warns = append(warns, fmt.Sprintf("%s: %v", name, err))
		}
		warns = append(warns, w...)
		return name
	}
	m.HouseFile = aux(m.HouseFile, base+"-house.xml", func(d []byte) ([]string, error) { return ReadHouses(m, d) })
	if m.SpawnFormat == SpawnsSeparate {
		m.SpawnFile = aux(m.SpawnFile, base+"-monster.xml", func(d []byte) ([]string, error) { return ReadSpawns(m, d, false) })
		m.NpcFile = aux(m.NpcFile, base+"-npc.xml", func(d []byte) ([]string, error) { return ReadSpawns(m, d, true) })
	} else {
		m.SpawnFile = aux(m.SpawnFile, base+"-spawn.xml", func(d []byte) ([]string, error) { return ReadSpawns(m, d, false) })
	}
	if m.ZoneFile != "" {
		m.ZoneFile = aux(m.ZoneFile, base+"-zones.xml", func(d []byte) ([]string, error) { return nil, ReadZones(m, d) })
	}
	// Loading spawns sets tiles; a freshly loaded map has no changes.
	m.ClearModified()
	return m, warns, nil
}

// ClearModified clears the modified state of every tile.
func (m *Map) ClearModified() {
	for _, c := range m.chunks {
		// Packed tiles have no state.
		c.eachPtr(func(t *Tile) { t.State &^= StateModified })
	}
}

// Save writes the map and its auxiliary files next to it. File names come
// from the map (defaults derive from the map name). It returns warnings.
func Save(path string, m *Map, items Items, f Format) ([]string, error) {
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	def := func(name *string, suffix string) {
		if *name == "" {
			*name = base + suffix
		}
		*name = baseName(*name)
	}
	def(&m.HouseFile, "-house.xml")
	if m.SpawnFormat == SpawnsSeparate {
		def(&m.SpawnFile, "-monster.xml")
		def(&m.NpcFile, "-npc.xml")
	} else {
		def(&m.SpawnFile, "-spawn.xml")
	}
	if len(m.Zones) > 0 {
		def(&m.ZoneFile, "-zones.xml")
	}
	data, err := EncodeOTBM(m, items, f)
	if err != nil {
		return nil, err
	}
	var warns []string
	if err := writeAtomic(path, data); err != nil {
		return nil, err
	}
	if err := writeAtomic(filepath.Join(dir, m.HouseFile), WriteHouses(m)); err != nil {
		return nil, err
	}
	monsters, npcs, unsaved := WriteSpawns(m)
	if err := writeAtomic(filepath.Join(dir, m.SpawnFile), monsters); err != nil {
		return nil, err
	}
	if npcs != nil {
		if err := writeAtomic(filepath.Join(dir, m.NpcFile), npcs); err != nil {
			return nil, err
		}
	}
	if m.ZoneFile != "" {
		if err := writeAtomic(filepath.Join(dir, m.ZoneFile), WriteZones(m)); err != nil {
			return nil, err
		}
	}
	if unsaved > 0 {
		warns = append(warns, fmt.Sprintf("%d creatures are outside every spawn and were not saved", unsaved))
	}
	m.ClearModified()
	return warns, nil
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

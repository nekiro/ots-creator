package otmap

import "testing"

func TestChunkPacking(t *testing.T) {
	m := New(100, 100)
	plain := Position{1, 1, 7}
	rich := Position{2, 1, 7}
	m.SetTile(plain, &Tile{Ground: Item{ID: 100}, Items: []Item{{ID: 200, Subtype: 3}}})
	m.SetTile(rich, &Tile{Ground: Item{ID: 100}, Flags: FlagPZ})
	c := m.Chunk(KeyOf(plain))
	if len(c.packed) != 1 || len(c.ptrs) != 1 || m.TileCount() != 2 {
		t.Fatalf("packed %d ptrs %d tiles %d", len(c.packed), len(c.ptrs), m.TileCount())
	}
	// A packed tile reads back as a copy; changing it leaves the map alone.
	got := m.Tile(plain)
	if got.Ground.ID != 100 || len(got.Items) != 1 || got.Items[0].ID != 200 || got.Items[0].Subtype != 3 {
		t.Fatalf("unpacked %+v", got)
	}
	got.Items[0].ID = 999
	if m.Tile(plain).Items[0].ID != 200 {
		t.Fatal("a copy changed the map")
	}
	// Put keeps the tile itself: the editor's working copy.
	w := m.Tile(plain).Clone()
	m.Put(plain, w)
	w.Items[0].ID = 201
	if m.Tile(plain) != w || m.Tile(plain).Items[0].ID != 201 || c.dead != 1 {
		t.Fatalf("working copy not kept (dead %d)", c.dead)
	}
	// Removing and counting.
	m.SetTile(rich, nil)
	if m.Has(rich) || m.TileCount() != 1 || len(c.freePtrs) != 0 && c.ptrs[c.freePtrs[0]] != nil {
		t.Fatal("removal")
	}
	n := 0
	m.Each(func(p Position, tl *Tile) {
		n++
		if p != plain {
			t.Fatalf("each at %v", p)
		}
	})
	if n != 1 {
		t.Fatalf("each saw %d tiles", n)
	}
}

func TestChunkRepack(t *testing.T) {
	m := New(100, 100)
	for i := range 64 {
		m.SetTile(Position{uint16(i % 32), uint16(i / 32), 7}, &Tile{Ground: Item{ID: uint16(100 + i)}})
	}
	c := m.Chunk(MakeKey(0, 0, 7))
	// Replace most packed tiles with working copies: the dead ones are dropped.
	for i := range 48 {
		p := Position{uint16(i % 32), uint16(i / 32), 7}
		m.Put(p, m.Tile(p).Clone())
	}
	if live := len(c.packed) - c.dead; live != 16 {
		t.Fatalf("live packed %d (packed %d, dead %d)", live, len(c.packed), c.dead)
	}
	if len(c.packed) >= 64 {
		t.Fatalf("not repacked: %d packed", len(c.packed))
	}
	for i := range 64 {
		if g := m.Tile(Position{uint16(i % 32), uint16(i / 32), 7}).Ground.ID; g != uint16(100+i) {
			t.Fatalf("tile %d ground %d", i, g)
		}
	}
}

func TestForgetAuxFileNames(t *testing.T) {
	m := New(32, 32)
	m.HouseFile, m.SpawnFile, m.NpcFile, m.ZoneFile = "h.xml", "s.xml", "n.xml", "z.xml"
	m.ForgetAuxFileNames()
	if m.HouseFile != "" || m.SpawnFile != "" || m.NpcFile != "" || m.ZoneFile != "" {
		t.Fatalf("names kept: %+v", m)
	}
}

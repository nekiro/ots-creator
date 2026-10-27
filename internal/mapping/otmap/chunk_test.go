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

func TestPeekAndEachStored(t *testing.T) {
	gameMap := New(100, 100)
	plain := Position{1, 1, 7}
	house := Position{40, 1, 7}
	gameMap.SetTile(plain, &Tile{Ground: Item{ID: 100}, Items: []Item{{ID: 200}}})
	gameMap.SetTile(house, &Tile{Ground: Item{ID: 101}, HouseID: 5})
	var scratch Tile
	var buf [4]Item
	if tile := gameMap.Peek(plain, &scratch, buf[:]); tile != &scratch || tile.Ground.ID != 100 || tile.Items[0].ID != 200 {
		t.Fatalf("peek packed %+v", tile)
	}
	if tile := gameMap.PeekAt(40, 1, 7, &scratch, buf[:]); tile == nil || tile.HouseID != 5 || tile == &scratch {
		t.Fatalf("peek stored %+v", tile)
	}
	if gameMap.PeekAt(-1, 1, 7, &scratch, buf[:]) != nil || gameMap.Peek(Position{2, 2, 7}, &scratch, buf[:]) != nil {
		t.Fatal("peek of no tile")
	}
	var visited []Position
	gameMap.EachStored(func(pos Position, tile *Tile) {
		visited = append(visited, pos)
		if tile.HouseID != 5 {
			t.Fatalf("stored tile %+v", tile)
		}
	})
	if len(visited) != 1 || visited[0] != house {
		t.Fatalf("each stored visited %v", visited)
	}
	if gameMap.stored(plain) != nil || gameMap.stored(house) == nil {
		t.Fatal("stored lookup")
	}
}

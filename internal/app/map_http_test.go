package app

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

func TestGeoChunk(t *testing.T) {
	ms, s := mapTestService(t)
	p, _ := s.Project()
	px := make([]byte, 32*32*4)
	for i := 0; i < len(px); i += 4 {
		px[i], px[i+3] = 200, 255
	}
	ids, err := p.AddSprites([][]byte{px})
	if err != nil {
		t.Fatal(err)
	}
	g, _ := p.Thing(thing.CategoryItem, 100)
	g.FrameGroups[0].Sprites[0] = ids[0]
	if err := p.UpdateThing(g); err != nil {
		t.Fatal(err)
	}
	info, err := ms.NewMap(NewMapOptions{Width: 256, Height: 256})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 100, Points: []otmap.Position{{X: 33, Y: 34, Z: 7}}, Size: 1}); err != nil {
		t.Fatal(err)
	}
	k := otmap.MakeKey(1, 1, 7)
	rec := httptest.NewRecorder()
	ms.serveMap(rec, httptest.NewRequest("GET", fmt.Sprintf("/res/map/1/geo?k=%d,%d&o=0", k, otmap.MakeKey(5, 5, 7)), nil), []string{fmt.Sprint(info.ID), "geo"})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	b := inflate(t, rec.Body.Bytes())
	u32 := func(o int) uint32 { return binary.LittleEndian.Uint32(b[o:]) }
	if u32(0) != uint32(k) {
		t.Fatalf("key %d", u32(0))
	}
	n, sprites := int(u32(4)), int(u32(16))
	// A 3x3 square of ground around (33, 34).
	if n != 9 || sprites != 1 {
		t.Fatalf("%d quads, %d sprites", n, sprites)
	}
	q := b[24:]
	x, y := int16(binary.LittleEndian.Uint16(q)), int16(binary.LittleEndian.Uint16(q[2:]))
	if x != 0 || y != 32 || binary.LittleEndian.Uint32(q[4:]) != ids[0] || q[8] != 255 || q[11] != 255 {
		t.Fatalf("quad %v", q[:16])
	}
	// Diagonal by diagonal, each from its bottom-left end, as the client draws.
	order := [9][2]int16{{0, 1}, {0, 2}, {1, 1}, {0, 3}, {1, 2}, {2, 1}, {1, 3}, {2, 2}, {2, 3}}
	for i, want := range order {
		at := q[i*16:]
		if x, y := int16(binary.LittleEndian.Uint16(at)), int16(binary.LittleEndian.Uint16(at[2:])); x != want[0]*32 || y != want[1]*32 {
			t.Fatalf("quad %d at %d,%d, want tile %v", i, x, y, want)
		}
		if tx, ty := int16(binary.LittleEndian.Uint16(at[12:])), int16(binary.LittleEndian.Uint16(at[14:])); tx != want[0] || ty != want[1] {
			t.Fatalf("quad %d of tile %d,%d, want %v", i, tx, ty, want)
		}
	}
	if u32(24+n*16) != ids[0] {
		t.Fatalf("sprite list %d", u32(24+n*16))
	}
	// The grounds hide the light under them (128 bytes of shade bits); the empty chunk follows.
	shadeBytes := int(u32(24 + n*16 + 4))
	rest := b[24+n*16+4+4+shadeBytes:]
	if shadeBytes != 128 || len(rest) != 28 || binary.LittleEndian.Uint32(rest[4:]) != 0 {
		t.Fatalf("empty chunk %v", rest)
	}
	// A block of 4x4 chunks from (0, 0) holds the square, placed from the block's corner.
	rec = httptest.NewRecorder()
	ms.serveMap(rec, httptest.NewRequest("GET", fmt.Sprintf("/res/map/1/geo?k=%d&o=0&b=4", otmap.MakeKey(0, 0, 7)), nil), []string{fmt.Sprint(info.ID), "geo"})
	b = inflate(t, rec.Body.Bytes())
	if n := int(u32(4)); n != 9 {
		t.Fatalf("block: %d quads", n)
	}
	if x, y := int16(binary.LittleEndian.Uint16(b[24:])), int16(binary.LittleEndian.Uint16(b[26:])); x != 32*32 || y != 33*32 {
		t.Fatalf("block quad at %d,%d", x, y)
	}
}

// Geometry requests share the read lock with each other while edits wait
// for the write lock: none may block forever or fail.
func TestGeoConcurrent(t *testing.T) {
	ms, _ := mapTestService(t)
	info, err := ms.NewMap(NewMapOptions{Width: 256, Height: 256})
	if err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("/res/map/1/geo?k=%d,%d&o=0&b=1", otmap.MakeKey(0, 0, 7), otmap.MakeKey(1, 1, 7))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				rec := httptest.NewRecorder()
				ms.serveMap(rec, httptest.NewRequest("GET", url, nil), []string{fmt.Sprint(info.ID), "geo"})
				if rec.Code != 200 {
					t.Errorf("%d %s", rec.Code, rec.Body)
					return
				}
			}
		}()
	}
	for j := 0; j < 20; j++ {
		if _, err := ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 100, Points: []otmap.Position{{X: uint16(10 + j), Y: 10, Z: 7}}, Size: 1}); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}

// inflate reads a geometry response (raw deflate).
func inflate(t *testing.T, b []byte) []byte {
	t.Helper()
	out, err := io.ReadAll(flate.NewReader(bytes.NewReader(b)))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

package app

import (
	"encoding/binary"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/mapedit"
	"github.com/nekiro/ots-creator/internal/materials"
	"github.com/nekiro/ots-creator/internal/otmap"
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

func mapTestService(t testing.TB) (*MapService, *Session) {
	t.Helper()
	s := NewSession(nil)
	v := client.FindByValue(1098)[0]
	p := project.New(v, client.Features{})
	// Item 100 is a ground, 101 a common item, 102 a border.
	for i := 0; i < 2; i++ {
		if _, err := p.AddThing(thing.CategoryItem); err != nil {
			t.Fatal(err)
		}
	}
	g, _ := p.Thing(thing.CategoryItem, 100)
	g.Props.Ground = true
	g.Props.MiniMap = true
	g.Props.MiniMapColor = 30
	if err := p.UpdateThing(g); err != nil {
		t.Fatal(err)
	}
	s.Set(p)
	store := NewMapStore(filepath.Join(t.TempDir(), "mapeditor.json"))
	return NewMapService(s, store), s
}

func TestMapServiceRoundTrip(t *testing.T) {
	ms, _ := mapTestService(t)
	st := ms.Status()
	if !st.Data.Ready || st.Data.Mode != "client" || st.Data.Items != 3 {
		t.Fatalf("status = %+v", st.Data)
	}
	info, err := ms.NewMap(NewMapOptions{Width: 256, Height: 256})
	if err != nil {
		t.Fatal(err)
	}
	if !info.Changed {
		t.Fatal("new map not marked changed")
	}
	ch, err := ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 100, Points: []otmap.Position{{X: 10, Y: 10, Z: 7}}, Size: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Chunks) != 1 || ch.Info.Tiles != 9 || !ch.Info.CanUndo {
		t.Fatalf("change = %+v", ch)
	}
	if _, err := ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 101, Points: []otmap.Position{{X: 10, Y: 10, Z: 7}}}); err != nil {
		t.Fatal(err)
	}
	// Chunk endpoint.
	rec := httptest.NewRecorder()
	ms.serveMap(rec, httptest.NewRequest("GET", fmt.Sprintf("/res/map/%d/chunks?k=%d", info.ID, ch.Chunks[0]), nil), []string{fmt.Sprint(info.ID), "chunks"})
	body := rec.Body.Bytes()
	if rec.Code != 200 || len(body) < 6 {
		t.Fatalf("chunks: %d %q", rec.Code, body)
	}
	if k := binary.LittleEndian.Uint32(body); k != ch.Chunks[0] {
		t.Fatalf("key %d", k)
	}
	if n := binary.LittleEndian.Uint16(body[4:]); n != 9 {
		t.Fatalf("%d tiles in chunk", n)
	}
	// Minimap endpoint.
	rec = httptest.NewRecorder()
	ms.serveMap(rec, httptest.NewRequest("GET", "/res/map/1/minimap?x=0&y=0&z=7&w=32&h=32", nil), []string{fmt.Sprint(info.ID), "minimap"})
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("minimap %d", rec.Code)
	}
	// Inspector.
	tile, err := ms.Tile(info.ID, otmap.Position{X: 10, Y: 10, Z: 7})
	if err != nil || tile.Ground == nil || len(tile.Items) != 1 || tile.Items[0].ID != 101 {
		t.Fatalf("tile = %+v %v", tile, err)
	}
	aid := uint16(77)
	if _, err := ms.UpdateItem(info.ID, mapedit.ItemRef{Pos: otmap.Position{X: 10, Y: 10, Z: 7}, Index: 0}, mapedit.ItemPatch{ActionID: &aid}); err != nil {
		t.Fatal(err)
	}
	// Save and reopen.
	path := filepath.Join(t.TempDir(), "test.otbm")
	if _, err := ms.SaveMap(info.ID, path); err != nil {
		t.Fatal(err)
	}
	if ms.AnyChanged() {
		t.Fatal("changed after save")
	}
	ms.CloseMap(info.ID)
	res, err := ms.OpenMap(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 0 || res.Info.Tiles != 9 {
		t.Fatalf("open = %+v", res)
	}
	tile, _ = ms.Tile(res.Info.ID, otmap.Position{X: 10, Y: 10, Z: 7})
	if tile.Items[0].Ext == nil || tile.Items[0].Ext.ActionID != 77 {
		t.Fatalf("action id lost: %+v", tile.Items[0])
	}
	if recent := ms.Status().Recent; len(recent) != 1 || recent[0].Path != path {
		t.Fatalf("recent = %+v", recent)
	}
	// Undo of a draw after reopening.
	ms.Draw(res.Info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 101, Points: []otmap.Position{{X: 50, Y: 50, Z: 7}}})
	if u, _ := ms.Undo(res.Info.ID); u.Info.Tiles != 9 || u.Info.Changed {
		t.Fatalf("undo = %+v", u.Info)
	}
	// Table, appearances and palette.
	tab, err := ms.Items()
	if err != nil || len(tab.IDs) != 3 || tab.Flags[0]&tab.Flagbits["ground"] == 0 {
		t.Fatalf("items = %+v", tab)
	}
	app, err := ms.Appearances(thing.CategoryItem, []uint32{100, 999})
	if err != nil || len(app) != 1 || app[0].Flags&512 == 0 {
		t.Fatalf("appearances = %+v", app)
	}
	pal, err := ms.Palette()
	if err != nil || len(pal.Tilesets) == 0 {
		t.Fatalf("palette = %+v %v", pal, err)
	}
	// A brush saved through the service is drawable right away.
	if err := ms.SaveBrush("", materials.Brush{Name: "dirt", Type: materials.TypeGround, Items: []materials.Chance{{ID: 100, Chance: 1}}}, []string{"Mine"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.Draw(res.Info.ID, mapedit.DrawRequest{Kind: mapedit.KindGround, Name: "dirt", Points: []otmap.Position{{X: 70, Y: 70, Z: 7}}}); err != nil {
		t.Fatal(err)
	}
	if pal, _ := ms.Palette(); pal.Tilesets[0].Name != "Mine" || len(pal.Tilesets[0].Terrain) != 1 {
		t.Fatalf("tileset = %+v", pal.Tilesets[0])
	}
	// Collections given on save are where the brush is afterwards.
	dirt := materials.Brush{Name: "dirt", Type: materials.TypeGround, Items: []materials.Chance{{ID: 100, Chance: 1}}}
	if err := ms.SaveBrush("dirt", dirt, []string{"Other", "Third"}); err != nil {
		t.Fatal(err)
	}
	pal, _ = ms.Palette()
	in := map[string]int{}
	for _, ts := range pal.Tilesets {
		in[ts.Name] = len(ts.Terrain)
	}
	if in["Mine"] != 0 || in["Other"] != 1 || in["Third"] != 1 {
		t.Fatalf("collections = %v", in)
	}
}

func TestMapServiceNeedsClient(t *testing.T) {
	ms := NewMapService(NewSession(nil), NewMapStore(""))
	if _, err := ms.NewMap(NewMapOptions{}); err == nil {
		t.Fatal("map without client")
	}
	if st := ms.Status(); st.Data.Ready {
		t.Fatal("ready without client")
	}
}

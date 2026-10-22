package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/formats/itemsotb"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func workingConversionService(t *testing.T) (*MapService, int, int) {
	t.Helper()
	ms, _ := mapTestService(t)
	file := itemsotb.New(1, 3, 17)
	ground := itemsotb.NewItem(1000, 100)
	ground.Group = itemsotb.GroupGround
	container := itemsotb.NewItem(1001, 101)
	container.Group = itemsotb.GroupContainer
	stack := itemsotb.NewItem(1002, 102)
	stack.Flags |= itemsotb.FlagStackable
	file.Items = []*itemsotb.Item{ground, container, stack}
	data, err := file.Encode()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "items.otb")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.SetData(MapDataConfig{ItemsMode: itemsOTB, OTBPath: path}); err != nil {
		t.Fatal(err)
	}
	one, err := ms.NewMap(NewMapOptions{Width: 512, Height: 512})
	if err != nil {
		t.Fatal(err)
	}
	two, err := ms.NewMap(NewMapOptions{Width: 512, Height: 512})
	if err != nil {
		t.Fatal(err)
	}
	pos := otmap.Position{X: 10, Y: 10, Z: 7}
	for _, id := range []int{one.ID, two.ID} {
		ms.maps[id].editor.Map.SetTile(pos, &otmap.Tile{Ground: otmap.Item{ID: 1000}})
	}
	return ms, one.ID, two.ID
}

func TestConvertLoadedMapIncludesUnsavedEdits(t *testing.T) {
	ms, id, other := workingConversionService(t)
	source := filepath.Join(t.TempDir(), "original.otbm")
	if _, err := ms.SaveMap(id, source); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(source)
	pos := otmap.Position{X: 10, Y: 10, Z: 7}
	tx := ms.maps[id].editor.Begin("Add container", 0)
	tx.Edit(pos).Items = []otmap.Item{{ID: 1001, Ext: &otmap.ItemExt{ActionID: 1234, Items: []otmap.Item{{ID: 1002, Subtype: 5}}}}}
	tx.Commit()
	change, err := ms.ConvertMapToClientIDs(id)
	if err != nil || !change.All || change.Count != 3 || change.Info.ItemMode != otitems.ModeClient || !change.Info.Changed {
		t.Fatalf("conversion = %+v %v", change, err)
	}
	tile, err := ms.Tile(id, pos)
	if err != nil || tile.Ground.ID != 100 || tile.Ground.ClientID != 100 || tile.Items[0].ID != 101 || tile.Items[0].Ext.Items[0].ID != 102 || tile.Items[0].Ext.ActionID != 1234 {
		t.Fatalf("converted tile = %+v %v", tile, err)
	}
	unchanged, _ := os.ReadFile(source)
	if !bytes.Equal(original, unchanged) {
		t.Fatal("conversion wrote to disk")
	}
	otherTile, _ := ms.Tile(other, pos)
	if otherTile.Ground.ID != 1000 || otherTile.Ground.ClientID != 100 {
		t.Fatal("other map's IDs or rendering changed")
	}
	for _, tab := range []struct {
		id   int
		mode otitems.Mode
	}{{id, otitems.ModeClient}, {other, otitems.ModeOTB}, {id, otitems.ModeClient}} {
		view, err := ms.ViewData(tab.id, true)
		if err != nil || view.Data.Mode != tab.mode || view.Items.Mode != string(tab.mode) {
			t.Fatalf("view data = %+v %v", view.Data, err)
		}
		items, _ := ms.Items()
		if items.Mode != string(tab.mode) {
			t.Fatal("palette tools use another map's ID mode")
		}
	}
	// Reloading client objects must keep converted and unconverted tabs separate.
	if _, err := ms.Reload(); err != nil {
		t.Fatal(err)
	}
	if info, _ := ms.Info(id); info.ItemMode != otitems.ModeClient {
		t.Fatal("reload reverted the converted map's mode")
	}
	if info, _ := ms.Info(other); info.ItemMode != otitems.ModeOTB {
		t.Fatal("reload changed the other tab's mode")
	}
	if _, err := ms.ConvertMapToClientIDs(id); err == nil {
		t.Fatal("converted the same map twice")
	}
	undo, err := ms.Undo(id)
	if err != nil || !undo.All || undo.Info.ItemMode != otitems.ModeOTB || !undo.Info.Changed {
		t.Fatalf("undo = %+v %v", undo, err)
	}
	tile, _ = ms.Tile(id, pos)
	if tile.Items[0].ID != 1001 || tile.Items[0].Ext.Items[0].ID != 1002 {
		t.Fatal("undo lost the unsaved container")
	}
	redo, err := ms.Redo(id)
	if err != nil || !redo.All || redo.Info.ItemMode != otitems.ModeClient {
		t.Fatalf("redo = %+v %v", redo, err)
	}
	// A normal Save writes client IDs, including unsaved container contents.
	if _, err := ms.SaveMap(id, source); err != nil {
		t.Fatal(err)
	}
	saved, _, err := otmap.Load(source, ms.dataFor(ms.maps[id]).items, otmap.Format{})
	if err != nil || saved.Tile(pos).Ground.ID != 100 || saved.Tile(pos).Items[0].Ext.Items[0].ID != 102 {
		t.Fatalf("saved client map = %v", err)
	}
}

func TestLoadedMapConversionFailureIsAtomic(t *testing.T) {
	ms, id, _ := workingConversionService(t)
	pos := otmap.Position{X: 11, Y: 11, Z: 7}
	ms.maps[id].editor.Map.SetTile(pos, &otmap.Tile{Items: []otmap.Item{{ID: 9999}}})
	ms.maps[id].editor.MarkSaved()
	if _, err := ms.ConvertMapToClientIDs(id); err == nil {
		t.Fatal("missing mapping accepted")
	}
	info, _ := ms.Info(id)
	tile, _ := ms.Tile(id, otmap.Position{X: 10, Y: 10, Z: 7})
	if info.Changed || info.CanUndo || info.ItemMode != otitems.ModeOTB || tile.Ground.ID != 1000 {
		t.Fatal("failed conversion partially changed the map or its mode")
	}
	if _, err := ms.ConvertMapToClientIDs(999); err != ErrNoMap {
		t.Fatalf("invalid map ID = %v", err)
	}
}

func TestClipboardDoesNotMixServerAndClientIDs(t *testing.T) {
	ms, id, other := workingConversionService(t)
	pos := otmap.Position{X: 10, Y: 10, Z: 7}
	if _, err := ms.SelectTiles(other, []otmap.Position{pos}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.Copy(other); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.ConvertMapToClientIDs(id); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.Paste(id, otmap.Position{X: 20, Y: 20, Z: 7}); err == nil {
		t.Fatal("pasted server IDs into a client ID map")
	}
	if _, err := ms.Paste(other, otmap.Position{X: 20, Y: 20, Z: 7}); err != nil {
		t.Fatal(err)
	}
}

package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// newTestMap opens a new map with ground (100) and an item (101) at
// (10..12, 10, 7).
func newTestMap(t *testing.T) (*MapService, int) {
	t.Helper()
	ms, _ := mapTestService(t)
	info, err := ms.NewMap(NewMapOptions{Width: 64, Height: 64})
	if err != nil {
		t.Fatal(err)
	}
	for x := uint16(10); x <= 12; x++ {
		at := []otmap.Position{{X: x, Y: 10, Z: 7}}
		mustChange(t)(ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 100, Points: at}))
		mustChange(t)(ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 101, Points: at}))
	}
	return ms, info.ID
}

func mustChange(t *testing.T) func(MapChange, error) MapChange {
	return func(change MapChange, err error) MapChange {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return change
	}
}

func TestMapServiceUnknownMap(t *testing.T) {
	ms, _ := mapTestService(t)
	if _, err := ms.Info(42); !errors.Is(err, ErrNoMap) {
		t.Fatalf("Info err = %v", err)
	}
	if _, err := ms.Stats(42); !errors.Is(err, ErrNoMap) {
		t.Fatalf("Stats err = %v", err)
	}
	if _, err := ms.RemoveZone(42, 1); !errors.Is(err, ErrNoMap) {
		t.Fatalf("edit err = %v", err)
	}
}

func TestMapServiceItemEdits(t *testing.T) {
	ms, id := newTestMap(t)
	at := otmap.Position{X: 10, Y: 10, Z: 7}
	ref := mapedit.ItemRef{Pos: at, Index: 0}
	mustChange(t)(ms.AddItem(id, at, 101))
	if tile, _ := ms.Tile(id, at); len(tile.Items) != 2 || tile.Items[0].Kind != "" {
		t.Fatalf("tile after AddItem = %+v", tile)
	}
	mustChange(t)(ms.MoveItem(id, at, 0, 1))
	mustChange(t)(ms.RemoveItem(id, ref))
	mustChange(t)(ms.UpdateTile(id, at, mapedit.TilePatch{}))
	if tile, _ := ms.Tile(id, at); len(tile.Items) != 1 {
		t.Fatalf("tile after RemoveItem = %+v", tile)
	}
	if _, err := ms.RotateItem(id, ref); err == nil {
		t.Fatal("rotated an item without rotation")
	}
	if _, err := ms.SwitchDoor(id, at, 0); err == nil {
		t.Fatal("switched an item that is no door")
	}
	steps, err := ms.History(id)
	if err != nil || len(steps["undo"]) == 0 {
		t.Fatalf("history = %v, %v", steps, err)
	}
	if redo := mustChange(t)(ms.Undo(id)); !redo.Info.CanRedo {
		t.Fatal("nothing to redo after undo")
	}
	mustChange(t)(ms.Redo(id))
	if empty, _ := ms.Tile(id, otmap.Position{X: 1, Y: 1, Z: 7}); empty.Exists {
		t.Fatal("empty tile exists")
	}
}

func TestMapServiceMeta(t *testing.T) {
	ms, id := newTestMap(t)
	exit := otmap.Position{X: 10, Y: 10, Z: 7}
	town := mustChange(t)(ms.SaveTown(id, otmap.Town{Name: "Thais", Temple: exit}))
	if town.Count == 0 || !town.Meta {
		t.Fatalf("town change = %+v", town)
	}
	house := mustChange(t)(ms.SaveHouse(id, otmap.House{Name: "Villa", TownID: uint32(town.Count), Exit: exit}))
	mustChange(t)(ms.UpdateTile(id, exit, mapedit.TilePatch{HouseID: ptr(uint32(house.Count))}))
	mustChange(t)(ms.SaveWaypoint(id, "", otmap.Waypoint{Name: "depot", Pos: exit}))
	zone := mustChange(t)(ms.SaveZone(id, otmap.Zone{Name: "arena"}))

	tile, err := ms.Tile(id, exit)
	if err != nil || tile.HouseExit != "Villa" || tile.HouseName != "Villa" || tile.Temple != "Thais" || len(tile.Waypoints) != 1 {
		t.Fatalf("tile = %+v, %v", tile, err)
	}
	if tiles, _ := ms.HouseTiles(id, uint32(house.Count)); len(tiles) != 1 {
		t.Fatalf("house tiles = %v", tiles)
	}
	info, _ := ms.Info(id)
	if len(info.Towns) != 1 || len(info.Houses) != 1 || len(info.Waypoints) != 1 || len(info.Zones) != 1 {
		t.Fatalf("info = %+v", info)
	}
	mustChange(t)(ms.RemoveZone(id, uint16(zone.Count)))
	mustChange(t)(ms.RemoveWaypoint(id, "depot"))
	mustChange(t)(ms.RemoveHouse(id, uint32(house.Count)))
	mustChange(t)(ms.RemoveTown(id, uint32(town.Count)))
	mustChange(t)(ms.SetProperties(id, mapedit.Properties{Description: "test", Width: 64, Height: 64}))
	info, _ = ms.Info(id)
	if len(info.Towns)+len(info.Houses)+len(info.Waypoints)+len(info.Zones) != 0 || info.Props.Description != "test" {
		t.Fatalf("info after removes = %+v", info)
	}
}

func ptr[T any](value T) *T { return &value }

func TestMapServiceSelectionAndClipboard(t *testing.T) {
	ms, id := newTestMap(t)
	if _, err := ms.Copy(id); err == nil {
		t.Fatal("copied an empty selection")
	}
	if all := mustChange(t)(ms.SelectAll(id, 7)); all.Count != 3 {
		t.Fatalf("select all = %d", all.Count)
	}
	mustChange(t)(ms.ClearSelection(id))
	rect := mapedit.Rect{From: otmap.Position{X: 10, Y: 10, Z: 7}, To: otmap.Position{X: 11, Y: 10, Z: 7}}
	if selected := mustChange(t)(ms.SelectRect(id, rect, mapedit.FloorsCurrent, false, false, false)); selected.Count != 2 {
		t.Fatalf("select rect = %d", selected.Count)
	}
	if copied, err := ms.Copy(id); err != nil || copied != 2 || ms.Clipboard() == nil {
		t.Fatalf("copy = %d, %v", copied, err)
	}
	mustChange(t)(ms.Paste(id, otmap.Position{X: 20, Y: 20, Z: 7}))
	mustChange(t)(ms.MoveSelection(id, 0, 5, 0))
	if deleted := mustChange(t)(ms.DeleteSelection(id)); deleted.Count == 0 {
		t.Fatal("nothing deleted")
	}
	mustChange(t)(ms.SelectTiles(id, []otmap.Position{{X: 12, Y: 10, Z: 7}}, false))
	mustChange(t)(ms.SelectAt(id, otmap.Position{X: 12, Y: 10, Z: 7}, false, false, false))
	mustChange(t)(ms.SelectItem(id, otmap.Position{X: 12, Y: 10, Z: 7}, 0, true))
	if cut := mustChange(t)(ms.Cut(id)); cut.Count != 1 {
		t.Fatalf("cut = %d tiles", cut.Count)
	}
	if _, err := ms.Cut(id); err == nil {
		t.Fatal("cut an empty selection")
	}
}

func TestMapServiceTools(t *testing.T) {
	ms, id := newTestMap(t)
	for name := range mapTools {
		change, err := ms.Tool(id, name, false)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if change.All != (name == "clearModified") {
			t.Fatalf("%s: All = %v", name, change.All)
		}
	}
	if _, err := ms.Tool(id, "fly", false); err == nil {
		t.Fatal("unknown tool ran")
	}
	found, err := ms.Find(id, mapedit.FindQuery{IDs: []uint16{101}})
	if err != nil || len(found.Results) != 3 {
		t.Fatalf("find = %+v, %v", found, err)
	}
	if _, err := ms.Replace(id, []uint16{101}, nil, false); err == nil {
		t.Fatal("replace without pairs ran")
	}
	if replaced := mustChange(t)(ms.Replace(id, []uint16{101}, []uint16{0}, false)); replaced.Count != 3 {
		t.Fatalf("replaced %d", replaced.Count)
	}
	if stats, err := ms.Stats(id); err != nil || stats.Tiles != 3 {
		t.Fatalf("stats = %+v, %v", stats, err)
	}
}

func TestMapServiceMaterials(t *testing.T) {
	ms, id := newTestMap(t)
	borderID, err := ms.SaveBorder(materials.Border{})
	if err != nil || borderID == 0 {
		t.Fatalf("border = %d, %v", borderID, err)
	}
	if borders, _ := ms.Borders(); len(borders) != 1 {
		t.Fatalf("borders = %v", borders)
	}
	if err := ms.SaveTileset("", materials.Tileset{Name: "Rocks"}); err != nil {
		t.Fatal(err)
	}
	if err := ms.AddToTileset("Rocks", []uint16{101}); err != nil {
		t.Fatal(err)
	}
	if err := ms.SaveCreature(materials.CreatureType{Name: "Rat"}); err != nil {
		t.Fatal(err)
	}
	mustChange(t)(ms.SelectAll(id, 7))
	if err := ms.DoodadFromSelection(id, "stones", []string{"Rocks"}); err != nil {
		t.Fatal(err)
	}
	brush, err := ms.Brush("stones")
	if err != nil || brush.Type != materials.TypeDoodad || brush.Look != 101 {
		t.Fatalf("brush = %+v, %v", brush, err)
	}
	palette, _ := ms.Palette()
	if len(palette.Creatures) != 1 || palette.Brushes[0].Preview == nil {
		t.Fatalf("palette = %+v", palette)
	}
	exported := filepath.Join(t.TempDir(), "materials.json")
	if err := ms.ExportMaterials(exported); err != nil {
		t.Fatal(err)
	}
	if err := ms.RemoveBrush("stones"); err != nil {
		t.Fatal(err)
	}
	if err := ms.RemoveTileset("Rocks"); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.Brush("stones"); err == nil {
		t.Fatal("removed brush found")
	}
	result, err := ms.ImportMaterials(exported)
	if err != nil || result.Data.Brushes != 1 || result.Data.Tilesets != 1 {
		t.Fatalf("import = %+v, %v", result, err)
	}
	if _, err := ms.ImportRME(t.TempDir()); err == nil {
		t.Fatal("imported RME data from an empty folder")
	}
	if err := ms.SaveBrush("", materials.Brush{}, nil); err == nil {
		t.Fatal("saved a brush without a name")
	}
}

func TestMapServiceFiles(t *testing.T) {
	ms, id := newTestMap(t)
	dir := t.TempDir()
	written, err := ms.ExportMinimap(id, dir, []int{-1, 7, 8})
	if err != nil || len(written) != 2 {
		t.Fatalf("minimap = %v, %v", written, err)
	}
	if err := ms.SaveImage(filepath.Join(dir, "shot"), []byte("png")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "shot.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.SaveMap(id, ""); err == nil {
		t.Fatal("saved an untitled map without a path")
	}
	first := filepath.Join(dir, "first")
	if _, err := ms.SaveMap(id, first); err != nil {
		t.Fatal(err)
	}
	// Save as names the auxiliary files after the new map.
	if _, err := ms.SaveMap(id, filepath.Join(dir, "second.otbm")); err != nil {
		t.Fatal(err)
	}
	info, _ := ms.Info(id)
	if info.Props.HouseFile != "second-house.xml" {
		t.Fatalf("house file = %q", info.Props.HouseFile)
	}
	imported := mustChange(t)(ms.ImportMap(id, first+".otbm", mapedit.ImportOptions{DY: 20}))
	if !imported.All || imported.Info.Tiles != 6 {
		t.Fatalf("import = %+v", imported.Info)
	}
	if opened, err := ms.OpenMap(filepath.Join(dir, "second.otbm")); err != nil || opened.Info.ID != id {
		t.Fatalf("reopening an open map = %+v, %v", opened, err)
	}
	if err := ms.RemoveRecent(""); err != nil || len(ms.Status().Recent) != 0 {
		t.Fatal("recent maps not cleared")
	}
	if empty, _ := ms.NewMap(NewMapOptions{}); empty.Props.Width != minMapSide {
		t.Fatalf("new map width = %d", empty.Props.Width)
	}
	if _, err := ms.ExportMinimap(id+1, dir, []int{7}); err == nil {
		t.Fatal("exported the minimap of an empty map")
	}
}

func TestMapServiceDataConfig(t *testing.T) {
	ms, _ := mapTestService(t)
	info, err := ms.SetData(MapDataConfig{ItemsMode: itemsClient, MaterialsPath: filepath.Join(t.TempDir(), "m.json")})
	if err != nil || info.Config.ItemsMode != itemsClient {
		t.Fatalf("set data = %+v, %v", info, err)
	}
	if reloaded, err := ms.Reload(); err != nil || reloaded.Materials != info.Materials {
		t.Fatalf("reload = %+v, %v", reloaded, err)
	}
	if _, err := ms.SetData(MapDataConfig{ItemsMode: itemsOTB, OTBPath: filepath.Join(t.TempDir(), "missing.otb")}); err == nil {
		t.Fatal("loaded a missing items.otb")
	}
	if err := ms.SetEditorSettings(mapedit.Settings{}); err != nil {
		t.Fatal(err)
	}
	if err := ms.SetKeys(map[string][]string{"undo": {"ctrl+u"}}); err != nil || ms.Status().Keys["undo"][0] != "ctrl+u" {
		t.Fatal("keys not stored")
	}
	if detected := ms.DetectData(t.TempDir()); detected.OTBPath != "" {
		t.Fatalf("detected = %+v", detected)
	}
}

package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/itemsotb"
	"github.com/nekiro/ots-creator/internal/thing"
)

// otbProject compiles a client with items 100..105 (sprites 1..6) and
// attaches an items.otb mapping server ids 1100.. to them, plus a
// deprecated item (server id 1200).
func otbProject(t *testing.T) (*Project, string) {
	t.Helper()
	project := reorderProject(t)
	dir := t.TempDir()
	if err := project.Compile(CompileOptions{DatPath: filepath.Join(dir, "Tibia.dat"), SprPath: filepath.Join(dir, "Tibia.spr"), Version: v1098(), Features: project.Info().Features}); err != nil {
		t.Fatal(err)
	}
	otbFile := itemsotb.New(3, 57, 1)
	for id := uint16(100); id <= 105; id++ {
		item := itemsotb.NewItem(id+1000, id)
		th, _ := project.Thing(thing.CategoryItem, uint32(id))
		item.Reload(th, nil)
		otbFile.Items = append(otbFile.Items, item)
	}
	otbFile.Items = append(otbFile.Items, &itemsotb.Item{Group: itemsotb.GroupDeprecated, Attrs: []itemsotb.Attr{{Type: itemsotb.AttrServerID, Data: []byte{0xB0, 0x04}}}})
	data, err := otbFile.Encode()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "items.otb")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.AttachOTB(path); err != nil {
		t.Fatal(err)
	}
	return project, path
}

func readOTB(t *testing.T, path string) map[uint16]*itemsotb.Item {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	otbFile, err := itemsotb.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	out := map[uint16]*itemsotb.Item{}
	for _, item := range otbFile.Items {
		out[item.ServerID()] = item
	}
	return out
}

func (p *Project) compileHere(t *testing.T) *OTBReport {
	t.Helper()
	in := p.Info()
	var st CompileStats
	if err := p.Compile(CompileOptions{DatPath: in.DatPath, SprPath: in.SprPath, Version: in.Version, Features: in.Features, Stats: &st}); err != nil {
		t.Fatal(err)
	}
	if st.OTB == nil {
		t.Fatal("no items.otb report")
	}
	if st.OTB.Error != "" {
		t.Fatal(st.OTB.Error)
	}
	return st.OTB
}

func TestOTBSync(t *testing.T) {
	project, path := otbProject(t)
	before, _ := os.ReadFile(path)

	// Nothing changed: the file stays as it is.
	if rep := project.compileHere(t); rep.Written || rep.Changed() {
		t.Fatalf("unchanged client rewrote items.otb: %+v", rep)
	}

	// 101 <-> 104 swap places, 102 becomes pickupable, 100 is duplicated.
	if _, err := project.MoveThings(thing.CategoryItem, []uint32{101}, 104, MoveOptions{}); err != nil {
		t.Fatal(err)
	}
	setItem(t, project, 102, true, 3)
	if _, err := project.DuplicateThings(thing.CategoryItem, []uint32{100}); err != nil {
		t.Fatal(err)
	}
	prev, err := project.OTBPreview()
	if err != nil {
		t.Fatal(err)
	}
	rep := project.compileHere(t)
	if !rep.Written || rep.Remapped != 2 || rep.Updated != 1 || rep.Added != 1 || rep.Missing != 0 {
		t.Fatalf("report %+v", rep)
	}
	if prev.Remapped != rep.Remapped || prev.Updated != rep.Updated || prev.Added != rep.Added {
		t.Fatalf("preview %+v differs from %+v", prev, rep)
	}
	items := readOTB(t, path)
	if items[1101].ClientID() != 104 || items[1104].ClientID() != 101 {
		t.Fatalf("moved items not followed: 1101->%d 1104->%d", items[1101].ClientID(), items[1104].ClientID())
	}
	if items[1102].Flags&itemsotb.FlagPickupable == 0 {
		t.Fatal("edited item not reloaded")
	}
	if items[1201] == nil || items[1201].ClientID() != 106 {
		t.Fatal("no server item for the new client item")
	}
	if items[1104].Attr(itemsotb.AttrSpriteHash) == nil {
		t.Fatal("no sprite hash")
	}
	if !items[1100].Equal(mustParse(t, before)[0]) {
		t.Fatal("untouched item changed")
	}
	if project.OTB().Items != 8 || project.OTB().MaxServerID != 1201 {
		t.Fatalf("info %+v", project.OTB())
	}

	// After saving, the new state is the base: nothing more to do.
	if rep := project.compileHere(t); rep.Written {
		t.Fatalf("second compile rewrote items.otb: %+v", rep)
	}
}

func TestOTBUndoneMove(t *testing.T) {
	project, path := otbProject(t)
	before, _ := os.ReadFile(path)
	if _, err := project.MoveThings(thing.CategoryItem, []uint32{101, 102}, 104, MoveOptions{Insert: true}); err != nil {
		t.Fatal(err)
	}
	project.Undo()
	project.Redo()
	project.Undo()
	if rep := project.compileHere(t); rep.Written {
		t.Fatalf("undone move rewrote items.otb: %+v", rep)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("file changed")
	}
}

func TestOTBRemovedItem(t *testing.T) {
	project, path := otbProject(t)
	if err := project.RemoveThings(thing.CategoryItem, []uint32{105}); err != nil {
		t.Fatal(err)
	}
	if rep := project.compileHere(t); rep.Missing != 1 || rep.Written {
		t.Fatalf("report %+v", rep)
	}
	if readOTB(t, path)[1105].ClientID() != 105 {
		t.Fatal("server item of a removed client item changed")
	}
}

func mustParse(t *testing.T, data []byte) []*itemsotb.Item {
	t.Helper()
	otbFile, err := itemsotb.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return otbFile.Items
}

func TestDetachOTB(t *testing.T) {
	project, _ := otbProject(t)
	if project.OTB() == nil {
		t.Fatal("items.otb should be attached")
	}
	project.DetachOTB()
	if project.OTB() != nil {
		t.Fatal("items.otb should be detached")
	}
	if _, err := project.OTBPreview(); err == nil {
		t.Fatal("preview without items.otb must fail")
	}
}

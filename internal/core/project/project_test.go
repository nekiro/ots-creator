package project

import (
	"bytes"
	"errors"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/obd"
)

func v1098() client.Version {
	version, _ := client.FindBySignatures(0x42A3, 0x57BBD603)
	return version
}

func solid(r, g, b byte) []byte {
	return bytes.Repeat([]byte{r, g, b, 255}, 32*32)
}

func TestNewProject(t *testing.T) {
	project := New(v1098(), client.Features{})
	info := project.Info()
	if info.Counts.Items != 100 || info.Counts.Outfits != 1 || info.Counts.Effects != 1 || info.Counts.Missiles != 1 || info.Counts.Sprites != 0 {
		t.Fatalf("counts %+v", info.Counts)
	}
	if !info.Features.Extended || !info.Features.FrameGroups || !info.Changed {
		t.Fatalf("info %+v", info)
	}
}

func TestCompileAndReopen(t *testing.T) {
	dir := t.TempDir()
	project := New(v1098(), client.Features{})
	ids, err := project.AddSprites([][]byte{solid(255, 0, 0), solid(0, 255, 0)})
	if err != nil || len(ids) != 2 || ids[0] != 1 {
		t.Fatalf("AddSprites %v %v", ids, err)
	}
	it, _ := project.Thing(thing.CategoryItem, 100)
	it.Props.Pickupable = true
	it.FrameGroups[0].Sprites[0] = 2
	if err := project.UpdateThing(it); err != nil {
		t.Fatal(err)
	}
	datPath, sprPath := filepath.Join(dir, "Tibia.dat"), filepath.Join(dir, "Tibia.spr")
	if err := project.Compile(CompileOptions{DatPath: datPath, SprPath: sprPath, Version: v1098(), Features: project.Info().Features, WriteOTFI: true}); err != nil {
		t.Fatal(err)
	}
	if project.Info().Changed || project.Info().CanUndo {
		t.Fatal("compile must reset changed and history")
	}
	if _, err := os.Stat(filepath.Join(dir, "Tibia.otfi")); err != nil {
		t.Fatal("otfi not written")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Fatalf("leftover temp files: %v", entries)
	}

	q, err := Open(OpenOptions{DatPath: datPath, SprPath: sprPath})
	if err != nil {
		t.Fatal(err)
	}
	if q.Info().Version.Value != 1098 {
		t.Fatalf("detected %v", q.Info().Version)
	}
	got, _ := q.Thing(thing.CategoryItem, 100)
	if !got.Props.Pickupable || got.FrameGroups[0].Sprites[0] != 2 {
		t.Fatalf("thing %+v", got)
	}
	pixels, err := q.SpritePixels(2)
	if err != nil || pixels[1] != 255 || pixels[0] != 0 {
		t.Fatalf("sprite 2 pixels %v %v", pixels[:4], err)
	}
	img, err := q.Render(thing.CategoryItem, 100, TexturePos{})
	if err != nil || img.Pix[1] != 255 {
		t.Fatalf("render %v", err)
	}
}

func TestOpenUnknownVersion(t *testing.T) {
	dir := t.TempDir()
	project := New(client.Version{Value: 1098, DatSignature: 1, SprSignature: 2}, client.Features{})
	datPath, sprPath := filepath.Join(dir, "a.dat"), filepath.Join(dir, "a.spr")
	if err := project.Compile(CompileOptions{DatPath: datPath, SprPath: sprPath, Version: client.Version{Value: 1098, DatSignature: 1, SprSignature: 2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(OpenOptions{DatPath: datPath, SprPath: sprPath}); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("err = %v", err)
	}
	version := v1098()
	if _, err := Open(OpenOptions{DatPath: datPath, SprPath: sprPath, Version: &version}); err != nil {
		t.Fatal(err)
	}
}

func TestCompileTransparencyChange(t *testing.T) {
	dir := t.TempDir()
	project := New(v1098(), client.Features{})
	half := bytes.Repeat([]byte{10, 20, 30, 128}, 32*32)
	project.AddSprites([][]byte{half})
	features := project.Info().Features
	features.Transparency = true
	d, value := filepath.Join(dir, "t.dat"), filepath.Join(dir, "t.spr")
	if err := project.Compile(CompileOptions{DatPath: d, SprPath: value, Version: v1098(), Features: features}); err != nil {
		t.Fatal(err)
	}
	q, err := Open(OpenOptions{DatPath: d, SprPath: value, Features: features})
	if err != nil {
		t.Fatal(err)
	}
	pixels, _ := q.SpritePixels(1)
	// Source project was opaque, so alpha became 255 when first stored.
	if pixels[3] != 255 || pixels[0] != 10 {
		t.Fatalf("pixel %v", pixels[:4])
	}
}

func TestUndoRedoThings(t *testing.T) {
	project := New(v1098(), client.Features{})
	id, _ := project.AddThing(thing.CategoryItem)
	if id != 101 {
		t.Fatalf("id %d", id)
	}
	it, _ := project.Thing(thing.CategoryItem, 101)
	it.Props.Stackable = true
	project.UpdateThing(it)
	dups, _ := project.DuplicateThings(thing.CategoryItem, []uint32{101})
	if dups[0] != 102 {
		t.Fatalf("dup %v", dups)
	}
	if project.Info().Counts.Items != 102 {
		t.Fatal("count")
	}
	project.Undo() // duplicate
	if project.Info().Counts.Items != 101 {
		t.Fatalf("after undo count %d", project.Info().Counts.Items)
	}
	project.Undo() // update
	got, _ := project.Thing(thing.CategoryItem, 101)
	if got.Props.Stackable {
		t.Fatal("update not undone")
	}
	project.Redo()
	got, _ = project.Thing(thing.CategoryItem, 101)
	if !got.Props.Stackable {
		t.Fatal("update not redone")
	}
	project.Redo()
	if project.Info().Counts.Items != 102 || project.Info().CanRedo {
		t.Fatal("duplicate not redone")
	}
	// New edit clears redo.
	project.Undo()
	project.AddThing(thing.CategoryEffect)
	if project.Info().CanRedo {
		t.Fatal("redo must be cleared")
	}
}

func TestRemoveThings(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddThing(thing.CategoryItem)
	project.AddThing(thing.CategoryItem) // 100,101,102
	it, _ := project.Thing(thing.CategoryItem, 101)
	it.Props.Pickupable = true
	project.UpdateThing(it)
	if err := project.RemoveThings(thing.CategoryItem, []uint32{101, 102}); err != nil {
		t.Fatal(err)
	}
	// 102 is removed first, then 101 becomes the last and is removed too.
	if project.Info().Counts.Items != 100 {
		t.Fatalf("count %d", project.Info().Counts.Items)
	}
	if _, err := project.Thing(thing.CategoryItem, 101); err == nil {
		t.Fatal("101 must be gone")
	}
	project.Undo()
	if project.Info().Counts.Items != 102 {
		t.Fatalf("undo count %d", project.Info().Counts.Items)
	}
	got, _ := project.Thing(thing.CategoryItem, 101)
	if !got.Props.Pickupable {
		t.Fatal("undo did not restore 101")
	}
	// First item cannot disappear.
	project.RemoveThings(thing.CategoryOutfit, []uint32{1})
	if project.Info().Counts.Outfits != 1 {
		t.Fatal("first outfit removed")
	}
	if err := project.RemoveThings(thing.CategoryItem, []uint32{999}); err == nil {
		t.Fatal("missing id must fail")
	}
}

func TestRemoveMiddleKeepsIDs(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddThing(thing.CategoryItem)
	project.AddThing(thing.CategoryItem)
	it, _ := project.Thing(thing.CategoryItem, 101)
	it.Props.Pickupable = true
	project.UpdateThing(it)
	project.RemoveThings(thing.CategoryItem, []uint32{101})
	if project.Info().Counts.Items != 102 {
		t.Fatal("middle removal must keep count")
	}
	got, _ := project.Thing(thing.CategoryItem, 101)
	if got.Props.Pickupable {
		t.Fatal("middle thing not cleared")
	}
}

func TestSpritesUndo(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddSprites([][]byte{solid(1, 1, 1), solid(2, 2, 2), solid(3, 3, 3)})
	if err := project.ReplaceSprite(2, solid(9, 9, 9)); err != nil {
		t.Fatal(err)
	}
	if err := project.RemoveSprites([]uint32{3, 1}); err != nil {
		t.Fatal(err)
	}
	if project.Info().Counts.Sprites != 2 {
		t.Fatalf("count %d", project.Info().Counts.Sprites)
	}
	if empty, _ := project.IsSpriteEmpty(1); !empty {
		t.Fatal("sprite 1 must be empty")
	}
	project.Undo()
	if project.Info().Counts.Sprites != 3 {
		t.Fatal("undo remove count")
	}
	pixels, _ := project.SpritePixels(3)
	if pixels[0] != 3 {
		t.Fatal("sprite 3 not restored")
	}
	project.Undo()
	pixels, _ = project.SpritePixels(2)
	if pixels[0] != 2 {
		t.Fatal("replace not undone")
	}
	project.Undo()
	if project.Info().Counts.Sprites != 0 {
		t.Fatal("add not undone")
	}
	project.Redo()
	pixels, _ = project.SpritePixels(3)
	if pixels[0] != 3 {
		t.Fatal("add not redone")
	}
	if err := project.ReplaceSprite(1, []byte{1}); err == nil {
		t.Fatal("bad pixel size must fail")
	}
}

func TestUpdateThingValidation(t *testing.T) {
	project := New(v1098(), client.Features{})
	it, _ := project.Thing(thing.CategoryItem, 100)
	it.FrameGroups[0].Sprites[0] = 5
	if err := project.UpdateThing(it); err == nil {
		t.Fatal("reference to missing sprite must fail")
	}
	it.ID = 500
	it.FrameGroups[0].Sprites[0] = 0
	if err := project.UpdateThing(it); err == nil {
		t.Fatal("missing thing must fail")
	}
}

func TestOBDImportExport(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddSprites([][]byte{solid(5, 5, 5)})
	o, _ := project.Thing(thing.CategoryOutfit, 1)
	o.FrameGroups[0].Sprites = []uint32{1, 0, 1, 0}
	o.Props.AnimateAlways = true
	if err := project.UpdateThing(o); err != nil {
		t.Fatal(err)
	}
	obdData, err := project.ExportOBD(thing.CategoryOutfit, 1, obd.Version3)
	if err != nil {
		t.Fatal(err)
	}
	file, err := obd.Encode(obdData)
	if err != nil {
		t.Fatal(err)
	}
	back, err := obd.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	id, err := project.ImportOBD(back, 0)
	if err != nil || id != 2 {
		t.Fatalf("import %d %v", id, err)
	}
	got, _ := project.Thing(thing.CategoryOutfit, 2)
	// Identical sprites are deduplicated, empty ones map to 0.
	s := got.FrameGroups[0].Sprites
	if s[0] != 2 || s[1] != 0 || s[2] != 2 || s[3] != 0 || project.Info().Counts.Sprites != 2 {
		t.Fatalf("sprites %v count %d", s, project.Info().Counts.Sprites)
	}
	if !got.Props.AnimateAlways {
		t.Fatal("props lost")
	}
	project.Undo()
	if project.Info().Counts.Outfits != 1 || project.Info().Counts.Sprites != 1 {
		t.Fatal("import undo")
	}
	if _, err := project.ImportOBD(back, 1); err != nil {
		t.Fatal(err)
	}
	if project.Info().Counts.Outfits != 1 {
		t.Fatal("replace import must not add")
	}
}

func TestWarnings(t *testing.T) {
	project := New(v1098(), client.Features{})
	it, _ := project.Thing(thing.CategoryItem, 100)
	it.Props.IsMarket = true
	project.UpdateThing(it)
	version, _ := client.FindBySignatures(0x4C28B721, 0x4C220594) // 8.60
	if w := project.Warnings(FormatDat, version); len(w) != 0 {
		t.Fatalf("8.60 supports market: %v", w)
	}
	version = client.FindByValue(854)[0]
	if w := project.Warnings(FormatDat, version); len(w) != 1 {
		t.Fatalf("8.54 warnings %v", w)
	}
}

func TestThumbnailPos(t *testing.T) {
	o := thing.New(1, thing.CategoryOutfit)
	if ThumbnailPos(o).PatternX != 2 {
		t.Fatal("outfit thumbnail must face south")
	}
	if ThumbnailPos(thing.New(100, thing.CategoryItem)).PatternX != 0 {
		t.Fatal("item thumbnail")
	}
}

func TestSetGroupPixels(t *testing.T) {
	project := New(v1098(), client.Features{})
	px := [][]byte{solid(1, 2, 3), make([]byte, 32*32*4), solid(1, 2, 3), solid(4, 4, 4)}
	if err := project.SetGroupPixels(thing.CategoryOutfit, 1, 0, px); err != nil {
		t.Fatal(err)
	}
	o, _ := project.Thing(thing.CategoryOutfit, 1)
	s := o.FrameGroups[0].Sprites
	if s[0] != 1 || s[1] != 0 || s[2] != 1 || s[3] != 2 {
		t.Fatalf("sprites %v", s)
	}
	if err := project.SetGroupPixels(thing.CategoryOutfit, 1, 0, px[:2]); err == nil {
		t.Fatal("slot count mismatch must fail")
	}
	if err := project.SetGroupPixels(thing.CategoryOutfit, 1, 3, px); err == nil {
		t.Fatal("bad group must fail")
	}
	project.Undo()
	if project.Info().Counts.Sprites != 0 {
		t.Fatal("undo")
	}
}

func TestUndoBackToSavedState(t *testing.T) {
	dir := t.TempDir()
	project := New(v1098(), client.Features{})
	options := CompileOptions{DatPath: filepath.Join(dir, "Tibia.dat"), SprPath: filepath.Join(dir, "Tibia.spr"), Version: v1098(), Features: project.Info().Features}
	if err := project.Compile(options); err != nil {
		t.Fatal(err)
	}
	project.AddThing(thing.CategoryItem)
	if !project.Info().Changed {
		t.Fatal("edit must mark the client changed")
	}
	project.Undo()
	if project.Info().Changed {
		t.Fatal("undo back to the compiled state must clear changed")
	}
	project.Redo()
	if !project.Info().Changed {
		t.Fatal("redo must mark the client changed again")
	}
}

func TestNewClientStaysChangedAfterUndo(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddThing(thing.CategoryItem)
	project.Undo()
	if !project.Info().Changed {
		t.Fatal("a client never written to disk is always changed")
	}
}

func TestImportSheetInfersOutfit(t *testing.T) {
	// 4 directions x 9 frames of 64 px: idle + 8 walking frames.
	img := image.NewNRGBA(image.Rect(0, 0, 256, 576))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i-3], img.Pix[i] = byte(i/1024), 255
	}
	project := New(v1098(), client.Features{})
	if err := project.ImportSheet(thing.CategoryOutfit, 1, img); err != nil {
		t.Fatal(err)
	}
	o, _ := project.Thing(thing.CategoryOutfit, 1)
	if len(o.FrameGroups) != 2 || o.FrameGroups[0].Frames != 1 || o.FrameGroups[1].Frames != 8 {
		t.Fatalf("groups %d", len(o.FrameGroups))
	}
	for _, group := range o.FrameGroups {
		if group.Width != 2 || group.Height != 2 || group.PatternX != 4 || group.Validate() != nil {
			t.Fatalf("group %+v", group)
		}
	}
	project.Undo()
	if o, _ := project.Thing(thing.CategoryOutfit, 1); o.FrameGroups[0].Width != 1 {
		t.Fatal("one undo step restores the outfit")
	}

	// Without frame groups every frame stays in one group.
	old := New(client.FindByValue(860)[0], client.Features{})
	if err := old.ImportSheet(thing.CategoryOutfit, 1, img); err != nil {
		t.Fatal(err)
	}
	if o, _ := old.Thing(thing.CategoryOutfit, 1); len(o.FrameGroups) != 1 || o.FrameGroups[0].Frames != 9 {
		t.Fatal("8.60 outfit")
	}
}

func TestUndoMarksSpritesUnedited(t *testing.T) {
	dir := t.TempDir()
	project := New(v1098(), client.Features{})
	project.AddSprites([][]byte{solid(255, 0, 0), solid(0, 255, 0)})
	options := CompileOptions{DatPath: filepath.Join(dir, "Tibia.dat"), SprPath: filepath.Join(dir, "Tibia.spr"), Version: v1098(), Features: project.Info().Features}
	if err := project.Compile(options); err != nil {
		t.Fatal(err)
	}
	project.ReplaceSprite(1, solid(1, 2, 3))
	if !project.sprites.edited(1) {
		t.Fatal("replaced sprite must be edited")
	}
	project.Undo()
	if project.sprites.edited(1) {
		t.Fatal("undo must restore the file sprite")
	}
	if pixels, _ := project.SpritePixels(1); pixels[0] != 255 {
		t.Fatal("pixels not restored")
	}
	project.Redo()
	if pixels, _ := project.SpritePixels(1); pixels[0] != 1 {
		t.Fatal("redo lost")
	}
}

func TestThingRange(t *testing.T) {
	project := New(v1098(), client.Features{})
	for range 2 {
		if _, err := project.AddThing(thing.CategoryItem); err != nil {
			t.Fatal(err)
		}
	}
	things := project.ThingRange(thing.CategoryItem, 0, 200)
	if len(things) != 3 || things[0].ID != 100 || things[2].ID != 102 {
		t.Fatalf("ThingRange = %d things", len(things))
	}
	things[0].Name = "changed"
	if original, _ := project.Thing(thing.CategoryItem, 100); original.Name != "" {
		t.Fatal("ThingRange must return copies")
	}
	if got := project.ThingRange(thing.CategoryItem, 101, 101); len(got) != 1 || got[0].ID != 101 {
		t.Fatal("single id range")
	}
}

func TestImportOBDsTo(t *testing.T) {
	source := New(v1098(), client.Features{})
	source.AddSprites([][]byte{solid(5, 5, 5)})
	item, _ := source.Thing(thing.CategoryItem, 100)
	item.FrameGroups[0].Sprites = []uint32{1}
	item.Name = "imported"
	if err := source.UpdateThing(item); err != nil {
		t.Fatal(err)
	}
	obdData, err := source.ExportOBD(thing.CategoryItem, 100, obd.Version3)
	if err != nil {
		t.Fatal(err)
	}

	project := New(v1098(), client.Features{})
	if _, err := project.ImportOBDsTo(thing.CategoryItem, []*obd.Data{obdData}, nil); err == nil {
		t.Fatal("target count mismatch must fail")
	}
	if _, err := project.ImportOBDsTo(thing.CategoryItem, []*obd.Data{obdData}, []uint32{50}); err == nil {
		t.Fatal("target below the first id must fail")
	}
	if _, err := project.ImportOBDsTo(thing.CategoryItem, []*obd.Data{obdData, obdData}, []uint32{101, 101}); err == nil {
		t.Fatal("duplicate target must fail")
	}
	if _, err := project.ImportOBDsTo(thing.CategoryOutfit, []*obd.Data{obdData}, []uint32{1}); err == nil {
		t.Fatal("wrong category must fail")
	}
	ids, err := project.ImportOBDsTo(thing.CategoryItem, []*obd.Data{obdData, obdData}, []uint32{100, 103})
	if err != nil || len(ids) != 2 || ids[1] != 103 {
		t.Fatalf("ImportOBDsTo = %v, %v", ids, err)
	}
	// Sprites are deduplicated per frame group only.
	if project.Info().Counts.Items != 103 || project.Info().Counts.Sprites != 2 {
		t.Fatalf("counts = %+v", project.Info().Counts)
	}
	for i, id := range []uint32{100, 103} {
		got, _ := project.Thing(thing.CategoryItem, id)
		if got.Name != "imported" || got.FrameGroups[0].Sprites[0] != uint32(i+1) {
			t.Fatalf("item %d = %+v", id, got)
		}
	}
	if gap, _ := project.Thing(thing.CategoryItem, 101); gap.Name != "" {
		t.Fatal("gap should hold an empty item")
	}
	if project.Undo() == "" || project.Info().Counts.Items != 100 {
		t.Fatal("import should be one undoable edit")
	}
}

func TestPasteThings(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddThing(thing.CategoryItem) // 101
	item := thing.New(5, thing.CategoryItem)
	item.Props.Stackable = true
	ids, err := project.PasteThings(thing.CategoryItem, 100, []*thing.Thing{item, item, item})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != 100 || ids[2] != 102 {
		t.Fatalf("ids %v", ids)
	}
	for _, id := range ids {
		got, _ := project.Thing(thing.CategoryItem, id)
		if !got.Props.Stackable || got.ID != id {
			t.Fatalf("pasted %+v", got)
		}
	}
	if project.Info().Counts.Items != 102 {
		t.Fatalf("count %d", project.Info().Counts.Items)
	}
	project.Undo()
	if got, _ := project.Thing(thing.CategoryItem, 100); got.Props.Stackable || project.Info().Counts.Items != 101 {
		t.Fatal("paste not undone")
	}
	if _, err := project.PasteThings(thing.CategoryItem, 103, []*thing.Thing{item}); err == nil {
		t.Fatal("paste past the end")
	}
	if _, err := project.PasteThings(thing.CategoryOutfit, 1, []*thing.Thing{item}); err == nil {
		t.Fatal("paste into another category")
	}
}

package project

import (
	"bytes"
	"errors"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/thing"
)

func v1098() client.Version {
	v, _ := client.FindBySignatures(0x42A3, 0x57BBD603)
	return v
}

func solid(r, g, b byte) []byte {
	return bytes.Repeat([]byte{r, g, b, 255}, 32*32)
}

func TestNewProject(t *testing.T) {
	p := New(v1098(), client.Features{})
	info := p.Info()
	if info.Counts.Items != 100 || info.Counts.Outfits != 1 || info.Counts.Effects != 1 || info.Counts.Missiles != 1 || info.Counts.Sprites != 0 {
		t.Fatalf("counts %+v", info.Counts)
	}
	if !info.Features.Extended || !info.Features.FrameGroups || !info.Changed {
		t.Fatalf("info %+v", info)
	}
}

func TestCompileAndReopen(t *testing.T) {
	dir := t.TempDir()
	p := New(v1098(), client.Features{})
	ids, err := p.AddSprites([][]byte{solid(255, 0, 0), solid(0, 255, 0)})
	if err != nil || len(ids) != 2 || ids[0] != 1 {
		t.Fatalf("AddSprites %v %v", ids, err)
	}
	it, _ := p.Thing(thing.CategoryItem, 100)
	it.Props.Pickupable = true
	it.FrameGroups[0].Sprites[0] = 2
	if err := p.UpdateThing(it); err != nil {
		t.Fatal(err)
	}
	datPath, sprPath := filepath.Join(dir, "Tibia.dat"), filepath.Join(dir, "Tibia.spr")
	if err := p.Compile(CompileOptions{DatPath: datPath, SprPath: sprPath, Version: v1098(), Features: p.Info().Features, WriteOTFI: true}); err != nil {
		t.Fatal(err)
	}
	if p.Info().Changed || p.Info().CanUndo {
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
	px, err := q.SpritePixels(2)
	if err != nil || px[1] != 255 || px[0] != 0 {
		t.Fatalf("sprite 2 pixels %v %v", px[:4], err)
	}
	img, err := q.Render(thing.CategoryItem, 100, TexturePos{})
	if err != nil || img.Pix[1] != 255 {
		t.Fatalf("render %v", err)
	}
}

func TestOpenUnknownVersion(t *testing.T) {
	dir := t.TempDir()
	p := New(client.Version{Value: 1098, DatSignature: 1, SprSignature: 2}, client.Features{})
	datPath, sprPath := filepath.Join(dir, "a.dat"), filepath.Join(dir, "a.spr")
	if err := p.Compile(CompileOptions{DatPath: datPath, SprPath: sprPath, Version: client.Version{Value: 1098, DatSignature: 1, SprSignature: 2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(OpenOptions{DatPath: datPath, SprPath: sprPath}); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("err = %v", err)
	}
	v := v1098()
	if _, err := Open(OpenOptions{DatPath: datPath, SprPath: sprPath, Version: &v}); err != nil {
		t.Fatal(err)
	}
}

func TestCompileTransparencyChange(t *testing.T) {
	dir := t.TempDir()
	p := New(v1098(), client.Features{})
	half := bytes.Repeat([]byte{10, 20, 30, 128}, 32*32)
	p.AddSprites([][]byte{half})
	f := p.Info().Features
	f.Transparency = true
	d, s := filepath.Join(dir, "t.dat"), filepath.Join(dir, "t.spr")
	if err := p.Compile(CompileOptions{DatPath: d, SprPath: s, Version: v1098(), Features: f}); err != nil {
		t.Fatal(err)
	}
	q, err := Open(OpenOptions{DatPath: d, SprPath: s, Features: f})
	if err != nil {
		t.Fatal(err)
	}
	px, _ := q.SpritePixels(1)
	// Source project was opaque, so alpha became 255 when first stored.
	if px[3] != 255 || px[0] != 10 {
		t.Fatalf("pixel %v", px[:4])
	}
}

func TestUndoRedoThings(t *testing.T) {
	p := New(v1098(), client.Features{})
	id, _ := p.AddThing(thing.CategoryItem)
	if id != 101 {
		t.Fatalf("id %d", id)
	}
	it, _ := p.Thing(thing.CategoryItem, 101)
	it.Props.Stackable = true
	p.UpdateThing(it)
	dups, _ := p.DuplicateThings(thing.CategoryItem, []uint32{101})
	if dups[0] != 102 {
		t.Fatalf("dup %v", dups)
	}
	if p.Info().Counts.Items != 102 {
		t.Fatal("count")
	}
	p.Undo() // duplicate
	if p.Info().Counts.Items != 101 {
		t.Fatalf("after undo count %d", p.Info().Counts.Items)
	}
	p.Undo() // update
	got, _ := p.Thing(thing.CategoryItem, 101)
	if got.Props.Stackable {
		t.Fatal("update not undone")
	}
	p.Redo()
	got, _ = p.Thing(thing.CategoryItem, 101)
	if !got.Props.Stackable {
		t.Fatal("update not redone")
	}
	p.Redo()
	if p.Info().Counts.Items != 102 || p.Info().CanRedo {
		t.Fatal("duplicate not redone")
	}
	// New edit clears redo.
	p.Undo()
	p.AddThing(thing.CategoryEffect)
	if p.Info().CanRedo {
		t.Fatal("redo must be cleared")
	}
}

func TestRemoveThings(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddThing(thing.CategoryItem)
	p.AddThing(thing.CategoryItem) // 100,101,102
	it, _ := p.Thing(thing.CategoryItem, 101)
	it.Props.Pickupable = true
	p.UpdateThing(it)
	if err := p.RemoveThings(thing.CategoryItem, []uint32{101, 102}); err != nil {
		t.Fatal(err)
	}
	// 102 is removed first, then 101 becomes the last and is removed too.
	if p.Info().Counts.Items != 100 {
		t.Fatalf("count %d", p.Info().Counts.Items)
	}
	if _, err := p.Thing(thing.CategoryItem, 101); err == nil {
		t.Fatal("101 must be gone")
	}
	p.Undo()
	if p.Info().Counts.Items != 102 {
		t.Fatalf("undo count %d", p.Info().Counts.Items)
	}
	got, _ := p.Thing(thing.CategoryItem, 101)
	if !got.Props.Pickupable {
		t.Fatal("undo did not restore 101")
	}
	// First item cannot disappear.
	p.RemoveThings(thing.CategoryOutfit, []uint32{1})
	if p.Info().Counts.Outfits != 1 {
		t.Fatal("first outfit removed")
	}
	if err := p.RemoveThings(thing.CategoryItem, []uint32{999}); err == nil {
		t.Fatal("missing id must fail")
	}
}

func TestRemoveMiddleKeepsIDs(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddThing(thing.CategoryItem)
	p.AddThing(thing.CategoryItem)
	it, _ := p.Thing(thing.CategoryItem, 101)
	it.Props.Pickupable = true
	p.UpdateThing(it)
	p.RemoveThings(thing.CategoryItem, []uint32{101})
	if p.Info().Counts.Items != 102 {
		t.Fatal("middle removal must keep count")
	}
	got, _ := p.Thing(thing.CategoryItem, 101)
	if got.Props.Pickupable {
		t.Fatal("middle thing not cleared")
	}
}

func TestSpritesUndo(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddSprites([][]byte{solid(1, 1, 1), solid(2, 2, 2), solid(3, 3, 3)})
	if err := p.ReplaceSprite(2, solid(9, 9, 9)); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveSprites([]uint32{3, 1}); err != nil {
		t.Fatal(err)
	}
	if p.Info().Counts.Sprites != 2 {
		t.Fatalf("count %d", p.Info().Counts.Sprites)
	}
	if empty, _ := p.IsSpriteEmpty(1); !empty {
		t.Fatal("sprite 1 must be empty")
	}
	p.Undo()
	if p.Info().Counts.Sprites != 3 {
		t.Fatal("undo remove count")
	}
	px, _ := p.SpritePixels(3)
	if px[0] != 3 {
		t.Fatal("sprite 3 not restored")
	}
	p.Undo()
	px, _ = p.SpritePixels(2)
	if px[0] != 2 {
		t.Fatal("replace not undone")
	}
	p.Undo()
	if p.Info().Counts.Sprites != 0 {
		t.Fatal("add not undone")
	}
	p.Redo()
	px, _ = p.SpritePixels(3)
	if px[0] != 3 {
		t.Fatal("add not redone")
	}
	if err := p.ReplaceSprite(1, []byte{1}); err == nil {
		t.Fatal("bad pixel size must fail")
	}
}

func TestUpdateThingValidation(t *testing.T) {
	p := New(v1098(), client.Features{})
	it, _ := p.Thing(thing.CategoryItem, 100)
	it.FrameGroups[0].Sprites[0] = 5
	if err := p.UpdateThing(it); err == nil {
		t.Fatal("reference to missing sprite must fail")
	}
	it.ID = 500
	it.FrameGroups[0].Sprites[0] = 0
	if err := p.UpdateThing(it); err == nil {
		t.Fatal("missing thing must fail")
	}
}

func TestOBDImportExport(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddSprites([][]byte{solid(5, 5, 5)})
	o, _ := p.Thing(thing.CategoryOutfit, 1)
	o.FrameGroups[0].Sprites = []uint32{1, 0, 1, 0}
	o.Props.AnimateAlways = true
	if err := p.UpdateThing(o); err != nil {
		t.Fatal(err)
	}
	d, err := p.ExportOBD(thing.CategoryOutfit, 1, obd.Version3)
	if err != nil {
		t.Fatal(err)
	}
	file, err := obd.Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	back, err := obd.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	id, err := p.ImportOBD(back, 0)
	if err != nil || id != 2 {
		t.Fatalf("import %d %v", id, err)
	}
	got, _ := p.Thing(thing.CategoryOutfit, 2)
	// Identical sprites are deduplicated, empty ones map to 0.
	s := got.FrameGroups[0].Sprites
	if s[0] != 2 || s[1] != 0 || s[2] != 2 || s[3] != 0 || p.Info().Counts.Sprites != 2 {
		t.Fatalf("sprites %v count %d", s, p.Info().Counts.Sprites)
	}
	if !got.Props.AnimateAlways {
		t.Fatal("props lost")
	}
	p.Undo()
	if p.Info().Counts.Outfits != 1 || p.Info().Counts.Sprites != 1 {
		t.Fatal("import undo")
	}
	if _, err := p.ImportOBD(back, 1); err != nil {
		t.Fatal(err)
	}
	if p.Info().Counts.Outfits != 1 {
		t.Fatal("replace import must not add")
	}
}

func TestWarnings(t *testing.T) {
	p := New(v1098(), client.Features{})
	it, _ := p.Thing(thing.CategoryItem, 100)
	it.Props.IsMarket = true
	p.UpdateThing(it)
	v, _ := client.FindBySignatures(0x4C28B721, 0x4C220594) // 8.60
	if w := p.Warnings(FormatDat, v); len(w) != 0 {
		t.Fatalf("8.60 supports market: %v", w)
	}
	v = client.FindByValue(854)[0]
	if w := p.Warnings(FormatDat, v); len(w) != 1 {
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
	p := New(v1098(), client.Features{})
	px := [][]byte{solid(1, 2, 3), make([]byte, 32*32*4), solid(1, 2, 3), solid(4, 4, 4)}
	if err := p.SetGroupPixels(thing.CategoryOutfit, 1, 0, px); err != nil {
		t.Fatal(err)
	}
	o, _ := p.Thing(thing.CategoryOutfit, 1)
	s := o.FrameGroups[0].Sprites
	if s[0] != 1 || s[1] != 0 || s[2] != 1 || s[3] != 2 {
		t.Fatalf("sprites %v", s)
	}
	if err := p.SetGroupPixels(thing.CategoryOutfit, 1, 0, px[:2]); err == nil {
		t.Fatal("slot count mismatch must fail")
	}
	if err := p.SetGroupPixels(thing.CategoryOutfit, 1, 3, px); err == nil {
		t.Fatal("bad group must fail")
	}
	p.Undo()
	if p.Info().Counts.Sprites != 0 {
		t.Fatal("undo")
	}
}

func TestUndoBackToSavedState(t *testing.T) {
	dir := t.TempDir()
	p := New(v1098(), client.Features{})
	o := CompileOptions{DatPath: filepath.Join(dir, "Tibia.dat"), SprPath: filepath.Join(dir, "Tibia.spr"), Version: v1098(), Features: p.Info().Features}
	if err := p.Compile(o); err != nil {
		t.Fatal(err)
	}
	p.AddThing(thing.CategoryItem)
	if !p.Info().Changed {
		t.Fatal("edit must mark the client changed")
	}
	p.Undo()
	if p.Info().Changed {
		t.Fatal("undo back to the compiled state must clear changed")
	}
	p.Redo()
	if !p.Info().Changed {
		t.Fatal("redo must mark the client changed again")
	}
}

func TestNewClientStaysChangedAfterUndo(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddThing(thing.CategoryItem)
	p.Undo()
	if !p.Info().Changed {
		t.Fatal("a client never written to disk is always changed")
	}
}

func TestImportSheetInfersOutfit(t *testing.T) {
	// 4 directions x 9 frames of 64 px: idle + 8 walking frames.
	img := image.NewNRGBA(image.Rect(0, 0, 256, 576))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i-3], img.Pix[i] = byte(i/1024), 255
	}
	p := New(v1098(), client.Features{})
	if err := p.ImportSheet(thing.CategoryOutfit, 1, img); err != nil {
		t.Fatal(err)
	}
	o, _ := p.Thing(thing.CategoryOutfit, 1)
	if len(o.FrameGroups) != 2 || o.FrameGroups[0].Frames != 1 || o.FrameGroups[1].Frames != 8 {
		t.Fatalf("groups %d", len(o.FrameGroups))
	}
	for _, g := range o.FrameGroups {
		if g.Width != 2 || g.Height != 2 || g.PatternX != 4 || g.Validate() != nil {
			t.Fatalf("group %+v", g)
		}
	}
	p.Undo()
	if o, _ := p.Thing(thing.CategoryOutfit, 1); o.FrameGroups[0].Width != 1 {
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

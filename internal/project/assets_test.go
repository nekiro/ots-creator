package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/assets"
	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

// buildAssets converts a small dat project into an asset folder: one 1x1
// item and one 2x2 animated item.
func buildAssets(t *testing.T) string {
	t.Helper()
	p := New(v1098(), client.Features{Transparency: true})
	ids, _ := p.AddSprites([][]byte{solid(9, 0, 0), solid(0, 9, 0), solid(0, 0, 9), solid(9, 9, 0), solid(0, 9, 9), solid(9, 0, 9)})
	it, _ := p.Thing(thing.CategoryItem, 100)
	it.FrameGroups[0].Sprites[0] = ids[0]
	it.Props.Pickupable = true
	if err := p.UpdateThing(it); err != nil {
		t.Fatal(err)
	}
	id, _ := p.AddThing(thing.CategoryItem)
	big, _ := p.Thing(thing.CategoryItem, id)
	g := big.FrameGroups[0]
	g.Resize(2, 2, 1, 1, 1, 1, 2, 100)
	copy(g.Sprites, []uint32{ids[1], ids[2], ids[3], ids[4], ids[5], ids[0], 0, ids[1]})
	big.Props.HasLight, big.Props.LightLevel, big.Props.LightColor = true, 4, 215
	if err := p.UpdateThing(big); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := p.Compile(CompileOptions{Format: FormatAssets, DatPath: dir}); err != nil {
		t.Fatal(err)
	}
	if info := p.Info(); info.Format != FormatAssets || info.Changed {
		t.Fatalf("after compile %+v", info)
	}
	return dir
}

func piece(t *testing.T, p *Project, id uint32) []byte {
	t.Helper()
	px, err := p.SpritePixels(id)
	if err != nil {
		t.Fatal(err)
	}
	return px
}

func TestAssetsRoundTrip(t *testing.T) {
	dir := buildAssets(t)
	p, err := OpenAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := assets.ReadCatalog(dir)
	if len(cat.Sheets) != 2 {
		t.Fatalf("sheets %+v", cat.Sheets)
	}
	it, _ := p.Thing(thing.CategoryItem, 100)
	if !it.Props.Pickupable || !bytes.Equal(piece(t, p, it.FrameGroups[0].Sprites[0]), solid(9, 0, 0)) {
		t.Fatal("1x1 item")
	}
	big, _ := p.Thing(thing.CategoryItem, 101)
	g := big.FrameGroups[0]
	if g.Width != 2 || g.Height != 2 || g.Frames != 2 || big.Props.LightColor != 215 {
		t.Fatalf("2x2 item %+v", g)
	}
	want := [][]byte{solid(0, 9, 0), solid(0, 0, 9), solid(9, 9, 0), solid(0, 9, 9), solid(9, 0, 9), solid(9, 0, 0), make([]byte, 32*32*4), solid(0, 9, 0)}
	for i, id := range g.Sprites {
		if !bytes.Equal(piece(t, p, id), want[i]) {
			t.Fatalf("piece %d (sprite %d) differs", i, id)
		}
	}

	// Edit one piece and a flag: only one new sheet, old ones stay.
	if err := p.ReplaceSprite(g.Sprites[0], solid(1, 2, 3)); err != nil {
		t.Fatal(err)
	}
	it.Props.Pickupable = false
	it.Props.Stackable = true
	p.UpdateThing(it)
	if err := p.Compile(CompileOptions{Format: FormatAssets, DatPath: dir}); err != nil {
		t.Fatal(err)
	}
	cat2, _ := assets.ReadCatalog(dir)
	if len(cat2.Sheets) != 3 || cat2.Sheets[0] != cat.Sheets[0] || cat2.Sheets[1] != cat.Sheets[1] || cat2.Sheets[2].SpriteType != assets.Sprite64x64 {
		t.Fatalf("sheets %+v", cat2.Sheets)
	}
	if _, err := os.Stat(filepath.Join(dir, cat.Appearances)); !os.IsNotExist(err) {
		t.Fatal("old appearances file must be removed")
	}
	q, err := OpenAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	it2, _ := q.Thing(thing.CategoryItem, 100)
	big2, _ := q.Thing(thing.CategoryItem, 101)
	if it2.Props.Pickupable || !it2.Props.Stackable {
		t.Fatal("flag edit lost")
	}
	if !bytes.Equal(piece(t, q, big2.FrameGroups[0].Sprites[0]), solid(1, 2, 3)) || !bytes.Equal(piece(t, q, big2.FrameGroups[0].Sprites[4]), solid(9, 0, 9)) {
		t.Fatal("edited sprite lost")
	}

	// Back to dat/spr.
	out := t.TempDir()
	o := CompileOptions{DatPath: filepath.Join(out, "Tibia.dat"), SprPath: filepath.Join(out, "Tibia.spr"), Version: v1098(), Features: client.Features{Transparency: true}}
	if err := q.Compile(o); err != nil {
		t.Fatal(err)
	}
	r, err := Open(OpenOptions{DatPath: o.DatPath, SprPath: o.SprPath, Features: o.Features})
	if err != nil {
		t.Fatal(err)
	}
	big3, _ := r.Thing(thing.CategoryItem, 101)
	if big3.FrameGroups[0].Width != 2 || !bytes.Equal(piece(t, r, big3.FrameGroups[0].Sprites[0]), solid(1, 2, 3)) {
		t.Fatal("dat conversion")
	}
}

func TestAssetsWarnings(t *testing.T) {
	p := New(v1098(), client.Features{})
	it, _ := p.Thing(thing.CategoryItem, 100)
	it.FrameGroups[0].Resize(3, 1, 1, 1, 1, 1, 1, 0)
	it.Props.HasCharges = true
	p.UpdateThing(it)
	if w := p.Warnings(FormatAssets, client.Version{}); len(w) != 2 {
		t.Fatalf("%v", w)
	}
	if err := p.Compile(CompileOptions{Format: FormatAssets, DatPath: t.TempDir()}); err == nil {
		t.Fatal("3x1 objects cannot be written")
	}
}

// TestOfficialAssetsEdit opens the asset folder in OTS_ASSETS, edits one
// item and compiles to a copy.
func TestOfficialAssetsEdit(t *testing.T) {
	src := os.Getenv("OTS_ASSETS")
	if src == "" {
		t.Skip("OTS_ASSETS not set")
	}
	p, err := OpenAssets(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", p.Info().Counts)
	img, err := p.Render(thing.CategoryOutfit, 128, TexturePos{PatternX: 2})
	if err != nil || img.Bounds().Dx() != 64 {
		t.Fatalf("render %v %v", err, img.Bounds())
	}
	it, _ := p.Thing(thing.CategoryItem, 3031) // gold coin
	it.Props.Rotatable = true
	if err := p.UpdateThing(it); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := p.Compile(CompileOptions{Format: FormatAssets, DatPath: dir}); err != nil {
		t.Fatal(err)
	}
	q, err := OpenAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	it2, _ := q.Thing(thing.CategoryItem, 3031)
	if !it2.Props.Rotatable || q.Info().Counts != p.Info().Counts {
		t.Fatal("edit lost")
	}
}

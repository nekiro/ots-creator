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

// Converting from dat must not hand out sprite id 0, clients read it as "no sprite".
func TestAssetsSpriteIDsStartAtOne(t *testing.T) {
	cat, err := assets.ReadCatalog(buildAssets(t))
	if err != nil {
		t.Fatal(err)
	}
	if cat.Sheets[0].First != 1 {
		t.Fatalf("first sheet starts at %d", cat.Sheets[0].First)
	}
}

// Folders written by older versions hold sprite id 0; compiling them again
// gives that sprite a new id and keeps every other sprite.
func TestResolveRewritesSpriteZero(t *testing.T) {
	p, err := OpenAssets(buildAssets(t))
	if err != nil {
		t.Fatal(err)
	}
	shifted := &assets.Catalog{}
	for _, sheet := range p.assets.catalog.Sheets {
		sheet.First--
		sheet.Last--
		shifted.Sheets = append(shifted.Sheets, sheet)
	}
	writer := &assetWriter{p: p, pieces: assets.NewPieceMap(shifted), known: map[string]*newSprite{}}

	small, _ := p.Thing(thing.CategoryItem, 100)
	if _, err := writer.resolve(small.FrameGroups[0].Sprites[:1], 1, 1); err != nil {
		t.Fatal(err)
	}
	if len(writer.order) != 1 {
		t.Fatalf("sprite 0 must become a new sprite, got %d new", len(writer.order))
	}

	big, _ := p.Thing(thing.CategoryItem, 101)
	pieces := big.FrameGroups[0].Sprites[:4]
	want, _ := writer.pieces.Sprite(pieces, 2, 2)
	got, err := writer.resolve(pieces, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want == 0 || got != want || len(writer.order) != 1 {
		t.Fatalf("untouched sprite %d resolved to %d, %d new", want, got, len(writer.order))
	}
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

	// Edit one piece and a flag: the 1x1 sheet stays; the 2x2 sheet is now
	// partly used, so its other sprite moves to the new sheet and it goes.
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
	if len(cat2.Sheets) != 2 || cat2.Sheets[0] != cat.Sheets[0] || cat2.Sheets[1].SpriteType != assets.Sprite64x64 || cat2.Sheets[1].Count() != 2 {
		t.Fatalf("sheets %+v", cat2.Sheets)
	}
	if _, err := os.Stat(filepath.Join(dir, cat.Sheets[1].File)); !os.IsNotExist(err) {
		t.Fatal("repacked sheet file must be removed")
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

// Removing the only object of a sheet drops the sheet from the catalog and
// the disk, and a copy to another folder leaves it out.
func TestAssetsDropUnusedSheet(t *testing.T) {
	dir := buildAssets(t)
	p, err := OpenAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := assets.ReadCatalog(dir)
	if err := p.RemoveThings(thing.CategoryItem, []uint32{101}); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if err := p.Compile(CompileOptions{Format: FormatAssets, DatPath: other}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other, cat.Sheets[1].File)); !os.IsNotExist(err) {
		t.Fatal("unused sheet must not be copied")
	}
	if _, err := os.Stat(filepath.Join(other, cat.Appearances)); !os.IsNotExist(err) {
		t.Fatal("old appearances must not be copied")
	}

	p, _ = OpenAssets(dir)
	p.RemoveThings(thing.CategoryItem, []uint32{101})
	var stats CompileStats
	if err := p.Compile(CompileOptions{Format: FormatAssets, DatPath: dir, Stats: &stats}); err != nil {
		t.Fatal(err)
	}
	if gone, _ := os.Stat(filepath.Join(other, cat.Sheets[0].File)); stats.SheetsRemoved != 1 || stats.Freed <= 0 || gone == nil {
		t.Fatalf("stats %+v", stats)
	}
	cat2, _ := assets.ReadCatalog(dir)
	if len(cat2.Sheets) != 1 || cat2.Sheets[0] != cat.Sheets[0] {
		t.Fatalf("sheets %+v", cat2.Sheets)
	}
	if _, err := os.Stat(filepath.Join(dir, cat.Sheets[1].File)); !os.IsNotExist(err) {
		t.Fatal("unused sheet file must be removed")
	}
	q, err := OpenAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	it, _ := q.Thing(thing.CategoryItem, 100)
	if !bytes.Equal(piece(t, q, it.FrameGroups[0].Sprites[0]), solid(9, 0, 0)) {
		t.Fatal("kept sprite lost")
	}
}

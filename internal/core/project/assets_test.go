package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/assets"
)

func buildAssets(t *testing.T) string {
	t.Helper()
	project := New(v1098(), client.Features{Transparency: true})
	ids, _ := project.AddSprites([][]byte{solid(9, 0, 0), solid(0, 9, 0), solid(0, 0, 9), solid(9, 9, 0), solid(0, 9, 9), solid(9, 0, 9)})
	it, _ := project.Thing(thing.CategoryItem, 100)
	it.FrameGroups[0].Sprites[0] = ids[0]
	it.Props.Pickupable = true
	if err := project.UpdateThing(it); err != nil {
		t.Fatal(err)
	}
	id, _ := project.AddThing(thing.CategoryItem)
	big, _ := project.Thing(thing.CategoryItem, id)
	group := big.FrameGroups[0]
	group.Resize(2, 2, 1, 1, 1, 1, 2, 100)
	copy(group.Sprites, []uint32{ids[1], ids[2], ids[3], ids[4], ids[5], ids[0], 0, ids[1]})
	big.Props.HasLight, big.Props.LightLevel, big.Props.LightColor = true, 4, 215
	if err := project.UpdateThing(big); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := project.Compile(CompileOptions{Format: FormatAssets, DatPath: dir}); err != nil {
		t.Fatal(err)
	}
	if info := project.Info(); info.Format != FormatAssets || info.Changed {
		t.Fatalf("after compile %+v", info)
	}
	return dir
}

// Converting from dat must not hand out sprite id 0, clients read it as "no sprite".
func TestAssetsSpriteIDsStartAtOne(t *testing.T) {
	catalog, err := assets.ReadCatalog(buildAssets(t))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Sheets[0].First != 1 {
		t.Fatalf("first sheet starts at %d", catalog.Sheets[0].First)
	}
}

// Folders written by older versions hold sprite id 0; compiling them again
// gives that sprite a new id and keeps every other sprite.
func TestResolveRewritesSpriteZero(t *testing.T) {
	project, err := OpenAssets(buildAssets(t))
	if err != nil {
		t.Fatal(err)
	}
	shifted := &assets.Catalog{}
	for _, sheet := range project.assets.catalog.Sheets {
		sheet.First--
		sheet.Last--
		shifted.Sheets = append(shifted.Sheets, sheet)
	}
	writer := &assetWriter{project: project, pieces: assets.NewPieceMap(shifted), known: map[string]*newSprite{}}

	small, _ := project.Thing(thing.CategoryItem, 100)
	if _, err := writer.resolve(small.FrameGroups[0].Sprites[:1], 1, 1); err != nil {
		t.Fatal(err)
	}
	if len(writer.order) != 1 {
		t.Fatalf("sprite 0 must become a new sprite, got %d new", len(writer.order))
	}

	big, _ := project.Thing(thing.CategoryItem, 101)
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

func piece(t *testing.T, project *Project, id uint32) []byte {
	t.Helper()
	pixels, err := project.SpritePixels(id)
	if err != nil {
		t.Fatal(err)
	}
	return pixels
}

func TestAssetsRoundTrip(t *testing.T) {
	dir := buildAssets(t)
	project, err := OpenAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	catalog, _ := assets.ReadCatalog(dir)
	if len(catalog.Sheets) != 2 {
		t.Fatalf("sheets %+v", catalog.Sheets)
	}
	it, _ := project.Thing(thing.CategoryItem, 100)
	if !it.Props.Pickupable || !bytes.Equal(piece(t, project, it.FrameGroups[0].Sprites[0]), solid(9, 0, 0)) {
		t.Fatal("1x1 item")
	}
	big, _ := project.Thing(thing.CategoryItem, 101)
	group := big.FrameGroups[0]
	if group.Width != 2 || group.Height != 2 || group.Frames != 2 || big.Props.LightColor != 215 {
		t.Fatalf("2x2 item %+v", group)
	}
	want := [][]byte{solid(0, 9, 0), solid(0, 0, 9), solid(9, 9, 0), solid(0, 9, 9), solid(9, 0, 9), solid(9, 0, 0), make([]byte, 32*32*4), solid(0, 9, 0)}
	for i, id := range group.Sprites {
		if !bytes.Equal(piece(t, project, id), want[i]) {
			t.Fatalf("piece %d (sprite %d) differs", i, id)
		}
	}

	// Edit one piece and a flag: the 1x1 sheet stays; the 2x2 sheet is now
	// partly used, so its other sprite moves to the new sheet and it goes.
	if err := project.ReplaceSprite(group.Sprites[0], solid(1, 2, 3)); err != nil {
		t.Fatal(err)
	}
	it.Props.Pickupable = false
	it.Props.Stackable = true
	project.UpdateThing(it)
	if err := project.Compile(CompileOptions{Format: FormatAssets, DatPath: dir}); err != nil {
		t.Fatal(err)
	}
	cat2, _ := assets.ReadCatalog(dir)
	if len(cat2.Sheets) != 2 || cat2.Sheets[0] != catalog.Sheets[0] || cat2.Sheets[1].SpriteType != assets.Sprite64x64 || cat2.Sheets[1].Count() != 2 {
		t.Fatalf("sheets %+v", cat2.Sheets)
	}
	if _, err := os.Stat(filepath.Join(dir, catalog.Sheets[1].File)); !os.IsNotExist(err) {
		t.Fatal("repacked sheet file must be removed")
	}
	if _, err := os.Stat(filepath.Join(dir, catalog.Appearances)); !os.IsNotExist(err) {
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

	out := t.TempDir()
	options := CompileOptions{DatPath: filepath.Join(out, "Tibia.dat"), SprPath: filepath.Join(out, "Tibia.spr"), Version: v1098(), Features: client.Features{Transparency: true}}
	if err := q.Compile(options); err != nil {
		t.Fatal(err)
	}
	r, err := Open(OpenOptions{DatPath: options.DatPath, SprPath: options.SprPath, Features: options.Features})
	if err != nil {
		t.Fatal(err)
	}
	big3, _ := r.Thing(thing.CategoryItem, 101)
	if big3.FrameGroups[0].Width != 2 || !bytes.Equal(piece(t, r, big3.FrameGroups[0].Sprites[0]), solid(1, 2, 3)) {
		t.Fatal("dat conversion")
	}
}

func TestAssetsWarnings(t *testing.T) {
	project := New(v1098(), client.Features{})
	it, _ := project.Thing(thing.CategoryItem, 100)
	it.FrameGroups[0].Resize(3, 1, 1, 1, 1, 1, 1, 0)
	it.Props.HasCharges = true
	project.UpdateThing(it)
	if w := project.Warnings(FormatAssets, client.Version{}); len(w) != 2 {
		t.Fatalf("%v", w)
	}
	if err := project.Compile(CompileOptions{Format: FormatAssets, DatPath: t.TempDir()}); err == nil {
		t.Fatal("3x1 objects cannot be written")
	}
}

// An outfit of a dat/spr client keeps idle and walking frames in one group;
// appearances need them as separate idle and walking groups.
func TestAssetsSplitLegacyOutfits(t *testing.T) {
	project := New(v1098(), client.Features{})
	ids, _ := project.AddSprites([][]byte{solid(9, 0, 0), solid(0, 9, 0), solid(0, 0, 9)})
	outfit, _ := project.Thing(thing.CategoryOutfit, 1)
	group := outfit.FrameGroups[0]
	group.Resize(1, 1, 1, 1, 1, 1, 3, 300)
	copy(group.Sprites, ids)
	if err := project.UpdateThing(outfit); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := project.Compile(CompileOptions{Format: FormatAssets, DatPath: dir}); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	outfit, _ = reopened.Thing(thing.CategoryOutfit, 1)
	if len(outfit.FrameGroups) != 2 {
		t.Fatalf("%d frame groups", len(outfit.FrameGroups))
	}
	idle, walking := outfit.FrameGroups[0], outfit.FrameGroups[1]
	if idle.Type != thing.FrameGroupDefault || idle.Frames != 1 || walking.Type != thing.FrameGroupWalking || walking.Frames != 2 {
		t.Fatalf("idle %+v walking %+v", idle, walking)
	}
	if !bytes.Equal(piece(t, reopened, idle.Sprites[0]), solid(9, 0, 0)) || !bytes.Equal(piece(t, reopened, walking.Sprites[1]), solid(0, 0, 9)) {
		t.Fatal("frames moved to the wrong group")
	}
	if err := reopened.Compile(CompileOptions{Format: FormatAssets, DatPath: dir}); err != nil {
		t.Fatal(err)
	}
	if outfit, _ = reopened.Thing(thing.CategoryOutfit, 1); len(outfit.FrameGroups) != 2 {
		t.Fatalf("%d frame groups after a second compile", len(outfit.FrameGroups))
	}
}

func TestOfficialAssetsEdit(t *testing.T) {
	src := os.Getenv("OTS_ASSETS")
	if src == "" {
		t.Skip("OTS_ASSETS not set")
	}
	project, err := OpenAssets(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", project.Info().Counts)
	img, err := project.Render(thing.CategoryOutfit, 128, TexturePos{PatternX: 2})
	if err != nil || img.Bounds().Dx() != 64 {
		t.Fatalf("render %v %v", err, img.Bounds())
	}
	it, _ := project.Thing(thing.CategoryItem, 3031) // gold coin
	it.Props.Rotatable = true
	if err := project.UpdateThing(it); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := project.Compile(CompileOptions{Format: FormatAssets, DatPath: dir}); err != nil {
		t.Fatal(err)
	}
	q, err := OpenAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	it2, _ := q.Thing(thing.CategoryItem, 3031)
	if !it2.Props.Rotatable || q.Info().Counts != project.Info().Counts {
		t.Fatal("edit lost")
	}
}

// Removing the only object of a sheet drops the sheet from the catalog and
// the disk, and a copy to another folder leaves it out.
func TestAssetsDropUnusedSheet(t *testing.T) {
	dir := buildAssets(t)
	project, err := OpenAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	catalog, _ := assets.ReadCatalog(dir)
	if err := project.RemoveThings(thing.CategoryItem, []uint32{101}); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if err := project.Compile(CompileOptions{Format: FormatAssets, DatPath: other}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other, catalog.Sheets[1].File)); !os.IsNotExist(err) {
		t.Fatal("unused sheet must not be copied")
	}
	if _, err := os.Stat(filepath.Join(other, catalog.Appearances)); !os.IsNotExist(err) {
		t.Fatal("old appearances must not be copied")
	}

	project, _ = OpenAssets(dir)
	project.RemoveThings(thing.CategoryItem, []uint32{101})
	var stats CompileStats
	if err := project.Compile(CompileOptions{Format: FormatAssets, DatPath: dir, Stats: &stats}); err != nil {
		t.Fatal(err)
	}
	if gone, _ := os.Stat(filepath.Join(other, catalog.Sheets[0].File)); stats.SheetsRemoved != 1 || stats.Freed <= 0 || gone == nil {
		t.Fatalf("stats %+v", stats)
	}
	cat2, _ := assets.ReadCatalog(dir)
	if len(cat2.Sheets) != 1 || cat2.Sheets[0] != catalog.Sheets[0] {
		t.Fatalf("sheets %+v", cat2.Sheets)
	}
	if _, err := os.Stat(filepath.Join(dir, catalog.Sheets[1].File)); !os.IsNotExist(err) {
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

// Compiling into a folder replaces its sheets and appearances files: ones
// the new catalog does not list (left from an earlier compile) are removed,
// other files stay.
func TestAssetsRemoveStaleFiles(t *testing.T) {
	project, err := OpenAssets(buildAssets(t))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	stale := []string{"sprites-stale.bmp.lzma", "appearances-stale.dat"}
	for _, name := range append(stale, "keep.txt") {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := project.Compile(CompileOptions{Format: FormatAssets, DatPath: dir}); err != nil {
		t.Fatal(err)
	}
	for _, name := range stale {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s must be removed", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatal("other files must stay")
	}
	catalog, err := assets.ReadCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range catalog.Files() {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("listed %s missing", name)
		}
	}
}

func TestAssetsVersion(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "assets")
	if got := AssetsVersion(dir); got.Value != 1200 || got.Name != "12+ (assets)" {
		t.Fatalf("without package.json: %+v", got)
	}
	for _, testCase := range []struct {
		version string
		value   uint16
		name    string
	}{
		{"13.40.12345", 1340, "13.40"},
		{"14.150.1", 1499, "14.150"},
		{"0.5.1", 1200, "12+ (assets)"},
	} {
		if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"version":"`+testCase.version+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := AssetsVersion(dir); got.Value != testCase.value || got.Name != testCase.name {
			t.Fatalf("%s: %+v", testCase.version, got)
		}
	}
}

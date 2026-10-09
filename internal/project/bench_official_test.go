package project

import (
	"os"
	"testing"
	"time"

	"github.com/nekiro/ots-creator/internal/thing"
)

// TestProfileOfficialAssets times the slow paths on a real Tibia asset
// folder (OTS_ASSETS). It only logs; run with -v.
func TestProfileOfficialAssets(t *testing.T) {
	src := os.Getenv("OTS_ASSETS")
	if src == "" {
		t.Skip("OTS_ASSETS not set")
	}
	step := func(name string, fn func() error) {
		t.Helper()
		start := time.Now()
		if err := fn(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		t.Logf("%-28s %v", name, time.Since(start).Round(time.Millisecond))
	}
	var p, q *Project
	step("OpenAssets", func() (err error) { p, err = OpenAssets(src); return })
	t.Logf("counts %+v", p.Info().Counts)
	step("OpenAssets (second)", func() (err error) { q, err = OpenAssets(src); return })
	step("Thumbnails 2000 items", func() error {
		for id := uint32(100); id < 2100; id++ {
			if t, err := p.Thing(thing.CategoryItem, id); err == nil {
				p.Render(thing.CategoryItem, id, ThumbnailPos(t))
			}
		}
		return nil
	})
	step("FindThings used", func() error { _, err := p.FindThings(thing.CategoryItem, Filter{Content: ContentUsed}); return err })
	step("Diff items (same client)", func() error { _, err := Diff(p, q, thing.CategoryItem); return err })
	step("Diff outfits", func() error { _, err := Diff(p, q, thing.CategoryOutfit); return err })
	step("FindSprites empty", func() error { _, err := p.FindSprites(SpritesEmpty); return err })
	step("FindSprites duplicate", func() error { _, err := p.FindSprites(SpritesDuplicate); return err })
	step("FindSprites unused", func() error { _, err := p.FindSprites(SpritesUnused); return err })
	step("Optimize (dry: undo)", func() error {
		_, err := p.OptimizeSprites(OptimizeOptions{Duplicates: true, Empty: true})
		p.Undo()
		return err
	})
	step("Edit + Compile", func() error {
		it, _ := p.Thing(thing.CategoryItem, 3031)
		it.Props.Rotatable = !it.Props.Rotatable
		if err := p.UpdateThing(it); err != nil {
			return err
		}
		return p.Compile(CompileOptions{Format: FormatAssets, DatPath: t.TempDir()})
	})
}

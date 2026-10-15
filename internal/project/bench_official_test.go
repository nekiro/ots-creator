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
	all := OptimizeOptions{Duplicates: true, Empty: true, Unused: true}
	step("AnalyzeSprites (cold)", func() error { _, err := p.AnalyzeSprites(all, nil); return err })
	step("AnalyzeSprites (cached)", func() error { _, err := p.AnalyzeSprites(all, nil); return err })
	step("FindSprites empty", func() error { _, err := p.FindSprites(SpritesEmpty); return err })
	step("FindSprites duplicate", func() error { _, err := p.FindSprites(SpritesDuplicate); return err })
	step("FindSprites unused", func() error { _, err := p.FindSprites(SpritesUnused); return err })
	step("Optimize (dry: undo)", func() error {
		_, err := p.OptimizeSprites(OptimizeOptions{Duplicates: true, Empty: true}, nil)
		p.Undo()
		return err
	})
	out := t.TempDir()
	step("Optimize + Compile", func() error {
		if _, err := q.OptimizeSprites(OptimizeOptions{Duplicates: true, Empty: true, Unused: true}, nil); err != nil {
			return err
		}
		return q.Compile(CompileOptions{Format: FormatAssets, DatPath: out})
	})
	t.Logf("folder size %d MB -> %d MB", dirSize(src)>>20, dirSize(out)>>20)
	// Every sprite of every object must look the same after optimizing and
	// compiling (p is back at the original after the undo above).
	step("Verify optimized output", func() error {
		r, err := OpenAssets(out)
		if err != nil {
			return err
		}
		hashOf := func(x *Project, id uint32) spriteHash {
			if id == 0 {
				return spriteHash{}
			}
			h, err := x.sprites.hash(id)
			if err != nil {
				t.Fatal(err)
			}
			return h
		}
		for _, c := range thing.Categories {
			for i, a := range p.things.Things[c] {
				b := r.things.Things[c][i]
				if a == nil {
					continue
				}
				for gi, g := range a.FrameGroups {
					for si, id := range g.Sprites {
						// Hashes use per-store seeds: compare pixels on mismatch.
						pa, _ := p.sprites.pixels(id)
						if hashOf(p, id) == (spriteHash{}) && hashOf(r, b.FrameGroups[gi].Sprites[si]) == (spriteHash{}) {
							continue
						}
						pb, _ := r.sprites.pixels(b.FrameGroups[gi].Sprites[si])
						if string(pa) != string(pb) {
							t.Fatalf("%v %d group %d sprite %d differs", c, a.ID, gi, si)
						}
					}
				}
			}
		}
		return nil
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

func dirSize(dir string) int64 {
	var n int64
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() {
			n += info.Size()
		}
	}
	return n
}

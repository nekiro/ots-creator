package project

import (
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

// bigClient compiles a client the size of a real 10.98 one (30k items,
// 60k sprites) and returns its paths.
func bigClient(b *testing.B) (string, string) {
	b.Helper()
	dir := b.TempDir()
	p := New(v1098(), client.Features{})
	px := make([][]byte, 60000)
	for i := range px {
		s := solid(byte(i), byte(i>>8), 7)
		for k := 0; k < len(s); k += 12 {
			s[k+3] = 0 // some transparency, like real sprites
		}
		px[i] = s
	}
	p.AddSprites(px)
	for i := 0; i < 30000; i++ {
		id, _ := p.AddThing(thing.CategoryItem)
		t, _ := p.Thing(thing.CategoryItem, id)
		t.FrameGroups[0].Sprites[0] = uint32(i*2%60000 + 1)
		t.Props.Pickupable = i%3 == 0
		p.UpdateThing(t)
	}
	o := CompileOptions{DatPath: filepath.Join(dir, "Tibia.dat"), SprPath: filepath.Join(dir, "Tibia.spr"), Version: v1098(), Features: p.Info().Features}
	if err := p.Compile(o); err != nil {
		b.Fatal(err)
	}
	return o.DatPath, o.SprPath
}

func BenchmarkBigClient(b *testing.B) {
	dat, spr := bigClient(b)
	open := func() *Project {
		p, err := Open(OpenOptions{DatPath: dat, SprPath: spr})
		if err != nil {
			b.Fatal(err)
		}
		return p
	}
	b.Run("Open", func(b *testing.B) {
		for range b.N {
			open()
		}
	})
	p := open()
	b.Run("FindUsed", func(b *testing.B) {
		for range b.N {
			p.FindThings(thing.CategoryItem, Filter{Content: ContentUsed})
		}
	})
	b.Run("RenderThumb", func(b *testing.B) {
		for i := range b.N {
			p.Render(thing.CategoryItem, uint32(100+i%30000), TexturePos{})
		}
	})
	b.Run("ReplaceSpriteDelta", func(b *testing.B) {
		px := solid(1, 2, 3)
		for range b.N {
			p.ReplaceSprite(5, px)
			p.TakeDelta()
		}
	})
	b.Run("Compile", func(b *testing.B) {
		dir := b.TempDir()
		for range b.N {
			o := CompileOptions{DatPath: filepath.Join(dir, "a.dat"), SprPath: filepath.Join(dir, "a.spr"), Version: v1098(), Features: p.Info().Features}
			if err := p.Compile(o); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("AddSprites10k", func(b *testing.B) {
		px := make([][]byte, 10000)
		for i := range px {
			px[i] = solid(byte(i), 3, 3)
		}
		for range b.N {
			p.AddSprites(px)
			p.Undo()
		}
	})
	b.Run("Optimize", func(b *testing.B) {
		for range b.N {
			p.OptimizeSprites(OptimizeOptions{Duplicates: true, Empty: true})
			p.Undo()
		}
	})
}

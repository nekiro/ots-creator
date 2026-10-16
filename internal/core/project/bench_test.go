package project

import (
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// bigClient compiles a client the size of a real 10.98 one (30k items,
// 60k sprites) and returns its paths.
func bigClient(b *testing.B) (string, string) {
	b.Helper()
	dir := b.TempDir()
	project := New(v1098(), client.Features{})
	px := make([][]byte, 60000)
	for i := range px {
		s := solid(byte(i), byte(i>>8), 7)
		for pieceIndex := 0; pieceIndex < len(s); pieceIndex += 12 {
			s[pieceIndex+3] = 0 // some transparency, like real sprites
		}
		px[i] = s
	}
	project.AddSprites(px)
	for i := 0; i < 30000; i++ {
		id, _ := project.AddThing(thing.CategoryItem)
		object, _ := project.Thing(thing.CategoryItem, id)
		object.FrameGroups[0].Sprites[0] = uint32(i*2%60000 + 1)
		object.Props.Pickupable = i%3 == 0
		project.UpdateThing(object)
	}
	options := CompileOptions{DatPath: filepath.Join(dir, "Tibia.dat"), SprPath: filepath.Join(dir, "Tibia.spr"), Version: v1098(), Features: project.Info().Features}
	if err := project.Compile(options); err != nil {
		b.Fatal(err)
	}
	return options.DatPath, options.SprPath
}

func BenchmarkBigClient(b *testing.B) {
	dat, spr := bigClient(b)
	open := func() *Project {
		project, err := Open(OpenOptions{DatPath: dat, SprPath: spr})
		if err != nil {
			b.Fatal(err)
		}
		return project
	}
	b.Run("Open", func(b *testing.B) {
		for range b.N {
			open()
		}
	})
	project := open()
	b.Run("FindUsed", func(b *testing.B) {
		for range b.N {
			project.FindThings(thing.CategoryItem, Filter{Content: ContentUsed})
		}
	})
	b.Run("RenderThumb", func(b *testing.B) {
		for i := range b.N {
			project.Render(thing.CategoryItem, uint32(100+i%30000), TexturePos{})
		}
	})
	b.Run("ReplaceSpriteDelta", func(b *testing.B) {
		pixels := solid(1, 2, 3)
		for range b.N {
			project.ReplaceSprite(5, pixels)
			project.TakeDelta()
		}
	})
	b.Run("Compile", func(b *testing.B) {
		dir := b.TempDir()
		for range b.N {
			options := CompileOptions{DatPath: filepath.Join(dir, "a.dat"), SprPath: filepath.Join(dir, "a.spr"), Version: v1098(), Features: project.Info().Features}
			if err := project.Compile(options); err != nil {
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
			project.AddSprites(px)
			project.Undo()
		}
	})
	other := open()
	b.Run("DiffSame", func(b *testing.B) {
		for range b.N {
			if _, err := Diff(project, other, thing.CategoryItem); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Transfer1k", func(b *testing.B) {
		ids := make([]uint32, 1000)
		for i := range ids {
			ids[i] = uint32(100 + i*7)
		}
		for range b.N {
			if _, err := Transfer(project, other, thing.CategoryItem, ids, true); err != nil {
				b.Fatal(err)
			}
			project.Undo()
		}
	})
	b.Run("Optimize", func(b *testing.B) {
		for range b.N {
			project.OptimizeSprites(OptimizeOptions{Duplicates: true, Empty: true}, nil)
			project.Undo()
		}
	})
}

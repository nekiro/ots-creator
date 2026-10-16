package app

import (
	"testing"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/maprender"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func BenchmarkGeometry(b *testing.B) {
	ms, s := mapTestService(b)
	proj, _ := s.Project()
	ids, _ := proj.AddSprites([][]byte{make([]byte, 32*32*4), make([]byte, 32*32*4)})
	for i, id := range []uint32{100, 101} {
		g, _ := proj.Thing(thing.CategoryItem, id)
		g.FrameGroups[0].Sprites[0] = ids[i]
		proj.UpdateThing(g)
	}
	info, _ := ms.NewMap(NewMapOptions{Width: 1024, Height: 1024})
	var pts []otmap.Position
	for y := 0; y < 1024; y += 3 {
		for x := 0; x < 1024; x += 3 {
			pts = append(pts, otmap.Position{X: uint16(x), Y: uint16(y), Z: 7})
		}
	}
	ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 100, Points: pts, Size: 1})
	ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 101, Points: pts, Size: 1})
	opened, data, _ := ms.get(info.ID)
	scene := ms.scene(opened, data)
	var keys []otmap.ChunkKey
	for cy := 0; cy < 32; cy++ {
		for cx := 0; cx < 32; cx++ {
			keys = append(keys, otmap.MakeKey(uint16(cx), uint16(cy), 7))
		}
	}
	b.ResetTimer()
	var size int
	for i := 0; i < b.N; i++ {
		_, size, _ = maprender.BuildGeometry(scene, keys, maprender.GeometryOptions{})
	}
	b.ReportMetric(float64(size)/1e6, "MB")
}

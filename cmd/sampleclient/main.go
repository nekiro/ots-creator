// Command sampleclient writes a small 10.98 client with generated sprites.
// It is used for manual UI testing without shipping Tibia assets.
//
//	go run ./cmd/sampleclient -out ./testdata/sample
package main

import (
	"flag"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

const size = 32

type canvas []byte

func newCanvas() canvas { return make(canvas, size*size*4) }

func (c canvas) set(x, y int, col color.NRGBA) {
	if x < 0 || y < 0 || x >= size || y >= size {
		return
	}
	i := (y*size + x) * 4
	c[i], c[i+1], c[i+2], c[i+3] = col.R, col.G, col.B, col.A
}

func (c canvas) disc(cx, cy, r float64, col color.NRGBA) {
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if math.Hypot(float64(x)-cx, float64(y)-cy) <= r {
				c.set(x, y, col)
			}
		}
	}
}

func (c canvas) rect(x0, y0, x1, y1 int, col color.NRGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c.set(x, y, col)
		}
	}
}

func hue(h float64) color.NRGBA {
	r := 0.5 + 0.5*math.Cos(2*math.Pi*(h))
	g := 0.5 + 0.5*math.Cos(2*math.Pi*(h-1.0/3))
	b := 0.5 + 0.5*math.Cos(2*math.Pi*(h-2.0/3))
	return color.NRGBA{uint8(r * 255), uint8(g * 255), uint8(b * 255), 255}
}

func main() {
	out := flag.String("out", "testdata/sample", "output directory")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	v, _ := client.FindBySignatures(0x42A3, 0x57BBD603)
	p := project.New(v, client.Features{})

	// Items: grounds, gems and an animated torch.
	for i := 0; i < 160; i++ {
		c := newCanvas()
		col := hue(float64(i) / 160)
		if i%4 == 0 {
			c.rect(0, 0, size, size, color.NRGBA{col.R / 3, col.G / 3, col.B / 3, 255})
			for k := 0; k < 40; k++ {
				c.set((k*7+i)%size, (k*13+i*3)%size, col)
			}
		} else {
			c.disc(16, 18, 6+float64(i%5), col)
			c.disc(13, 15, 2, color.NRGBA{255, 255, 255, 200})
		}
		ids, err := p.AddSprites([][]byte{c})
		if err != nil {
			log.Fatal(err)
		}
		id := uint32(100)
		if i > 0 {
			if id, err = p.AddThing(thing.CategoryItem); err != nil {
				log.Fatal(err)
			}
		}
		t, _ := p.Thing(thing.CategoryItem, id)
		t.FrameGroups[0].Sprites[0] = ids[0]
		if i%4 == 0 {
			t.Props.Ground, t.Props.GroundSpeed = true, 150
		} else {
			t.Props.Pickupable, t.Props.Stackable = true, i%3 == 0
		}
		if err := p.UpdateThing(t); err != nil {
			log.Fatal(err)
		}
	}

	// Animated item (torch-like, 2x2 tiles, 4 frames).
	{
		id, _ := p.AddThing(thing.CategoryItem)
		t, _ := p.Thing(thing.CategoryItem, id)
		g := t.FrameGroups[0]
		g.Resize(2, 2, 1, 1, 1, 1, 4, 150)
		g.ExactSize = 64
		for f := 0; f < 4; f++ {
			for h := 0; h < 2; h++ {
				for w := 0; w < 2; w++ {
					c := newCanvas()
					cx := float64((1-w)*size) + 0
					cy := float64((1-h)*size) + 0
					r := 18 + float64(f)*3
					for y := 0; y < size; y++ {
						for x := 0; x < size; x++ {
							gx, gy := float64(x)+cx, float64(y)+cy
							d := math.Hypot(gx-32, gy-36)
							if d < r {
								k := 1 - d/r
								c.set(x, y, color.NRGBA{255, uint8(120 + 135*k), uint8(40 * k), uint8(255 * k)})
							}
						}
					}
					ids, _ := p.AddSprites([][]byte{c})
					g.Sprites[g.SpriteIndex(w, h, 0, 0, 0, 0, f)] = ids[0]
				}
			}
		}
		t.Props.HasLight, t.Props.LightLevel, t.Props.LightColor = true, 7, 206
		t.Props.AnimateAlways = true
		if err := p.UpdateThing(t); err != nil {
			log.Fatal(err)
		}
	}

	// Outfit: 4 directions, template layer, 1 addon, idle + walking.
	{
		t, _ := p.Thing(thing.CategoryOutfit, 1)
		makeGroup := func(frames uint8) *thing.FrameGroup {
			g := thing.NewFrameGroup()
			g.Resize(1, 1, 2, 4, 2, 1, frames, 120)
			for f := 0; f < int(frames); f++ {
				for d := 0; d < 4; d++ {
					for py := 0; py < 2; py++ {
						base, tpl := newCanvas(), newCanvas()
						bob := 0
						if f%2 == 1 {
							bob = 1
						}
						if py == 0 {
							skin := color.NRGBA{230, 190, 150, 255}
							base.disc(16, 9+float64(bob), 5, color.NRGBA{200, 200, 200, 255}) // head (template)
							tpl.disc(16, 9+float64(bob), 5, color.NRGBA{255, 255, 0, 255})
							base.disc(16, 11+float64(bob), 3, skin)
							base.rect(10, 15+bob, 22, 24+bob, color.NRGBA{210, 210, 210, 255}) // body
							tpl.rect(10, 15+bob, 22, 24+bob, color.NRGBA{255, 0, 0, 255})
							base.rect(11, 24, 21, 29, color.NRGBA{190, 190, 190, 255}) // legs
							tpl.rect(11, 24, 21, 29, color.NRGBA{0, 255, 0, 255})
							base.rect(11, 29, 21, 31, color.NRGBA{170, 170, 170, 255}) // feet
							tpl.rect(11, 29, 21, 31, color.NRGBA{0, 0, 255, 255})
							// direction marker
							dx := []int{0, 5, 0, -5}[d]
							dy := []int{-5, 0, 3, 0}[d]
							base.disc(16+float64(dx), 18+float64(dy+bob), 1.5, color.NRGBA{40, 40, 40, 255})
						} else {
							base.rect(8, 4+bob, 24, 6+bob, color.NRGBA{200, 160, 40, 255}) // hat addon
						}
						ids, _ := p.AddSprites([][]byte{base, tpl})
						g.Sprites[g.SpriteIndex(0, 0, 0, d, py, 0, f)] = ids[0]
						g.Sprites[g.SpriteIndex(0, 0, 1, d, py, 0, f)] = ids[1]
					}
				}
			}
			return g
		}
		idle := makeGroup(1)
		walk := makeGroup(8)
		walk.Type = 1
		t.FrameGroups = []*thing.FrameGroup{idle, walk}
		if err := p.UpdateThing(t); err != nil {
			log.Fatal(err)
		}
	}

	// Effect: expanding ring, 6 frames.
	{
		t, _ := p.Thing(thing.CategoryEffect, 1)
		g := t.FrameGroups[0]
		g.Resize(1, 1, 1, 1, 1, 1, 6, 100)
		for f := 0; f < 6; f++ {
			c := newCanvas()
			r := 3 + float64(f)*2.4
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					d := math.Abs(math.Hypot(float64(x)-16, float64(y)-16) - r)
					if d < 1.6 {
						c.set(x, y, color.NRGBA{120, 200, 255, uint8(255 - f*30)})
					}
				}
			}
			ids, _ := p.AddSprites([][]byte{c})
			g.Sprites[f] = ids[0]
		}
		if err := p.UpdateThing(t); err != nil {
			log.Fatal(err)
		}
	}

	// Missile: arrow in 9 directions (3x3 patterns).
	{
		t, _ := p.Thing(thing.CategoryMissile, 1)
		g := t.FrameGroups[0]
		g.Resize(1, 1, 1, 3, 3, 1, 1, 75)
		for py := 0; py < 3; py++ {
			for px := 0; px < 3; px++ {
				c := newCanvas()
				dx, dy := float64(px-1), float64(py-1)
				for k := -8.0; k <= 8; k += 0.5 {
					c.set(int(16+dx*k), int(16+dy*k), color.NRGBA{220, 220, 220, 255})
				}
				c.disc(16+dx*8, 16+dy*8, 2, color.NRGBA{255, 80, 80, 255})
				ids, _ := p.AddSprites([][]byte{c})
				g.Sprites[g.SpriteIndex(0, 0, 0, px, py, 0, 0)] = ids[0]
			}
		}
		if err := p.UpdateThing(t); err != nil {
			log.Fatal(err)
		}
	}

	datPath := filepath.Join(*out, "Tibia.dat")
	sprPath := filepath.Join(*out, "Tibia.spr")
	err := p.Compile(project.CompileOptions{DatPath: datPath, SprPath: sprPath, Version: v, Features: p.Info().Features, WriteOTFI: true})
	if err != nil {
		log.Fatal(err)
	}
	info := p.Info()
	log.Printf("wrote %s (%d items, %d sprites)", *out, info.Counts.Items-99, info.Counts.Sprites)
}

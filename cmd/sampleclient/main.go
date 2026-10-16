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

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

const size = 32

// Signatures of the 10.98 client files.
const (
	datSignature1098 = 0x42A3
	sprSignature1098 = 0x57BBD603
)

const (
	// generatedItems is the number of plain items (grounds and gems).
	generatedItems = 160
	// groundSpeckles is the number of light pixels on a ground sprite.
	groundSpeckles = 40
	// firstItemID is the first item id of a client.
	firstItemID = 100
)

type canvas []byte

func newCanvas() canvas { return make(canvas, size*size*4) }

func (c canvas) set(x, y int, col color.NRGBA) {
	if x < 0 || y < 0 || x >= size || y >= size {
		return
	}
	offset := (y*size + x) * 4
	c[offset], c[offset+1], c[offset+2], c[offset+3] = col.R, col.G, col.B, col.A
}

func (c canvas) disc(centerX, centerY, radius float64, col color.NRGBA) {
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if math.Hypot(float64(x)-centerX, float64(y)-centerY) <= radius {
				c.set(x, y, col)
			}
		}
	}
}

func (c canvas) rect(minX, minY, maxX, maxY int, col color.NRGBA) {
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			c.set(x, y, col)
		}
	}
}

func hue(position float64) color.NRGBA {
	red := 0.5 + 0.5*math.Cos(2*math.Pi*(position))
	green := 0.5 + 0.5*math.Cos(2*math.Pi*(position-1.0/3))
	blue := 0.5 + 0.5*math.Cos(2*math.Pi*(position-2.0/3))
	return color.NRGBA{uint8(red * 255), uint8(green * 255), uint8(blue * 255), 255}
}

func main() {
	out := flag.String("out", "testdata/sample", "output directory")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	version, _ := client.FindBySignatures(datSignature1098, sprSignature1098)
	proj := project.New(version, client.Features{})
	makeItems(proj)
	makeAnimatedItem(proj)
	makeOutfit(proj)
	makeEffect(proj)
	makeMissile(proj)
	writeOutput(proj, version, *out)
}

// makeItems adds grounds and gems.
func makeItems(proj *project.Project) {
	for i := 0; i < generatedItems; i++ {
		sprite := newCanvas()
		col := hue(float64(i) / generatedItems)
		if i%4 == 0 {
			sprite.rect(0, 0, size, size, color.NRGBA{col.R / 3, col.G / 3, col.B / 3, 255})
			for speckle := 0; speckle < groundSpeckles; speckle++ {
				sprite.set((speckle*7+i)%size, (speckle*13+i*3)%size, col)
			}
		} else {
			sprite.disc(16, 18, 6+float64(i%5), col)
			sprite.disc(13, 15, 2, color.NRGBA{255, 255, 255, 200})
		}
		ids, err := proj.AddSprites([][]byte{sprite})
		if err != nil {
			log.Fatal(err)
		}
		id := uint32(firstItemID)
		if i > 0 {
			if id, err = proj.AddThing(thing.CategoryItem); err != nil {
				log.Fatal(err)
			}
		}
		item, _ := proj.Thing(thing.CategoryItem, id)
		item.FrameGroups[0].Sprites[0] = ids[0]
		if i%4 == 0 {
			item.Props.Ground, item.Props.GroundSpeed = true, 150
		} else {
			item.Props.Pickupable, item.Props.Stackable = true, i%3 == 0
		}
		if err := proj.UpdateThing(item); err != nil {
			log.Fatal(err)
		}
	}
}

// makeAnimatedItem adds a torch-like item: 2x2 tiles, 4 frames.
func makeAnimatedItem(proj *project.Project) {
	id, _ := proj.AddThing(thing.CategoryItem)
	item, _ := proj.Thing(thing.CategoryItem, id)
	group := item.FrameGroups[0]
	group.Resize(2, 2, 1, 1, 1, 1, 4, 150)
	group.ExactSize = 64
	for frame := 0; frame < 4; frame++ {
		for tileY := 0; tileY < 2; tileY++ {
			for tileX := 0; tileX < 2; tileX++ {
				sprite := newCanvas()
				offsetX := float64((1-tileX)*size) + 0
				offsetY := float64((1-tileY)*size) + 0
				radius := 18 + float64(frame)*3
				for y := 0; y < size; y++ {
					for x := 0; x < size; x++ {
						globalX, globalY := float64(x)+offsetX, float64(y)+offsetY
						distance := math.Hypot(globalX-32, globalY-36)
						if distance < radius {
							strength := 1 - distance/radius
							sprite.set(x, y, color.NRGBA{255, uint8(120 + 135*strength), uint8(40 * strength), uint8(255 * strength)})
						}
					}
				}
				ids, _ := proj.AddSprites([][]byte{sprite})
				group.Sprites[group.SpriteIndex(tileX, tileY, 0, 0, 0, 0, frame)] = ids[0]
			}
		}
	}
	item.Props.HasLight, item.Props.LightLevel, item.Props.LightColor = true, 7, 206
	item.Props.AnimateAlways = true
	if err := proj.UpdateThing(item); err != nil {
		log.Fatal(err)
	}
}

// makeOutfit fills outfit 1: 4 directions, template layer, 1 addon, idle
// and walking.
func makeOutfit(proj *project.Project) {
	outfit, _ := proj.Thing(thing.CategoryOutfit, 1)
	idle := outfitGroup(proj, 1)
	walk := outfitGroup(proj, 8)
	walk.Type = 1
	outfit.FrameGroups = []*thing.FrameGroup{idle, walk}
	if err := proj.UpdateThing(outfit); err != nil {
		log.Fatal(err)
	}
}

func outfitGroup(proj *project.Project, frames uint8) *thing.FrameGroup {
	group := thing.NewFrameGroup()
	group.Resize(1, 1, 2, 4, 2, 1, frames, 120)
	for frame := 0; frame < int(frames); frame++ {
		for direction := 0; direction < 4; direction++ {
			for addon := 0; addon < 2; addon++ {
				base, template := newCanvas(), newCanvas()
				bob := 0
				if frame%2 == 1 {
					bob = 1
				}
				if addon == 0 {
					skin := color.NRGBA{230, 190, 150, 255}
					base.disc(16, 9+float64(bob), 5, color.NRGBA{200, 200, 200, 255}) // head (template)
					template.disc(16, 9+float64(bob), 5, color.NRGBA{255, 255, 0, 255})
					base.disc(16, 11+float64(bob), 3, skin)
					base.rect(10, 15+bob, 22, 24+bob, color.NRGBA{210, 210, 210, 255}) // body
					template.rect(10, 15+bob, 22, 24+bob, color.NRGBA{255, 0, 0, 255})
					base.rect(11, 24, 21, 29, color.NRGBA{190, 190, 190, 255}) // legs
					template.rect(11, 24, 21, 29, color.NRGBA{0, 255, 0, 255})
					base.rect(11, 29, 21, 31, color.NRGBA{170, 170, 170, 255}) // feet
					template.rect(11, 29, 21, 31, color.NRGBA{0, 0, 255, 255})
					// direction marker
					dx := []int{0, 5, 0, -5}[direction]
					dy := []int{-5, 0, 3, 0}[direction]
					base.disc(16+float64(dx), 18+float64(dy+bob), 1.5, color.NRGBA{40, 40, 40, 255})
				} else {
					base.rect(8, 4+bob, 24, 6+bob, color.NRGBA{200, 160, 40, 255}) // hat addon
				}
				ids, _ := proj.AddSprites([][]byte{base, template})
				group.Sprites[group.SpriteIndex(0, 0, 0, direction, addon, 0, frame)] = ids[0]
				group.Sprites[group.SpriteIndex(0, 0, 1, direction, addon, 0, frame)] = ids[1]
			}
		}
	}
	return group
}

// makeEffect fills effect 1: an expanding ring, 6 frames.
func makeEffect(proj *project.Project) {
	effect, _ := proj.Thing(thing.CategoryEffect, 1)
	group := effect.FrameGroups[0]
	group.Resize(1, 1, 1, 1, 1, 1, 6, 100)
	for frame := 0; frame < 6; frame++ {
		sprite := newCanvas()
		radius := 3 + float64(frame)*2.4
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				distance := math.Abs(math.Hypot(float64(x)-16, float64(y)-16) - radius)
				if distance < 1.6 {
					sprite.set(x, y, color.NRGBA{120, 200, 255, uint8(255 - frame*30)})
				}
			}
		}
		ids, _ := proj.AddSprites([][]byte{sprite})
		group.Sprites[frame] = ids[0]
	}
	if err := proj.UpdateThing(effect); err != nil {
		log.Fatal(err)
	}
}

// makeMissile fills missile 1: an arrow in 9 directions (3x3 patterns).
func makeMissile(proj *project.Project) {
	missile, _ := proj.Thing(thing.CategoryMissile, 1)
	group := missile.FrameGroups[0]
	group.Resize(1, 1, 1, 3, 3, 1, 1, 75)
	for patternY := 0; patternY < 3; patternY++ {
		for patternX := 0; patternX < 3; patternX++ {
			sprite := newCanvas()
			dx, dy := float64(patternX-1), float64(patternY-1)
			for step := -8.0; step <= 8; step += 0.5 {
				sprite.set(int(16+dx*step), int(16+dy*step), color.NRGBA{220, 220, 220, 255})
			}
			sprite.disc(16+dx*8, 16+dy*8, 2, color.NRGBA{255, 80, 80, 255})
			ids, _ := proj.AddSprites([][]byte{sprite})
			group.Sprites[group.SpriteIndex(0, 0, 0, patternX, patternY, 0, 0)] = ids[0]
		}
	}
	if err := proj.UpdateThing(missile); err != nil {
		log.Fatal(err)
	}
}

// writeOutput compiles the client into dir.
func writeOutput(proj *project.Project, version client.Version, dir string) {
	datPath := filepath.Join(dir, "Tibia.dat")
	sprPath := filepath.Join(dir, "Tibia.spr")
	err := proj.Compile(project.CompileOptions{DatPath: datPath, SprPath: sprPath, Version: version, Features: proj.Info().Features, WriteOTFI: true})
	if err != nil {
		log.Fatal(err)
	}
	info := proj.Info()
	log.Printf("wrote %s (%d items, %d sprites)", dir, info.Counts.Items-(firstItemID-1), info.Counts.Sprites)
}

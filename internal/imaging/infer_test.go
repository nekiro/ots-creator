package imaging

import (
	"image"
	"image/color"
	"testing"

	"github.com/nekiro/ots-creator/internal/thing"
)

// sheet fills cols x rows textures of tex px; fill(col, row) returns the
// color and how many pixel rows of the texture are painted (0 = empty).
func sheet(cols, rows, tex int, fill func(col, row int) (color.NRGBA, int)) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, cols*tex, rows*tex))
	for r := range rows {
		for c := range cols {
			col, n := fill(c, r)
			for y := range n {
				for x := range tex {
					img.SetNRGBA(c*tex+x, r*tex+y, col)
				}
			}
		}
	}
	return img
}

var body = color.NRGBA{120, 60, 30, 255}

func full(int, int) (color.NRGBA, int) { return body, 60 }

func layoutOf(g *thing.FrameGroup) [7]int {
	return [7]int{int(g.Width), int(g.Height), int(g.Layers), int(g.PatternX), int(g.PatternY), int(g.PatternZ), int(g.Frames)}
}

func TestInferOutfit(t *testing.T) {
	blank := thing.New(1, thing.CategoryOutfit)
	mask := func(c, _ int) (color.NRGBA, int) {
		if c%2 == 1 {
			return color.NRGBA{255, 255, 0, 255}, 40 // yellow template
		}
		return body, 60
	}
	addons := func(_, r int) (color.NRGBA, int) {
		if r%3 == 0 {
			return body, 60
		}
		return body, 12 // small overlay
	}
	cases := []struct {
		name string
		img  *image.NRGBA
		want [7]int
	}{
		{"4x3 of 64 px: old outfit, three frames", sheet(4, 3, 64, full), [7]int{2, 2, 1, 4, 1, 1, 3}},
		{"4x9 of 64 px: idle + eight walking frames", sheet(4, 9, 64, full), [7]int{2, 2, 1, 4, 1, 1, 9}},
		{"32 px textures", sheet(4, 3, 32, full), [7]int{1, 1, 1, 4, 1, 1, 3}},
		{"mask layer", sheet(8, 3, 64, mask), [7]int{2, 2, 2, 4, 1, 1, 3}},
		{"mounted patterns", sheet(8, 3, 64, full), [7]int{2, 2, 1, 4, 1, 2, 3}},
		{"addons", sheet(4, 9, 64, addons), [7]int{2, 2, 1, 4, 3, 1, 3}},
	}
	for _, tc := range cases {
		g, err := InferLayout(tc.img, thing.CategoryOutfit, blank, 32)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := layoutOf(g); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
		if w, h := g.SheetSize(32); w != tc.img.Rect.Dx() || h != tc.img.Rect.Dy() {
			t.Errorf("%s: layout sheet is %dx%d", tc.name, w, h)
		}
	}
}

func TestInferKeepsCurrentLayout(t *testing.T) {
	// A 1x1 outfit with idle (2 frames) and walking (4 frames) groups: its
	// own exported sheet keeps that layout.
	cur := thing.New(1, thing.CategoryOutfit)
	idle := cur.FrameGroups[0]
	idle.Resize(1, 1, 1, 4, 1, 1, 2, 300)
	idle.Sprites[0] = 7
	walk := idle.Clone()
	walk.Type = thing.FrameGroupWalking
	walk.Resize(1, 1, 1, 4, 1, 1, 4, 300)
	cur.FrameGroups = append(cur.FrameGroups, walk)
	g, err := InferLayout(sheet(4, 6, 32, full), thing.CategoryOutfit, cur, 32)
	if err != nil {
		t.Fatal(err)
	}
	if got := layoutOf(g); got != [7]int{1, 1, 1, 4, 1, 1, 6} {
		t.Fatalf("got %v", got)
	}
}

func TestInferPlain(t *testing.T) {
	item := thing.New(100, thing.CategoryItem)
	g, err := InferLayout(sheet(4, 2, 32, full), thing.CategoryItem, item, 32)
	if err != nil {
		t.Fatal(err)
	}
	if got := layoutOf(g); got != [7]int{1, 1, 1, 4, 1, 1, 2} {
		t.Fatalf("item got %v", got)
	}
	g, err = InferLayout(sheet(3, 3, 32, full), thing.CategoryMissile, thing.New(1, thing.CategoryMissile), 32)
	if err != nil {
		t.Fatal(err)
	}
	if got := layoutOf(g); got != [7]int{1, 1, 1, 3, 3, 1, 1} {
		t.Fatalf("missile got %v", got)
	}
	if _, err := InferLayout(image.NewNRGBA(image.Rect(0, 0, 40, 32)), thing.CategoryItem, item, 32); err == nil {
		t.Fatal("40 px wide sheet must fail")
	}
}

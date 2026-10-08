package thing

import "testing"

func TestCategoryRoundTrip(t *testing.T) {
	for _, c := range Categories {
		got, err := ParseCategory(c.String())
		if err != nil || got != c {
			t.Fatalf("ParseCategory(%q) = %v, %v", c.String(), got, err)
		}
	}
	if _, err := ParseCategory("bogus"); err == nil {
		t.Fatal("expected error")
	}
	if CategoryItem.MinID() != 100 || CategoryOutfit.MinID() != 1 {
		t.Fatal("wrong MinID")
	}
}

func layout() *FrameGroup {
	g := &FrameGroup{Width: 2, Height: 2, Layers: 2, PatternX: 4, PatternY: 3, PatternZ: 2, Frames: 3}
	g.Sprites = make([]uint32, g.TotalSprites())
	return g
}

func TestTotals(t *testing.T) {
	g := layout()
	if g.TotalSprites() != 2*2*2*4*3*2*3 {
		t.Fatalf("TotalSprites = %d", g.TotalSprites())
	}
	if g.TotalTextures() != 2*4*3*2*3 {
		t.Fatalf("TotalTextures = %d", g.TotalTextures())
	}
	w, h := g.SheetSize(32)
	if w != 2*4*2*2*32 || h != 3*3*2*32 {
		t.Fatalf("SheetSize = %dx%d", w, h)
	}
}

// SpriteIndex must enumerate every slot exactly once in the client order:
// width fastest, then height, layer, patternX, patternY, patternZ, frame.
func TestSpriteIndexIsBijective(t *testing.T) {
	g := layout()
	next := 0
	for f := 0; f < int(g.Frames); f++ {
		for z := 0; z < int(g.PatternZ); z++ {
			for y := 0; y < int(g.PatternY); y++ {
				for x := 0; x < int(g.PatternX); x++ {
					for l := 0; l < int(g.Layers); l++ {
						for h := 0; h < int(g.Height); h++ {
							for w := 0; w < int(g.Width); w++ {
								if got := g.SpriteIndex(w, h, l, x, y, z, f); got != next {
									t.Fatalf("SpriteIndex(%d,%d,%d,%d,%d,%d,%d) = %d, want %d", w, h, l, x, y, z, f, got, next)
								}
								next++
							}
						}
					}
				}
			}
		}
	}
	if next != g.TotalSprites() {
		t.Fatal("did not cover all sprites")
	}
}

func TestTextureIndexMatchesSpriteIndex(t *testing.T) {
	g := layout()
	area := int(g.Width) * int(g.Height)
	for f := 0; f < int(g.Frames); f++ {
		for x := 0; x < int(g.PatternX); x++ {
			for l := 0; l < int(g.Layers); l++ {
				ti := g.TextureIndex(l, x, 1, 1, f)
				si := g.SpriteIndex(0, 0, l, x, 1, 1, f)
				if ti*area != si {
					t.Fatalf("texture %d * %d != sprite %d", ti, area, si)
				}
			}
		}
	}
}

func TestResizeKeepsSprites(t *testing.T) {
	g := layout()
	for i := range g.Sprites {
		g.Sprites[i] = uint32(i + 1)
	}
	want := g.Sprites[g.SpriteIndex(1, 1, 1, 2, 1, 0, 1)]
	g.Resize(2, 2, 2, 4, 2, 1, 2, 100)
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := g.Sprites[g.SpriteIndex(1, 1, 1, 2, 1, 0, 1)]; got != want {
		t.Fatalf("sprite moved: got %d want %d", got, want)
	}
	if len(g.Durations) != 2 {
		t.Fatalf("durations = %d", len(g.Durations))
	}
	g.Resize(1, 1, 1, 1, 1, 1, 1, 100)
	if g.Durations != nil || len(g.Sprites) != 1 || g.Sprites[0] != 1 {
		t.Fatalf("shrink failed: %+v", g)
	}
}

func TestValidate(t *testing.T) {
	th := New(100, CategoryItem)
	if err := th.Validate(); err != nil {
		t.Fatal(err)
	}
	th.FrameGroups[0].Frames = 2
	th.FrameGroups[0].Sprites = make([]uint32, 2)
	if th.Validate() == nil {
		t.Fatal("missing durations should fail")
	}
	th.FrameGroups[0].EnsureDurations(500)
	if err := th.Validate(); err != nil {
		t.Fatal(err)
	}
	th.FrameGroups = append(th.FrameGroups, NewFrameGroup())
	if th.Validate() == nil {
		t.Fatal("item with two groups should fail")
	}
	o := New(1, CategoryOutfit)
	if o.FrameGroups[0].PatternX != 4 || len(o.FrameGroups[0].Sprites) != 4 {
		t.Fatal("outfit should default to 4 directions")
	}
}

func TestCloneIsDeep(t *testing.T) {
	th := New(1, CategoryOutfit)
	c := th.Clone()
	c.FrameGroups[0].Sprites[0] = 99
	if th.FrameGroups[0].Sprites[0] == 99 {
		t.Fatal("clone shares sprites")
	}
}

func TestFlagByKey(t *testing.T) {
	p := Properties{Ground: true}
	if v, ok := p.FlagByKey("ground"); !v || !ok {
		t.Fatal("ground")
	}
	if v, ok := p.FlagByKey("container"); v || !ok {
		t.Fatal("container")
	}
	if _, ok := p.FlagByKey("groundSpeed"); ok {
		t.Fatal("groundSpeed is not a flag")
	}
	if _, ok := p.FlagByKey("nope"); ok {
		t.Fatal("unknown key")
	}
}

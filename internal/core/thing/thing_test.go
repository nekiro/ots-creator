package thing

import "testing"

func TestCategoryRoundTrip(t *testing.T) {
	for _, category := range Categories {
		got, err := ParseCategory(category.String())
		if err != nil || got != category {
			t.Fatalf("ParseCategory(%q) = %v, %v", category.String(), got, err)
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
	group := &FrameGroup{Width: 2, Height: 2, Layers: 2, PatternX: 4, PatternY: 3, PatternZ: 2, Frames: 3}
	group.Sprites = make([]uint32, group.TotalSprites())
	return group
}

func TestTotals(t *testing.T) {
	group := layout()
	if group.TotalSprites() != 2*2*2*4*3*2*3 {
		t.Fatalf("TotalSprites = %d", group.TotalSprites())
	}
	if group.TotalTextures() != 2*4*3*2*3 {
		t.Fatalf("TotalTextures = %d", group.TotalTextures())
	}
	tileX, tileY := group.SheetSize(32)
	if tileX != 2*4*2*2*32 || tileY != 3*3*2*32 {
		t.Fatalf("SheetSize = %dx%d", tileX, tileY)
	}
}

// SpriteIndex must enumerate every slot exactly once in the client order:
// width fastest, then height, layer, patternX, patternY, patternZ, frame.
func TestSpriteIndexIsBijective(t *testing.T) {
	group := layout()
	next := 0
	for frame := 0; frame < int(group.Frames); frame++ {
		for z := 0; z < int(group.PatternZ); z++ {
			for y := 0; y < int(group.PatternY); y++ {
				for x := 0; x < int(group.PatternX); x++ {
					for layer := 0; layer < int(group.Layers); layer++ {
						for tileY := 0; tileY < int(group.Height); tileY++ {
							for tileX := 0; tileX < int(group.Width); tileX++ {
								if got := group.SpriteIndex(tileX, tileY, layer, x, y, z, frame); got != next {
									t.Fatalf("SpriteIndex(%d,%d,%d,%d,%d,%d,%d) = %d, want %d", tileX, tileY, layer, x, y, z, frame, got, next)
								}
								next++
							}
						}
					}
				}
			}
		}
	}
	if next != group.TotalSprites() {
		t.Fatal("did not cover all sprites")
	}
}

func TestTextureIndexMatchesSpriteIndex(t *testing.T) {
	group := layout()
	area := int(group.Width) * int(group.Height)
	for frame := 0; frame < int(group.Frames); frame++ {
		for x := 0; x < int(group.PatternX); x++ {
			for layer := 0; layer < int(group.Layers); layer++ {
				textureIndex := group.TextureIndex(layer, x, 1, 1, frame)
				spriteIndex := group.SpriteIndex(0, 0, layer, x, 1, 1, frame)
				if textureIndex*area != spriteIndex {
					t.Fatalf("texture %d * %d != sprite %d", textureIndex, area, spriteIndex)
				}
			}
		}
	}
}

func TestResizeKeepsSprites(t *testing.T) {
	group := layout()
	for i := range group.Sprites {
		group.Sprites[i] = uint32(i + 1)
	}
	want := group.Sprites[group.SpriteIndex(1, 1, 1, 2, 1, 0, 1)]
	group.Resize(2, 2, 2, 4, 2, 1, 2, 100)
	if err := group.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := group.Sprites[group.SpriteIndex(1, 1, 1, 2, 1, 0, 1)]; got != want {
		t.Fatalf("sprite moved: got %d want %d", got, want)
	}
	if len(group.Durations) != 2 {
		t.Fatalf("durations = %d", len(group.Durations))
	}
	group.Resize(1, 1, 1, 1, 1, 1, 1, 100)
	if group.Durations != nil || len(group.Sprites) != 1 || group.Sprites[0] != 1 {
		t.Fatalf("shrink failed: %+v", group)
	}
}

func TestValidate(t *testing.T) {
	object := New(100, CategoryItem)
	if err := object.Validate(); err != nil {
		t.Fatal(err)
	}
	object.FrameGroups[0].Frames = 2
	object.FrameGroups[0].Sprites = make([]uint32, 2)
	if object.Validate() == nil {
		t.Fatal("missing durations should fail")
	}
	object.FrameGroups[0].EnsureDurations(500)
	if err := object.Validate(); err != nil {
		t.Fatal(err)
	}
	object.FrameGroups = append(object.FrameGroups, NewFrameGroup())
	if object.Validate() == nil {
		t.Fatal("item with two groups should fail")
	}
	outfit := New(1, CategoryOutfit)
	if outfit.FrameGroups[0].PatternX != 4 || len(outfit.FrameGroups[0].Sprites) != 4 {
		t.Fatal("outfit should default to 4 directions")
	}
}

func TestCloneIsDeep(t *testing.T) {
	object := New(1, CategoryOutfit)
	clone := object.Clone()
	clone.FrameGroups[0].Sprites[0] = 99
	if object.FrameGroups[0].Sprites[0] == 99 {
		t.Fatal("clone shares sprites")
	}
}

func TestFlagByKey(t *testing.T) {
	props := Properties{Ground: true}
	if value, ok := props.FlagByKey("ground"); !value || !ok {
		t.Fatal("ground")
	}
	if value, ok := props.FlagByKey("container"); value || !ok {
		t.Fatal("container")
	}
	if _, ok := props.FlagByKey("groundSpeed"); ok {
		t.Fatal("groundSpeed is not a flag")
	}
	if _, ok := props.FlagByKey("nope"); ok {
		t.Fatal("unknown key")
	}
}

func TestCategoryValidAndDefaultDuration(t *testing.T) {
	durations := map[Category]uint32{CategoryItem: 500, CategoryOutfit: 300, CategoryEffect: 100, CategoryMissile: 75, CategoryInvalid: 100}
	for category, want := range durations {
		if got := category.DefaultDuration(); got != want {
			t.Fatalf("%s DefaultDuration = %d, want %d", category, got, want)
		}
		if category.Valid() != (category != CategoryInvalid) {
			t.Fatalf("%s Valid = %v", category, category.Valid())
		}
	}
	if Category(5).Valid() || Category(5).String() != "invalid" {
		t.Fatal("category 5 should be invalid")
	}
}

func TestAssetOnly(t *testing.T) {
	props := Properties{Ground: true}
	if flags := props.AssetOnly(); flags != nil {
		t.Fatalf("AssetOnly = %v, want none", flags)
	}
	props.Corpse, props.WrapKit = true, true
	flags := props.AssetOnly()
	if len(flags) != 2 || flags[0] != "corpse" || flags[1] != "wrapKit" {
		t.Fatalf("AssetOnly = %v", flags)
	}
}

func TestGroupAndSpriteIDs(t *testing.T) {
	outfit := New(1, CategoryOutfit)
	walking := NewFrameGroup()
	walking.Type = FrameGroupWalking
	walking.Sprites[0] = 7
	outfit.FrameGroups = append(outfit.FrameGroups, walking)
	if outfit.Group(FrameGroupWalking) != walking || outfit.Group(FrameGroupDefault) != outfit.FrameGroups[0] {
		t.Fatal("Group returned the wrong group")
	}
	item := New(100, CategoryItem)
	if item.Group(FrameGroupWalking) != item.FrameGroups[0] {
		t.Fatal("Group should fall back to the first group")
	}
	if (&Thing{}).Group(FrameGroupDefault) != nil {
		t.Fatal("Group of a thing without groups should be nil")
	}
	ids := outfit.SpriteIDs()
	if len(ids) != 5 || ids[4] != 7 {
		t.Fatalf("SpriteIDs = %v", ids)
	}
}

func TestStacked(t *testing.T) {
	if _, ok := (&Thing{}).Stacked(); ok {
		t.Fatal("thing without groups should not stack")
	}
	item := New(100, CategoryItem)
	single, ok := item.Stacked()
	if !ok || single == item.FrameGroups[0] || single.Frames != 1 {
		t.Fatal("single group should return a copy")
	}

	outfit := New(1, CategoryOutfit)
	idle := outfit.FrameGroups[0]
	for i := range idle.Sprites {
		idle.Sprites[i] = uint32(i + 1)
	}
	walking := idle.Clone()
	walking.Resize(1, 1, 1, 4, 1, 1, 2, 300)
	walking.Durations[0] = FrameDuration{Min: 50, Max: 60}
	outfit.FrameGroups = append(outfit.FrameGroups, walking)
	stacked, ok := outfit.Stacked()
	if !ok || stacked.Frames != 3 || len(stacked.Sprites) != 12 {
		t.Fatalf("stacked = %+v, ok %v", stacked, ok)
	}
	// idle had no durations, so all three frames get the default.
	if len(stacked.Durations) != 3 || stacked.Durations[0].Min != 300 {
		t.Fatalf("durations = %v", stacked.Durations)
	}
	if err := stacked.Validate(); err != nil {
		t.Fatal(err)
	}

	idle.Resize(1, 1, 1, 4, 1, 1, 2, 300)
	stacked, _ = outfit.Stacked()
	if len(stacked.Durations) != 4 || stacked.Durations[2].Min != 50 {
		t.Fatalf("matching durations should be kept: %v", stacked.Durations)
	}

	walking.Resize(2, 1, 1, 4, 1, 1, 2, 300)
	if _, ok := outfit.Stacked(); ok {
		t.Fatal("different layouts should not stack")
	}
}

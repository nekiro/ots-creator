package imaging

import (
	"fmt"
	"image"

	"github.com/nekiro/ots-creator/internal/thing"
)

// InferLayout guesses the frame group a sprite sheet was exported from, so
// a sheet can be dropped on any object. The result is one group with every
// frame (outfit idle and walking frames together, as in ObjectBuilder
// sheets); its sprites are empty.
//
// The sheet layout is the one of Sheet: columns are pattern Z x pattern X x
// layers (layers innermost), rows are frames x pattern Y. cur is the object
// the sheet goes into; its texture size and patterns win when they fit.
func InferLayout(img *image.NRGBA, c thing.Category, cur *thing.Thing, size int) (*thing.FrameGroup, error) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w == 0 || h == 0 || w%size != 0 || h%size != 0 {
		return nil, fmt.Errorf("sheet is %dx%d, not a multiple of %d px", w, h, size)
	}
	var curGroup *thing.FrameGroup
	if cur != nil && len(cur.FrameGroups) > 0 {
		curGroup = cur.FrameGroups[0]
	}
	// The current layout, when the sheet has exactly its size (the only
	// layout ObjectBuilder accepts).
	if curGroup != nil {
		merged := mergedLayout(cur)
		if mw, mh := merged.SheetSize(size); mw == w && mh == h {
			return merged, nil
		}
	}
	var g *thing.FrameGroup
	if c == thing.CategoryOutfit {
		g = inferOutfit(img, cur, size)
	} else {
		g = inferPlain(img, c, curGroup, hasSprites(cur), size)
	}
	if g == nil {
		return nil, fmt.Errorf("cannot find a layout for a %dx%d sheet", w, h)
	}
	g.ExactSize = uint8(min(int(max(g.Width, g.Height))*size, 255))
	g.Sprites = make([]uint32, g.TotalSprites())
	if err := checkLimits(g); err != nil {
		return nil, err
	}
	return g, nil
}

func checkLimits(g *thing.FrameGroup) error {
	if g.TotalSprites() > 1<<16 {
		return fmt.Errorf("sheet layout needs %d sprites", g.TotalSprites())
	}
	return nil
}

func hasSprites(t *thing.Thing) bool {
	if t == nil {
		return false
	}
	for _, g := range t.FrameGroups {
		for _, id := range g.Sprites {
			if id != 0 {
				return true
			}
		}
	}
	return false
}

// mergedLayout is the layout of a thing's sheet with every frame group
// stacked: idle frames then walking frames, when the groups share a layout.
func mergedLayout(t *thing.Thing) *thing.FrameGroup {
	g, ok := t.Stacked()
	if !ok {
		g = t.FrameGroups[0].Clone()
	}
	g.Durations = nil
	g.Sprites = make([]uint32, g.TotalSprites())
	return g
}

// inferOutfit: four directions per row; eight or sixteen columns are a
// color mask layer (yellow/red/green/blue template) or mounted patterns.
func inferOutfit(img *image.NRGBA, cur *thing.Thing, size int) *thing.FrameGroup {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	curTex := 0
	if hasSprites(cur) {
		if g := cur.FrameGroups[0]; g.Width == g.Height {
			curTex = int(g.Width) * size
		}
	}
	type option struct{ cols, tex int }
	var options []option
	for _, cols := range []int{4, 8, 16} {
		if w%cols != 0 {
			continue
		}
		tex := w / cols
		if tex%size == 0 && h%tex == 0 && tex/size <= 8 {
			options = append(options, option{cols, tex})
		}
	}
	if len(options) == 0 {
		return nil
	}
	// The current texture size, else 64 px (2x2, every outfit since 7.x
	// that is not a plain 32 px one), else the first that fits.
	pick := options[0]
	for _, want := range []int{curTex, 2 * size} {
		found := false
		for _, o := range options {
			if want != 0 && o.tex == want {
				pick, found = o, true
				break
			}
		}
		if found {
			break
		}
	}
	tiles := uint8(pick.tex / size)
	g := &thing.FrameGroup{Width: tiles, Height: tiles, Layers: 1, PatternX: 4, PatternY: 1, PatternZ: 1}
	switch pick.cols {
	case 8:
		if isMaskColumns(img, pick.tex, 8) {
			g.Layers = 2
		} else {
			g.PatternZ = 2 // mounted
		}
	case 16:
		g.Layers, g.PatternZ = 2, 2
	}
	rows := h / pick.tex
	if rows%3 == 0 && looksLikeAddons(img, pick.tex, rows) {
		g.PatternY = 3
	}
	frames := rows / int(g.PatternY)
	if frames > 255 {
		return nil
	}
	g.Frames = uint8(frames)
	return g
}

// isMaskColumns reports whether every odd column holds only the outfit
// template colors, i.e. the sheet has a second (mask) layer.
func isMaskColumns(img *image.NRGBA, tex, cols int) bool {
	opaque, template := 0, 0
	b := img.Rect
	for col := 1; col < cols; col += 2 {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X + col*tex; x < b.Min.X+(col+1)*tex; x++ {
				i := img.PixOffset(x, y)
				r, g, bl, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
				if a == 0 || (r == 0xFF && g == 0 && bl == 0xFF) {
					continue
				}
				opaque++
				hi := func(v uint8) bool { return v >= 0xE0 }
				lo := func(v uint8) bool { return v <= 0x20 }
				switch {
				case hi(r) && hi(g) && lo(bl), hi(r) && lo(g) && lo(bl), lo(r) && hi(g) && lo(bl), lo(r) && lo(g) && hi(bl):
					template++
				}
			}
		}
	}
	return opaque > 0 && template*10 >= opaque*9
}

// looksLikeAddons reports whether rows come in groups of three where the
// second and third rows are small overlays (addon 1 and 2) of the first.
func looksLikeAddons(img *image.NRGBA, tex, rows int) bool {
	counts := make([]int, rows)
	b := img.Rect
	for r := range rows {
		for y := b.Min.Y + r*tex; y < b.Min.Y+(r+1)*tex; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				i := img.PixOffset(x, y)
				if img.Pix[i+3] != 0 && !(img.Pix[i] == 0xFF && img.Pix[i+1] == 0 && img.Pix[i+2] == 0xFF) {
					counts[r]++
				}
			}
		}
	}
	for f := 0; f < rows; f += 3 {
		base := counts[f]
		if base == 0 || counts[f+1]*10 > base*7 || counts[f+2]*10 > base*7 {
			return false
		}
	}
	return true
}

// inferPlain handles items, effects and missiles: the current texture
// size when it fits, columns are pattern X and rows are frames. A 3x3
// missile sheet holds the eight directions.
func inferPlain(img *image.NRGBA, c thing.Category, cur *thing.FrameGroup, keep bool, size int) *thing.FrameGroup {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	tw, th := 1, 1
	if keep && cur != nil && w%(int(cur.Width)*size) == 0 && h%(int(cur.Height)*size) == 0 {
		tw, th = int(cur.Width), int(cur.Height)
	}
	cols, rows := w/(tw*size), h/(th*size)
	g := &thing.FrameGroup{Width: uint8(tw), Height: uint8(th), Layers: 1, PatternY: 1, PatternZ: 1}
	switch {
	case c == thing.CategoryMissile && cols == 3 && rows == 3:
		g.PatternX, g.PatternY, g.Frames = 3, 3, 1
	case keep && cur != nil && cols == int(cur.PatternZ)*int(cur.PatternX)*int(cur.Layers) && rows%int(cur.PatternY) == 0:
		g.PatternX, g.PatternY, g.PatternZ, g.Layers = cur.PatternX, cur.PatternY, cur.PatternZ, cur.Layers
		g.Frames = uint8(min(rows/int(cur.PatternY), 255))
	default:
		if cols > 255 || rows > 255 {
			return nil
		}
		g.PatternX, g.Frames = uint8(cols), uint8(rows)
	}
	return g
}

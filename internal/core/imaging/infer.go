package imaging

import (
	"fmt"
	"image"

	"github.com/nekiro/ots-creator/internal/core/thing"
)

// InferLayout guesses the frame group a sprite sheet was exported from, so
// a sheet can be dropped on any object. The result is one group with every
// frame (outfit idle and walking frames together, as in ObjectBuilder
// sheets); its sprites are empty.
//
// The sheet layout is the one of Sheet: columns are pattern Z x pattern X x
// layers (layers innermost), rows are frames x pattern Y. current is the object
// the sheet goes into; its texture size and patterns win when they fit.
func InferLayout(img *image.NRGBA, category thing.Category, current *thing.Thing, size int) (*thing.FrameGroup, error) {
	width, height := img.Rect.Dx(), img.Rect.Dy()
	if width == 0 || height == 0 || width%size != 0 || height%size != 0 {
		return nil, fmt.Errorf("sheet is %dx%d, not a multiple of %d px", width, height, size)
	}
	var currentGroup *thing.FrameGroup
	if current != nil && len(current.FrameGroups) > 0 {
		currentGroup = current.FrameGroups[0]
	}
	// The current layout, when the sheet has exactly its size (the only
	// layout ObjectBuilder accepts).
	if currentGroup != nil {
		merged := mergedLayout(current)
		if mergedWidth, mergedHeight := merged.SheetSize(size); mergedWidth == width && mergedHeight == height {
			return merged, nil
		}
	}
	var group *thing.FrameGroup
	if category == thing.CategoryOutfit {
		group = inferOutfit(img, current, size)
	} else {
		group = inferPlain(img, category, currentGroup, hasSprites(current), size)
	}
	if group == nil {
		return nil, fmt.Errorf("cannot find a layout for a %dx%d sheet", width, height)
	}
	group.ExactSize = uint8(min(int(max(group.Width, group.Height))*size, 255))
	group.Sprites = make([]uint32, group.TotalSprites())
	if err := checkLimits(group); err != nil {
		return nil, err
	}
	return group, nil
}

func checkLimits(group *thing.FrameGroup) error {
	if group.TotalSprites() > 1<<16 {
		return fmt.Errorf("sheet layout needs %d sprites", group.TotalSprites())
	}
	return nil
}

func hasSprites(object *thing.Thing) bool {
	if object == nil {
		return false
	}
	for _, group := range object.FrameGroups {
		for _, id := range group.Sprites {
			if id != 0 {
				return true
			}
		}
	}
	return false
}

// mergedLayout is the layout of a thing's sheet with every frame group
// stacked: idle frames then walking frames, when the groups share a layout.
func mergedLayout(object *thing.Thing) *thing.FrameGroup {
	group, ok := object.Stacked()
	if !ok {
		group = object.FrameGroups[0].Clone()
	}
	group.Durations = nil
	group.Sprites = make([]uint32, group.TotalSprites())
	return group
}

// inferOutfit: four directions per row; eight or sixteen columns are a
// color mask layer (yellow/red/green/blue template) or mounted patterns.
func inferOutfit(img *image.NRGBA, current *thing.Thing, size int) *thing.FrameGroup {
	width, height := img.Rect.Dx(), img.Rect.Dy()
	currentTexture := 0
	if hasSprites(current) {
		if group := current.FrameGroups[0]; group.Width == group.Height {
			currentTexture = int(group.Width) * size
		}
	}
	type layoutOption struct{ cols, texture int }
	var options []layoutOption
	for _, cols := range []int{4, 8, 16} {
		if width%cols != 0 {
			continue
		}
		texture := width / cols
		if texture%size == 0 && height%texture == 0 && texture/size <= 8 {
			options = append(options, layoutOption{cols, texture})
		}
	}
	if len(options) == 0 {
		return nil
	}
	// The current texture size, else 64 px (2x2, every outfit since 7.x
	// that is not a plain 32 px one), else the first that fits.
	pick := options[0]
	for _, want := range []int{currentTexture, 2 * size} {
		found := false
		for _, candidate := range options {
			if want != 0 && candidate.texture == want {
				pick, found = candidate, true
				break
			}
		}
		if found {
			break
		}
	}
	tiles := uint8(pick.texture / size)
	group := &thing.FrameGroup{Width: tiles, Height: tiles, Layers: 1, PatternX: 4, PatternY: 1, PatternZ: 1}
	switch pick.cols {
	case 8:
		if isMaskColumns(img, pick.texture, 8) {
			group.Layers = 2
		} else {
			group.PatternZ = 2 // mounted
		}
	case 16:
		group.Layers, group.PatternZ = 2, 2
	}
	rows := height / pick.texture
	if rows%3 == 0 && looksLikeAddons(img, pick.texture, rows) {
		group.PatternY = 3
	}
	frames := rows / int(group.PatternY)
	if frames > 255 {
		return nil
	}
	group.Frames = uint8(frames)
	return group
}

// isMaskColumns reports whether every odd column holds only the outfit
// template colors, i.e. the sheet has a second (mask) layer.
func isMaskColumns(img *image.NRGBA, texture, cols int) bool {
	opaque, template := 0, 0
	bounds := img.Rect
	for col := 1; col < cols; col += 2 {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X + col*texture; x < bounds.Min.X+(col+1)*texture; x++ {
				i := img.PixOffset(x, y)
				red, green, blue, alpha := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
				if alpha == 0 || (red == 0xFF && green == 0 && blue == 0xFF) {
					continue
				}
				opaque++
				high := func(value uint8) bool { return value >= 0xE0 }
				low := func(value uint8) bool { return value <= 0x20 }
				switch {
				case high(red) && high(green) && low(blue), high(red) && low(green) && low(blue), low(red) && high(green) && low(blue), low(red) && low(green) && high(blue):
					template++
				}
			}
		}
	}
	return opaque > 0 && template*10 >= opaque*9
}

// looksLikeAddons reports whether rows come in groups of three where the
// second and third rows are small overlays (addon 1 and 2) of the first.
func looksLikeAddons(img *image.NRGBA, texture, rows int) bool {
	counts := make([]int, rows)
	bounds := img.Rect
	for row := range rows {
		for y := bounds.Min.Y + row*texture; y < bounds.Min.Y+(row+1)*texture; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				i := img.PixOffset(x, y)
				if img.Pix[i+3] != 0 && !(img.Pix[i] == 0xFF && img.Pix[i+1] == 0 && img.Pix[i+2] == 0xFF) {
					counts[row]++
				}
			}
		}
	}
	for first := 0; first < rows; first += 3 {
		base := counts[first]
		if base == 0 || counts[first+1]*10 > base*7 || counts[first+2]*10 > base*7 {
			return false
		}
	}
	return true
}

// inferPlain handles items, effects and missiles: the current texture
// size when it fits, columns are pattern X and rows are frames. A 3x3
// missile sheet holds the eight directions.
func inferPlain(img *image.NRGBA, category thing.Category, current *thing.FrameGroup, keep bool, size int) *thing.FrameGroup {
	width, height := img.Rect.Dx(), img.Rect.Dy()
	tilesWide, tilesHigh := 1, 1
	if keep && current != nil && width%(int(current.Width)*size) == 0 && height%(int(current.Height)*size) == 0 {
		tilesWide, tilesHigh = int(current.Width), int(current.Height)
	}
	cols, rows := width/(tilesWide*size), height/(tilesHigh*size)
	group := &thing.FrameGroup{Width: uint8(tilesWide), Height: uint8(tilesHigh), Layers: 1, PatternY: 1, PatternZ: 1}
	switch {
	case category == thing.CategoryMissile && cols == 3 && rows == 3:
		group.PatternX, group.PatternY, group.Frames = 3, 3, 1
	case keep && current != nil && cols == int(current.PatternZ)*int(current.PatternX)*int(current.Layers) && rows%int(current.PatternY) == 0:
		group.PatternX, group.PatternY, group.PatternZ, group.Layers = current.PatternX, current.PatternY, current.PatternZ, current.Layers
		group.Frames = uint8(min(rows/int(current.PatternY), 255))
	default:
		if cols > 255 || rows > 255 {
			return nil
		}
		group.PatternX, group.Frames = uint8(cols), uint8(rows)
	}
	return group
}

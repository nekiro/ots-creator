package app

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// AllGroups selects every frame group of a thing in one sheet (idle frames
// then walking frames).
const AllGroups = -1

// ExportSheet writes the sprite sheet of one frame group, or of the whole
// thing with AllGroups. The format follows the file extension (png, bmp or
// jpg); formats without alpha get a magenta background.
func (ts *ThingService) ExportSheet(category thing.Category, id uint32, group int, path string, transparent bool) error {
	p, err := ts.session.Project()
	if err != nil {
		return err
	}
	img, err := renderSheet(p, category, id, group, transparent)
	if err != nil {
		return err
	}
	data, err := imaging.Encode(img, imaging.FormatOf(path), imaging.Magenta)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, fsutil.FileMode)
}

// renderSheet draws the sprite sheet of one frame group (or AllGroups) on
// magenta (like ObjectBuilder) or a transparent background.
func renderSheet(p *project.Project, category thing.Category, id uint32, group int, transparent bool) (*image.NRGBA, error) {
	found, err := p.Thing(category, id)
	if err != nil {
		return nil, err
	}
	var frameGroup *thing.FrameGroup
	switch {
	case group == AllGroups:
		var ok bool
		if frameGroup, ok = found.Stacked(); !ok {
			return nil, errors.New("idle and walking groups have different layouts; export them one by one")
		}
	case group < 0 || group >= len(found.FrameGroups):
		return nil, fmt.Errorf("no frame group %d", group)
	default:
		frameGroup = found.FrameGroups[group]
	}
	background := imaging.Magenta
	if transparent {
		background = color.NRGBA{}
	}
	var firstErr error
	img := imaging.Sheet(frameGroup, p.SpriteSize(), background, func(slot int) []byte {
		spriteID := frameGroup.Sprites[slot]
		if spriteID == 0 {
			return nil
		}
		pixels, err := p.SpritePixels(spriteID)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return pixels
	})
	return img, firstErr
}

// ImportSheet loads a sheet image into a thing. A sheet with the size of
// the given frame group replaces that group's sprites; any other sheet
// sets the layout from the image (see project.ImportSheet).
func (ts *ThingService) ImportSheet(category thing.Category, id uint32, group int, path string) error {
	img, err := imaging.Load(path)
	if err != nil {
		return err
	}
	return ts.importSheet(category, id, group, img)
}

// PasteSheet is ImportSheet for an image from the clipboard (PNG, BMP,
// GIF or JPEG bytes).
func (ts *ThingService) PasteSheet(category thing.Category, id uint32, group int, data []byte) error {
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("clipboard image: %w", err)
	}
	return ts.importSheet(category, id, group, img)
}

func (ts *ThingService) importSheet(category thing.Category, id uint32, group int, img *image.NRGBA) error {
	return ts.editAll(func(p *project.Project) error {
		found, err := p.Thing(category, id)
		if err != nil {
			return err
		}
		if group < 0 || group >= len(found.FrameGroups) {
			return fmt.Errorf("no frame group %d", group)
		}
		frameGroup := found.FrameGroups[group]
		width, height := frameGroup.SheetSize(p.SpriteSize())
		if width != img.Rect.Dx() || height != img.Rect.Dy() {
			return p.ImportSheet(category, id, img)
		}
		pixels, err := imaging.SliceSheet(img, frameGroup, p.SpriteSize())
		if err != nil {
			return err
		}
		return p.SetGroupPixels(category, id, group, pixels)
	})
}

// SetPixels replaces the pixels of sprite slots of one frame group after
// painting (see project.SetSlotPixels).
func (ts *ThingService) SetPixels(category thing.Category, id uint32, group int, slots []int, pixels [][]byte) error {
	return ts.editAll(func(p *project.Project) error {
		return p.SetSlotPixels(category, id, group, slots, pixels, "Paint")
	})
}

// ShiftPixels moves the pixels of every texture of a frame group, or with
// one set only the texture at frame and pattern x, y, z (all its layers).
func (ts *ThingService) ShiftPixels(category thing.Category, id uint32, group, dx, dy int, one bool, frame, x, y, z int) error {
	return ts.editAll(func(p *project.Project) error {
		texture := project.TexturePos{Frame: frame, PatternX: x, PatternY: y, PatternZ: z}
		return p.ShiftPixels(category, id, group, dx, dy, !one, texture)
	})
}

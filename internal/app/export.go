package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// maxSheetSprites is the most sprites ExportAllSheets puts in one sheet.
const maxSheetSprites = 64 * 64

// ExportSummary reports an export of a whole client.
type ExportSummary struct {
	Files int `json:"files"`
	// Skipped counts empty objects or sprites that were left out.
	Skipped int `json:"skipped"`
}

// ExportAll writes every object of every category into subfolders of dir
// (items, outfits, effects, missiles), named like ExportObjects, in format
// (ObjectOTOBJ or ObjectOBD). With skipEmpty objects without visible
// sprites are left out.
func (ts *ThingService) ExportAll(dir string, skipEmpty bool, format string) (ExportSummary, error) {
	p, err := ts.session.Project()
	if err != nil {
		return ExportSummary{}, err
	}
	content := project.ContentAll
	if skipEmpty {
		content = project.ContentUsed
	}
	type job struct {
		category thing.Category
		id       uint32
	}
	var jobs []job
	total := 0
	for _, category := range thing.Categories {
		ids, err := p.FindThings(category, project.Filter{Content: content})
		if err != nil {
			return ExportSummary{}, err
		}
		all, _ := p.FindThings(category, project.Filter{})
		total += len(all)
		if len(ids) == 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Join(dir, categoryFolder(category)), fsutil.DirMode); err != nil {
			return ExportSummary{}, err
		}
		for _, id := range ids {
			jobs = append(jobs, job{category, id})
		}
	}
	err = ts.session.parallel("Exporting objects", len(jobs), func(i int) error {
		next := jobs[i]
		_, err := exportObject(p, next.category, next.id, filepath.Join(dir, categoryFolder(next.category)), format)
		return err
	})
	return ExportSummary{Files: len(jobs), Skipped: total - len(jobs)}, err
}

func categoryFolder(category thing.Category) string { return category.String() + "s" }

// ExportAll writes every sprite as {id}.{format} into dir (see Export).
// With skipEmpty fully transparent sprites are left out.
func (ss *SpriteService) ExportAll(dir, format string, skipEmpty bool) (ExportSummary, error) {
	format = imaging.FormatOf("x." + format)
	p, err := ss.session.Project()
	if err != nil {
		return ExportSummary{}, err
	}
	spriteCount := int(p.Info().Counts.Sprites)
	size := p.SpriteSize()
	var skipped atomic.Int64
	err = ss.session.parallel("Exporting sprites", spriteCount, func(i int) error {
		id := uint32(i + 1)
		if skipEmpty {
			if empty, err := p.IsSpriteEmpty(id); err != nil || empty {
				skipped.Add(1)
				return err
			}
		}
		pixels, err := p.SpritePixels(id)
		if err != nil {
			return err
		}
		data, err := imaging.Encode(spriteImage(pixels, size), format, imaging.Magenta)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.%s", id, format)), data, fsutil.FileMode)
	})
	return ExportSummary{Files: spriteCount - int(skipped.Load()), Skipped: int(skipped.Load())}, err
}

// ExportAllSheets writes every sprite into sprite sheets of columns x rows
// sprites, named sprites_{first}-{last}.{format}. Sprites keep their order,
// left to right and top to bottom, so the position in a sheet gives the id;
// empty sprites stay as gaps. Formats without alpha get a magenta
// background, like ExportSheet; transparent keeps alpha in png.
func (ss *SpriteService) ExportAllSheets(dir, format string, columns, rows int, transparent bool) (ExportSummary, error) {
	format = imaging.FormatOf("x." + format)
	if columns < 1 || rows < 1 || columns*rows > maxSheetSprites {
		return ExportSummary{}, fmt.Errorf("a sheet holds 1 to %d sprites, not %d x %d", maxSheetSprites, columns, rows)
	}
	p, err := ss.session.Project()
	if err != nil {
		return ExportSummary{}, err
	}
	spriteCount := int(p.Info().Counts.Sprites)
	size := p.SpriteSize()
	perSheet := columns * rows
	sheets := (spriteCount + perSheet - 1) / perSheet
	background := imaging.Magenta
	if transparent && format == "png" {
		background = color.NRGBA{}
	}
	err = ss.session.parallel("Exporting sprite sheets", sheets, func(i int) error {
		first := i*perSheet + 1
		last := min(first+perSheet-1, spriteCount)
		// The last sheet only gets the rows it needs.
		usedRows := (last - first + columns) / columns
		img := image.NewNRGBA(image.Rect(0, 0, columns*size, usedRows*size))
		draw.Draw(img, img.Rect, &image.Uniform{C: background}, image.Point{}, draw.Src)
		for id := first; id <= last; id++ {
			pixels, err := p.SpritePixels(uint32(id))
			if err != nil {
				return err
			}
			slot := id - first
			imaging.DrawOver(img, (slot%columns)*size, (slot/columns)*size, size, pixels)
		}
		data, err := imaging.Encode(img, format, imaging.Magenta)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, fmt.Sprintf("sprites_%d-%d.%s", first, last, format)), data, fsutil.FileMode)
	})
	return ExportSummary{Files: sheets}, err
}

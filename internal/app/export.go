package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

// EventProgress carries a Progress while a long operation runs.
const EventProgress = "app:progress"

// Progress reports how far a long operation is.
type Progress struct {
	Label string `json:"label"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// ExportSummary reports an export of a whole client.
type ExportSummary struct {
	Files int `json:"files"`
	// Skipped counts empty objects or sprites that were left out.
	Skipped int `json:"skipped"`
}

// progressEvery is the minimum time between progress events.
const progressEvery = 150 * time.Millisecond

// parallel runs fn for 0..n-1 on all cores and reports progress. It stops
// at the first error and returns it.
func (s *Session) parallel(label string, n int, fn func(i int) error) error {
	var next, done atomic.Int64
	var failed atomic.Bool
	var firstErr error
	var errOnce sync.Once
	stop, stopped := make(chan struct{}), make(chan struct{})
	ticker := time.NewTicker(progressEvery)
	defer ticker.Stop()
	go func() {
		defer close(stopped)
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s.notify(EventProgress, Progress{Label: label, Done: int(done.Load()), Total: n})
			}
		}
	}()
	var wg sync.WaitGroup
	for range min(n, runtime.GOMAXPROCS(0)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !failed.Load() {
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				if err := fn(i); err != nil {
					errOnce.Do(func() { firstErr = err })
					failed.Store(true)
					return
				}
				done.Add(1)
			}
		}()
	}
	wg.Wait()
	close(stop)
	<-stopped // events are never sent concurrently
	s.notify(EventProgress, Progress{Label: label, Done: n, Total: n})
	return firstErr
}

// ExportAll writes every object of every category as OBD files into
// subfolders of dir (items, outfits, effects, missiles), named
// {category}_{id}.obd like ExportOBD. With skipEmpty objects without
// visible sprites are left out.
func (ts *ThingService) ExportAll(dir string, skipEmpty bool) (ExportSummary, error) {
	p, err := ts.s.Project()
	if err != nil {
		return ExportSummary{}, err
	}
	content := project.ContentAll
	if skipEmpty {
		content = project.ContentUsed
	}
	type job struct {
		c  thing.Category
		id uint32
	}
	var jobs []job
	total := 0
	for _, c := range thing.Categories {
		ids, err := p.FindThings(c, project.Filter{Content: content})
		if err != nil {
			return ExportSummary{}, err
		}
		all, _ := p.FindThings(c, project.Filter{})
		total += len(all)
		if len(ids) == 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Join(dir, categoryFolder(c)), 0o755); err != nil {
			return ExportSummary{}, err
		}
		for _, id := range ids {
			jobs = append(jobs, job{c, id})
		}
	}
	err = ts.s.parallel("Exporting objects", len(jobs), func(i int) error {
		j := jobs[i]
		d, err := p.ExportOBD(j.c, j.id, obd.Version3)
		if err != nil {
			return err
		}
		data, err := obd.Encode(d)
		if err != nil {
			return fmt.Errorf("%s %d: %w", j.c, j.id, err)
		}
		return os.WriteFile(filepath.Join(dir, categoryFolder(j.c), fmt.Sprintf("%s_%d.obd", j.c, j.id)), data, 0o644)
	})
	return ExportSummary{Files: len(jobs), Skipped: total - len(jobs)}, err
}

func categoryFolder(c thing.Category) string { return c.String() + "s" }

// ExportAll writes every sprite as {id}.{format} into dir (see Export).
// With skipEmpty fully transparent sprites are left out.
func (ss *SpriteService) ExportAll(dir, format string, skipEmpty bool) (ExportSummary, error) {
	format = imaging.FormatOf("x." + format)
	p, err := ss.s.Project()
	if err != nil {
		return ExportSummary{}, err
	}
	n := int(p.Info().Counts.Sprites)
	size := p.SpriteSize()
	var skipped atomic.Int64
	err = ss.s.parallel("Exporting sprites", n, func(i int) error {
		id := uint32(i + 1)
		if skipEmpty {
			if empty, err := p.IsSpriteEmpty(id); err != nil || empty {
				skipped.Add(1)
				return err
			}
		}
		px, err := p.SpritePixels(id)
		if err != nil {
			return err
		}
		data, err := imaging.Encode(imagingNRGBA(px, size), format, imaging.Magenta)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.%s", id, format)), data, 0o644)
	})
	return ExportSummary{Files: n - int(skipped.Load()), Skipped: int(skipped.Load())}, err
}

// ExportAllSheets writes every sprite into sprite sheets of columns x rows
// sprites, named sprites_{first}-{last}.{format}. Sprites keep their order,
// left to right and top to bottom, so the position in a sheet gives the id;
// empty sprites stay as gaps. Formats without alpha get a magenta
// background, like ExportSheet; transparent keeps alpha in png.
func (ss *SpriteService) ExportAllSheets(dir, format string, columns, rows int, transparent bool) (ExportSummary, error) {
	format = imaging.FormatOf("x." + format)
	if columns < 1 || rows < 1 || columns*rows > 64*64 {
		return ExportSummary{}, fmt.Errorf("a sheet holds 1 to 4096 sprites, not %d x %d", columns, rows)
	}
	p, err := ss.s.Project()
	if err != nil {
		return ExportSummary{}, err
	}
	n := int(p.Info().Counts.Sprites)
	size := p.SpriteSize()
	per := columns * rows
	sheets := (n + per - 1) / per
	bg := imaging.Magenta
	if transparent && format == "png" {
		bg = color.NRGBA{}
	}
	err = ss.s.parallel("Exporting sprite sheets", sheets, func(i int) error {
		first := i*per + 1
		last := min(first+per-1, n)
		// The last sheet only gets the rows it needs.
		h := (last - first + columns) / columns
		img := image.NewNRGBA(image.Rect(0, 0, columns*size, h*size))
		draw.Draw(img, img.Rect, &image.Uniform{C: bg}, image.Point{}, draw.Src)
		for id := first; id <= last; id++ {
			px, err := p.SpritePixels(uint32(id))
			if err != nil {
				return err
			}
			k := id - first
			imaging.DrawOver(img, (k%columns)*size, (k/columns)*size, size, px)
		}
		data, err := imaging.Encode(img, format, imaging.Magenta)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, fmt.Sprintf("sprites_%d-%d.%s", first, last, format)), data, 0o644)
	})
	return ExportSummary{Files: sheets}, err
}

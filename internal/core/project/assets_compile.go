package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/nekiro/ots-creator/internal/fsutil"

	"github.com/nekiro/ots-creator/internal/formats/assets"
	"github.com/nekiro/ots-creator/internal/formats/dat"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// copyChunk is the number of files one worker copies at a time when the
// source folder is copied to another directory.
const copyChunk = 16

// compileAssets writes the project as an asset folder in dir. Sheets that
// already exist are kept; new and edited sprites go to new sheets.
func (p *Project) compileAssets(dir string, progress func(done, total int), stats *CompileStats) error {
	if err := os.MkdirAll(dir, fsutil.DirMode); err != nil {
		return err
	}
	base := &assets.Catalog{}
	writer := &assetWriter{project: p, known: map[string]*newSprite{}, repacked: map[uint32]*newSprite{}}
	file := &assets.File{Things: p.things.Things}
	if p.assets != nil {
		base = p.assets.catalog
		writer.pieces = p.assets.pieces
		file = p.assets.file
		file.Things = p.things.Things
	}
	appearancesData, kept, groups, err := writer.encodePasses(file, base)
	if err != nil {
		return err
	}
	obsolete := obsoleteFiles(base, kept)
	copied := p.filesToCopy(dir, base, obsolete)
	done := newCounter(len(copied)+len(groups), progress)
	if err := p.copySourceFiles(dir, copied, done); err != nil {
		return err
	}
	sheets, addedBytes, err := writeSheets(dir, groups, done)
	if err != nil {
		return err
	}
	appearances, err := writeCatalog(dir, base, appearancesData, append(slices.Clone(kept), sheets...))
	if err != nil {
		return err
	}
	// A new sheet can have the name of an obsolete one (same content): it
	// stays.
	for _, sheet := range sheets {
		delete(obsolete, sheet.File)
	}
	if stats != nil {
		*stats = p.compileStats(base, obsolete, addedBytes)
	}
	// The previous appearances file and the dropped or repacked sheets of
	// this folder are no longer listed.
	if p.assets != nil && fsutil.SamePath(p.assets.dir, dir) {
		delete(obsolete, appearances)
		for name := range obsolete {
			os.Remove(filepath.Join(dir, name))
		}
	}
	return p.reloadAssets(dir)
}

// encodePasses encodes the appearances twice: pass 1 finds the new sprites,
// pass 2 writes their final ids. It returns the encoded appearances, the
// sheets of base that stay and the new sprites grouped by sheet.
func (w *assetWriter) encodePasses(file *assets.File, base *assets.Catalog) ([]byte, []assets.Sheet, [][]*newSprite, error) {
	if _, err := assets.Encode(file, w.resolve); err != nil {
		return nil, nil, nil, err
	}
	kept, err := w.repack(base)
	if err != nil {
		return nil, nil, nil, err
	}
	groups := w.assign(base.NextSpriteID())
	appearancesData, err := assets.Encode(file, w.resolve)
	if err != nil {
		return nil, nil, nil, err
	}
	return appearancesData, kept, groups, nil
}

// obsoleteFiles returns the sheets of base that are not kept (dropped or
// repacked) and its appearances file, which is replaced.
func obsoleteFiles(base *assets.Catalog, kept []assets.Sheet) map[string]bool {
	obsolete := map[string]bool{base.Appearances: true}
	for _, sheet := range base.Sheets {
		obsolete[sheet.File] = true
	}
	for _, sheet := range kept {
		delete(obsolete, sheet.File)
	}
	return obsolete
}

// filesToCopy lists the files of the source folder a compile to another
// directory copies: every file of base that is not obsolete.
func (p *Project) filesToCopy(dir string, base *assets.Catalog, obsolete map[string]bool) []string {
	var names []string
	if p.assets != nil && !fsutil.SamePath(p.assets.dir, dir) {
		for _, name := range base.Files() {
			if !obsolete[name] {
				names = append(names, name)
			}
		}
	}
	return names
}

// copySourceFiles copies files of the source folder to dir (thousands of
// sheets: in parallel, hard links where possible).
func (p *Project) copySourceFiles(dir string, names []string, done *counter) error {
	if len(names) == 0 {
		return nil
	}
	return parallelRange(0, uint32(len(names)-1), copyChunk, func(from, to uint32) error {
		for _, name := range names[from : to+1] {
			if err := fsutil.CopyFile(filepath.Join(p.assets.dir, name), filepath.Join(dir, name)); err != nil {
				return err
			}
			done.tick()
		}
		return nil
	})
}

// writeSheets encodes the new sprite groups as sheets on all cores and
// writes them to dir. It returns the sheets and the bytes of new files.
func writeSheets(dir string, groups [][]*newSprite, done *counter) ([]assets.Sheet, int64, error) {
	sheets := make([]assets.Sheet, len(groups))
	errs := make([]error, len(groups))
	// Sheet files are named by the hash of their data: a name that exists
	// already holds the same sheet (sprites of different types with the same
	// pixels, or a copied sheet), so it is written once.
	var writtenMu sync.Mutex
	written := map[string]bool{}
	var addedBytes atomic.Int64
	writeSheet := func(name string, data []byte) error {
		writtenMu.Lock()
		duplicate := written[name]
		written[name] = true
		writtenMu.Unlock()
		if duplicate {
			return nil
		}
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		addedBytes.Add(int64(len(data)))
		return fsutil.WriteAtomic(path, data)
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(runtime.NumCPU(), max(len(groups), 1)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				var data []byte
				if data, sheets[i], errs[i] = encodeSheet(groups[i]); errs[i] == nil {
					errs[i] = writeSheet(sheets[i].File, data)
				}
				done.tick()
			}
		}()
	}
	for i := range groups {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, 0, err
	}
	return sheets, addedBytes.Load(), nil
}

// writeCatalog writes the appearances file and a catalog listing it and
// sheets. It returns the name of the appearances file.
func writeCatalog(dir string, base *assets.Catalog, appearancesData []byte, sheets []assets.Sheet) (string, error) {
	appearances := "appearances-" + hash(appearancesData) + ".dat"
	if err := fsutil.WriteAtomic(filepath.Join(dir, appearances), appearancesData); err != nil {
		return "", err
	}
	catalogData, err := base.With(appearances, sheets).Marshal()
	if err != nil {
		return "", err
	}
	if err := fsutil.WriteAtomic(filepath.Join(dir, assets.CatalogFile), catalogData); err != nil {
		return "", err
	}
	return appearances, nil
}

// compileStats counts the obsolete sheets of base and the bytes they free,
// minus the bytes of new sheet files.
func (p *Project) compileStats(base *assets.Catalog, obsolete map[string]bool, addedBytes int64) CompileStats {
	stats := CompileStats{Freed: -addedBytes}
	for _, sheet := range base.Sheets {
		if !obsolete[sheet.File] {
			continue
		}
		stats.SheetsRemoved++
		if p.assets != nil {
			if info, err := os.Stat(filepath.Join(p.assets.dir, sheet.File)); err == nil {
				stats.Freed += info.Size()
			}
		}
	}
	return stats
}

// reloadAssets reopens the compiled folder so piece ids match the new
// sheets.
func (p *Project) reloadAssets(dir string) error {
	state, err := loadAssets(dir)
	if err != nil {
		return fmt.Errorf("reopen compiled assets: %w", err)
	}
	p.assets = state
	p.things = &dat.File{Things: state.file.Things}
	p.sprites = newSpriteStore(newSheetBase(dir, state.catalog, state.pieces), assetFeatures.SpriteSize, true)
	p.format = FormatAssets
	p.version = AssetsVersion(dir)
	p.features = assetFeatures
	p.datPath, p.sprPath = dir, ""
	return nil
}

// assetWarnings lists what cannot be stored in an asset folder.
func (p *Project) assetWarnings() []string {
	var out []string
	for _, category := range thing.Categories {
		for _, object := range p.things.Things[category] {
			if unsupported := assets.Unsupported(&object.Props); len(unsupported) > 0 {
				out = append(out, fmt.Sprintf("%s %d: %v", category, object.ID, unsupported))
			}
			for _, group := range object.FrameGroups {
				if _, ok := assets.SpriteTypeFor(int(group.Width), int(group.Height)); !ok {
					out = append(out, fmt.Sprintf("%s %d: %dx%d tiles (assets allow up to 2x2)", category, object.ID, group.Width, group.Height))
					break
				}
			}
		}
	}
	return out
}

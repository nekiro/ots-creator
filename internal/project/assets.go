package project

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/nekiro/ots-creator/internal/assets"
	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/dat"
	"github.com/nekiro/ots-creator/internal/spr"
	"github.com/nekiro/ots-creator/internal/thing"
)

// Format is the file format of a client.
type Format string

const (
	// FormatDat is a Tibia.dat + Tibia.spr pair (up to 10.x, OTClient).
	FormatDat Format = "dat"
	// FormatAssets is the asset folder of Tibia 12+: protobuf appearances
	// and LZMA sprite sheets.
	FormatAssets Format = "assets"
)

// assetFeatures are fixed by the format: 32 bit ids, alpha, per frame
// durations and outfit frame groups.
var assetFeatures = client.Features{Extended: true, Transparency: true, ImprovedAnimations: true, FrameGroups: true, SpriteSize: client.DefaultSpriteSize}

// assetState is what an assets project needs to write the folder back.
type assetState struct {
	dir     string
	catalog *assets.Catalog
	pieces  *assets.PieceMap
	file    *assets.File
}

// AssetsVersion returns the version shown for an asset folder.
func AssetsVersion(dir string) client.Version {
	v := client.Version{Value: 1200, Name: "12+ (assets)"}
	if name := assets.ClientVersion(dir); name != "" {
		major, minor, _ := strings.Cut(name, ".")
		a, _ := strconv.Atoi(major)
		b, _ := strconv.Atoi(minor)
		if a > 0 {
			v = client.Version{Value: uint16(a*100 + min(b, 99)), Name: name}
		}
	}
	return v
}

// OpenAssets loads an asset folder (or its catalog, or a Tibia install
// folder).
func OpenAssets(path string) (*Project, error) {
	dir, err := assets.FindDir(path)
	if err != nil {
		return nil, err
	}
	st, err := loadAssets(dir)
	if err != nil {
		return nil, err
	}
	return &Project{
		version:  AssetsVersion(dir),
		features: assetFeatures,
		format:   FormatAssets,
		datPath:  dir,
		things:   &dat.File{Things: st.file.Things},
		sprites:  newSpriteStore(newSheetBase(dir, st.catalog, st.pieces), assetFeatures.SpriteSize, true),
		assets:   st,
	}, nil
}

func loadAssets(dir string) (*assetState, error) {
	cat, err := assets.ReadCatalog(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, cat.Appearances))
	if err != nil {
		return nil, err
	}
	pieces := assets.NewPieceMap(cat)
	f, err := assets.Decode(data, pieces)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cat.Appearances, err)
	}
	return &assetState{dir: dir, catalog: cat, pieces: pieces, file: f}, nil
}

// sheetBase serves 32x32 pieces of sprite sheets, decoding sheets on
// demand and keeping the most recently used ones.
type sheetBase struct {
	dir    string
	cat    *assets.Catalog
	pieces *assets.PieceMap

	mu    sync.Mutex
	cache map[int]*list.Element
	lru   *list.List // of *sheetEntry, most recent first
}

type sheetEntry struct {
	index int
	once  sync.Once
	px    []byte
	err   error
}

// maxCachedSheets bounds the decoded sheets in memory (576 KiB each).
const maxCachedSheets = 128

func newSheetBase(dir string, cat *assets.Catalog, pieces *assets.PieceMap) *sheetBase {
	return &sheetBase{dir: dir, cat: cat, pieces: pieces, cache: map[int]*list.Element{}, lru: list.New()}
}

func (b *sheetBase) Count() uint32 { return b.pieces.Count() }

func (b *sheetBase) sheet(i int) ([]byte, error) {
	b.mu.Lock()
	el, ok := b.cache[i]
	if ok {
		b.lru.MoveToFront(el)
	} else {
		el = b.lru.PushFront(&sheetEntry{index: i})
		b.cache[i] = el
		for b.lru.Len() > maxCachedSheets {
			old := b.lru.Back()
			b.lru.Remove(old)
			delete(b.cache, old.Value.(*sheetEntry).index)
		}
	}
	e := el.Value.(*sheetEntry)
	b.mu.Unlock()
	// Decode outside the lock so other sheets load in parallel.
	e.once.Do(func() {
		var data []byte
		if data, e.err = os.ReadFile(filepath.Join(b.dir, b.cat.Sheets[i].File)); e.err == nil {
			e.px, e.err = assets.DecodeSheet(data)
		}
	})
	return e.px, e.err
}

// piecePixels returns the RGBA pixels of one piece.
func (b *sheetBase) piecePixels(id uint32) ([]byte, error) {
	p, ok := b.pieces.Locate(id)
	if !ok {
		return nil, fmt.Errorf("sprite %d out of range [1,%d]", id, b.pieces.Count())
	}
	sheet, err := b.sheet(p.Sheet)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", b.cat.Sheets[p.Sheet].File, err)
	}
	const t = assets.TileSize
	out := make([]byte, t*t*4)
	for y := range t {
		o := ((p.Y+y)*assets.SheetSize + p.X) * 4
		copy(out[y*t*4:(y+1)*t*4], sheet[o:o+t*4])
	}
	return out, nil
}

func (b *sheetBase) Compressed(id uint32) ([]byte, error) {
	px, err := b.piecePixels(id)
	if err != nil {
		return nil, err
	}
	return spr.Compress(px, assets.TileSize, true), nil
}

// newSprite is a sprite created by an edit, waiting for a sheet.
type newSprite struct {
	typ int
	px  []byte
	id  uint32
}

// assetWriter turns piece ids back into official sprites while compiling.
type assetWriter struct {
	p      *Project
	pieces *assets.PieceMap // nil when converting from dat/spr
	known  map[string]*newSprite
	order  []*newSprite
}

// resolve implements assets.Resolver. Untouched pieces of an existing
// sprite map back to it; anything else becomes a new sprite (identical
// textures share one).
func (w *assetWriter) resolve(pieces []uint32, tw, th int) (uint32, error) {
	if w.pieces != nil {
		if id, ok := w.pieces.Sprite(pieces, tw, th); ok && !w.anyEdited(pieces) {
			return id, nil
		}
	}
	typ, _ := assets.SpriteTypeFor(tw, th)
	const t = assets.TileSize
	pw := tw * t
	px := make([]byte, pw*th*t*4)
	for k, id := range pieces {
		if id == 0 {
			continue
		}
		piece, err := w.p.sprites.pixels(id)
		if err != nil {
			return 0, err
		}
		x, y := (tw-1-k%tw)*t, (th-1-k/tw)*t
		for row := range t {
			copy(px[((y+row)*pw+x)*4:((y+row)*pw+x+t)*4], piece[row*t*4:(row+1)*t*4])
		}
	}
	key := strconv.Itoa(typ) + string(px)
	s, ok := w.known[key]
	if !ok {
		s = &newSprite{typ: typ, px: px}
		w.known[key] = s
		w.order = append(w.order, s)
	}
	return s.id, nil
}

func (w *assetWriter) anyEdited(pieces []uint32) bool {
	for _, id := range pieces {
		if w.p.sprites.edited(id) {
			return true
		}
	}
	return false
}

// assign gives the new sprites ids after next, grouped by type so that
// each sheet holds a contiguous id range, and returns the sheets.
func (w *assetWriter) assign(next uint32) [][]*newSprite {
	var sheets [][]*newSprite
	for typ := assets.Sprite32x32; typ <= assets.Sprite64x64; typ++ {
		var cur []*newSprite
		for _, s := range w.order {
			if s.typ != typ {
				continue
			}
			if len(cur) == assets.SheetCapacity(typ) {
				sheets = append(sheets, cur)
				cur = nil
			}
			s.id = next
			next++
			cur = append(cur, s)
		}
		if len(cur) > 0 {
			sheets = append(sheets, cur)
		}
	}
	return sheets
}

func encodeSheet(sprites []*newSprite) ([]byte, assets.Sheet, error) {
	px := make([]byte, assets.SheetSize*assets.SheetSize*4)
	typ := sprites[0].typ
	for i, s := range sprites {
		x, y, sw, sh := assets.SpriteRect(typ, i)
		for row := range sh {
			o := ((y+row)*assets.SheetSize + x) * 4
			copy(px[o:o+sw*4], s.px[row*sw*4:(row+1)*sw*4])
		}
	}
	data, err := assets.EncodeSheet(px)
	if err != nil {
		return nil, assets.Sheet{}, err
	}
	sheet := assets.Sheet{File: "sprites-" + hash(data) + ".bmp.lzma", SpriteType: typ, First: sprites[0].id, Last: sprites[len(sprites)-1].id}
	return data, sheet, nil
}

func hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// compileAssets writes the project as an asset folder in dir. Sheets that
// already exist are kept; new and edited sprites go to new sheets.
func (p *Project) compileAssets(dir string, progress func(done, total int)) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := &assets.Catalog{}
	w := &assetWriter{p: p, known: map[string]*newSprite{}}
	f := &assets.File{Things: p.things.Things}
	if p.assets != nil {
		base = p.assets.catalog
		w.pieces = p.assets.pieces
		f = p.assets.file
		f.Things = p.things.Things
	}
	// Pass 1 finds the new sprites, pass 2 writes their final ids.
	if _, err := assets.Encode(f, w.resolve); err != nil {
		return err
	}
	groups := w.assign(base.NextSpriteID())
	data, err := assets.Encode(f, w.resolve)
	if err != nil {
		return err
	}

	// Copy the files of the source folder when writing somewhere else
	// (thousands of sheets: in parallel, hard links where possible).
	var names []string
	if p.assets != nil && !sameDir(p.assets.dir, dir) {
		names = base.Files()
	}
	done := newCounter(len(names)+len(groups), progress)
	if len(names) > 0 {
		err := parallelRange(0, uint32(len(names)-1), 16, func(from, to uint32) error {
			for _, name := range names[from : to+1] {
				if err := copyFile(filepath.Join(p.assets.dir, name), filepath.Join(dir, name)); err != nil {
					return err
				}
				done.tick()
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	sheets := make([]assets.Sheet, len(groups))
	errs := make([]error, len(groups))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(runtime.NumCPU(), max(len(groups), 1)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				var data []byte
				if data, sheets[i], errs[i] = encodeSheet(groups[i]); errs[i] == nil {
					errs[i] = writeAtomic(filepath.Join(dir, sheets[i].File), data)
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
		return err
	}

	appearances := "appearances-" + hash(data) + ".dat"
	if err := writeAtomic(filepath.Join(dir, appearances), data); err != nil {
		return err
	}
	cat := base.With(appearances, sheets)
	catData, err := cat.Marshal()
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, assets.CatalogFile), catData); err != nil {
		return err
	}
	// The previous appearances file of this folder is no longer listed.
	if p.assets != nil && sameDir(p.assets.dir, dir) && base.Appearances != appearances {
		os.Remove(filepath.Join(dir, base.Appearances))
	}

	// Reload so piece ids match the new sheets.
	st, err := loadAssets(dir)
	if err != nil {
		return fmt.Errorf("reopen compiled assets: %w", err)
	}
	p.assets = st
	p.things = &dat.File{Things: st.file.Things}
	p.sprites = newSpriteStore(newSheetBase(dir, st.catalog, st.pieces), assetFeatures.SpriteSize, true)
	p.format = FormatAssets
	p.version = AssetsVersion(dir)
	p.features = assetFeatures
	p.datPath, p.sprPath = dir, ""
	return nil
}

func sameDir(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// copyFile copies src to dst. Asset files are never changed in place (every
// write goes to a new file that is renamed over), so a hard link is a safe
// and much faster copy when both are on one volume.
func copyFile(src, dst string) error {
	if st, err := os.Stat(dst); err == nil && !st.IsDir() {
		return nil // content addressed names: an existing file is the same
	}
	if os.Link(src, dst) == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

// assetWarnings lists what cannot be stored in an asset folder.
func (p *Project) assetWarnings() []string {
	var out []string
	for _, c := range thing.Categories {
		for _, t := range p.things.Things[c] {
			if u := assets.Unsupported(&t.Props); len(u) > 0 {
				out = append(out, fmt.Sprintf("%s %d: %v", c, t.ID, u))
			}
			for _, g := range t.FrameGroups {
				if _, ok := assets.SpriteTypeFor(int(g.Width), int(g.Height)); !ok {
					out = append(out, fmt.Sprintf("%s %d: %dx%d tiles (assets allow up to 2x2)", c, t.ID, g.Width, g.Height))
					break
				}
			}
		}
	}
	return out
}

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
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

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
	p := &Project{
		version:  AssetsVersion(dir),
		features: assetFeatures,
		format:   FormatAssets,
		datPath:  dir,
		things:   &dat.File{Things: st.file.Things},
		sprites:  newSpriteStore(newSheetBase(dir, st.catalog, st.pieces), assetFeatures.SpriteSize, true),
		assets:   st,
	}
	p.resetBase()
	return p, nil
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
	if err := checkSheets(dir, cat); err != nil {
		return nil, err
	}
	pieces := assets.NewPieceMap(cat)
	f, err := assets.Decode(data, pieces)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cat.Appearances, err)
	}
	return &assetState{dir: dir, catalog: cat, pieces: pieces, file: f}, nil
}

// checkSheets fails when sprite sheets of the catalog are missing, which
// would otherwise show as objects without pixels.
func checkSheets(dir string, cat *assets.Catalog) error {
	missing, first := 0, ""
	for _, sh := range cat.Sheets {
		if _, err := os.Stat(filepath.Join(dir, sh.File)); err != nil {
			if missing == 0 {
				first = sh.File
			}
			missing++
		}
	}
	if missing > 0 {
		return fmt.Errorf("%d of %d sprite sheets listed in %s are missing (first: %s)", missing, len(cat.Sheets), assets.CatalogFile, first)
	}
	return nil
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
	// byHash lists base pieces by content hash (see sameContent).
	byHash map[spriteHash][]uint32
	// used marks the existing sprites the things still reference;
	// repacked maps the ones moved out of a partly used sheet.
	used     map[uint32]bool
	repacked map[uint32]*newSprite
}

// resolve implements assets.Resolver. Untouched pieces of an existing
// sprite map back to it; anything else becomes a new sprite (identical
// textures share one). Sprite id 0, which older conversions handed out,
// is written again as a new sprite because clients read 0 as "no sprite".
func (w *assetWriter) resolve(pieces []uint32, tw, th int) (uint32, error) {
	if w.pieces != nil {
		if id, ok := w.pieces.Sprite(pieces, tw, th); ok && id != 0 && !w.anyEdited(pieces) {
			return w.existing(id), nil
		}
		if id, ok, err := w.sameContent(pieces, tw, th); err != nil || ok {
			return w.existing(id), err
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
	return w.add(typ, px).id, nil
}

// add queues a new sprite; identical ones share one.
func (w *assetWriter) add(typ int, px []byte) *newSprite {
	key := strconv.Itoa(typ) + string(px)
	s, ok := w.known[key]
	if !ok {
		s = &newSprite{typ: typ, px: px}
		w.known[key] = s
		w.order = append(w.order, s)
	}
	return s
}

// existing returns the id an existing sprite gets in the output.
func (w *assetWriter) existing(id uint32) uint32 {
	if s, ok := w.repacked[id]; ok {
		return s.id
	}
	if w.used == nil {
		w.used = map[uint32]bool{}
	}
	w.used[id] = true
	return id
}

// repack decides what happens to the sheets of the folder after the first
// pass: fully used sheets stay, unused ones are dropped and the used
// sprites of the others move to new sheets, so removed sprites free disk
// space. It returns the sheets that stay.
func (w *assetWriter) repack(base *assets.Catalog) ([]assets.Sheet, error) {
	sb, ok := w.p.sprites.base.(*sheetBase)
	if !ok || w.pieces == nil {
		return base.Sheets, nil
	}
	var keep []assets.Sheet
	for i, sh := range base.Sheets {
		n := 0
		for id := sh.First; id <= sh.Last; id++ {
			if w.used[id] {
				n++
			}
		}
		switch n {
		case sh.Count():
			keep = append(keep, sh)
		case 0:
		default:
			sheet, err := sb.sheet(i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", sh.File, err)
			}
			for id := sh.First; id <= sh.Last; id++ {
				if !w.used[id] {
					continue
				}
				x, y, sw, sh2 := assets.SpriteRect(sh.SpriteType, int(id-sh.First))
				px := make([]byte, sw*sh2*4)
				for row := range sh2 {
					o := ((y+row)*assets.SheetSize + x) * 4
					copy(px[row*sw*4:(row+1)*sw*4], sheet[o:o+sw*4])
				}
				w.repacked[id] = w.add(sh.SpriteType, px)
			}
		}
	}
	return keep, nil
}

// sameContent finds an existing sprite whose pieces look exactly like
// pieces (by content hash), for pieces that were moved or merged by an
// optimization. Hashes are cached, so this rarely decodes a sheet.
func (w *assetWriter) sameContent(pieces []uint32, tw, th int) (uint32, bool, error) {
	if len(pieces) != tw*th {
		return 0, false, nil
	}
	hashes := make([]spriteHash, len(pieces))
	var key spriteHash
	for k, id := range pieces {
		if id == 0 {
			continue
		}
		h, err := w.p.sprites.hash(id)
		if err != nil {
			return 0, false, err
		}
		hashes[k] = h
		if key == (spriteHash{}) {
			key = h
		}
	}
	if key == (spriteHash{}) {
		return 0, false, nil // fully transparent
	}
	if w.byHash == nil {
		if err := w.indexBase(); err != nil {
			return 0, false, err
		}
	}
	for _, b := range w.byHash[key] {
		at, ok := w.pieces.Locate(b)
		if !ok || at.Sprite == 0 {
			continue
		}
		first, pw, ph, ok := w.pieces.Pieces(at.Sprite)
		if !ok || pw != tw || ph != th {
			continue
		}
		same := true
		for k, h := range hashes {
			want, err := w.p.sprites.baseHash(first + uint32(k))
			if err != nil {
				return 0, false, err
			}
			if h != want {
				same = false
				break
			}
		}
		if same {
			return at.Sprite, true, nil
		}
	}
	return 0, false, nil
}

// indexBase lists the base pieces by content hash.
func (w *assetWriter) indexBase() error {
	s := w.p.sprites
	n := s.baseCount()
	hashes := make([]spriteHash, n+1)
	err := parallelRange(1, n, spriteChunk, func(from, to uint32) error {
		for b := from; b <= to; b++ {
			h, err := s.baseHash(b)
			if err != nil {
				return err
			}
			hashes[b] = h
		}
		return nil
	})
	if err != nil {
		return err
	}
	w.byHash = map[spriteHash][]uint32{}
	for b := uint32(1); b <= n; b++ {
		if h := hashes[b]; h != (spriteHash{}) {
			w.byHash[h] = append(w.byHash[h], b)
		}
	}
	return nil
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
func (p *Project) compileAssets(dir string, progress func(done, total int), stats *CompileStats) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := &assets.Catalog{}
	w := &assetWriter{p: p, known: map[string]*newSprite{}, repacked: map[uint32]*newSprite{}}
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
	keep, err := w.repack(base)
	if err != nil {
		return err
	}
	groups := w.assign(base.NextSpriteID())
	data, err := assets.Encode(f, w.resolve)
	if err != nil {
		return err
	}

	// Sheets of the folder that are not kept (dropped or repacked) and its
	// appearances file, which is replaced.
	gone := map[string]bool{base.Appearances: true}
	for _, sh := range base.Sheets {
		gone[sh.File] = true
	}
	for _, sh := range keep {
		delete(gone, sh.File)
	}

	// Copy the files of the source folder when writing somewhere else
	// (thousands of sheets: in parallel, hard links where possible).
	var names []string
	if p.assets != nil && !sameDir(p.assets.dir, dir) {
		for _, name := range base.Files() {
			if !gone[name] {
				names = append(names, name)
			}
		}
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
	// Sheet files are named by the hash of their data: a name that exists
	// already holds the same sheet (sprites of different types with the same
	// pixels, or a sheet copied above), so it is written once.
	var writtenMu sync.Mutex
	written := map[string]bool{}
	var added atomic.Int64 // bytes of new sheet files
	writeSheet := func(name string, data []byte) error {
		writtenMu.Lock()
		dup := written[name]
		written[name] = true
		writtenMu.Unlock()
		if dup {
			return nil
		}
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		added.Add(int64(len(data)))
		return writeAtomic(path, data)
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
		return err
	}

	appearances := "appearances-" + hash(data) + ".dat"
	if err := writeAtomic(filepath.Join(dir, appearances), data); err != nil {
		return err
	}
	cat := base.With(appearances, append(slices.Clone(keep), sheets...))
	catData, err := cat.Marshal()
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, assets.CatalogFile), catData); err != nil {
		return err
	}
	// A new sheet can have the name of a gone one (same content): it stays.
	for _, sh := range sheets {
		delete(gone, sh.File)
	}
	if stats != nil {
		*stats = CompileStats{Freed: -added.Load()}
		for _, sh := range base.Sheets {
			if gone[sh.File] {
				stats.SheetsRemoved++
				if p.assets != nil {
					if st, err := os.Stat(filepath.Join(p.assets.dir, sh.File)); err == nil {
						stats.Freed += st.Size()
					}
				}
			}
		}
	}
	// The previous appearances file and the dropped or repacked sheets of
	// this folder are no longer listed.
	if p.assets != nil && sameDir(p.assets.dir, dir) {
		delete(gone, appearances)
		for name := range gone {
			os.Remove(filepath.Join(dir, name))
		}
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

package project

import (
	"container/list"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/formats/assets"
	"github.com/nekiro/ots-creator/internal/formats/spr"
)

// sheetBase serves 32x32 pieces of sprite sheets, decoding sheets on
// demand and keeping the most recently used ones.
type sheetBase struct {
	dir     string
	catalog *assets.Catalog
	pieces  *assets.PieceMap

	mu    sync.Mutex
	cache map[int]*list.Element
	lru   *list.List // of *sheetEntry, most recent first
}

type sheetEntry struct {
	index  int
	once   sync.Once
	pixels []byte
	err    error
}

// maxCachedSheets bounds the decoded sheets in memory (576 KiB each).
const maxCachedSheets = 128

func newSheetBase(dir string, catalog *assets.Catalog, pieces *assets.PieceMap) *sheetBase {
	return &sheetBase{dir: dir, catalog: catalog, pieces: pieces, cache: map[int]*list.Element{}, lru: list.New()}
}

func (b *sheetBase) Count() uint32 { return b.pieces.Count() }

func (b *sheetBase) sheet(i int) ([]byte, error) {
	b.mu.Lock()
	element, ok := b.cache[i]
	if ok {
		b.lru.MoveToFront(element)
	} else {
		element = b.lru.PushFront(&sheetEntry{index: i})
		b.cache[i] = element
		for b.lru.Len() > maxCachedSheets {
			old := b.lru.Back()
			b.lru.Remove(old)
			delete(b.cache, old.Value.(*sheetEntry).index)
		}
	}
	entry := element.Value.(*sheetEntry)
	b.mu.Unlock()
	// Decode outside the lock so other sheets load in parallel.
	entry.once.Do(func() {
		var data []byte
		if data, entry.err = os.ReadFile(filepath.Join(b.dir, b.catalog.Sheets[i].File)); entry.err == nil {
			entry.pixels, entry.err = assets.DecodeSheet(data)
		}
	})
	return entry.pixels, entry.err
}

func (b *sheetBase) piecePixels(id uint32) ([]byte, error) {
	piece, ok := b.pieces.Locate(id)
	if !ok {
		return nil, fmt.Errorf("sprite %d out of range [1,%d]", id, b.pieces.Count())
	}
	sheet, err := b.sheet(piece.Sheet)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", b.catalog.Sheets[piece.Sheet].File, err)
	}
	const tileSize = assets.TileSize
	out := make([]byte, tileSize*tileSize*4)
	imaging.CopyRect(out, tileSize, 0, 0, sheet, assets.SheetSize, piece.X, piece.Y, tileSize, tileSize)
	return out, nil
}

func (b *sheetBase) Compressed(id uint32) ([]byte, error) {
	pixels, err := b.piecePixels(id)
	if err != nil {
		return nil, err
	}
	return spr.Compress(pixels, assets.TileSize, true), nil
}

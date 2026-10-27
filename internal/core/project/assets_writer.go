package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/formats/assets"
)

type newSprite struct {
	spriteType int
	pixels     []byte
	id         uint32
}

type assetWriter struct {
	project *Project
	pieces  *assets.PieceMap // nil when converting from dat/spr
	known   map[string]*newSprite
	order   []*newSprite
	byHash  map[spriteHash][]uint32
	// used marks the existing sprites the things still reference;
	// repacked maps the ones moved out of a partly used sheet.
	used     map[uint32]bool
	repacked map[uint32]*newSprite
}

// resolve implements assets.Resolver. Untouched pieces of an existing
// sprite map back to it; anything else becomes a new sprite (identical
// textures share one). Sprite id 0, which older conversions handed out,
// is written again as a new sprite because clients read 0 as "no sprite".
func (w *assetWriter) resolve(pieces []uint32, tilesWide, tilesHigh int) (uint32, error) {
	if w.pieces != nil {
		if id, ok := w.pieces.Sprite(pieces, tilesWide, tilesHigh); ok && id != 0 && !w.anyEdited(pieces) {
			return w.existing(id), nil
		}
		if id, ok, err := w.sameContent(pieces, tilesWide, tilesHigh); err != nil || ok {
			return w.existing(id), err
		}
	}
	spriteType, _ := assets.SpriteTypeFor(tilesWide, tilesHigh)
	const tileSize = assets.TileSize
	rowWidth := tilesWide * tileSize
	pixels := make([]byte, rowWidth*tilesHigh*tileSize*4)
	for pieceIndex, id := range pieces {
		if id == 0 {
			continue
		}
		piece, err := w.project.sprites.pixels(id)
		if err != nil {
			return 0, err
		}
		x, y := (tilesWide-1-pieceIndex%tilesWide)*tileSize, (tilesHigh-1-pieceIndex/tilesWide)*tileSize
		imaging.CopyRect(pixels, rowWidth, x, y, piece, tileSize, 0, 0, tileSize, tileSize)
	}
	return w.add(spriteType, pixels).id, nil
}

func (w *assetWriter) add(spriteType int, pixels []byte) *newSprite {
	key := strconv.Itoa(spriteType) + string(pixels)
	sprite, ok := w.known[key]
	if !ok {
		sprite = &newSprite{spriteType: spriteType, pixels: pixels}
		w.known[key] = sprite
		w.order = append(w.order, sprite)
	}
	return sprite
}

func (w *assetWriter) existing(id uint32) uint32 {
	if sprite, ok := w.repacked[id]; ok {
		return sprite.id
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
	sheetSource, ok := w.project.sprites.base.(*sheetBase)
	if !ok || w.pieces == nil {
		return base.Sheets, nil
	}
	var kept []assets.Sheet
	for i, sheet := range base.Sheets {
		usedCount := 0
		for id := sheet.First; id <= sheet.Last; id++ {
			if w.used[id] {
				usedCount++
			}
		}
		switch usedCount {
		case sheet.Count():
			kept = append(kept, sheet)
		case 0:
		default:
			sheetPixels, err := sheetSource.sheet(i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", sheet.File, err)
			}
			for id := sheet.First; id <= sheet.Last; id++ {
				if !w.used[id] {
					continue
				}
				x, y, spriteWidth, spriteHeight := assets.SpriteRect(sheet.SpriteType, int(id-sheet.First))
				pixels := make([]byte, spriteWidth*spriteHeight*4)
				imaging.CopyRect(pixels, spriteWidth, 0, 0, sheetPixels, assets.SheetSize, x, y, spriteWidth, spriteHeight)
				w.repacked[id] = w.add(sheet.SpriteType, pixels)
			}
		}
	}
	return kept, nil
}

// sameContent finds an existing sprite whose pieces look exactly like
// pieces (by content hash), for pieces that were moved or merged by an
// optimization. Hashes are cached, so this rarely decodes a sheet.
func (w *assetWriter) sameContent(pieces []uint32, tilesWide, tilesHigh int) (uint32, bool, error) {
	if len(pieces) != tilesWide*tilesHigh {
		return 0, false, nil
	}
	hashes := make([]spriteHash, len(pieces))
	var key spriteHash
	for pieceIndex, id := range pieces {
		if id == 0 {
			continue
		}
		contentHash, err := w.project.sprites.hash(id)
		if err != nil {
			return 0, false, err
		}
		hashes[pieceIndex] = contentHash
		if key == (spriteHash{}) {
			key = contentHash
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
	for _, baseID := range w.byHash[key] {
		location, ok := w.pieces.Locate(baseID)
		if !ok || location.Sprite == 0 {
			continue
		}
		first, piecesWide, piecesHigh, ok := w.pieces.Pieces(location.Sprite)
		if !ok || piecesWide != tilesWide || piecesHigh != tilesHigh {
			continue
		}
		same := true
		for pieceIndex, contentHash := range hashes {
			want, err := w.project.sprites.baseHash(first + uint32(pieceIndex))
			if err != nil {
				return 0, false, err
			}
			if contentHash != want {
				same = false
				break
			}
		}
		if same {
			return location.Sprite, true, nil
		}
	}
	return 0, false, nil
}

func (w *assetWriter) indexBase() error {
	store := w.project.sprites
	count := store.baseCount()
	hashes := make([]spriteHash, count+1)
	err := parallelRange(1, count, spriteChunk, func(from, to uint32) error {
		for baseID := from; baseID <= to; baseID++ {
			contentHash, err := store.baseHash(baseID)
			if err != nil {
				return err
			}
			hashes[baseID] = contentHash
		}
		return nil
	})
	if err != nil {
		return err
	}
	w.byHash = map[spriteHash][]uint32{}
	for baseID := uint32(1); baseID <= count; baseID++ {
		if contentHash := hashes[baseID]; contentHash != (spriteHash{}) {
			w.byHash[contentHash] = append(w.byHash[contentHash], baseID)
		}
	}
	return nil
}

func (w *assetWriter) anyEdited(pieces []uint32) bool {
	for _, id := range pieces {
		if w.project.sprites.edited(id) {
			return true
		}
	}
	return false
}

// assign gives the new sprites ids after next, grouped by type so that
// each sheet holds a contiguous id range, and returns the sheets.
func (w *assetWriter) assign(next uint32) [][]*newSprite {
	var sheets [][]*newSprite
	for spriteType := assets.Sprite32x32; spriteType <= assets.Sprite64x64; spriteType++ {
		var current []*newSprite
		for _, sprite := range w.order {
			if sprite.spriteType != spriteType {
				continue
			}
			if len(current) == assets.SheetCapacity(spriteType) {
				sheets = append(sheets, current)
				current = nil
			}
			sprite.id = next
			next++
			current = append(current, sprite)
		}
		if len(current) > 0 {
			sheets = append(sheets, current)
		}
	}
	return sheets
}

func encodeSheet(sprites []*newSprite) ([]byte, assets.Sheet, error) {
	pixels := make([]byte, assets.SheetSize*assets.SheetSize*4)
	spriteType := sprites[0].spriteType
	for i, sprite := range sprites {
		x, y, spriteWidth, spriteHeight := assets.SpriteRect(spriteType, i)
		imaging.CopyRect(pixels, assets.SheetSize, x, y, sprite.pixels, spriteWidth, 0, 0, spriteWidth, spriteHeight)
	}
	data, err := assets.EncodeSheet(pixels)
	if err != nil {
		return nil, assets.Sheet{}, err
	}
	sheet := assets.Sheet{File: "sprites-" + hash(data) + ".bmp.lzma", SpriteType: spriteType, First: sprites[0].id, Last: sprites[len(sprites)-1].id}
	return data, sheet, nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

package assets

import "slices"

// PieceMap numbers the 32x32 pieces of every sprite in a catalog, from 1
// in sheet order. It implements SpriteMap.
type PieceMap struct {
	sheets []Sheet
	first  []uint32 // first piece id of each sheet
	count  uint32
}

// NewPieceMap builds the piece numbering of a catalog.
func NewPieceMap(catalog *Catalog) *PieceMap {
	pieceMap := &PieceMap{sheets: catalog.Sheets, first: make([]uint32, len(catalog.Sheets))}
	next := uint32(1)
	for i, sheet := range catalog.Sheets {
		pieceMap.first[i] = next
		next += uint32(sheet.Count() * tiles(sheet.SpriteType))
	}
	pieceMap.count = next - 1
	return pieceMap
}

func tiles(spriteType int) int {
	width, height, _ := SpriteDims(spriteType)
	return (width / TileSize) * (height / TileSize)
}

// Count returns the number of pieces.
func (m *PieceMap) Count() uint32 { return m.count }

// Pieces implements SpriteMap.
func (m *PieceMap) Pieces(id uint32) (first uint32, width, height int, ok bool) {
	i, ok := findSheet(m.sheets, id)
	if !ok {
		return 0, 0, 0, false
	}
	sheet := m.sheets[i]
	pixelWidth, pixelHeight, _ := SpriteDims(sheet.SpriteType)
	width, height = pixelWidth/TileSize, pixelHeight/TileSize
	return m.first[i] + (id-sheet.First)*uint32(width*height), width, height, true
}

// Piece locates a piece: its sheet, the sprite index in that sheet and the
// pixel position of the piece inside the sheet.
type Piece struct {
	Sheet  int
	Sprite uint32 // official sprite id
	X, Y   int
}

// Locate finds a piece by id.
func (m *PieceMap) Locate(piece uint32) (Piece, bool) {
	if piece == 0 || piece > m.count {
		return Piece{}, false
	}
	i, _ := slices.BinarySearch(m.first, piece+1)
	i-- // last sheet starting at or before piece
	sheet := m.sheets[i]
	piecesPerSprite := uint32(tiles(sheet.SpriteType))
	offset := piece - m.first[i]
	spriteIndex, pieceIndex := int(offset/piecesPerSprite), int(offset%piecesPerSprite)
	spriteX, spriteY, spriteWidth, spriteHeight := SpriteRect(sheet.SpriteType, spriteIndex)
	tilesWide, tilesHigh := spriteWidth/TileSize, spriteHeight/TileSize
	tileColumn, tileRow := pieceIndex%tilesWide, pieceIndex/tilesWide
	// Tile (0,0) is the bottom-right one.
	return Piece{
		Sheet:  i,
		Sprite: sheet.First + uint32(spriteIndex),
		X:      spriteX + (tilesWide-1-tileColumn)*TileSize,
		Y:      spriteY + (tilesHigh-1-tileRow)*TileSize,
	}, true
}

// Sprite returns the official sprite whose pieces are exactly pieces, in
// tile order, for a width x height texture.
func (m *PieceMap) Sprite(pieces []uint32, width, height int) (uint32, bool) {
	if len(pieces) == 0 || pieces[0] == 0 {
		return 0, false
	}
	located, ok := m.Locate(pieces[0])
	if !ok {
		return 0, false
	}
	first, spriteWidth, spriteHeight, _ := m.Pieces(located.Sprite)
	if spriteWidth != width || spriteHeight != height || len(pieces) != width*height {
		return 0, false
	}
	for i, id := range pieces {
		if id != first+uint32(i) {
			return 0, false
		}
	}
	return located.Sprite, true
}

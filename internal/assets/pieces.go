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
func NewPieceMap(c *Catalog) *PieceMap {
	m := &PieceMap{sheets: c.Sheets, first: make([]uint32, len(c.Sheets))}
	next := uint32(1)
	for i, s := range c.Sheets {
		m.first[i] = next
		next += uint32(s.Count() * tiles(s.SpriteType))
	}
	m.count = next - 1
	return m
}

func tiles(spriteType int) int {
	w, h, _ := SpriteDims(spriteType)
	return (w / TileSize) * (h / TileSize)
}

// Count returns the number of pieces.
func (m *PieceMap) Count() uint32 { return m.count }

func (m *PieceMap) sheetOf(id uint32) (int, bool) {
	i, found := slices.BinarySearchFunc(m.sheets, id, func(s Sheet, id uint32) int {
		switch {
		case s.Last < id:
			return -1
		case s.First > id:
			return 1
		}
		return 0
	})
	return i, found
}

// Pieces implements SpriteMap.
func (m *PieceMap) Pieces(id uint32) (first uint32, w, h int, ok bool) {
	i, ok := m.sheetOf(id)
	if !ok {
		return 0, 0, 0, false
	}
	s := m.sheets[i]
	pw, ph, _ := SpriteDims(s.SpriteType)
	w, h = pw/TileSize, ph/TileSize
	return m.first[i] + (id-s.First)*uint32(w*h), w, h, true
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
	s := m.sheets[i]
	n := uint32(tiles(s.SpriteType))
	off := piece - m.first[i]
	idx, k := int(off/n), int(off%n)
	sx, sy, sw, sh := SpriteRect(s.SpriteType, idx)
	w, h := sw/TileSize, sh/TileSize
	tw, th := k%w, k/w
	// Tile (0,0) is the bottom-right one.
	return Piece{Sheet: i, Sprite: s.First + uint32(idx), X: sx + (w-1-tw)*TileSize, Y: sy + (h-1-th)*TileSize}, true
}

// Sprite returns the official sprite whose pieces are exactly pieces, in
// tile order, for a w x h texture.
func (m *PieceMap) Sprite(pieces []uint32, w, h int) (uint32, bool) {
	if len(pieces) == 0 || pieces[0] == 0 {
		return 0, false
	}
	p, ok := m.Locate(pieces[0])
	if !ok {
		return 0, false
	}
	first, pw, ph, _ := m.Pieces(p.Sprite)
	if pw != w || ph != h || len(pieces) != w*h {
		return 0, false
	}
	for k, id := range pieces {
		if id != first+uint32(k) {
			return 0, false
		}
	}
	return p.Sprite, true
}

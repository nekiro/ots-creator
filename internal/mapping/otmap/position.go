package otmap

import "cmp"

// Map limits.
const (
	MaxFloor    = 15
	GroundFloor = 7
	ChunkBits   = 5
	ChunkSize   = 1 << ChunkBits
)

// Position is a map position.
type Position struct {
	X uint16 `json:"x"`
	Y uint16 `json:"y"`
	Z uint8  `json:"z"`
}

// Add returns p moved by dx, dy, dz; ok is false when the result leaves the
// valid range.
func (p Position) Add(dx, dy, dz int) (Position, bool) {
	x, y, z := int(p.X)+dx, int(p.Y)+dy, int(p.Z)+dz
	if x < 0 || y < 0 || z < 0 || x > 0xFFFF || y > 0xFFFF || z > MaxFloor {
		return Position{}, false
	}
	return Position{uint16(x), uint16(y), uint8(z)}, true
}

// ComparePositions orders positions by floor, then row, then column.
func ComparePositions(left, right Position) int {
	return cmp.Or(cmp.Compare(left.Z, right.Z), cmp.Compare(left.Y, right.Y), cmp.Compare(left.X, right.X))
}

// ChunkKey identifies a chunk.
type ChunkKey uint32

// KeyOf returns the key of the chunk holding a position.
func KeyOf(pos Position) ChunkKey {
	return MakeKey(pos.X>>ChunkBits, pos.Y>>ChunkBits, pos.Z)
}

// MakeKey builds a key from chunk coordinates.
func MakeKey(chunkX, chunkY uint16, z uint8) ChunkKey {
	return ChunkKey(uint32(chunkX)<<15 | uint32(chunkY)<<4 | uint32(z&15))
}

// Split returns the chunk coordinates of a key.
func (k ChunkKey) Split() (chunkX, chunkY uint16, z uint8) {
	return uint16(k >> 15), uint16(k>>4) & 0x7FF, uint8(k & 15)
}

func localIndex(pos Position) int {
	return int(pos.Y&(ChunkSize-1))<<ChunkBits | int(pos.X&(ChunkSize-1))
}

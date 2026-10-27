// Package maprender encodes map chunks for the map renderer of the
// frontend: the tile data the editor shows (chunks.go) and the quads the
// GPU draws (geometry.go). Both are compact little endian binary formats
// read by frontend/src/lib/map (chunks.ts and geo.ts).
package maprender

import (
	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

type ThingSource interface {
	Thing(category thing.Category, id uint32) (*thing.Thing, error)
}

type Scene struct {
	Map        *otmap.Map
	Items      *otitems.DB
	Things     ThingSource
	SpriteSize int
	// SpritePixels returns the RGBA pixels of a sprite. Optional: without
	// it lights come from the middle of things and no thing is opaque.
	SpritePixels func(id uint32) ([]byte, error)
	IsBlocking   func(tile *otmap.Tile) bool
	// Outfit returns the look of a creature by name (zero when unknown).
	Outfit func(creature string) materials.Outfit
	// MarkedItems are items covered with a colored square (RGB) when
	// ViewMarkedItems is on, by server id.
	MarkedItems map[uint16][3]uint8
}

const MaxChunksPerRequest = 512

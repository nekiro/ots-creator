// Package maprender encodes map chunks for the map renderer of the
// frontend: the tile data the editor shows (chunks.go) and the quads the
// GPU draws (geometry.go). Both are compact little endian binary formats
// read by frontend/src/lib/map (chunks.ts and geo.ts).
package maprender

import (
	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// ThingSource gives the client things sprites are drawn from.
type ThingSource interface {
	Thing(category thing.Category, id uint32) (*thing.Thing, error)
}

// Scene is everything a map is drawn from.
type Scene struct {
	Map    *otmap.Map
	Items  *otitems.DB
	Things ThingSource
	// SpriteSize is the size in pixels of one sprite (one tile).
	SpriteSize int
	// IsBlocking reports whether a tile blocks walking.
	IsBlocking func(tile *otmap.Tile) bool
	// Outfit returns the look of a creature by name (zero when unknown).
	Outfit func(creature string) materials.Outfit
}

// MaxChunksPerRequest limits the chunks of one request.
const MaxChunksPerRequest = 512

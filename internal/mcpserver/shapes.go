package mcpserver

import (
	"errors"
	"math"
	"math/rand/v2"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

const (
	// maxShapeSide limits the width and height of polygons and blobs.
	maxShapeSide = 512
	// defaultRoughness is the roughness of blobs without one.
	defaultRoughness = 0.35
	// blobHarmonics are the waves added up for a blob's edge: low ones bend
	// it, high ones make it ragged.
	blobHarmonics = 7
)

// polygonTiles returns the tiles inside a polygon or on its edges.
func polygonTiles(corners []Pos) ([]otmap.Position, error) {
	if len(corners) < 3 {
		return nil, errors.New("a polygon needs at least 3 corners")
	}
	points, err := positions(corners)
	if err != nil {
		return nil, err
	}
	minX, minY, maxX, maxY := math.MaxInt, math.MaxInt, 0, 0
	for _, point := range points {
		minX, minY = min(minX, int(point.X)), min(minY, int(point.Y))
		maxX, maxY = max(maxX, int(point.X)), max(maxY, int(point.Y))
	}
	if maxX-minX > maxShapeSide || maxY-minY > maxShapeSide {
		return nil, errors.New("the polygon is too big")
	}
	floor := points[0].Z
	var tiles []otmap.Position
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if insidePolygon(points, float64(x), float64(y)) {
				tiles = append(tiles, otmap.Position{X: uint16(x), Y: uint16(y), Z: floor})
			}
		}
	}
	return tiles, nil
}

// insidePolygon reports whether a point is inside a polygon (even-odd rule)
// or on one of its edges.
func insidePolygon(corners []otmap.Position, x, y float64) bool {
	inside := false
	for i := range corners {
		ax, ay := float64(corners[i].X), float64(corners[i].Y)
		next := corners[(i+1)%len(corners)]
		bx, by := float64(next.X), float64(next.Y)
		if onSegment(ax, ay, bx, by, x, y) {
			return true
		}
		if (ay > y) != (by > y) && x < ax+(y-ay)*(bx-ax)/(by-ay) {
			inside = !inside
		}
	}
	return inside
}

// onSegment reports whether x, y is within half a tile of the segment from
// a to b, so diagonal edges get no gaps.
func onSegment(ax, ay, bx, by, x, y float64) bool {
	dx, dy := bx-ax, by-ay
	length := dx*dx + dy*dy
	along := 0.0
	if length > 0 {
		along = min(max(((x-ax)*dx+(y-ay)*dy)/length, 0), 1)
	}
	nearX, nearY := ax+along*dx-x, ay+along*dy-y
	return nearX*nearX+nearY*nearY <= 0.25
}

// tiles returns the tiles of a blob: an ellipse whose radius changes with
// the angle by a sum of waves with random phases.
func (blob Blob) tiles() ([]otmap.Position, error) {
	center, err := blob.Center.position()
	if err != nil {
		return nil, err
	}
	radiusX, radiusY := blob.RadiusX, blob.RadiusY
	if radiusY <= 0 {
		radiusY = radiusX
	}
	if radiusX <= 0 || radiusX > maxShapeSide/2 || radiusY > maxShapeSide/2 {
		return nil, errors.New("blob radius must be 1 to 256")
	}
	roughness := blob.Roughness
	if roughness <= 0 {
		roughness = defaultRoughness
	}
	roughness = min(roughness, 1)
	random := rand.New(rand.NewPCG(uint64(blob.Seed), uint64(center.X)<<16|uint64(center.Y)))
	var phases, amplitudes [blobHarmonics]float64
	total := 0.0
	for harmonic := range blobHarmonics {
		phases[harmonic] = random.Float64() * 2 * math.Pi
		// Waves get weaker the more of them fit around the edge.
		amplitudes[harmonic] = (0.5 + random.Float64()) / float64(harmonic+2)
		total += amplitudes[harmonic]
	}
	edge := func(angle float64) float64 {
		wave := 0.0
		for harmonic := range blobHarmonics {
			wave += amplitudes[harmonic] * math.Sin(float64(harmonic+2)*angle+phases[harmonic])
		}
		// The edge stays between 1-roughness/2 and 1+roughness/2 of the radius.
		return 1 + roughness/2*wave/total
	}
	reach := int(math.Ceil(1 + roughness/2))
	var tiles []otmap.Position
	for dy := -radiusY * reach; dy <= radiusY*reach; dy++ {
		for dx := -radiusX * reach; dx <= radiusX*reach; dx++ {
			normalX, normalY := float64(dx)/float64(radiusX), float64(dy)/float64(radiusY)
			if math.Hypot(normalX, normalY) > edge(math.Atan2(normalY, normalX)) {
				continue
			}
			if pos, ok := center.Add(dx, dy, 0); ok {
				tiles = append(tiles, pos)
			}
		}
	}
	return tiles, nil
}

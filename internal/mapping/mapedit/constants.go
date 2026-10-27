package mapedit

import "errors"

// Every constant of the package, by topic. Numbers that tune the editor
// (limits, defaults) and the numbering shared with materials (RME) live
// here so they can be found and changed in one place.

const (
	KindRaw            = "raw"
	KindGround         = "ground"
	KindWall           = "wall"
	KindDoor           = "door"
	KindDoodad         = "doodad"
	KindCarpet         = "carpet"
	KindTable          = "table"
	KindEraser         = "eraser"
	KindHouse          = "house"
	KindHouseExit      = "houseExit"
	KindWaypoint       = "waypoint"
	KindSpawn          = "spawn"
	KindSpawnNPC       = "spawnNpc"
	KindCreature       = "creature"
	KindFlag           = "flag"
	KindZone           = "zone"
	KindOptionalBorder = "optionalBorder"
)

const (
	FloorsCurrent = "current"
	FloorsLower   = "lower"
	FloorsVisible = "visible"
)

// shapeCircle is the round brush shape; any other shape is square.
const shapeCircle = "circle"

// Values of ImportOptions.Houses. Any other value merges.
const (
	importHousesSkip     = "skip"
	importHousesRenumber = "renumber"
)

var ErrNoBrush = errors.New("unknown brush")

var ErrNoItem = errors.New("item not found")

const (
	maxDrawTiles = 250000
	// floodFillRadius limits a flood fill to a window around its start.
	floodFillRadius = 128
	// maxAutoTiles is the most changed tiles autoAround still fixes up.
	maxAutoTiles = 100000
	// maxSpawnRadius bounds the search for a spawn covering a tile.
	maxSpawnRadius = 30
	// maxWallRedirects limits how far a wall redirect chain is followed.
	maxWallRedirects = 8
	maxHistory       = 400
	defaultFindLimit = 5000
	// findTextPreviewLen is how many bytes of a text a search result shows.
	findTextPreviewLen = 40
)

const (
	defaultSpawnTimeSeconds = 60
	// rngStream is the PCG stream of the editor's random source.
	rngStream = 0x9e3779b97f4a7c15
)

const (
	// circleEdgeSlack widens a circle brush a little so tiles exactly on
	// its edge are left out consistently.
	circleEdgeSlack       = 0.005
	doodadSmallCircleArea = 5
	doodadPlaceRetries    = 5
)

// lowerFloorSpan is how many floors below an underground floor the lower
// and visible selection modes reach.
const lowerFloorSpan = 2

// neighbors8 lists the offsets NW, N, NE, W, E, SW, S, SE (bit order of
// the border tables).
var neighbors8 = [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

const (
	bitNW = 1 << iota
	bitN
	bitNE
	bitW
	bitE
	bitSW
	bitS
	bitSE
)

var orthogonalSteps = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

const (
	wallN = 1 << iota
	wallW
	wallE
	wallS
)

type neighborOffset struct {
	bit    int
	dx, dy int
}

var wallNeighborOffsets = [...]neighborOffset{{wallN, 0, -1}, {wallW, -1, 0}, {wallE, 1, 0}, {wallS, 0, 1}}

var neighborOffsets8 = func() (offsets [len(neighbors8)]neighborOffset) {
	for i, offset := range neighbors8 {
		offsets[i] = neighborOffset{1 << i, offset[0], offset[1]}
	}
	return offsets
}()

// Stack orders of items (see topOrder).
const (
	commonTopOrder = 0
	borderTopOrder = 1
	// bottomTopOrder is the order of bottom items (walls and the like).
	bottomTopOrder = 2
)

// Z orders of border clusters that do not take the z order of a brush.
const (
	// optionalBorderZ puts optional (mountain) borders above all others.
	optionalBorderZ = 1 << 30
	// innerZilchBorderZ is the z order of an inner border toward empty
	// tiles.
	innerZilchBorderZ = 5000
)

// Border alignments (also carpet pieces), RME numbering.
const (
	edgeNone = iota
	edgeN
	edgeE
	edgeS
	edgeW
	edgeCNW
	edgeCNE
	edgeCSW
	edgeCSE
	edgeDNW
	edgeDNE
	edgeDSE
	edgeDSW
	edgeCenter
	edgeCount
)

var edgeIndex = map[string]int{
	"n": edgeN, "e": edgeE, "s": edgeS, "w": edgeW, "cnw": edgeCNW, "cne": edgeCNE, "csw": edgeCSW, "cse": edgeCSE,
	"dnw": edgeDNW, "dne": edgeDNE, "dse": edgeDSE, "dsw": edgeDSW, "c": edgeCenter,
}

// Wall alignments, RME numbering (index into materials.WallAlignments).
const (
	wallPole         = 0
	wallHorizontal   = 6
	wallVertical     = 9
	wallNWDiagonal   = 3
	wallUntouchable  = 16
	wallAlignmentMax = 17
)

// Table alignments (index into materials.TableAlignments).
const (
	tableNorth = iota
	tableSouth
	tableEast
	tableWest
	tableHorizontal
	tableVertical
	tableAlone
	tableCount
)

// Door types: index into materials.DoorTypes plus one; 0 is undefined.
const (
	doorUndefined  = 0
	doorTypeNormal = 2
)

// toAll and toNone are the special targets of border rules.
const (
	toNone = 0
	toAll  = -1
	// toUnknown is the id of an unknown brush name: it matches nothing.
	toUnknown = -2
)

// Client view range used by RemoveUnreachable: tiles seen to each side
// horizontally and vertically, and floors seen up and down.
const (
	clientViewRangeX      = 9
	clientViewRangeY      = 7
	clientViewFloorsRange = 2
)

const creatureDirectionCount = 4

// The minimap palette is a 6x6x6 color cube: each channel has 6 levels,
// 51 apart.
const (
	minimapLevels     = 6
	minimapColorCount = minimapLevels * minimapLevels * minimapLevels
	minimapLevelStep  = 51
)

package otmap

import (
	"errors"

	"github.com/nekiro/ots-creator/internal/otitems"
)

// OTBM node types.
const (
	nodeRoot      = 0
	nodeMapData   = 2
	nodeTileArea  = 4
	nodeTile      = 5
	nodeItem      = 6
	nodeTowns     = 12
	nodeTown      = 13
	nodeHouseTile = 14
	nodeWaypoints = 15
	nodeWaypoint  = 16
	nodeTileZone  = 19
)

// OTBM attributes.
const (
	attrDescription   = 1
	attrTileFlags     = 3
	attrActionID      = 4
	attrUniqueID      = 5
	attrText          = 6
	attrDesc          = 7
	attrTeleDest      = 8
	attrItem          = 9
	attrDepotID       = 10
	attrSpawnFile     = 11
	attrRuneCharges   = 12
	attrHouseFile     = 13
	attrHouseDoorID   = 14
	attrCount         = 15
	attrDuration      = 16
	attrDecayingState = 17
	attrWrittenDate   = 18
	attrWrittenBy     = 19
	attrSleeperGUID   = 20
	attrSleepStart    = 21
	attrCharges       = 22
	attrSpawnNpcFile  = 23
	attrZoneFile      = 24
	attrAttributeMap  = 128
)

// Generator is written into saved maps as the first description.
var Generator = "OTS Creator"

// ErrNotOTBM is returned for files that are not OTBM maps.
var ErrNotOTBM = errors.New("not an OTBM map")

// Items is what the codec needs to know about item types.
type Items interface {
	Get(id uint16) *otitems.Type
}

// Format holds OTBM variants of server forks. The zero value is the
// standard format.
type Format struct {
	// CountU16 stores the count attribute as uint16 instead of uint8.
	CountU16 bool `json:"countU16"`
}

// maxWarnings bounds the memory of a badly broken map; past it warnings are
// only counted, per problem.
const maxWarnings = 100_000

// rawSizes are attributes stored verbatim, by their payload size; -1 is a
// uint16-prefixed string.
var rawSizes = map[byte]int{
	attrDuration: 4, attrDecayingState: 1, attrWrittenDate: 4, attrWrittenBy: -1,
	attrSleeperGUID: 4, attrSleepStart: 4,
}

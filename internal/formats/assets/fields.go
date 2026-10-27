package assets

// Field numbers of the messages in the appearances protobuf file
// (appearances.proto of the official client), grouped per message.

const (
	appearancesObject  = 1
	appearancesOutfit  = 2
	appearancesEffect  = 3
	appearancesMissile = 4
)

const (
	appearanceID          = 1
	appearanceFrameGroup  = 2 // repeated
	appearanceFlags       = 3
	appearanceName        = 4
	appearanceDescription = 5
)

const (
	frameGroupFixedType  = 1
	frameGroupID         = 2
	frameGroupSpriteInfo = 3
)

const (
	groupOutfitIdle    = 0
	groupObjectInitial = 2
)

const (
	spriteInfoPatternWidth   = 1
	spriteInfoPatternHeight  = 2
	spriteInfoPatternDepth   = 3
	spriteInfoLayers         = 4
	spriteInfoSpriteID       = 5 // repeated
	spriteInfoAnimation      = 6
	spriteInfoBoundingSquare = 7
	spriteInfoIsOpaque       = 8
)

const (
	animationDefaultStartPhase = 1
	animationSynchronized      = 2
	animationRandomStartPhase  = 3
	animationLoopType          = 4
	animationLoopCount         = 5
	animationSpritePhase       = 6 // repeated
)

const (
	loopPingPong = -1
	loopInfinite = 0
	loopCounted  = 1
)

const (
	phaseDurationMin = 1
	phaseDurationMax = 2
)

// AppearanceFlags field numbers the editor maps to thing.Properties. The
// flags without payload are listed in boolFlags.
const (
	flagBank                  = 1
	flagWrite                 = 10
	flagWriteOnce             = 11
	flagHook                  = 21
	flagLight                 = 23
	flagShift                 = 26
	flagHeight                = 27
	flagAutomap               = 30
	flagLensHelp              = 31
	flagClothes               = 34
	flagDefaultAction         = 35
	flagMarket                = 36
	flagNpcSaleData           = 40 // repeated
	flagChangedToExpire       = 41
	flagCyclopedia            = 44
	flagUpgradeClassification = 48
	flagLast                  = 57 // wrapkit, the highest flag the editor knows
	flagHookSouth             = 70
	flagHookEast              = 71
)

// payloadValue is the field of the flag messages with a single number
// (bank waypoints, write max_text_length, height elevation, ...).
const payloadValue = 1

const (
	hookDirectionSouth = 1
	hookDirectionEast  = 2
)

const (
	lightBrightness = 1
	lightColor      = 2
)

const (
	shiftX = 1
	shiftY = 2
)

const (
	marketCategory = 1
	marketTradeAs  = 2
	marketShowAs   = 3
	marketVocation = 5 // repeated PLAYER_PROFESSION
	marketMinLevel = 6
	marketName     = 7
)

const (
	npcName      = 1
	npcLocation  = 2
	npcSalePrice = 3
	npcBuyPrice  = 4
	npcCurrency  = 5
	npcQuestFlag = 6
)

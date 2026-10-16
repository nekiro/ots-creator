package assets

// Field numbers of the messages in the appearances protobuf file
// (appearances.proto of the official client), grouped per message.

// Appearances (the root message) fields holding the thing lists.
const (
	appearancesObject  = 1
	appearancesOutfit  = 2
	appearancesEffect  = 3
	appearancesMissile = 4
)

// Appearance fields.
const (
	appearanceID          = 1
	appearanceFrameGroup  = 2 // repeated
	appearanceFlags       = 3
	appearanceName        = 4
	appearanceDescription = 5
)

// FrameGroup fields.
const (
	frameGroupFixedType  = 1
	frameGroupID         = 2
	frameGroupSpriteInfo = 3
)

// Fixed frame groups.
const (
	groupOutfitIdle    = 0
	groupObjectInitial = 2
)

// SpriteInfo fields.
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

// SpriteAnimation fields.
const (
	animationDefaultStartPhase = 1
	animationSynchronized      = 2
	animationRandomStartPhase  = 3
	animationLoopType          = 4
	animationLoopCount         = 5
	animationSpritePhase       = 6 // repeated
)

// Loop types of SpriteAnimation (ANIMATION_LOOP_TYPE).
const (
	loopPingPong = -1
	loopInfinite = 0
	loopCounted  = 1
)

// SpritePhase fields.
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

// AppearanceFlagHook directions.
const (
	hookDirectionSouth = 1
	hookDirectionEast  = 2
)

// AppearanceFlagLight fields.
const (
	lightBrightness = 1
	lightColor      = 2
)

// AppearanceFlagShift fields.
const (
	shiftX = 1
	shiftY = 2
)

// AppearanceFlagMarket fields.
const (
	marketCategory = 1
	marketTradeAs  = 2
	marketShowAs   = 3
	marketVocation = 5 // repeated PLAYER_PROFESSION
	marketMinLevel = 6
	marketName     = 7
)

// AppearanceFlagNPC fields (one NPC sale entry).
const (
	npcName      = 1
	npcLocation  = 2
	npcSalePrice = 3
	npcBuyPrice  = 4
	npcCurrency  = 5
	npcQuestFlag = 6
)

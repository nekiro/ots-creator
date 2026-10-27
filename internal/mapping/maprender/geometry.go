package maprender

import (
	"encoding/binary"
	"math"
	"slices"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// View flags the chunk geometry depends on (GeometryOptions.ViewFlags; see
// GEO in frontend/src/lib/map/render/geo.ts).
const (
	ViewPathing = 1 << iota
	ViewHouses
	ViewSpecial
	ViewZones
	ViewSpawns
	ViewHighlight
	ViewGhostItems
	ViewCreatures
	ViewOnlyModified
	ViewAnimation
	// ViewFar leaves out what does not shape the map seen from far away:
	// movable and pickupable items.
	ViewFar
	// ViewMarkedItems covers Scene.MarkedItems with a colored square.
	ViewMarkedItems
)

// Sprite values of the geometry besides sprite ids (quads of sprites are
// one sprite size a side): spriteHidden hides the quad (an animation frame
// without a sprite), spriteComposed marks a colored outfit piece (the low
// bits index the chunk's composed table) and spriteRect a colored rectangle
// (width in bits 15-29, height in bits 0-14).
const (
	spriteHidden   = 0xFFFFFFFF
	spriteComposed = 0x80000000
	spriteRect     = 0x40000000
	rectSizeBits   = 15
	rectSizeMax    = 1<<rectSizeBits - 1
)

const (
	MaxBlock = 8
	// maxElevation is the most items on a tile raise the items above them.
	maxElevation = 24
	// outfitCacheKey tags outfit ids in the thing cache (items use client
	// ids, which are below it).
	outfitCacheKey = 1 << 20
)

const (
	topCommon = 0
	topBorder = 1
	topBottom = 2
	topTop    = 3
)

// Hook positions of hangable items: the pattern x they are drawn with on a
// wall with hooks.
const (
	hookNone  = 0
	hookSouth = 1
	hookEast  = 2
)

type GeometryOptions struct {
	ViewFlags uint32
	House     uint32
	// Block makes every key stand for Block x Block chunks from it, built
	// into one geometry (fewer draws far out). 0 or 1: single chunks.
	Block int
}

type tint struct{ red, green, blue, alpha float64 }

var white = tint{1, 1, 1, 1}

func (color tint) scaled(factor float64) tint {
	return tint{color.red * factor, color.green * factor, color.blue * factor, color.alpha}
}

func (color tint) withAlpha(alpha float64) tint {
	color.alpha = alpha
	return color
}

var (
	unknownItemColor     = tint{0.8, 0, 0, 0.6}
	missingSpriteColor   = tint{0.8, 0, 0, 0.5}
	unknownCreatureColor = tint{1, 0.5, 0.2, 0.8}
	unknownNPCColor      = tint{0.3, 0.6, 1, 0.8}
	selectedSpawnColor   = tint{0.4, 0.6, 1, 0.3}
)

// Tints of the view overlays: factors the color channels of a tile are
// multiplied by.
const (
	selectedDim         = 0.5
	ghostItemAlpha      = 0.5
	markedItemAlpha     = 2.0 / 3
	emptyZoneAlpha      = 0.3
	blockingTint        = 2.0 / 3
	houseTint           = 0.5
	otherHouseTint      = 0.5
	protectionZoneTint  = 0.5
	noLogoutTint        = 0.5
	noPVPTint           = 0.5
	pvpZoneGreenFromRed = 0.25
	pvpZoneBlueTint     = 2.0 / 3
	zoneRedTint         = 0.85
	zoneGreenTint       = 0.7
	spawnTint           = 0.7
)

// highlightByItemCount darkens tiles by how many items they hold (1 to 5
// or more).
var highlightByItemCount = [...]float64{0.75, 0.6, 0.48, 0.4, 0.33}

// Patterns of fluids (8 colors in rows of 4) and stacks (4 x 2 counts).
const (
	fluidColors         = 8
	fluidPatternColumns = 4
	stackPatternColumns = 4
	stackPatternRows    = 2
)

// stackCountThresholds start the pattern columns of the second stack row.
var stackCountThresholds = [...]int{5, 10, 25, 50}

type thingLook struct {
	frameGroup       *thing.FrameGroup
	offsetX, offsetY int
	elevation        int
	hangable         bool
	// lightLevel and lightColor: the light the thing gives (level 0: none).
	lightLevel, lightColor uint8
	lightOrigins           map[int]lightOrigin
	// ground, groundBorder, fullGround and translucent are client flags
	// (see canShade).
	ground, groundBorder, fullGround, translucent bool
	// opacity: opacityUnknown until isOpaque computes it.
	opacity uint8
}

// geometryBuilder builds the quads of chunks. It is not safe for
// concurrent use: every worker has its own (and its own thing cache).
type geometryBuilder struct {
	scene   *Scene
	options GeometryOptions
	// looks caches client things: item client ids, outfits tagged with
	// outfitCacheKey.
	looks map[uint32]*thingLook
	// quads, animated and composed are the sections of the chunk being
	// built (see chunk), with their entry counts.
	quads         []byte
	quadCount     int
	animated      []byte
	animatedCount int
	composed      []byte
	composedCount int
	lights        []byte
	lightCount    int
	// shades holds shadePlanes bit planes, each one bit per tile row by
	// row: the tiles that hide the light of the floors under them (see
	// canShade), those of them that only dim it (hasTranslucentThing) and
	// the tiles with a ground (hasGround).
	shades    []byte
	hasShades bool
	sprites   map[uint32]struct{}
	// chunks are the chunks of the key being built, row by row; scratch
	// and scratchItems hold the packed tile being drawn.
	chunks       []*otmap.Chunk
	scratch      otmap.Tile
	scratchItems [16]otmap.Item
	tileX, tileY uint16
	// onTop is set while the tile's top items are drawn: their quads are
	// tagged (tileTop) so creatures drawn later can go under them.
	onTop bool
}

func newGeometryBuilder(scene *Scene, options GeometryOptions) *geometryBuilder {
	return &geometryBuilder{scene: scene, options: options, looks: map[uint32]*thingLook{}, sprites: map[uint32]struct{}{}}
}

func (b *geometryBuilder) view(flag uint32) bool { return b.options.ViewFlags&flag != 0 }

// tileTop is set in the tile y of the quads of top items: the renderer
// draws creatures of the play view between a tile's items and its top
// items.
const tileTop = 0x8000

// quad appends one quad, 16 bytes: int16 x, y (world pixels from the
// chunk's corner), uint32 sprite (see spriteRect), rgba8 color, uint16 x, y
// of the tile it belongs to (from the chunk's corner, y with tileTop on top
// items; the renderer orders overlapping quads of different chunks by it).
func (b *geometryBuilder) quad(x, y int, sprite uint32, color tint) {
	b.quads = binary.LittleEndian.AppendUint16(b.quads, uint16(int16(x)))
	b.quads = binary.LittleEndian.AppendUint16(b.quads, uint16(int16(y)))
	b.quads = binary.LittleEndian.AppendUint32(b.quads, sprite)
	b.quads = append(b.quads, unitByte(color.red), unitByte(color.green), unitByte(color.blue), unitByte(color.alpha))
	tileY := b.tileY
	if b.onTop {
		tileY |= tileTop
	}
	b.quads = binary.LittleEndian.AppendUint16(b.quads, b.tileX)
	b.quads = binary.LittleEndian.AppendUint16(b.quads, tileY)
	b.quadCount++
}

func unitByte(value float64) uint8 {
	return uint8(math.Round(min(max(value, 0), 1) * 255))
}

func (b *geometryBuilder) rect(x, y, width, height int, color tint) {
	size := uint32(min(max(width, 0), rectSizeMax))<<rectSizeBits | uint32(min(max(height, 0), rectSizeMax))
	b.quad(x, y, spriteRect|size, color)
}

func (b *geometryBuilder) lookOf(category thing.Category, id uint32) *thingLook {
	cacheKey := id
	if category == thing.CategoryOutfit {
		cacheKey |= outfitCacheKey
	}
	if look, ok := b.looks[cacheKey]; ok {
		return look
	}
	var look *thingLook
	if found, err := b.scene.Things.Thing(category, id); err == nil && len(found.FrameGroups) > 0 {
		group := found.FrameGroups[0]
		if group.PatternX > 0 && group.PatternY > 0 && group.PatternZ > 0 && group.Width > 0 && group.Height > 0 && group.Layers > 0 {
			look = &thingLook{frameGroup: group, hangable: found.Props.Hangable, ground: found.Props.Ground, groundBorder: found.Props.GroundBorder, fullGround: found.Props.FullGround, translucent: found.Props.Translucent}
			if found.Props.HasOffset {
				look.offsetX, look.offsetY = int(found.Props.OffsetX), int(found.Props.OffsetY)
			}
			if found.Props.HasElevation {
				look.elevation = int(found.Props.Elevation)
			}
			if found.Props.HasLight && found.Props.LightLevel > 0 {
				look.lightLevel, look.lightColor = uint8(min(found.Props.LightLevel, 255)), uint8(found.Props.LightColor)
			}
		}
	}
	b.looks[cacheKey] = look
	return look
}

type pattern struct{ x, y, z int }

// drawLook emits the pieces of one texture with its bottom-right tile at
// (anchorX, anchorY). recordFrames records the pieces of animated items
// (clientID) for frame patching; colors makes composed outfit pieces.
func (b *geometryBuilder) drawLook(look *thingLook, clientID uint32, at pattern, anchorX, anchorY int, color tint, recordFrames bool, colors *[4]uint8) {
	group := look.frameGroup
	spriteSize := b.scene.SpriteSize
	layers := int(group.Layers)
	if colors != nil {
		layers = 1
	}
	recordFrames = recordFrames && colors == nil && group.Frames > 1
	anchorX -= look.offsetX
	anchorY -= look.offsetY
	for layer := 0; layer < layers; layer++ {
		for row := 0; row < int(group.Height); row++ {
			for column := 0; column < int(group.Width); column++ {
				index := group.SpriteIndex(column, row, layer, at.x, at.y, at.z, 0)
				if index >= len(group.Sprites) {
					continue
				}
				spriteID := group.Sprites[index]
				x, y := anchorX-column*spriteSize, anchorY-row*spriteSize
				if recordFrames {
					b.animated = binary.LittleEndian.AppendUint32(b.animated, uint32(b.quadCount))
					b.animated = binary.LittleEndian.AppendUint32(b.animated, clientID)
					b.animated = binary.LittleEndian.AppendUint32(b.animated, uint32(index))
					b.animatedCount++
				}
				switch {
				case spriteID == 0:
					if recordFrames {
						b.quad(x, y, spriteHidden, color)
					}
				case colors != nil:
					b.composedPiece(group, index, spriteID, x, y, color, colors)
				default:
					b.sprites[spriteID] = struct{}{}
					b.quad(x, y, spriteID, color)
				}
			}
		}
	}
}

// composedPiece emits an outfit piece colored with its template sprite,
// the sprite of the next layer.
func (b *geometryBuilder) composedPiece(group *thing.FrameGroup, index int, spriteID uint32, x, y int, color tint, colors *[4]uint8) {
	templateSprite := uint32(0)
	if templateIndex := index + int(group.Height)*int(group.Width); templateIndex < len(group.Sprites) {
		templateSprite = group.Sprites[templateIndex]
	}
	b.composed = binary.LittleEndian.AppendUint32(b.composed, uint32(b.quadCount))
	b.composed = binary.LittleEndian.AppendUint32(b.composed, spriteID)
	b.composed = binary.LittleEndian.AppendUint32(b.composed, templateSprite)
	b.composed = append(b.composed, colors[0], colors[1], colors[2], colors[3])
	b.quad(x, y, spriteComposed|uint32(b.composedCount), color)
	b.composedCount++
}

func (b *geometryBuilder) item(item otmap.Item, position tilePosition, anchorX, anchorY int, color tint, hook int) int {
	spriteSize := b.scene.SpriteSize
	itemType := b.scene.Items.Get(item.ID)
	if itemType == nil {
		b.rect(anchorX, anchorY, spriteSize, spriteSize, unknownItemColor.withAlpha(unknownItemColor.alpha*color.alpha))
		return 0
	}
	if markColor, marked := b.scene.MarkedItems[item.ID]; marked && b.view(ViewMarkedItems) {
		defer b.rect(anchorX, anchorY, spriteSize, spriteSize, tint{float64(markColor[0]) / 255, float64(markColor[1]) / 255, float64(markColor[2]) / 255, markedItemAlpha * color.alpha})
	}
	if itemType.Flags.Has(otitems.FlagMeta) {
		return 0
	}
	look := b.lookOf(thing.CategoryItem, uint32(itemType.ClientID))
	if look == nil {
		b.rect(anchorX, anchorY, spriteSize, spriteSize, missingSpriteColor.withAlpha(missingSpriteColor.alpha*color.alpha))
		return 0
	}
	at := itemPattern(itemType, look, item.Subtype, position, hook)
	b.drawLook(look, uint32(itemType.ClientID), at, anchorX, anchorY, color, b.view(ViewAnimation), nil)
	if look.lightLevel > 0 {
		b.light(look, at, anchorX, anchorY)
	}
	return look.elevation
}

// itemPattern picks the pattern the client draws an item with: by fluid
// color, hook, stack count or position.
func itemPattern(itemType *otitems.Type, look *thingLook, subtype uint16, position tilePosition, hook int) pattern {
	group := look.frameGroup
	at := pattern{z: position.z % int(group.PatternZ)}
	switch {
	case itemType.Flags&(otitems.FlagSplash|otitems.FlagFluidContainer) != 0:
		fluid := int(subtype) % fluidColors
		at.x, at.y = fluid%fluidPatternColumns%int(group.PatternX), fluid/fluidPatternColumns%int(group.PatternY)
	case look.hangable && hook != hookNone:
		at.x = min(hook, int(group.PatternX)-1)
	case itemType.Flags.Has(otitems.FlagStackable) && group.PatternX == stackPatternColumns && group.PatternY == stackPatternRows:
		at.x, at.y = stackPattern(int(subtype))
	default:
		at.x, at.y = position.x%int(group.PatternX), position.y%int(group.PatternY)
	}
	return at
}

// stackPattern is the pattern of a stack of count items: counts 1 to 4
// have one each in the first row, larger stacks share the second row.
func stackPattern(count int) (x, y int) {
	switch {
	case count <= 0:
		return 0, 0
	case count < stackCountThresholds[0]:
		return count - 1, 0
	}
	for column, threshold := range stackCountThresholds[1:] {
		if count < threshold {
			return column, 1
		}
	}
	return len(stackCountThresholds) - 1, 1
}

// creature emits a creature as the client draws it standing still.
func (b *geometryBuilder) creature(creature otmap.Creature, position tilePosition, anchorX, anchorY int) {
	spriteSize := b.scene.SpriteSize
	color := white
	if creature.Selected {
		color = white.scaled(selectedDim)
	}
	outfit := b.scene.Outfit(creature.Name)
	if outfit.LookItem != 0 {
		b.item(otmap.Item{ID: outfit.LookItem}, position, anchorX, anchorY, color, hookNone)
		return
	}
	if outfit.LookType == 0 {
		marker := unknownCreatureColor
		if creature.NPC {
			marker = unknownNPCColor
		}
		markerSize := spriteSize / 2
		b.rect(anchorX+markerSize/2, anchorY+markerSize/2, markerSize, markerSize, marker)
		return
	}
	look := b.lookOf(thing.CategoryOutfit, uint32(outfit.LookType))
	if look == nil {
		return
	}
	b.outfit(look, outfit, int(creature.Direction&3), anchorX, anchorY, color)
}

func (b *geometryBuilder) outfit(look *thingLook, outfit materials.Outfit, direction, anchorX, anchorY int, color tint) {
	group := look.frameGroup
	direction = min(direction, int(group.PatternX)-1)
	mounted := 0
	if outfit.Mount != 0 && group.PatternZ > 1 {
		if mount := b.lookOf(thing.CategoryOutfit, uint32(outfit.Mount)); mount != nil {
			mountDirection := min(direction, int(mount.frameGroup.PatternX)-1)
			b.drawLook(mount, 0, pattern{x: mountDirection}, anchorX, anchorY, color, false, nil)
			mounted = 1
		}
	}
	var colors *[4]uint8
	if group.Layers > 1 {
		colors = &[4]uint8{outfit.Head, outfit.Body, outfit.Legs, outfit.Feet}
	}
	b.drawLook(look, 0, pattern{x: direction, z: mounted}, anchorX, anchorY, color, false, colors)
	// Addon n (bit n) is drawn with pattern y n.
	for _, addon := range [...]int{1, 2} {
		if int(outfit.Addons)&addon != 0 && int(group.PatternY) > addon {
			b.drawLook(look, 0, pattern{x: direction, y: addon, z: mounted}, anchorX, anchorY, color, false, colors)
		}
	}
}

type tilePosition struct{ x, y, z int }

// tileTint is the color a tile's ground and borders are drawn with: the
// view overlays (pathing, houses, zones...) darken channels.
func (b *geometryBuilder) tileTint(tile *otmap.Tile, hasThings bool) tint {
	color := white
	special := b.view(ViewSpecial)
	if b.view(ViewPathing) && hasThings && b.scene.IsBlocking(tile) {
		color.green *= blockingTint
		color.blue *= blockingTint
	}
	if b.view(ViewHouses) && tile.HouseID != 0 {
		color.red *= houseTint
		if tile.HouseID != b.options.House {
			color.green *= otherHouseTint
		}
	} else if special && tile.Flags&otmap.FlagPZ != 0 {
		color.red *= protectionZoneTint
		color.blue *= protectionZoneTint
	}
	if special && tile.Flags&otmap.FlagPVPZone != 0 {
		color.green = color.red * pvpZoneGreenFromRed
		color.blue *= pvpZoneBlueTint
	}
	if special && tile.Flags&otmap.FlagNoLogout != 0 {
		color.blue *= noLogoutTint
	}
	if special && tile.Flags&otmap.FlagNoPVP != 0 {
		color.green *= noPVPTint
	}
	if b.view(ViewZones) && len(tile.Zones()) > 0 {
		color.red *= zoneRedTint
		color.green *= zoneGreenTint
	}
	if b.view(ViewSpawns) && (tile.Spawn() != nil || tile.SpawnNPC() != nil) {
		color.green *= spawnTint
		color.blue *= spawnTint
	}
	if b.view(ViewHighlight) && len(tile.Items) > 0 {
		factor := highlightByItemCount[min(len(tile.Items), len(highlightByItemCount))-1]
		color.red *= factor
		color.green *= factor
	}
	return color
}

// tileHook is where hangable items of the tile hang: on its last wall
// with a hook.
func (b *geometryBuilder) tileHook(tile *otmap.Tile) int {
	hook := hookNone
	for _, item := range tile.Items {
		if itemType := b.scene.Items.Get(item.ID); itemType != nil {
			if itemType.Flags.Has(otitems.FlagHookSouth) {
				hook = hookSouth
			} else if itemType.Flags.Has(otitems.FlagHookEast) {
				hook = hookEast
			}
		}
	}
	return hook
}

func selectionDim(item otmap.Item) float64 {
	if item.Selected {
		return selectedDim
	}
	return 1
}

// tile emits one tile, stacked as the client does: ground, borders,
// bottom items, common items, creatures, top items.
func (b *geometryBuilder) tile(tile *otmap.Tile, position tilePosition, anchorX, anchorY int) {
	spriteSize := b.scene.SpriteSize
	things := len(tile.Items)
	if tile.Ground.ID != 0 {
		things++
	}
	color := b.tileTint(tile, things > 0)
	hook := b.tileHook(tile)
	elevation := 0
	if b.canShade(tile) {
		b.shade(shadePlane)
		if b.hasTranslucentThing(tile) {
			b.shade(translucentPlane)
		}
	}
	if b.hasGround(tile) {
		b.shade(groundPlane)
	}
	if tile.Ground.ID != 0 {
		elevation += b.item(tile.Ground, position, anchorX, anchorY, color.scaled(selectionDim(tile.Ground)), hook)
	}
	var topItems []otmap.Item
	for _, pass := range [...]uint8{topBorder, topBottom, topCommon} {
		for _, item := range tile.Items {
			itemType := b.scene.Items.Get(item.ID)
			topOrder := uint8(topCommon)
			if itemType != nil {
				topOrder = itemType.TopOrder
			}
			if topOrder == topTop {
				if pass == topCommon {
					topItems = append(topItems, item)
				}
				continue
			}
			if topOrder != pass || b.skip(itemType) {
				continue
			}
			itemColor := white
			if topOrder == topBorder || topOrder == topBottom {
				itemColor = color
			}
			itemColor = itemColor.scaled(selectionDim(item))
			if b.view(ViewGhostItems) && topOrder == topCommon && itemType != nil && itemType.Flags.Has(otitems.FlagPickupable) {
				itemColor.alpha = ghostItemAlpha
			}
			elevation += b.item(item, position, anchorX-elevation, anchorY-elevation, itemColor, hook)
			elevation = min(elevation, maxElevation)
		}
	}
	creatures := tile.Creatures()
	if b.view(ViewCreatures) {
		for _, creature := range creatures {
			b.creature(creature, position, anchorX-elevation, anchorY-elevation)
		}
	}
	// Top items start again from the tile's corner (client: Tile::drawTop).
	b.onTop = true
	topElevation := 0
	for _, item := range topItems {
		if b.skip(b.scene.Items.Get(item.ID)) {
			continue
		}
		topElevation += b.item(item, position, anchorX-topElevation, anchorY-topElevation, white.scaled(selectionDim(item)), hook)
		topElevation = min(topElevation, maxElevation)
	}
	b.onTop = false
	if things == 0 && b.view(ViewSpecial|ViewHouses) && (tile.Flags&otmap.FlagPZ != 0 || tile.HouseID != 0) {
		b.rect(anchorX, anchorY, spriteSize, spriteSize, color.withAlpha(emptyZoneAlpha))
	}
	if tile.Selected() && things == 0 && len(creatures) == 0 && (tile.Spawn() != nil || tile.SpawnNPC() != nil) {
		b.rect(anchorX, anchorY, spriteSize, spriteSize, selectedSpawnColor)
	}
}

func (b *geometryBuilder) skip(itemType *otitems.Type) bool {
	return b.view(ViewFar) && itemType != nil && itemType.Flags&(otitems.FlagMoveable|otitems.FlagPickupable) != 0
}

// chunk appends the geometry of one chunk to out:
//
//	u32 key, u32 quads, u32 animated, u32 composed, u32 sprites, u32
//	lights, then quads (16 bytes each, see quad), animated pieces (u32
//	quad, u32 client item id, u32 frame 0 sprite index), composed pieces
//	(u32 quad, u32 sprite, u32 template sprite, u8 head, body, legs, feet),
//	sprite ids (u32 each, every plain sprite the quads use), light sources
//	(8 bytes each, see light), then u32 shade bytes and the shade bit
//	planes (see shades; none when no tile has a bit set).
func (b *geometryBuilder) chunk(key otmap.ChunkKey, out []byte) []byte {
	b.quads, b.quadCount = b.quads[:0], 0
	b.animated, b.animatedCount = b.animated[:0], 0
	b.composed, b.composedCount = b.composed[:0], 0
	b.lights, b.lightCount = b.lights[:0], 0
	clear(b.sprites)
	chunkX, chunkY, floor := key.Split()
	originX, originY := int(chunkX)<<otmap.ChunkBits, int(chunkY)<<otmap.ChunkBits
	block := max(b.options.Block, 1)
	b.chunks = b.chunks[:0]
	found := false
	for dy := 0; dy < block; dy++ {
		for dx := 0; dx < block; dx++ {
			chunk := b.scene.Map.Chunk(otmap.MakeKey(chunkX+uint16(dx), chunkY+uint16(dy), floor))
			b.chunks = append(b.chunks, chunk)
			found = found || chunk != nil
		}
	}
	// Tiles diagonal by diagonal from the top-left corner, each diagonal
	// from its bottom-left end, as the client draws them: a sprite reaches
	// up and left of its tile, over tiles of earlier diagonals and over
	// the tile left of it, which the tile below-left of it (on the same
	// diagonal) reaches too and is drawn before it.
	size := block << otmap.ChunkBits
	shadeBytes := shadePlanes * size * size / 8
	b.shades, b.hasShades = slices.Grow(b.shades[:0], shadeBytes)[:shadeBytes], false
	clear(b.shades)
	spriteSize := b.scene.SpriteSize
	for diagonal := 0; found && diagonal < 2*size-1; diagonal++ {
		for localX := max(0, diagonal-size+1); localX <= min(diagonal, size-1); localX++ {
			localY := diagonal - localX
			chunk := b.chunks[(localY>>otmap.ChunkBits)*block+localX>>otmap.ChunkBits]
			if chunk == nil {
				continue
			}
			tile := chunk.Peek(localX&(otmap.ChunkSize-1), localY&(otmap.ChunkSize-1), &b.scratch, b.scratchItems[:])
			if tile == nil || b.view(ViewOnlyModified) && tile.State&otmap.StateModified == 0 {
				continue
			}
			b.tileX, b.tileY = uint16(localX), uint16(localY)
			position := tilePosition{x: originX + localX, y: originY + localY, z: int(floor)}
			b.tile(tile, position, localX*spriteSize, localY*spriteSize)
		}
	}
	out = binary.LittleEndian.AppendUint32(out, uint32(key))
	out = binary.LittleEndian.AppendUint32(out, uint32(b.quadCount))
	out = binary.LittleEndian.AppendUint32(out, uint32(b.animatedCount))
	out = binary.LittleEndian.AppendUint32(out, uint32(b.composedCount))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(b.sprites)))
	out = binary.LittleEndian.AppendUint32(out, uint32(b.lightCount))
	out = append(out, b.quads...)
	out = append(out, b.animated...)
	out = append(out, b.composed...)
	for spriteID := range b.sprites {
		out = binary.LittleEndian.AppendUint32(out, spriteID)
	}
	out = append(out, b.lights...)
	if !b.hasShades {
		return binary.LittleEndian.AppendUint32(out, 0)
	}
	out = binary.LittleEndian.AppendUint32(out, uint32(len(b.shades)))
	return append(out, b.shades...)
}

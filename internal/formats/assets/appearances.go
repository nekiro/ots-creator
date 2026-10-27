package assets

import (
	"fmt"
	"slices"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/binio"
)

// The editor works with 32x32 pieces like dat/spr clients. An official
// sprite of 64x64 pixels becomes 2x2 pieces of a frame group with width and
// height 2; piece k of a sprite is the tile (k%w, k/w) counted from the
// bottom-right corner, the order thing.FrameGroup uses for its tiles.

type SpriteMap interface {
	Pieces(id uint32) (first uint32, width, height int, ok bool)
}

// Resolver returns the official sprite id for the pieces of one texture
// (width*height piece ids in tile order).
type Resolver func(pieces []uint32, width, height int) (uint32, error)

// Original is kept in thing.Thing.Extra of decoded things. Things the
// editor did not touch are written back byte for byte, and the fields the
// model does not know (names, NPC sale data, newer flags) survive edits.
type Original struct {
	raw     []byte
	thing   *thing.Thing
	sprites [][]uint32 // official sprite ids per frame group
}

// File is a decoded appearances file. Things holds every category from its
// first id; ids missing from the file are blank things that are not written
// back.
type File struct {
	Things map[thing.Category][]*thing.Thing
	rest   []byte // other top level fields (special meaning ids)
}

var listFields = map[thing.Category]protowire.Number{
	thing.CategoryItem:    appearancesObject,
	thing.CategoryOutfit:  appearancesOutfit,
	thing.CategoryEffect:  appearancesEffect,
	thing.CategoryMissile: appearancesMissile,
}

func Decode(data []byte, spriteMap SpriteMap) (*File, error) {
	root, err := parse(data)
	if err != nil {
		return nil, err
	}
	file := &File{Things: map[thing.Category][]*thing.Thing{}}
	for _, entry := range root {
		category := categoryOf(entry.num)
		if category == thing.CategoryInvalid {
			file.rest = append(file.rest, entry.raw...)
			continue
		}
		if entry.typ != protowire.BytesType {
			return nil, errWire
		}
		appearance, err := decodeAppearance(entry.payload, category, spriteMap)
		if err != nil {
			return nil, err
		}
		if appearance.ID < category.MinID() {
			return nil, fmt.Errorf("appearances: %s id %d is below %d", category, appearance.ID, category.MinID())
		}
		list := file.Things[category]
		i := int(appearance.ID - category.MinID())
		for len(list) <= i {
			list = append(list, nil)
		}
		if list[i] != nil {
			return nil, fmt.Errorf("appearances: duplicate %s %d", category, appearance.ID)
		}
		list[i] = appearance
		file.Things[category] = list
	}
	for _, category := range thing.Categories {
		list := file.Things[category]
		for i, appearance := range list {
			if appearance == nil {
				list[i] = thing.New(category.MinID()+uint32(i), category)
			}
		}
		if len(list) == 0 {
			file.Things[category] = []*thing.Thing{thing.New(category.MinID(), category)}
		}
	}
	return file, nil
}

func categoryOf(num protowire.Number) thing.Category {
	for category, listNum := range listFields {
		if listNum == num {
			return category
		}
	}
	return thing.CategoryInvalid
}

func decodeAppearance(raw []byte, category thing.Category, spriteMap SpriteMap) (*thing.Thing, error) {
	msg, err := parse(raw)
	if err != nil {
		return nil, err
	}
	appearance := &thing.Thing{ID: uint32(msg.uint(appearanceID)), Category: category}
	// Texts are Latin-1 like in the dat format.
	if name, ok := msg.get(appearanceName); ok {
		appearance.Name = binio.DecodeLatin1(name.payload)
	}
	if description, ok := msg.get(appearanceDescription); ok {
		appearance.Description = binio.DecodeLatin1(description.payload)
	}
	decodeFlags(msg.sub(appearanceFlags), &appearance.Props)
	appearance.NpcSales = decodeNpcSales(msg.sub(appearanceFlags))
	original := &Original{raw: raw, thing: appearance}
	for _, groupMsg := range msg.all(appearanceFrameGroup) {
		if category != thing.CategoryOutfit && len(appearance.FrameGroups) == 1 {
			break // only outfits have several groups in the editor
		}
		group, spriteIDs, err := decodeGroup(groupMsg, category, spriteMap)
		if err != nil {
			return nil, fmt.Errorf("appearances: %s %d: %w", category, appearance.ID, err)
		}
		appearance.FrameGroups = append(appearance.FrameGroups, group)
		original.sprites = append(original.sprites, spriteIDs)
	}
	if len(appearance.FrameGroups) == 0 {
		return nil, fmt.Errorf("appearances: %s %d has no frame group", category, appearance.ID)
	}
	appearance.Extra = original
	return appearance, nil
}

// countByte converts a layout count to a byte; an unset count means 1.
func countByte(value uint64, what string) (uint8, error) {
	if value == 0 {
		value = 1
	}
	if value > 255 {
		return 0, fmt.Errorf("%s %d is too large", what, value)
	}
	return uint8(value), nil
}

func decodeGroup(groupMsg message, category thing.Category, spriteMap SpriteMap) (*thing.FrameGroup, []uint32, error) {
	spriteInfo := groupMsg.sub(frameGroupSpriteInfo)
	group := &thing.FrameGroup{}
	if category == thing.CategoryOutfit && groupMsg.uint(frameGroupFixedType) != groupOutfitIdle {
		group.Type = thing.FrameGroupWalking
	}
	var err error
	if group.PatternX, err = countByte(spriteInfo.uint(spriteInfoPatternWidth), "pattern width"); err != nil {
		return nil, nil, err
	}
	if group.PatternY, err = countByte(spriteInfo.uint(spriteInfoPatternHeight), "pattern height"); err != nil {
		return nil, nil, err
	}
	if group.PatternZ, err = countByte(spriteInfo.uint(spriteInfoPatternDepth), "pattern depth"); err != nil {
		return nil, nil, err
	}
	if group.Layers, err = countByte(spriteInfo.uint(spriteInfoLayers), "layers"); err != nil {
		return nil, nil, err
	}
	animation := spriteInfo.sub(spriteInfoAnimation)
	phases := animation.all(animationSpritePhase)
	if group.Frames, err = countByte(uint64(len(phases)), "phase count"); err != nil {
		return nil, nil, err
	}
	if group.Frames > 1 {
		decodeAnimation(group, animation, phases)
	}
	wideIDs := spriteInfo.uints(spriteInfoSpriteID)
	spriteIDs := make([]uint32, len(wideIDs))
	for i, id := range wideIDs {
		spriteIDs[i] = uint32(id)
	}
	textures := group.TotalTextures()
	if len(spriteIDs) != textures {
		return nil, nil, fmt.Errorf("%d sprites for %d textures", len(spriteIDs), textures)
	}
	width, height := 1, 1
	if len(spriteIDs) > 0 {
		if _, spriteWidth, spriteHeight, ok := spriteMap.Pieces(spriteIDs[0]); ok {
			width, height = spriteWidth, spriteHeight
		}
	}
	group.Width, group.Height = uint8(width), uint8(height)
	group.ExactSize = uint8(min(max(spriteInfo.uint(spriteInfoBoundingSquare), uint64(TileSize*max(width, height))), 255))
	piecesPerTexture := width * height
	group.Sprites = make([]uint32, textures*piecesPerTexture)
	for i, id := range spriteIDs {
		first, spriteWidth, spriteHeight, ok := spriteMap.Pieces(id)
		if !ok {
			continue // missing sprites show as empty tiles
		}
		for tileRow := range min(spriteHeight, height) {
			for tileColumn := range min(spriteWidth, width) {
				group.Sprites[i*piecesPerTexture+tileRow*width+tileColumn] = first + uint32(tileRow*spriteWidth+tileColumn)
			}
		}
	}
	return group, spriteIDs, nil
}

func decodeAnimation(group *thing.FrameGroup, animation message, phases []message) {
	if animation.bool(animationSynchronized) {
		group.Mode = thing.AnimationSync
	}
	switch int32(animation.uint(animationLoopType)) {
	case loopPingPong:
		group.LoopCount = -1
	case loopCounted:
		group.LoopCount = int32(animation.uint(animationLoopCount))
	}
	if animation.bool(animationRandomStartPhase) {
		group.StartFrame = -1
	} else if startPhase := animation.uint(animationDefaultStartPhase); startPhase < uint64(group.Frames) {
		group.StartFrame = int8(min(startPhase, 127))
	}
	for _, phase := range phases {
		group.Durations = append(group.Durations, thing.FrameDuration{
			Min: uint32(phase.uint(phaseDurationMin)),
			Max: uint32(phase.uint(phaseDurationMax)),
		})
	}
}

// Blank reports whether an appearance is a placeholder for an id missing from the file
// (or an object removed in the editor): nothing about it is worth writing.
func Blank(appearance *thing.Thing) bool {
	if appearance.Extra != nil || appearance.Props != (thing.Properties{}) || appearance.Name != "" || appearance.Description != "" || len(appearance.NpcSales) > 0 {
		return false
	}
	for _, group := range appearance.FrameGroups {
		for _, id := range group.Sprites {
			if id != 0 {
				return false
			}
		}
	}
	return true
}

func Encode(file *File, resolve Resolver) ([]byte, error) {
	var out []byte
	for _, category := range thing.Categories {
		for _, appearance := range file.Things[category] {
			if appearance == nil || Blank(appearance) {
				continue
			}
			encoded, err := encodeAppearance(appearance, resolve)
			if err != nil {
				return nil, err
			}
			out = putBytes(out, listFields[category], encoded)
		}
	}
	return append(out, file.rest...), nil
}

func textureIDs(appearance *thing.Thing, group *thing.FrameGroup, resolve Resolver) ([]uint32, error) {
	width, height := int(group.Width), int(group.Height)
	if _, ok := SpriteTypeFor(width, height); !ok {
		return nil, fmt.Errorf("%s %d is %dx%d tiles; assets support up to 2x2", appearance.Category, appearance.ID, width, height)
	}
	piecesPerTexture := width * height
	spriteIDs := make([]uint32, group.TotalTextures())
	for i := range spriteIDs {
		id, err := resolve(group.Sprites[i*piecesPerTexture:(i+1)*piecesPerTexture], width, height)
		if err != nil {
			return nil, fmt.Errorf("%s %d: %w", appearance.Category, appearance.ID, err)
		}
		spriteIDs[i] = id
	}
	return spriteIDs, nil
}

func encodeAppearance(appearance *thing.Thing, resolve Resolver) ([]byte, error) {
	if err := appearance.Validate(); err != nil {
		return nil, err
	}
	spriteIDs := make([][]uint32, len(appearance.FrameGroups))
	for i, group := range appearance.FrameGroups {
		var err error
		if spriteIDs[i], err = textureIDs(appearance, group, resolve); err != nil {
			return nil, err
		}
	}
	original, _ := appearance.Extra.(*Original)
	if original != nil && original.thing == appearance && slices.EqualFunc(spriteIDs, original.sprites, slices.Equal) {
		return original.raw, nil
	}
	var originalMsg message
	var originalGroups []message
	var originalSpriteIDs [][]uint32
	if original != nil {
		originalMsg, _ = parse(original.raw)
		originalGroups = originalMsg.all(appearanceFrameGroup)
		originalSpriteIDs = original.sprites
	}
	out := putUint(nil, appearanceID, uint64(appearance.ID))
	for groupIndex, group := range appearance.FrameGroups {
		var originalGroup message
		var originalIDs []uint32
		if groupIndex < len(originalGroups) {
			originalGroup = originalGroups[groupIndex]
		}
		if groupIndex < len(originalSpriteIDs) {
			originalIDs = originalSpriteIDs[groupIndex]
		}
		sameSprites := slices.Equal(spriteIDs[groupIndex], originalIDs)
		out = putBytes(out, appearanceFrameGroup, encodeGroup(appearance.Category, group, spriteIDs[groupIndex], originalGroup, sameSprites))
	}
	out = putBytes(out, appearanceFlags, encodeFlags(&appearance.Props, appearance.NpcSales, originalMsg.sub(appearanceFlags)))
	if appearance.Name != "" {
		out = putBytes(out, appearanceName, binio.EncodeLatin1(appearance.Name))
	}
	if appearance.Description != "" {
		out = putBytes(out, appearanceDescription, binio.EncodeLatin1(appearance.Description))
	}
	return originalMsg.appendOthers(out, appearanceID, appearanceFrameGroup, appearanceFlags, appearanceName, appearanceDescription), nil
}

func encodeGroup(category thing.Category, group *thing.FrameGroup, spriteIDs []uint32, originalGroup message, sameSprites bool) []byte {
	fixedType := uint64(groupObjectInitial)
	if category == thing.CategoryOutfit {
		fixedType = uint64(group.Type)
	}
	out := putUint(nil, frameGroupFixedType, fixedType)
	out = putUint(out, frameGroupID, fixedType) // the official files use the group type as id
	out = putBytes(out, frameGroupSpriteInfo, encodeSpriteInfo(group, spriteIDs, originalGroup.sub(frameGroupSpriteInfo), sameSprites))
	return originalGroup.appendOthers(out, frameGroupFixedType, frameGroupID, frameGroupSpriteInfo)
}

func encodeSpriteInfo(group *thing.FrameGroup, spriteIDs []uint32, originalInfo message, sameSprites bool) []byte {
	out := putUint(nil, spriteInfoPatternWidth, uint64(group.PatternX))
	out = putUint(out, spriteInfoPatternHeight, uint64(group.PatternY))
	out = putUint(out, spriteInfoPatternDepth, uint64(group.PatternZ))
	out = putUint(out, spriteInfoLayers, uint64(group.Layers))
	for _, id := range spriteIDs {
		out = putUint(out, spriteInfoSpriteID, uint64(id))
	}
	if group.Frames > 1 {
		out = putBytes(out, spriteInfoAnimation, encodeAnimation(group, originalInfo.sub(spriteInfoAnimation)))
	}
	// is_opaque lets the client skip drawing what lies below; it is only
	// kept while the sprites are the same.
	if opaque, ok := originalInfo.get(spriteInfoIsOpaque); ok && sameSprites {
		out = append(out, opaque.raw...)
	} else {
		out = putUint(out, spriteInfoIsOpaque, 0)
	}
	return originalInfo.appendOthers(out, spriteInfoPatternWidth, spriteInfoPatternHeight, spriteInfoPatternDepth,
		spriteInfoLayers, spriteInfoSpriteID, spriteInfoAnimation, spriteInfoIsOpaque)
}

func encodeAnimation(group *thing.FrameGroup, originalAnimation message) []byte {
	var out []byte
	if group.StartFrame > 0 {
		out = putUint(out, animationDefaultStartPhase, uint64(group.StartFrame))
	}
	out = putUint(out, animationSynchronized, boolToUint(group.Mode == thing.AnimationSync))
	out = putBool(out, animationRandomStartPhase, group.StartFrame < 0)
	switch {
	case group.LoopCount < 0:
		out = putInt(out, animationLoopType, loopPingPong)
	case group.LoopCount == 0:
		out = putInt(out, animationLoopType, loopInfinite)
	default:
		out = putInt(out, animationLoopType, loopCounted)
		out = putUint(out, animationLoopCount, uint64(group.LoopCount))
	}
	for _, duration := range group.Durations {
		phase := putUint(nil, phaseDurationMin, uint64(duration.Min))
		phase = putUint(phase, phaseDurationMax, uint64(duration.Max))
		out = putBytes(out, animationSpritePhase, phase)
	}
	return originalAnimation.appendOthers(out, animationDefaultStartPhase, animationSynchronized, animationRandomStartPhase,
		animationLoopType, animationLoopCount, animationSpritePhase)
}

func boolToUint(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}

package assets

import (
	"fmt"
	"slices"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/nekiro/ots-creator/internal/binio"
	"github.com/nekiro/ots-creator/internal/thing"
)

// The editor works with 32x32 pieces like dat/spr clients. An official
// sprite of 64x64 pixels becomes 2x2 pieces of a frame group with width and
// height 2; piece k of a sprite is the tile (k%w, k/w) counted from the
// bottom-right corner, the order thing.FrameGroup uses for its tiles.

// SpriteMap maps official sprite ids to piece ids.
type SpriteMap interface {
	// Pieces returns the first piece id of a sprite and its size in tiles.
	Pieces(id uint32) (first uint32, w, h int, ok bool)
}

// Resolver returns the official sprite id for the pieces of one texture
// (w*h piece ids in tile order).
type Resolver func(pieces []uint32, w, h int) (uint32, error)

// Original is kept in thing.Thing.Extra of decoded things. Things the
// editor did not touch are written back byte for byte, and the fields the
// model does not know (names, NPC sale data, newer flags) survive edits.
type Original struct {
	raw []byte
	// thing is the decoded thing; edits replace the thing pointer.
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
	thing.CategoryItem:    1,
	thing.CategoryOutfit:  2,
	thing.CategoryEffect:  3,
	thing.CategoryMissile: 4,
}

// Appearance fields with texts.
const (
	fieldName        = 4
	fieldDescription = 5
)

// Fixed frame groups.
const (
	groupOutfitIdle    = 0
	groupObjectInitial = 2
)

// Decode parses an appearances file.
func Decode(data []byte, sm SpriteMap) (*File, error) {
	root, err := parse(data)
	if err != nil {
		return nil, err
	}
	f := &File{Things: map[thing.Category][]*thing.Thing{}}
	for _, fl := range root {
		c := categoryOf(fl.num)
		if c == thing.CategoryInvalid {
			f.rest = append(f.rest, fl.raw...)
			continue
		}
		if fl.typ != protowire.BytesType {
			return nil, errWire
		}
		t, err := decodeAppearance(fl.b, c, sm)
		if err != nil {
			return nil, err
		}
		if t.ID < c.MinID() {
			return nil, fmt.Errorf("appearances: %s id %d is below %d", c, t.ID, c.MinID())
		}
		list := f.Things[c]
		i := int(t.ID - c.MinID())
		for len(list) <= i {
			list = append(list, nil)
		}
		if list[i] != nil {
			return nil, fmt.Errorf("appearances: duplicate %s %d", c, t.ID)
		}
		list[i] = t
		f.Things[c] = list
	}
	for _, c := range thing.Categories {
		list := f.Things[c]
		for i, t := range list {
			if t == nil {
				list[i] = thing.New(c.MinID()+uint32(i), c)
			}
		}
		if len(list) == 0 {
			f.Things[c] = []*thing.Thing{thing.New(c.MinID(), c)}
		}
	}
	return f, nil
}

func categoryOf(n protowire.Number) thing.Category {
	for c, num := range listFields {
		if num == n {
			return c
		}
	}
	return thing.CategoryInvalid
}

func decodeAppearance(raw []byte, c thing.Category, sm SpriteMap) (*thing.Thing, error) {
	m, err := parse(raw)
	if err != nil {
		return nil, err
	}
	t := &thing.Thing{ID: uint32(m.uint(1)), Category: c}
	// Texts are Latin-1 like in the dat format.
	if f, ok := m.get(fieldName); ok {
		t.Name = binio.DecodeLatin1(f.b)
	}
	if f, ok := m.get(fieldDescription); ok {
		t.Description = binio.DecodeLatin1(f.b)
	}
	decodeFlags(m.sub(3), &t.Props)
	orig := &Original{raw: raw, thing: t}
	for _, gm := range m.all(2) {
		if c != thing.CategoryOutfit && len(t.FrameGroups) == 1 {
			break // only outfits have several groups in the editor
		}
		g, ids, err := decodeGroup(gm, c, sm)
		if err != nil {
			return nil, fmt.Errorf("appearances: %s %d: %w", c, t.ID, err)
		}
		t.FrameGroups = append(t.FrameGroups, g)
		orig.sprites = append(orig.sprites, ids)
	}
	if len(t.FrameGroups) == 0 {
		return nil, fmt.Errorf("appearances: %s %d has no frame group", c, t.ID)
	}
	t.Extra = orig
	return t, nil
}

func u8(v uint64, what string) (uint8, error) {
	if v == 0 {
		v = 1
	}
	if v > 255 {
		return 0, fmt.Errorf("%s %d is too large", what, v)
	}
	return uint8(v), nil
}

func decodeGroup(gm message, c thing.Category, sm SpriteMap) (*thing.FrameGroup, []uint32, error) {
	si := gm.sub(3)
	g := &thing.FrameGroup{}
	if c == thing.CategoryOutfit && gm.uint(1) != groupOutfitIdle {
		g.Type = thing.FrameGroupWalking
	}
	var err error
	if g.PatternX, err = u8(si.uint(1), "pattern width"); err != nil {
		return nil, nil, err
	}
	if g.PatternY, err = u8(si.uint(2), "pattern height"); err != nil {
		return nil, nil, err
	}
	if g.PatternZ, err = u8(si.uint(3), "pattern depth"); err != nil {
		return nil, nil, err
	}
	if g.Layers, err = u8(si.uint(4), "layers"); err != nil {
		return nil, nil, err
	}
	anim := si.sub(6)
	phases := anim.all(6)
	if g.Frames, err = u8(uint64(len(phases)), "phase count"); err != nil {
		return nil, nil, err
	}
	if g.Frames > 1 {
		if anim.bool(2) {
			g.Mode = thing.AnimationSync
		}
		switch int32(anim.uint(4)) {
		case -1:
			g.LoopCount = -1
		case 1:
			g.LoopCount = int32(anim.uint(5))
		}
		if anim.bool(3) {
			g.StartFrame = -1
		} else if s := anim.uint(1); s < uint64(g.Frames) {
			g.StartFrame = int8(min(s, 127))
		}
		for _, p := range phases {
			g.Durations = append(g.Durations, thing.FrameDuration{Min: uint32(p.uint(1)), Max: uint32(p.uint(2))})
		}
	}
	ids64 := si.uints(5)
	ids := make([]uint32, len(ids64))
	for i, v := range ids64 {
		ids[i] = uint32(v)
	}
	textures := g.TotalTextures()
	if len(ids) != textures {
		return nil, nil, fmt.Errorf("%d sprites for %d textures", len(ids), textures)
	}
	w, h := 1, 1
	if len(ids) > 0 {
		if _, pw, ph, ok := sm.Pieces(ids[0]); ok {
			w, h = pw, ph
		}
	}
	g.Width, g.Height = uint8(w), uint8(h)
	g.ExactSize = uint8(min(max(si.uint(7), uint64(TileSize*max(w, h))), 255))
	n := w * h
	g.Sprites = make([]uint32, textures*n)
	for i, id := range ids {
		first, pw, ph, ok := sm.Pieces(id)
		if !ok {
			continue // missing sprites show as empty tiles
		}
		for th := range min(ph, h) {
			for tw := range min(pw, w) {
				g.Sprites[i*n+th*w+tw] = first + uint32(th*pw+tw)
			}
		}
	}
	return g, ids, nil
}

// Blank reports whether t is a placeholder for an id missing from the file
// (or an object removed in the editor): nothing about it is worth writing.
func Blank(t *thing.Thing) bool {
	if t.Extra != nil || t.Props != (thing.Properties{}) || t.Name != "" || t.Description != "" {
		return false
	}
	for _, g := range t.FrameGroups {
		for _, id := range g.Sprites {
			if id != 0 {
				return false
			}
		}
	}
	return true
}

// Encode writes an appearances file. Blank things are left out.
func Encode(f *File, resolve Resolver) ([]byte, error) {
	var out []byte
	for _, c := range thing.Categories {
		for _, t := range f.Things[c] {
			if t == nil || Blank(t) {
				continue
			}
			b, err := encodeAppearance(t, resolve)
			if err != nil {
				return nil, err
			}
			out = putBytes(out, listFields[c], b)
		}
	}
	return append(out, f.rest...), nil
}

// textureIDs resolves the official sprite ids of every texture of a group.
func textureIDs(t *thing.Thing, g *thing.FrameGroup, resolve Resolver) ([]uint32, error) {
	w, h := int(g.Width), int(g.Height)
	if _, ok := SpriteTypeFor(w, h); !ok {
		return nil, fmt.Errorf("%s %d is %dx%d tiles; assets support up to 2x2", t.Category, t.ID, w, h)
	}
	n := w * h
	ids := make([]uint32, g.TotalTextures())
	for i := range ids {
		id, err := resolve(g.Sprites[i*n:(i+1)*n], w, h)
		if err != nil {
			return nil, fmt.Errorf("%s %d: %w", t.Category, t.ID, err)
		}
		ids[i] = id
	}
	return ids, nil
}

func encodeAppearance(t *thing.Thing, resolve Resolver) ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	ids := make([][]uint32, len(t.FrameGroups))
	for i, g := range t.FrameGroups {
		var err error
		if ids[i], err = textureIDs(t, g, resolve); err != nil {
			return nil, err
		}
	}
	orig, _ := t.Extra.(*Original)
	if orig != nil && orig.thing == t && slices.EqualFunc(ids, orig.sprites, slices.Equal) {
		return orig.raw, nil
	}
	var om message
	var origGroups []message
	var origIDs [][]uint32
	if orig != nil {
		om, _ = parse(orig.raw)
		origGroups = om.all(2)
		origIDs = orig.sprites
	}
	b := putUint(nil, 1, uint64(t.ID))
	for gi, g := range t.FrameGroups {
		var og message
		var oids []uint32
		if gi < len(origGroups) {
			og = origGroups[gi]
		}
		if gi < len(origIDs) {
			oids = origIDs[gi]
		}
		b = putBytes(b, 2, encodeGroup(t.Category, g, ids[gi], og, slices.Equal(ids[gi], oids)))
	}
	b = putBytes(b, 3, encodeFlags(&t.Props, om.sub(3)))
	if t.Name != "" {
		b = putBytes(b, fieldName, binio.EncodeLatin1(t.Name))
	}
	if t.Description != "" {
		b = putBytes(b, fieldDescription, binio.EncodeLatin1(t.Description))
	}
	return om.appendOthers(b, 1, 2, 3, fieldName, fieldDescription), nil
}

func encodeGroup(c thing.Category, g *thing.FrameGroup, ids []uint32, og message, sameSprites bool) []byte {
	fixed := uint64(groupObjectInitial)
	if c == thing.CategoryOutfit {
		fixed = uint64(g.Type)
	}
	b := putUint(nil, 1, fixed)
	b = putUint(b, 2, fixed) // the official files use the group type as id
	b = putBytes(b, 3, encodeSpriteInfo(g, ids, og.sub(3), sameSprites))
	return og.appendOthers(b, 1, 2, 3)
}

func encodeSpriteInfo(g *thing.FrameGroup, ids []uint32, osi message, sameSprites bool) []byte {
	b := putUint(nil, 1, uint64(g.PatternX))
	b = putUint(b, 2, uint64(g.PatternY))
	b = putUint(b, 3, uint64(g.PatternZ))
	b = putUint(b, 4, uint64(g.Layers))
	for _, id := range ids {
		b = putUint(b, 5, uint64(id))
	}
	if g.Frames > 1 {
		b = putBytes(b, 6, encodeAnimation(g, osi.sub(6)))
	}
	// is_opaque lets the client skip drawing what lies below; it is only
	// kept while the sprites are the same.
	if f, ok := osi.get(8); ok && sameSprites {
		b = append(b, f.raw...)
	} else {
		b = putUint(b, 8, 0)
	}
	return osi.appendOthers(b, 1, 2, 3, 4, 5, 6, 8)
}

func encodeAnimation(g *thing.FrameGroup, oa message) []byte {
	var b []byte
	if g.StartFrame > 0 {
		b = putUint(b, 1, uint64(g.StartFrame))
	}
	b = putUint(b, 2, boolToUint(g.Mode == thing.AnimationSync))
	b = putBool(b, 3, g.StartFrame < 0)
	switch {
	case g.LoopCount < 0:
		b = putInt(b, 4, -1)
	case g.LoopCount == 0:
		b = putInt(b, 4, 0)
	default:
		b = putInt(b, 4, 1)
		b = putUint(b, 5, uint64(g.LoopCount))
	}
	for _, d := range g.Durations {
		p := putUint(nil, 1, uint64(d.Min))
		p = putUint(p, 2, uint64(d.Max))
		b = putBytes(b, 6, p)
	}
	return oa.appendOthers(b, 1, 2, 3, 4, 5, 6)
}

func boolToUint(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}

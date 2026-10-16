package project

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/spr"
)

// Diff statuses of one id in two clients.
const (
	DiffChanged = "changed"
	DiffOnlyA   = "onlyA" // the id does not exist in B, or is empty there
	DiffOnlyB   = "onlyB" // the id does not exist in A, or is empty there
)

// Parts of a thing that can differ (DiffEntry.Changes).
const (
	ChangeName      = "name"
	ChangeProps     = "props"
	ChangeNpcSales  = "npcSales"
	ChangeSize      = "size"      // width, height or exact size
	ChangePatterns  = "patterns"  // layers or patterns
	ChangeFrames    = "frames"    // frame count
	ChangeGroups    = "groups"    // frame groups that cannot be compared frame by frame
	ChangeAnimation = "animation" // durations, mode, loop count or start frame
	ChangeSprites   = "sprites"
)

// DiffEntry is one id that differs between two clients.
type DiffEntry struct {
	ID      uint32   `json:"id"`
	Status  string   `json:"status"`
	Changes []string `json:"changes"`
	// Empty marks an id that exists in one client only and has no sprite
	// with pixels there.
	Empty bool `json:"empty,omitempty"`
}

// DiffResult compares one category of two clients. Entries lists only the
// ids that differ, in order; Same counts the others.
type DiffResult struct {
	Entries []DiffEntry `json:"entries"`
	Same    int         `json:"same"`
}

// Diff compares the things of category in clientA and clientB. Sprites are
// compared by their pixels, not by their ids, so the same object stored
// under other sprite ids is equal. Differences that only come from the
// formats of the clients are ignored: an outfit with idle and walking
// groups equals the same frames in one group, animation settings count
// only when both clients store them, and exact size only when both use the
// same format.
func Diff(clientA, clientB *Project, category thing.Category) (DiffResult, error) {
	if !category.Valid() {
		return DiffResult{}, fmt.Errorf("invalid category %d", category)
	}
	if clientA.SpriteSize() != clientB.SpriteSize() {
		return DiffResult{}, fmt.Errorf("clients use %dpx and %dpx sprites", clientA.SpriteSize(), clientB.SpriteSize())
	}
	// Writers never wait for a second project while they hold their lock
	// (Transfer copies the source first), so holding both read locks is safe.
	clientA.mu.RLock()
	defer clientA.mu.RUnlock()
	clientB.mu.RLock()
	defer clientB.mu.RUnlock()
	options := compareOptions{
		animation: clientA.storesAnimation() && clientB.storesAnimation(),
		exactSize: clientA.format == clientB.format,
	}
	first, last := category.MinID(), max(clientA.things.MaxID(category), clientB.things.MaxID(category))
	if last < first {
		return DiffResult{Entries: []DiffEntry{}}, nil
	}
	// Chunks are compared in parallel (sprite reads may decode asset sheets)
	// and joined in id order.
	size := thingChunk(last - first + 1)
	parts := make([]DiffResult, (last-first)/size+1)
	err := parallelRange(first, last, size, func(from, to uint32) error {
		comparer := thingComparer{options: options, sprites: spriteComparer{storeA: clientA.sprites, storeB: clientB.sprites, cache: map[[2]uint32]bool{}}}
		part, err := diffRange(clientA, clientB, category, from, to, &comparer)
		parts[(from-first)/size] = part
		return err
	})
	if err != nil {
		return DiffResult{}, err
	}
	result := DiffResult{Entries: []DiffEntry{}}
	for _, part := range parts {
		result.Entries = append(result.Entries, part.Entries...)
		result.Same += part.Same
	}
	return result, nil
}

func diffRange(clientA, clientB *Project, category thing.Category, from, to uint32, comparer *thingComparer) (DiffResult, error) {
	var result DiffResult
	for id := from; id <= to; id++ {
		objectA, objectB := clientA.things.Get(category, id), clientB.things.Get(category, id)
		switch {
		case objectA == nil && objectB == nil:
		case objectB == nil:
			result.Entries = append(result.Entries, DiffEntry{ID: id, Status: DiffOnlyA, Empty: !clientA.hasPixels(objectA)})
		case objectA == nil:
			result.Entries = append(result.Entries, DiffEntry{ID: id, Status: DiffOnlyB, Empty: !clientB.hasPixels(objectB)})
		default:
			changes, err := comparer.compare(objectA, objectB)
			if err != nil {
				return DiffResult{}, fmt.Errorf("%s %d: %w", category, id, err)
			}
			if len(changes) == 0 {
				result.Same++
				continue
			}
			// An empty placeholder against a real object is a new object,
			// not a change.
			emptyA, emptyB := !clientA.hasPixels(objectA), !clientB.hasPixels(objectB)
			switch {
			case emptyA && !emptyB:
				result.Entries = append(result.Entries, DiffEntry{ID: id, Status: DiffOnlyB})
			case emptyB && !emptyA:
				result.Entries = append(result.Entries, DiffEntry{ID: id, Status: DiffOnlyA})
			default:
				result.Entries = append(result.Entries, DiffEntry{ID: id, Status: DiffChanged, Changes: changes})
			}
		}
	}
	return result, nil
}

// storesAnimation reports whether the client stores frame durations, loop
// count and start frame; otherwise they are defaults made up on load.
func (p *Project) storesAnimation() bool {
	return p.format == FormatAssets || p.features.ImprovedAnimations
}

type compareOptions struct {
	animation bool
	exactSize bool
}

type thingComparer struct {
	options compareOptions
	sprites spriteComparer
}

func (comparer *thingComparer) compare(objectA, objectB *thing.Thing) ([]string, error) {
	var out []string
	if objectA.Name != objectB.Name || objectA.Description != objectB.Description {
		out = append(out, ChangeName)
	}
	if objectA.Props != objectB.Props {
		out = append(out, ChangeProps)
	}
	if !slices.Equal(objectA.NpcSales, objectB.NpcSales) {
		out = append(out, ChangeNpcSales)
	}
	groupsA, groupsB := objectA.FrameGroups, objectB.FrameGroups
	if len(groupsA) != len(groupsB) {
		// One client splits outfits into idle and walking groups, the other
		// keeps every frame in one: compare all frames in one group.
		stackedA, okA := objectA.Stacked()
		stackedB, okB := objectB.Stacked()
		if !okA || !okB {
			return append(out, ChangeGroups), nil
		}
		groupsA, groupsB = []*thing.FrameGroup{stackedA}, []*thing.FrameGroup{stackedB}
	}
	geometry := false
	for i, groupA := range groupsA {
		groupB := groupsB[i]
		if groupA.Width != groupB.Width || groupA.Height != groupB.Height || (comparer.options.exactSize && groupA.ExactSize != groupB.ExactSize) {
			out = appendOnce(out, ChangeSize)
			geometry = true
		}
		if groupA.Layers != groupB.Layers || groupA.PatternX != groupB.PatternX || groupA.PatternY != groupB.PatternY || groupA.PatternZ != groupB.PatternZ {
			out = appendOnce(out, ChangePatterns)
			geometry = true
		}
		if groupA.Frames != groupB.Frames {
			out = appendOnce(out, ChangeFrames)
			geometry = true
		}
		if comparer.options.animation && groupA.Frames == groupB.Frames && groupA.Frames > 1 &&
			(groupA.Mode != groupB.Mode || groupA.LoopCount != groupB.LoopCount || groupA.StartFrame != groupB.StartFrame || !slices.Equal(groupA.Durations, groupB.Durations)) {
			out = appendOnce(out, ChangeAnimation)
		}
	}
	if geometry {
		return out, nil
	}
	for i, groupA := range groupsA {
		groupB := groupsB[i]
		for j, id := range groupA.Sprites {
			same, err := comparer.sprites.equal(id, groupB.Sprites[j])
			if err != nil {
				return nil, err
			}
			if !same {
				return append(out, ChangeSprites), nil
			}
		}
	}
	return out, nil
}

func appendOnce(list []string, value string) []string {
	if slices.Contains(list, value) {
		return list
	}
	return append(list, value)
}

// spriteComparer compares sprites of two stores by content.
type spriteComparer struct {
	storeA, storeB *spriteStore
	cache          map[[2]uint32]bool
}

func (s *spriteComparer) equal(idA, idB uint32) (bool, error) {
	key := [2]uint32{idA, idB}
	if same, ok := s.cache[key]; ok {
		return same, nil
	}
	compressedA, err := s.storeA.compressedOrEmpty(idA)
	if err != nil {
		return false, err
	}
	compressedB, err := s.storeB.compressedOrEmpty(idB)
	if err != nil {
		return false, err
	}
	var same bool
	if s.storeA.transparent == s.storeB.transparent {
		same = bytes.Equal(compressedA, compressedB)
	} else {
		// Different transparency settings encode the same pixels differently.
		pixelsA, err := spr.Decompress(compressedA, s.storeA.size, s.storeA.transparent)
		if err != nil {
			return false, err
		}
		pixelsB, err := spr.Decompress(compressedB, s.storeB.size, s.storeB.transparent)
		if err != nil {
			return false, err
		}
		same = bytes.Equal(pixelsA, pixelsB)
	}
	s.cache[key] = same
	return same, nil
}

// compressedOrEmpty is compressed with id 0 (no sprite) being empty.
func (s *spriteStore) compressedOrEmpty(id uint32) ([]byte, error) {
	if id == 0 {
		return nil, nil
	}
	return s.compressed(id)
}

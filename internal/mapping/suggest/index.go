// Package suggest learns from a map which items go together: on which
// grounds an item lies, what lies with it on one tile and what lies on the
// tiles around it, in which direction. The editor ranks with it the items
// that fit a place or go with the item being drawn, the items that take the
// same places as another one, and the parts that complete an object.
package suggest

import (
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Offsets are the 8 directions from a tile to its neighbors: N, NE, E, SE,
// S, SW, W, NW. A direction plus 4 (mod 8) is the opposite one.
var Offsets = [8][2]int{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}

func opposite(direction int) int { return (direction + 4) % 8 }

const (
	// blockBits sizes the unit the index counts, and counts again after an
	// edit: 4x4 chunks. Neighboring chunks repeat most pairs, so blocks keep
	// far fewer entries than chunks, and a block still counts in a few
	// milliseconds.
	blockBits = 2
	blockSide = otmap.ChunkSize << blockBits
	// ringSide is a block with a ring of one tile around it.
	ringSide = blockSide + 2
	// maxChunkCoord is the largest chunk coordinate of a map.
	maxChunkCoord = 0xFFFF >> otmap.ChunkBits
)

// entry is a count by key: an item, a ground or a pair packed as
// first<<16 | second.
type entry struct {
	key   uint32
	count int32
}

// nearEntry counts second on the neighbor of first in a direction.
type nearEntry struct {
	first, second uint16
	direction     uint8
	count         int32
}

// blockStats is what one block adds to the index; neighbor pairs count for
// the block of the center tile.
type blockStats struct {
	signature  uint64
	tiles      int32
	items      []entry // item
	grounds    []entry // ground
	onGround   []entry // ground<<16 | item on it
	stacked    []entry // item<<16 | item on the same tile
	groundNear []entry // ground<<16 | other ground of a neighbor tile
	near       []nearEntry
}

// pairs counts the second ids seen with a first id.
type pairs map[uint16]map[uint16]int32

func (counts pairs) add(first, second uint16, delta int32) {
	inner := counts[first]
	if inner == nil {
		inner = map[uint16]int32{}
		counts[first] = inner
	}
	inner[second] += delta
	if inner[second] <= 0 {
		delete(inner, second)
		if len(inner) == 0 {
			delete(counts, first)
		}
	}
}

// directed counts the ids seen on the neighbors of a first id, keyed
// direction<<16 | second.
type directed map[uint16]map[uint32]int32

func (counts directed) add(first uint16, key uint32, delta int32) {
	inner := counts[first]
	if inner == nil {
		inner = map[uint32]int32{}
		counts[first] = inner
	}
	inner[key] += delta
	if inner[key] <= 0 {
		delete(inner, key)
		if len(inner) == 0 {
			delete(counts, first)
		}
	}
}

// toward returns the ids seen in one direction of first.
func (counts directed) toward(first uint16, direction int) map[uint16]int32 {
	out := map[uint16]int32{}
	for key, count := range counts[first] {
		if int(key>>16) == direction {
			out[uint16(key)] += count
		}
	}
	return out
}

// around returns the ids seen in any direction of first.
func (counts directed) around(first uint16) map[uint16]int32 {
	out := map[uint16]int32{}
	for key, count := range counts[first] {
		out[uint16(key)] += count
	}
	return out
}

// counts are the sums of some blocks: the whole map or the area around a
// place.
type counts struct {
	tiles      int64
	items      map[uint16]int32
	grounds    map[uint16]int32
	onGround   pairs
	stacked    pairs
	groundNear pairs
	near       directed
}

func newCounts() *counts {
	return &counts{items: map[uint16]int32{}, grounds: map[uint16]int32{}, onGround: pairs{}, stacked: pairs{}, groundNear: pairs{}, near: directed{}}
}

func (sums *counts) apply(stats *blockStats, sign int32) {
	sums.tiles += int64(sign * stats.tiles)
	addSingles(sums.items, stats.items, sign)
	addSingles(sums.grounds, stats.grounds, sign)
	for _, item := range stats.onGround {
		sums.onGround.add(uint16(item.key>>16), uint16(item.key), sign*item.count)
	}
	for _, item := range stats.stacked {
		sums.stacked.add(uint16(item.key>>16), uint16(item.key), sign*item.count)
	}
	for _, item := range stats.groundNear {
		sums.groundNear.add(uint16(item.key>>16), uint16(item.key), sign*item.count)
	}
	for _, item := range stats.near {
		sums.near.add(item.first, uint32(item.direction)<<16|uint32(item.second), sign*item.count)
	}
}

func addSingles(sums map[uint16]int32, entries []entry, sign int32) {
	for _, item := range entries {
		id := uint16(item.key)
		sums[id] += sign * item.count
		if sums[id] <= 0 {
			delete(sums, id)
		}
	}
}

// itemChance is the share of tiles an item lies on.
func (sums *counts) itemChance(id uint16) float64 {
	if sums.tiles <= 0 {
		return 0
	}
	return float64(sums.items[id]) / float64(sums.tiles)
}

func (sums *counts) groundChance(id uint16) float64 {
	if sums.tiles <= 0 {
		return 0
	}
	return float64(sums.grounds[id]) / float64(sums.tiles)
}

// Index holds the counts of a map. It is safe for concurrent use; Refresh
// reads the map, so its caller holds at least a read lock of the map.
type Index struct {
	mu sync.Mutex
	// blocks are keyed like chunks, by block coordinates.
	blocks map[otmap.ChunkKey]*blockStats
	total  *counts
	tuning Tuning
}

func New() *Index {
	return &Index{blocks: map[otmap.ChunkKey]*blockStats{}, total: newCounts(), tuning: DefaultTuning}
}

// BlockOf returns the key of the block of a position.
func BlockOf(pos otmap.Position) otmap.ChunkKey {
	return otmap.MakeKey(pos.X>>(otmap.ChunkBits+blockBits), pos.Y>>(otmap.ChunkBits+blockBits), pos.Z)
}

// around sums the blocks within radius blocks of a block, on its floor.
func (index *Index) around(block otmap.ChunkKey, radius int) *counts {
	sums := newCounts()
	blockX, blockY, floor := block.Split()
	for y := int(blockY) - radius; y <= int(blockY)+radius; y++ {
		for x := int(blockX) - radius; x <= int(blockX)+radius; x++ {
			if x < 0 || y < 0 {
				continue
			}
			if stats := index.blocks[otmap.MakeKey(uint16(x), uint16(y), floor)]; stats != nil {
				sums.apply(stats, 1)
			}
		}
	}
	return sums
}

// Refresh counts again up to budget blocks that changed since they were
// counted (a block also counts again when a chunk around it changed) and
// drops the blocks that are gone. ignore leaves out items that say nothing
// about the look of a place (borders, editor items). It returns the number
// of changed blocks left.
func (index *Index) Refresh(gameMap *otmap.Map, ignore func(id uint16) bool, budget int) int {
	index.mu.Lock()
	defer index.mu.Unlock()
	present := map[otmap.ChunkKey]bool{}
	for _, chunk := range gameMap.Chunks() {
		present[otmap.MakeKey(chunk.X>>blockBits, chunk.Y>>blockBits, chunk.Z)] = true
	}
	for key, stats := range index.blocks {
		if !present[key] {
			index.total.apply(stats, -1)
			delete(index.blocks, key)
		}
	}
	var stale []otmap.ChunkKey
	for key := range present {
		if stats := index.blocks[key]; stats == nil || stats.signature != signature(gameMap, key) {
			stale = append(stale, key)
		}
	}
	slices.Sort(stale)
	counted := countBlocks(gameMap, ignore, stale[:min(budget, len(stale))])
	for i, stats := range counted {
		if old := index.blocks[stale[i]]; old != nil {
			index.total.apply(old, -1)
		}
		index.blocks[stale[i]] = stats
		index.total.apply(stats, 1)
	}
	return len(stale) - len(counted)
}

// countBlocks counts blocks on all cores; the map is only read.
func countBlocks(gameMap *otmap.Map, ignore func(id uint16) bool, keys []otmap.ChunkKey) []*blockStats {
	counted := make([]*blockStats, len(keys))
	var next atomic.Int64
	var group sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), len(keys)) {
		group.Go(func() {
			counter := newBlockCounter(ignore)
			for i := int(next.Add(1) - 1); i < len(keys); i = int(next.Add(1) - 1) {
				counted[i] = counter.count(gameMap, keys[i])
			}
		})
	}
	group.Wait()
	return counted
}

// signature changes when a chunk of the block or a chunk around it on its
// floor changes: chunk revisions only grow and a gone chunk drops its share.
func signature(gameMap *otmap.Map, block otmap.ChunkKey) uint64 {
	blockX, blockY, floor := block.Split()
	firstX, firstY := int(blockX)<<blockBits, int(blockY)<<blockBits
	var sum uint64
	for chunkY := max(firstY-1, 0); chunkY <= min(firstY+1<<blockBits, maxChunkCoord); chunkY++ {
		for chunkX := max(firstX-1, 0); chunkX <= min(firstX+1<<blockBits, maxChunkCoord); chunkX++ {
			key := otmap.MakeKey(uint16(chunkX), uint16(chunkY), floor)
			if gameMap.Chunk(key) != nil {
				sum += gameMap.ChunkRev(key) + 1
			}
		}
	}
	return sum
}

// blockCounter counts blocks with buffers kept between blocks.
type blockCounter struct {
	ignore func(id uint16) bool
	// grounds and itemSets are the tiles of a block and the ring around it,
	// row by row; the sets slice items, spans holds their bounds.
	grounds  []uint16
	itemSets [][]uint16
	spans    [][2]int32
	items    []uint16
	scratch  otmap.Tile
	buffer   []otmap.Item
	counts   [5]map[uint32]int32
	near     map[uint64]int32
}

func newBlockCounter(ignore func(id uint16) bool) *blockCounter {
	counter := &blockCounter{
		ignore: ignore, buffer: make([]otmap.Item, 0, 32), near: map[uint64]int32{},
		grounds: make([]uint16, ringSide*ringSide), itemSets: make([][]uint16, ringSide*ringSide), spans: make([][2]int32, ringSide*ringSide),
	}
	for i := range counter.counts {
		counter.counts[i] = map[uint32]int32{}
	}
	return counter
}

func (counter *blockCounter) count(gameMap *otmap.Map, block otmap.ChunkKey) *blockStats {
	blockX, blockY, floor := block.Split()
	originX, originY := int(blockX)*blockSide-1, int(blockY)*blockSide-1
	counter.items = counter.items[:0]
	for row := range ringSide {
		for column := range ringSide {
			cell := row*ringSide + column
			counter.grounds[cell] = 0
			start := len(counter.items)
			if tile := gameMap.PeekAt(originX+column, originY+row, int(floor), &counter.scratch, counter.buffer); tile != nil {
				counter.grounds[cell] = tile.Ground.ID
				for _, item := range tile.Items {
					if item.ID != 0 && !counter.ignore(item.ID) && !slices.Contains(counter.items[start:], item.ID) {
						counter.items = append(counter.items, item.ID)
					}
				}
			}
			counter.spans[cell] = [2]int32{int32(start), int32(len(counter.items))}
		}
	}
	for cell, span := range counter.spans {
		counter.itemSets[cell] = counter.items[span[0]:span[1]]
	}

	items, grounds, onGround, stacked, groundNear := counter.counts[0], counter.counts[1], counter.counts[2], counter.counts[3], counter.counts[4]
	near := counter.near
	stats := &blockStats{signature: signature(gameMap, block)}
	for row := 1; row <= blockSide; row++ {
		for column := 1; column <= blockSide; column++ {
			cell := row*ringSide + column
			ground, here := counter.grounds[cell], counter.itemSets[cell]
			if ground == 0 && len(here) == 0 {
				continue
			}
			stats.tiles++
			if ground != 0 {
				grounds[uint32(ground)]++
			}
			for _, item := range here {
				items[uint32(item)]++
				if ground != 0 {
					onGround[uint32(ground)<<16|uint32(item)]++
				}
				for _, other := range here {
					if other != item {
						stacked[uint32(item)<<16|uint32(other)]++
					}
				}
			}
			for direction, offset := range Offsets {
				neighbor := (row+offset[1])*ringSide + column + offset[0]
				for _, item := range here {
					for _, other := range counter.itemSets[neighbor] {
						near[uint64(item)<<32|uint64(direction)<<16|uint64(other)]++
					}
				}
				if neighborGround := counter.grounds[neighbor]; ground != 0 && neighborGround != 0 && neighborGround != ground {
					groundNear[uint32(ground)<<16|uint32(neighborGround)]++
				}
			}
		}
	}
	stats.items, stats.grounds = drain(items), drain(grounds)
	stats.onGround, stats.stacked, stats.groundNear = drain(onGround), drain(stacked), drain(groundNear)
	stats.near = make([]nearEntry, 0, len(near))
	for key, count := range near {
		stats.near = append(stats.near, nearEntry{first: uint16(key >> 32), direction: uint8(key >> 16), second: uint16(key), count: count})
	}
	clear(near)
	return stats
}

// drain moves counts into a compact list and empties the map.
func drain(sums map[uint32]int32) []entry {
	if len(sums) == 0 {
		return nil
	}
	list := make([]entry, 0, len(sums))
	for key, count := range sums {
		list = append(list, entry{key, count})
	}
	clear(sums)
	return list
}

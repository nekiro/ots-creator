package otmap

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/nekiro/ots-creator/internal/formats/xmltree"
)

// ReadSpawns applies a spawn file. npcFile marks a separate NPC spawn file
// (its areas become NPC spawns); a TFS file holds monsters and NPCs.
func ReadSpawns(gameMap *Map, data []byte, npcFile bool) ([]string, error) {
	doc, err := xmltree.Parse(data)
	if err != nil {
		return nil, err
	}
	var warnings []string
	for _, spawnNode := range doc.Children {
		if !spawnNode.Has("centerx") {
			continue
		}
		centerX, centerY, centerZ := spawnNode.Int("centerx"), spawnNode.Int("centery"), spawnNode.Int("centerz")
		radius := spawnNode.Int("radius")
		if centerX <= 0 || centerY <= 0 || centerZ < 0 || centerZ > MaxFloor || centerX > 0xFFFF || centerY > 0xFFFF {
			warnings = append(warnings, "spawn with a bad position discarded")
			continue
		}
		if radius < 1 {
			radius = 1
		}
		center := Position{uint16(centerX), uint16(centerY), uint8(centerZ)}
		tile := gameMap.Tile(center).Clone()
		if tile == nil {
			tile = &Tile{}
		}
		if npcFile {
			if tile.SpawnNPC() != nil {
				warnings = append(warnings, fmt.Sprintf("duplicate NPC spawn at %d,%d,%d discarded", centerX, centerY, centerZ))
				continue
			}
			tile.EnsureExt().SpawnNPC = &Spawn{Radius: int32(radius)}
		} else {
			if tile.Spawn() != nil {
				warnings = append(warnings, fmt.Sprintf("duplicate spawn at %d,%d,%d discarded", centerX, centerY, centerZ))
				continue
			}
			tile.EnsureExt().Spawn = &Spawn{Radius: int32(radius)}
		}
		gameMap.SetTile(center, tile)
		for _, creatureNode := range spawnNode.Children {
			if creatureNode.Name != "monster" && creatureNode.Name != "npc" {
				continue
			}
			name := creatureNode.Attrs["name"]
			if name == "" || !creatureNode.Has("x") || !creatureNode.Has("y") {
				warnings = append(warnings, fmt.Sprintf("creature without a name or position in spawn %d,%d,%d discarded", centerX, centerY, centerZ))
				continue
			}
			pos, ok := center.Add(creatureNode.Int("x"), creatureNode.Int("y"), 0)
			if !ok {
				continue
			}
			if creatureNode.Has("z") {
				if z := creatureNode.Int("z"); z >= 0 && z <= MaxFloor {
					pos.Z = uint8(z)
				}
			}
			creature := Creature{Name: name, NPC: creatureNode.Name == "npc" || npcFile, SpawnTime: int32(creatureNode.Int("spawntime")), Weight: uint8(creatureNode.Int("weight"))}
			if creatureNode.Has("direction") {
				creature.Direction = uint8(min(max(creatureNode.Int("direction"), 0), 3))
			}
			creatureTile := gameMap.Tile(pos).Clone()
			if creatureTile == nil {
				creatureTile = &Tile{}
			}
			creatureTile.EnsureExt().Creatures = append(creatureTile.EnsureExt().Creatures, creature)
			gameMap.SetTile(pos, creatureTile)
		}
	}
	return warnings, nil
}

type spawnArea struct {
	pos    Position
	radius int32
}

func spawnAreas(gameMap *Map, npc bool) []spawnArea {
	var out []spawnArea
	for _, chunk := range gameMap.chunks {
		chunk.eachStored(func(pos Position, tile *Tile) {
			spawn := tile.Spawn()
			if npc {
				spawn = tile.SpawnNPC()
			}
			if spawn != nil {
				out = append(out, spawnArea{pos, max(spawn.Radius, 1)})
			}
		})
	}
	slices.SortFunc(out, func(left, right spawnArea) int {
		return ComparePositions(left.pos, right.pos)
	})
	return out
}

// creatureRef is a creature by its tile and index on the tile.
type creatureRef struct {
	pos   Position
	index int
}

// WriteSpawns returns the spawn files. Maps with single spawns get one file
// (npcs is nil); maps with separate spawns get a monster file and an NPC
// file. unsaved counts creatures outside every matching spawn area.
func WriteSpawns(gameMap *Map) (monsters, npcs []byte, unsaved int) {
	saved := map[creatureRef]bool{}
	separate := gameMap.SpawnFormat == SpawnsSeparate
	write := func(root, areaTag, kind string, areas []spawnArea, want func(Creature) bool) []byte {
		w := newXMLWriter()
		w.elem(root, nil, false)
		for _, area := range areas {
			w.elem(areaTag, []xmlAttr{{"centerx", itoa(area.pos.X)}, {"centery", itoa(area.pos.Y)}, {"centerz", itoa(area.pos.Z)}, {"radius", itoa(area.radius)}}, false)
			radius := int(area.radius)
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					pos, ok := area.pos.Add(dx, dy, 0)
					if !ok {
						continue
					}
					tile := gameMap.stored(pos)
					if tile == nil {
						continue
					}
					for i, creature := range tile.Creatures() {
						ref := creatureRef{pos, i}
						if saved[ref] || !want(creature) {
							continue
						}
						saved[ref] = true
						tag := kind
						if tag == "" {
							tag = "monster"
							if creature.NPC {
								tag = "npc"
							}
						}
						attrs := []xmlAttr{{"name", creature.Name}, {"x", strconv.Itoa(dx)}, {"y", strconv.Itoa(dy)}, {"z", itoa(area.pos.Z)}, {"spawntime", itoa(creature.SpawnTime)}}
						if creature.Direction != 0 {
							attrs = append(attrs, xmlAttr{"direction", itoa(creature.Direction)})
						}
						if separate && creature.Weight != 0 && !creature.NPC {
							attrs = append(attrs, xmlAttr{"weight", itoa(creature.Weight)})
						}
						w.elem(tag, attrs, true)
					}
				}
			}
			w.end(areaTag)
		}
		w.end(root)
		return w.buf.Bytes()
	}
	if separate {
		monsters = write("monsters", "monster", "monster", spawnAreas(gameMap, false), func(creature Creature) bool { return !creature.NPC })
		npcs = write("npcs", "npc", "npc", spawnAreas(gameMap, true), func(creature Creature) bool { return creature.NPC })
	} else {
		areas := append(spawnAreas(gameMap, false), spawnAreas(gameMap, true)...)
		monsters = write("spawns", "spawn", "", areas, func(Creature) bool { return true })
	}
	for _, chunk := range gameMap.chunks {
		chunk.eachStored(func(pos Position, tile *Tile) {
			for i := range tile.Creatures() {
				if !saved[creatureRef{pos, i}] {
					unsaved++
				}
			}
		})
	}
	return monsters, npcs, unsaved
}

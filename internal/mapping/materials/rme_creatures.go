package materials

import (
	"strings"

	"github.com/nekiro/ots-creator/internal/formats/xmltree"
)

func ParseRMECreatures(data []byte) ([]*CreatureType, error) {
	root, err := xmltree.Parse(data)
	if err != nil {
		return nil, err
	}
	var out []*CreatureType
	for _, node := range root.Children {
		if node.Name != "creature" || node.Str("name") == "" {
			continue
		}
		creature := &CreatureType{Name: node.Str("name"), NPC: strings.EqualFold(node.Str("type"), "npc")}
		creature.Outfit = Outfit{
			LookType: node.U16("looktype"), Head: uint8(node.Int("lookhead")), Body: uint8(node.Int("lookbody")),
			Legs: uint8(node.Int("looklegs")), Feet: uint8(node.Int("lookfeet")), Addons: uint8(node.Int("lookaddon")),
			Mount: node.U16("lookmount"),
		}
		// lookitem is a server id in RME.
		creature.Outfit.LookItem = node.U16("lookitem")
		out = append(out, creature)
	}
	return out, nil
}

package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Terrain rules learn from a sample of a real map how tiles look: for
// every terrain class of a tile and the classes of its 8 neighbors, the
// ground and items (borders, walls, decoration) seen there. Painting a
// plan of classes then picks, for every tile, one of the looks seen with
// the same neighbors, so borders and walls fit the client without brushes.

// neighborOffsets are the 8 neighbors in signature order: N, NE, E, SE, S,
// SW, W, NW.
var neighborOffsets = [8][2]int{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}

// neighborWeights rate a differing neighbor in the nearest-signature
// search: orthogonal neighbors decide more than diagonal ones.
var neighborWeights = [8]int{3, 1, 3, 1, 3, 1, 3, 1}

const (
	// maxSampleTiles limits the tiles learn_terrain reads.
	maxSampleTiles = 512 * 512
	// maxPaintSide limits the rows and columns of paint_terrain.
	maxPaintSide = 256
)

var ruleNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// TerrainClass is a terrain of the rules: the grounds that make it.
type TerrainClass struct {
	Name    string `json:"name" jsonschema:"class name used in the legend of paint_terrain"`
	Grounds []int  `json:"grounds" jsonschema:"ground item ids of this terrain (see count_grounds)"`
}

// look is one way a tile was seen: ground, items and how often.
type look struct {
	Ground uint16   `json:"ground"`
	Items  []uint16 `json:"items,omitempty"`
	Count  int      `json:"count"`
}

// terrainRules are learned looks by signature.
type terrainRules struct {
	Name    string         `json:"name"`
	Classes []TerrainClass `json:"classes"`
	// Looks maps a signature (center class then the 8 neighbor classes,
	// see signature) to the looks seen with it.
	Looks map[string][]look `json:"looks"`
	// byGround and nearest are built on load.
	byGround map[uint16]int
	nearest  map[string]string
}

// Class codes besides the classes: a tile with a ground of no class, and
// no tile or no ground.
func (rules *terrainRules) otherCode() int { return len(rules.Classes) }
func (rules *terrainRules) noneCode() int  { return len(rules.Classes) + 1 }

func (rules *terrainRules) index() {
	rules.byGround = map[uint16]int{}
	for index, class := range rules.Classes {
		for _, ground := range class.Grounds {
			rules.byGround[uint16(ground)] = index
		}
	}
	rules.nearest = map[string]string{}
}

// classOf returns the class code of a ground (0: no tile or ground).
func (rules *terrainRules) classOf(ground uint16) int {
	if ground == 0 {
		return rules.noneCode()
	}
	if class, ok := rules.byGround[ground]; ok {
		return class
	}
	return rules.otherCode()
}

func signature(center int, neighbors [8]int) string {
	key := make([]byte, 9)
	key[0] = byte(center)
	for i, class := range neighbors {
		key[i+1] = byte(class)
	}
	return string(key)
}

// learn adds the tiles of a sample to the rules. tiles must include a ring
// of one tile around the sample area.
func (rules *terrainRules) learn(tiles map[otmap.Position]mapedit.TileSpec, area image.Rectangle, floor int, ignore map[uint16]bool) int {
	learned := 0
	classAt := func(x, y int) int {
		return rules.classOf(tiles[otmap.Position{X: uint16(x), Y: uint16(y), Z: uint8(floor)}].Ground)
	}
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			spec, ok := tiles[otmap.Position{X: uint16(x), Y: uint16(y), Z: uint8(floor)}]
			center := rules.classOf(spec.Ground)
			if !ok || center >= rules.otherCode() {
				continue
			}
			var neighbors [8]int
			for i, offset := range neighborOffsets {
				neighbors[i] = classAt(x+offset[0], y+offset[1])
			}
			key := signature(center, neighbors)
			spec.Items = slices.DeleteFunc(slices.Clone(spec.Items), func(item uint16) bool { return ignore[item] })
			looks := rules.Looks[key]
			found := slices.IndexFunc(looks, func(seen look) bool { return seen.Ground == spec.Ground && slices.Equal(seen.Items, spec.Items) })
			if found < 0 {
				looks = append(looks, look{Ground: spec.Ground, Items: slices.Clone(spec.Items)})
				found = len(looks) - 1
			}
			looks[found].Count++
			rules.Looks[key] = looks
			learned++
		}
	}
	return learned
}

// looksFor returns the looks of a signature, or of the most similar one
// learned for the same class; exact is false for the latter.
func (rules *terrainRules) looksFor(center int, neighbors [8]int) (looks []look, exact bool) {
	key := signature(center, neighbors)
	if looks := rules.Looks[key]; len(looks) > 0 {
		return looks, true
	}
	if near, ok := rules.nearest[key]; ok {
		return rules.Looks[near], false
	}
	best, bestDistance, bestCount := "", -1, 0
	for candidate, candidateLooks := range rules.Looks {
		if int(candidate[0]) != center {
			continue
		}
		distance := 0
		for i := range 8 {
			if int(candidate[i+1]) != neighbors[i] {
				distance += neighborWeights[i]
			}
		}
		count := 0
		for _, seen := range candidateLooks {
			count += seen.Count
		}
		if bestDistance < 0 || distance < bestDistance || (distance == bestDistance && count > bestCount) {
			best, bestDistance, bestCount = candidate, distance, count
		}
	}
	rules.nearest[key] = best
	return rules.Looks[best], false
}

// pick chooses a look by how often it was seen.
func pick(looks []look, random *rand.Rand) look {
	total := 0
	for _, seen := range looks {
		total += seen.Count
	}
	roll := random.IntN(max(total, 1))
	for _, seen := range looks {
		if roll < seen.Count {
			return seen
		}
		roll -= seen.Count
	}
	return looks[0]
}

// ruleStore keeps learned rules in memory and in files next to the map
// editor settings.
type ruleStore struct {
	mu    sync.Mutex
	dir   string
	rules map[string]*terrainRules
}

func (store *ruleStore) path(name string) string {
	return filepath.Join(store.dir, "terrain-rules", name+".json")
}

func (store *ruleStore) save(rules *terrainRules) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.rules == nil {
		store.rules = map[string]*terrainRules{}
	}
	store.rules[rules.Name] = rules
	if store.dir == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(store.path(rules.Name)), 0o755); err != nil {
		return err
	}
	return fsutil.WriteJSON(store.path(rules.Name), rules, "")
}

func (store *ruleStore) load(name string) (*terrainRules, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if rules := store.rules[name]; rules != nil {
		return rules, nil
	}
	if store.dir == "" || !ruleNamePattern.MatchString(name) {
		return nil, fmt.Errorf("no terrain rules %q: learn them with learn_terrain", name)
	}
	data, err := os.ReadFile(store.path(name))
	if err != nil {
		return nil, fmt.Errorf("no terrain rules %q: learn them with learn_terrain", name)
	}
	rules := &terrainRules{}
	if err := json.Unmarshal(data, rules); err != nil {
		return nil, err
	}
	rules.index()
	if store.rules == nil {
		store.rules = map[string]*terrainRules{}
	}
	store.rules[name] = rules
	return rules, nil
}

func (store *ruleStore) names() []string {
	store.mu.Lock()
	defer store.mu.Unlock()
	seen := map[string]bool{}
	for name := range store.rules {
		seen[name] = true
	}
	if store.dir != "" {
		files, _ := filepath.Glob(filepath.Join(store.dir, "terrain-rules", "*.json"))
		for _, file := range files {
			seen[strings.TrimSuffix(filepath.Base(file), ".json")] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Sample is an area of a real map to learn from.
type Sample struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Z      int `json:"z"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type learnArgs struct {
	MapArg
	Name    string         `json:"name" jsonschema:"name of the rules (letters, digits, - and _); learning again replaces them"`
	Classes []TerrainClass `json:"classes" jsonschema:"the terrains, each with all ground ids that make it; tiles whose ground is in no class count as other"`
	Samples []Sample       `json:"samples" jsonschema:"areas of the map that look as wanted, with every class and how they meet"`
	Ignore  []int          `json:"ignoreItems,omitempty" jsonschema:"item ids left out of the learned looks (furniture, quest items...)"`
}

type paintLegend struct {
	Symbol string `json:"symbol" jsonschema:"one character of the rows"`
	Class  string `json:"class" jsonschema:"terrain class of the rules; none removes the tile"`
}

type paintArgs struct {
	MapArg
	Rules  string        `json:"rules" jsonschema:"rules made by learn_terrain"`
	Origin Pos           `json:"origin" jsonschema:"position of the first character of the first row (north-west corner)"`
	Rows   []string      `json:"rows" jsonschema:"one string per row, north to south; spaces keep the tile as it is"`
	Legend []paintLegend `json:"legend"`
	Seed   int           `json:"seed,omitempty" jsonschema:"another seed picks other variants"`
	// Rim repaints the existing tiles around the plan so their borders fit.
	Rim bool `json:"rim,omitempty" jsonschema:"also repaint existing tiles of known classes right around the plan, so their borders fit the new terrain"`
}

// TileContent is a tile of set_tiles.
type TileContent struct {
	Pos    Pos   `json:"pos"`
	Ground int   `json:"ground" jsonschema:"ground item id (0: none)"`
	Items  []int `json:"items,omitempty" jsonschema:"item ids bottom to top"`
}

type setTilesArgs struct {
	MapArg
	Tiles []TileContent `json:"tiles"`
}

// maxSetTiles limits the tiles of one set_tiles call.
const maxSetTiles = 100000

type groundCountArgs struct {
	areaArgs
}

func (s *Server) addTerrainRuleTools(server *mcp.Server) {
	addTool(server, "count_grounds", "Counts the ground items of an area, with names: use it to define terrain classes for learn_terrain.",
		func(_ context.Context, args groundCountArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			area, err := args.area()
			if err != nil {
				return nil, err
			}
			specs, err := s.maps.AreaContents(id, area, args.Z)
			if err != nil {
				return nil, err
			}
			counts := map[uint16]int{}
			for _, spec := range specs {
				counts[spec.Ground]++
			}
			type groundCount struct {
				ID    uint16 `json:"id"`
				Name  string `json:"name"`
				Count int    `json:"count"`
			}
			out := []groundCount{}
			for ground, count := range counts {
				name := ""
				if item, err := s.maps.Item(ground); err == nil {
					name = item.Name
				}
				out = append(out, groundCount{ID: ground, Name: name, Count: count})
			}
			slices.SortFunc(out, func(first, second groundCount) int { return second.Count - first.Count })
			return out, nil
		})

	addTool(server, "learn_terrain", "Learns from areas of a real map how each terrain class looks next to each combination of neighbor classes (ground variants, borders, walls, decoration). paint_terrain then paints plans with those looks, so borders and walls fit the client without brushes.",
		func(_ context.Context, args learnArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			if !ruleNamePattern.MatchString(args.Name) {
				return nil, errors.New("the name may only have letters, digits, - and _")
			}
			if len(args.Classes) == 0 || len(args.Classes) > 200 {
				return nil, errors.New("give 1 to 200 classes")
			}
			rules := &terrainRules{Name: args.Name, Classes: args.Classes, Looks: map[string][]look{}}
			rules.index()
			learned, total := 0, 0
			ignore := map[uint16]bool{}
			for _, item := range args.Ignore {
				ignore[uint16(item)] = true
			}
			for _, sample := range args.Samples {
				area := image.Rect(sample.X, sample.Y, sample.X+sample.Width, sample.Y+sample.Height)
				total += area.Dx() * area.Dy()
				if area.Empty() || total > maxSampleTiles {
					return nil, errors.New("the samples are empty or too big")
				}
				specs, err := s.maps.AreaContents(id, area.Inset(-1), sample.Z)
				if err != nil {
					return nil, err
				}
				tiles := make(map[otmap.Position]mapedit.TileSpec, len(specs))
				for _, spec := range specs {
					tiles[spec.Pos] = spec
				}
				learned += rules.learn(tiles, area, sample.Z, ignore)
			}
			if learned == 0 {
				return nil, errors.New("no tile of the samples has a ground of the classes")
			}
			if err := s.terrain.save(rules); err != nil {
				return nil, err
			}
			perClass := map[string]int{}
			for key, looks := range rules.Looks {
				for _, seen := range looks {
					perClass[rules.Classes[key[0]].Name] += seen.Count
				}
			}
			return map[string]any{"rules": args.Name, "tiles": learned, "signatures": len(rules.Looks), "tilesPerClass": perClass}, nil
		})

	addTool(server, "set_tiles", "Sets tiles exactly: the ground and the items bottom to top (creatures and spawns stay), as one undo step. Ground 0 with no items removes what the tile shows.",
		func(_ context.Context, args setTilesArgs) (any, error) {
			if len(args.Tiles) > maxSetTiles {
				return nil, fmt.Errorf("at most %d tiles", maxSetTiles)
			}
			specs := make([]mapedit.TileSpec, 0, len(args.Tiles))
			for _, tile := range args.Tiles {
				pos, err := tile.Pos.position()
				if err != nil {
					return nil, err
				}
				ids, err := itemIDs(tile.Items)
				if err != nil {
					return nil, err
				}
				if tile.Ground < 0 || tile.Ground > 0xFFFF {
					return nil, errors.New("bad ground id")
				}
				specs = append(specs, mapedit.TileSpec{Pos: pos, Ground: uint16(tile.Ground), Items: ids})
			}
			stroke := s.nextStroke()
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.SetTiles(id, specs, stroke) })
		})

	addTool(server, "list_terrain_rules", "The terrain rules learned so far, with their classes.",
		func(context.Context, struct{}) (any, error) {
			out := []map[string]any{}
			for _, name := range s.terrain.names() {
				rules, err := s.terrain.load(name)
				if err != nil {
					continue
				}
				classes := make([]string, 0, len(rules.Classes))
				for _, class := range rules.Classes {
					classes = append(classes, class.Name)
				}
				out = append(out, map[string]any{"name": name, "classes": classes, "signatures": len(rules.Looks)})
			}
			return out, nil
		})

	addTool(server, "paint_terrain", "Paints a plan of terrain classes, one character per tile, with looks learned by learn_terrain: every tile gets a ground, borders, walls and decoration seen in the sample with the same neighbors. Replaces the ground and items of painted tiles (creatures and spawns stay) as one undo step.",
		func(_ context.Context, args paintArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			rules, err := s.terrain.load(args.Rules)
			if err != nil {
				return nil, err
			}
			return s.paintTerrain(id, rules, args)
		})
}

// paintTerrain paints a plan of classes.
func (s *Server) paintTerrain(id int, rules *terrainRules, args paintArgs) (any, error) {
	origin, err := args.Origin.position()
	if err != nil {
		return nil, err
	}
	if len(args.Rows) > maxPaintSide {
		return nil, fmt.Errorf("at most %d rows", maxPaintSide)
	}
	classByName := map[string]int{"none": rules.noneCode()}
	for index, class := range rules.Classes {
		classByName[class.Name] = index
	}
	bySymbol := map[rune]int{}
	for _, entry := range args.Legend {
		symbol := []rune(entry.Symbol)
		class, ok := classByName[entry.Class]
		if len(symbol) != 1 || symbol[0] == ' ' || !ok {
			return nil, fmt.Errorf("bad legend entry %q = %q (classes: %s)", entry.Symbol, entry.Class, strings.Join(slices.Sorted(mapsKeys(classByName)), ", "))
		}
		bySymbol[symbol[0]] = class
	}
	width := 0
	for _, row := range args.Rows {
		width = max(width, len([]rune(row)))
	}
	if width > maxPaintSide {
		return nil, fmt.Errorf("rows are at most %d characters", maxPaintSide)
	}
	area := image.Rect(int(origin.X), int(origin.Y), int(origin.X)+width, int(origin.Y)+len(args.Rows))
	floor := int(origin.Z)
	// The map around the plan: two tiles, for the rim and its neighbors.
	existing, err := s.maps.AreaContents(id, area.Inset(-2), floor)
	if err != nil {
		return nil, err
	}
	classes := map[[2]int]int{}
	for _, spec := range existing {
		classes[[2]int{int(spec.Pos.X), int(spec.Pos.Y)}] = rules.classOf(spec.Ground)
	}
	painted := map[[2]int]bool{}
	unknown := map[rune]bool{}
	for row, line := range args.Rows {
		for column, symbol := range []rune(line) {
			if symbol == ' ' {
				continue
			}
			class, ok := bySymbol[symbol]
			if !ok {
				unknown[symbol] = true
				continue
			}
			cell := [2]int{area.Min.X + column, area.Min.Y + row}
			if cell[0] > 0xFFFF || cell[1] > 0xFFFF {
				return nil, errors.New("the plan leaves the map")
			}
			classes[cell], painted[cell] = class, true
		}
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("symbols not in the legend: %s", string(slices.Sorted(mapsKeys(unknown))))
	}
	classAt := func(x, y int) int {
		if class, ok := classes[[2]int{x, y}]; ok {
			return class
		}
		return rules.noneCode()
	}
	targets := make([][2]int, 0, len(painted))
	for cell := range painted {
		targets = append(targets, cell)
	}
	if args.Rim {
		for cell := range painted {
			for _, offset := range neighborOffsets {
				near := [2]int{cell[0] + offset[0], cell[1] + offset[1]}
				if !painted[near] && classAt(near[0], near[1]) < rules.otherCode() && !slices.Contains(targets, near) {
					targets = append(targets, near)
				}
			}
		}
	}
	slices.SortFunc(targets, func(first, second [2]int) int {
		if first[1] != second[1] {
			return first[1] - second[1]
		}
		return first[0] - second[0]
	})
	random := rand.New(rand.NewPCG(uint64(args.Seed), uint64(area.Min.X)<<16|uint64(area.Min.Y)))
	specs := make([]mapedit.TileSpec, 0, len(targets))
	guessed := 0
	var guessedAt []Pos
	for _, cell := range targets {
		pos := otmap.Position{X: uint16(cell[0]), Y: uint16(cell[1]), Z: uint8(floor)}
		center := classAt(cell[0], cell[1])
		if center == rules.noneCode() {
			specs = append(specs, mapedit.TileSpec{Pos: pos})
			continue
		}
		var neighbors [8]int
		for i, offset := range neighborOffsets {
			neighbors[i] = classAt(cell[0]+offset[0], cell[1]+offset[1])
		}
		looks, exact := rules.looksFor(center, neighbors)
		if len(looks) == 0 {
			return nil, fmt.Errorf("the rules never saw class %s", rules.Classes[center].Name)
		}
		if !exact {
			guessed++
			if len(guessedAt) < 20 {
				guessedAt = append(guessedAt, Pos{X: cell[0], Y: cell[1], Z: floor})
			}
		}
		chosen := pick(looks, random)
		specs = append(specs, mapedit.TileSpec{Pos: pos, Ground: chosen.Ground, Items: chosen.Items})
	}
	change, err := s.maps.SetTiles(id, specs, s.nextStroke())
	s.changed(id, change)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"mapId": id, "tiles": len(specs), "changed": change.Count, "unsavedChanges": change.Info.Changed}
	if guessed > 0 {
		result["unseenNeighbors"] = guessed
		result["unseenAt"] = guessedAt
		result["hint"] = "these tiles have neighbors never seen together in the samples and got the look of the most similar case: check them, or learn from more samples"
	}
	return result, nil
}

func mapsKeys[K comparable, V any](values map[K]V) func(yield func(K) bool) {
	return func(yield func(K) bool) {
		for key := range values {
			if !yield(key) {
				return
			}
		}
	}
}

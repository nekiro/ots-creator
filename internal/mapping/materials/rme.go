package materials

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/nekiro/ots-creator/internal/formats/xmltree"
)

// ClientToServer converts a client id (RME "lookid") to a server id.
type ClientToServer func(clientID uint16) (uint16, bool)

const maxImportWarnings = 200

// maxVersionFolder is the highest folder name on the scale of client
// versions (4 digits); longer names are folded onto it.
const maxVersionFolder = 9999

type importer struct {
	materials *Materials
	warnings  []string
	visited   map[string]bool
	toServer  ClientToServer
}

func (im *importer) warn(format string, args ...any) {
	if len(im.warnings) < maxImportWarnings {
		im.warnings = append(im.warnings, fmt.Sprintf(format, args...))
	}
}

// FindRMEData returns the folder with a materials.xml inside dir: dir itself,
// or the version folder of an RME "data" folder closest to (and not newer
// than) clientVersion (e.g. 1098).
func FindRMEData(dir string, clientVersion int) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "materials.xml")); err == nil {
		return dir, nil
	}
	for _, sub := range []string{"data", filepath.Join("data", "materials")} {
		if _, err := os.Stat(filepath.Join(dir, sub, "materials.xml")); err == nil {
			return filepath.Join(dir, sub), nil
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	best, bestVersion := "", -1
	lowest, lowestVersion := "", math.MaxInt
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folderVersion, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, entry.Name(), "materials.xml")); err != nil {
			continue
		}
		// Folder names are 3-5 digits (740, 1098, 10100): compare on the
		// same scale as the client version.
		version := folderVersion
		for version > maxVersionFolder {
			version /= 10
		}
		if version <= clientVersion && version > bestVersion {
			best, bestVersion = entry.Name(), version
		}
		if version < lowestVersion {
			lowest, lowestVersion = entry.Name(), version
		}
	}
	if best == "" {
		best = lowest
	}
	if best == "" {
		return "", fmt.Errorf("no materials.xml found in %s", dir)
	}
	return filepath.Join(dir, best), nil
}

func ImportRME(dir string, toServer ClientToServer) (*Materials, []string, error) {
	im := &importer{materials: New(), visited: map[string]bool{}, toServer: toServer}
	if err := im.file(filepath.Join(dir, "materials.xml")); err != nil {
		return nil, nil, err
	}
	if data, err := os.ReadFile(filepath.Join(dir, "creatures.xml")); err == nil {
		creatures, err := ParseRMECreatures(data)
		if err != nil {
			im.warn("creatures.xml: %v", err)
		}
		im.materials.Creatures = append(im.materials.Creatures, creatures...)
	}
	// Brushes declared but never defined (forward declarations of another
	// version) are useless.
	im.materials.Brushes = slices.DeleteFunc(im.materials.Brushes, func(brush *Brush) bool {
		return brush.Type == "" || (brush.Type == TypeGround && len(brush.Items) == 0 && len(brush.Alternatives) == 0)
	})
	return im.materials, im.warnings, nil
}

func (im *importer) file(path string) error {
	abs, _ := filepath.Abs(path)
	if im.visited[abs] {
		return nil
	}
	im.visited[abs] = true
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	root, err := xmltree.Parse(data)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	for _, node := range root.Children {
		switch node.Name {
		case "include":
			include := node.Str("file")
			if include == "" {
				continue
			}
			if err := im.file(filepath.Join(filepath.Dir(path), include)); err != nil {
				im.warn("include %s: %v", include, err)
			}
		case "metaitem":
			from, to := idRange(node)
			for id := from; id <= to && id != 0; id++ {
				im.materials.MetaItems = append(im.materials.MetaItems, uint16(id))
			}
		case "border":
			border := borderNode(node)
			if border.ID == 0 {
				im.warn("border without an id")
				continue
			}
			if existing := im.materials.Border(border.ID); existing != nil {
				*existing = *border
			} else {
				im.materials.Borders = append(im.materials.Borders, border)
			}
		case "brush":
			im.brush(node)
		case "tileset":
			im.tileset(node)
		}
	}
	return nil
}

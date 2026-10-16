package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/formats/assets"
	"github.com/nekiro/ots-creator/internal/formats/dat"
)

// Format is the file format of a client.
type Format string

const (
	// FormatDat is a Tibia.dat + Tibia.spr pair (up to 10.x, OTClient).
	FormatDat Format = "dat"
	// FormatAssets is the asset folder of Tibia 12+: protobuf appearances
	// and LZMA sprite sheets.
	FormatAssets Format = "assets"
)

// assetFeatures are fixed by the format: 32 bit ids, alpha, per frame
// durations and outfit frame groups.
var assetFeatures = client.Features{Extended: true, Transparency: true, ImprovedAnimations: true, FrameGroups: true, SpriteSize: client.DefaultSpriteSize}

// assetState is what an assets project needs to write the folder back.
type assetState struct {
	dir     string
	catalog *assets.Catalog
	pieces  *assets.PieceMap
	file    *assets.File
}

// AssetsVersion returns the version shown for an asset folder.
func AssetsVersion(dir string) client.Version {
	version := client.Version{Value: 1200, Name: "12+ (assets)"}
	if name := assets.ClientVersion(dir); name != "" {
		major, minor, _ := strings.Cut(name, ".")
		majorNumber, _ := strconv.Atoi(major)
		minorNumber, _ := strconv.Atoi(minor)
		if majorNumber > 0 {
			version = client.Version{Value: uint16(majorNumber*100 + min(minorNumber, 99)), Name: name}
		}
	}
	return version
}

// OpenAssets loads an asset folder (or its catalog, or a Tibia install
// folder).
func OpenAssets(path string) (*Project, error) {
	dir, err := assets.FindDir(path)
	if err != nil {
		return nil, err
	}
	state, err := loadAssets(dir)
	if err != nil {
		return nil, err
	}
	project := &Project{
		version:  AssetsVersion(dir),
		features: assetFeatures,
		format:   FormatAssets,
		datPath:  dir,
		things:   &dat.File{Things: state.file.Things},
		sprites:  newSpriteStore(newSheetBase(dir, state.catalog, state.pieces), assetFeatures.SpriteSize, true),
		assets:   state,
	}
	project.resetBase()
	return project, nil
}

func loadAssets(dir string) (*assetState, error) {
	catalog, err := assets.ReadCatalog(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, catalog.Appearances))
	if err != nil {
		return nil, err
	}
	if err := checkSheets(dir, catalog); err != nil {
		return nil, err
	}
	pieces := assets.NewPieceMap(catalog)
	file, err := assets.Decode(data, pieces)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", catalog.Appearances, err)
	}
	return &assetState{dir: dir, catalog: catalog, pieces: pieces, file: file}, nil
}

// checkSheets fails when sprite sheets of the catalog are missing, which
// would otherwise show as objects without pixels.
func checkSheets(dir string, catalog *assets.Catalog) error {
	missing, first := 0, ""
	for _, sheet := range catalog.Sheets {
		if _, err := os.Stat(filepath.Join(dir, sheet.File)); err != nil {
			if missing == 0 {
				first = sheet.File
			}
			missing++
		}
	}
	if missing > 0 {
		return fmt.Errorf("%d of %d sprite sheets listed in %s are missing (first: %s)", missing, len(catalog.Sheets), assets.CatalogFile, first)
	}
	return nil
}

package app

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/formats/assets"
	"github.com/nekiro/ots-creator/internal/fsutil"
)

// ClientFiles describes a dat/spr pair found on disk before opening it.
type ClientFiles struct {
	// Format is FormatAssets for a Tibia 12+ asset folder; DatPath is then
	// the folder and Detected its version.
	Format       project.Format  `json:"format"`
	DatPath      string          `json:"datPath"`
	SprPath      string          `json:"sprPath"`
	DatSignature uint32          `json:"datSignature"`
	SprSignature uint32          `json:"sprSignature"`
	Detected     *client.Version `json:"detected"`
	Features     client.Features `json:"features"`
	HasOTFI      bool            `json:"hasOtfi"`
	// OTBPath is the items.otb remembered for the client ("" when none).
	OTBPath string `json:"otbPath"`
}

// inspectClient finds the client files of a client directory or of any of
// its dat, spr or otfi files: their signatures, version and OTFI features.
func inspectClient(path string) (ClientFiles, error) {
	// A dat or otfi in the chosen folder wins over an assets subfolder
	// (some clients ship both).
	_, datErr := findClientFile(path)
	topDat := datErr == nil && !fsutil.FileExists(filepath.Join(path, assets.CatalogFile))
	if dir, err := assets.FindDir(path); err == nil && !topDat {
		version := project.AssetsVersion(dir)
		return ClientFiles{Format: project.FormatAssets, DatPath: dir, Detected: &version, Features: client.DefaultFeatures(version.Value)}, nil
	}
	files := ClientFiles{Format: project.FormatDat}
	if fsutil.DirExists(path) {
		found, err := findClientFile(path)
		if err != nil {
			return files, err
		}
		path = found
	}
	dir := filepath.Dir(path)
	base := fsutil.TrimExt(filepath.Base(path))
	files.DatPath = filepath.Join(dir, base+".dat")
	files.SprPath = filepath.Join(dir, base+".spr")
	if err := files.applyOTFI(filepath.Join(dir, base+".otfi")); err != nil {
		return files, err
	}
	var err error
	if files.DatSignature, err = readSignature(files.DatPath); err != nil {
		return files, err
	}
	if files.SprSignature, err = readSignature(files.SprPath); err != nil {
		return files, splitSpritesError(files.SprPath, err)
	}
	if version, ok := client.FindBySignatures(files.DatSignature, files.SprSignature); ok {
		files.Detected = &version
		if !files.HasOTFI {
			files.Features = client.DefaultFeatures(version.Value)
		}
	}
	if files.Features.SpriteSize == 0 {
		files.Features.SpriteSize = client.DefaultSpriteSize
	}
	return files, nil
}

// applyOTFI reads the features and file names of an OTFI file, when there
// is one.
func (files *ClientFiles) applyOTFI(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	otfi, err := client.ParseOTFI(data)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	files.HasOTFI = true
	files.Features = otfi.Features
	if otfi.MetadataFile != "" {
		files.DatPath = filepath.Join(dir, otfi.MetadataFile)
	}
	if otfi.SpritesFile != "" {
		files.SprPath = filepath.Join(dir, otfi.SpritesFile)
	}
	return nil
}

// splitSpritesError explains a missing spr of a client that splits its
// sprites into Tibia1.spr, Tibia2.spr, …, which is not supported.
func splitSpritesError(sprPath string, err error) error {
	base := strings.TrimSuffix(sprPath, ".spr")
	if !os.IsNotExist(err) || !fsutil.FileExists(base+"1.spr") {
		return err
	}
	name := filepath.Base(base)
	return fmt.Errorf("%s is missing; this client splits its sprites into %s1.spr, %s2.spr, … which is not supported",
		filepath.Base(sprPath), name, name)
}

// findClientFile picks the client inside dir: an .otfi first (it names its
// dat and spr), then a .dat, preferring the Tibia base name.
func findClientFile(dir string) (string, error) {
	for _, ext := range []string{".otfi", ".dat"} {
		if path := filepath.Join(dir, "Tibia"+ext); fsutil.FileExists(path) {
			return path, nil
		}
		matches, _ := filepath.Glob(filepath.Join(dir, "*"+ext))
		if len(matches) > 0 {
			return matches[0], nil
		}
	}
	return "", fmt.Errorf("no .dat or .otfi file in %s", dir)
}

// readSignature reads the uint32 signature a dat or spr file starts with.
func readSignature(path string) (uint32, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	var signature [4]byte
	if _, err := io.ReadFull(file, signature[:]); err != nil {
		return 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return binary.LittleEndian.Uint32(signature[:]), nil
}

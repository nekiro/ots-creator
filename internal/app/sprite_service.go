package app

import (
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/fsutil"
)

type SpriteService struct {
	session *Session
}

func NewSpriteService(session *Session) *SpriteService { return &SpriteService{session: session} }

// ImportImages slices images into sprites and appends them. Magenta is
// treated as transparent; fully transparent tiles are skipped.
func (ss *SpriteService) ImportImages(paths []string) ([]uint32, error) {
	proj, err := ss.session.Project()
	if err != nil {
		return nil, err
	}
	var pixels [][]byte
	for _, path := range paths {
		img, err := loadSpriteImage(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		for _, tile := range imaging.SliceTiles(img, proj.SpriteSize()) {
			if !isTransparent(tile) {
				pixels = append(pixels, tile)
			}
		}
	}
	if len(pixels) == 0 {
		return nil, nil
	}
	ids, err := proj.AddSprites(pixels)
	if err == nil {
		ss.session.Changed()
	}
	return ids, err
}

func loadSpriteImage(path string) (*image.NRGBA, error) {
	img, err := imaging.Load(path)
	if err != nil {
		return nil, err
	}
	imaging.RemoveMagenta(img)
	imaging.Normalize(img)
	return img, nil
}

func isTransparent(pixels []byte) bool {
	for alpha := 3; alpha < len(pixels); alpha += 4 {
		if pixels[alpha] != 0 {
			return false
		}
	}
	return true
}

func (ss *SpriteService) Replace(id uint32, path string) error {
	proj, err := ss.session.Project()
	if err != nil {
		return err
	}
	img, err := loadSpriteImage(path)
	if err != nil {
		return err
	}
	if err := proj.ReplaceSprite(id, imaging.Tile(img, 0, 0, proj.SpriteSize())); err != nil {
		return err
	}
	ss.session.Changed()
	return nil
}

// Remove clears sprites (the last one is deleted).
func (ss *SpriteService) Remove(ids []uint32) error {
	proj, err := ss.session.Project()
	if err != nil {
		return err
	}
	if err := proj.RemoveSprites(ids); err != nil {
		return err
	}
	ss.session.Changed()
	return nil
}

// Export writes sprites as {id}.{format} into dir (png, bmp or jpg;
// formats without alpha get a magenta background).
func (ss *SpriteService) Export(ids []uint32, dir, format string) ([]string, error) {
	format = imaging.FormatOf("x." + format)
	proj, err := ss.session.Project()
	if err != nil {
		return nil, err
	}
	size := proj.SpriteSize()
	var out []string
	for _, id := range ids {
		path, err := writeSpriteFile(proj, id, size, dir, format)
		if err != nil {
			return out, err
		}
		out = append(out, path)
	}
	return out, nil
}

func writeSpriteFile(proj *project.Project, id uint32, size int, dir, format string) (string, error) {
	pixels, err := proj.SpritePixels(id)
	if err != nil {
		return "", err
	}
	data, err := imaging.Encode(spriteImage(pixels, size), format, imaging.Magenta)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.%s", id, format))
	return path, os.WriteFile(path, data, fsutil.FileMode)
}

// AddPixels appends sprites from raw RGBA pixels (size*size*4 bytes each),
// for example cut by the slicer. Fully transparent sprites are skipped.
func (ss *SpriteService) AddPixels(sprites [][]byte) ([]uint32, error) {
	proj, err := ss.session.Project()
	if err != nil {
		return nil, err
	}
	var pixels [][]byte
	for _, sprite := range sprites {
		if !isTransparent(sprite) {
			pixels = append(pixels, sprite)
		}
	}
	if len(pixels) == 0 {
		return []uint32{}, nil
	}
	ids, err := proj.AddSprites(pixels)
	if err == nil {
		ss.session.Changed()
	}
	return ids, err
}

func (ss *SpriteService) Optimize(options project.OptimizeOptions) (project.OptimizeResult, error) {
	proj, err := ss.session.Project()
	if err != nil {
		return project.OptimizeResult{}, err
	}
	result, err := proj.OptimizeSprites(options, ss.session.reporter("Optimizing sprites"))
	if err == nil && (result.Before != result.After || result.Things > 0) {
		ss.session.Changed()
	}
	return result, err
}

func (ss *SpriteService) Analyze(options project.OptimizeOptions) (project.OptimizeResult, error) {
	proj, err := ss.session.Project()
	if err != nil {
		return project.OptimizeResult{}, err
	}
	return proj.AnalyzeSprites(options, ss.session.reporter("Checking sprites"))
}

func (ss *SpriteService) Find(filter string) ([]uint32, error) {
	proj, err := ss.session.Project()
	if err != nil {
		return nil, err
	}
	return proj.FindSprites(filter)
}

func (ss *SpriteService) Users(id uint32) ([]project.ThingRef, error) {
	proj, err := ss.session.Project()
	if err != nil {
		return nil, err
	}
	return proj.SpriteUsers(id), nil
}

// ReplaceRefs points every use of the sprites in from at sprite to (0
// clears them) and returns the number of changed objects.
func (ss *SpriteService) ReplaceRefs(from []uint32, to uint32) (int, error) {
	proj, err := ss.session.Project()
	if err != nil {
		return 0, err
	}
	changed, err := proj.ReplaceSpriteRefs(from, to)
	if err == nil && changed > 0 {
		ss.session.Changed()
	}
	return changed, err
}

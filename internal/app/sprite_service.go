package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/project"
)

// SpriteService edits sprites.
type SpriteService struct {
	s *Session
}

// NewSpriteService returns the service.
func NewSpriteService(s *Session) *SpriteService { return &SpriteService{s: s} }

// ImportImages slices images into sprites and appends them. Magenta is
// treated as transparent; fully transparent tiles are skipped.
func (ss *SpriteService) ImportImages(paths []string) ([]uint32, error) {
	p, err := ss.s.Project()
	if err != nil {
		return nil, err
	}
	var pixels [][]byte
	for _, path := range paths {
		img, err := imaging.Load(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		imaging.RemoveMagenta(img)
		imaging.Normalize(img)
		for _, tile := range imaging.SliceTiles(img, p.SpriteSize()) {
			if !isTransparent(tile) {
				pixels = append(pixels, tile)
			}
		}
	}
	if len(pixels) == 0 {
		return nil, nil
	}
	ids, err := p.AddSprites(pixels)
	if err == nil {
		ss.s.Changed()
	}
	return ids, err
}

func isTransparent(px []byte) bool {
	for i := 3; i < len(px); i += 4 {
		if px[i] != 0 {
			return false
		}
	}
	return true
}

// Replace overwrites a sprite with the first tile of an image.
func (ss *SpriteService) Replace(id uint32, path string) error {
	p, err := ss.s.Project()
	if err != nil {
		return err
	}
	img, err := imaging.Load(path)
	if err != nil {
		return err
	}
	imaging.RemoveMagenta(img)
	imaging.Normalize(img)
	if err := p.ReplaceSprite(id, imaging.Tile(img, 0, 0, p.SpriteSize())); err != nil {
		return err
	}
	ss.s.Changed()
	return nil
}

// Remove clears sprites (the last one is deleted).
func (ss *SpriteService) Remove(ids []uint32) error {
	p, err := ss.s.Project()
	if err != nil {
		return err
	}
	if err := p.RemoveSprites(ids); err != nil {
		return err
	}
	ss.s.Changed()
	return nil
}

// Export writes sprites as {id}.{format} into dir (png, bmp or jpg;
// formats without alpha get a magenta background).
func (ss *SpriteService) Export(ids []uint32, dir, format string) ([]string, error) {
	format = imaging.FormatOf("x." + format)
	p, err := ss.s.Project()
	if err != nil {
		return nil, err
	}
	size := p.SpriteSize()
	var out []string
	for _, id := range ids {
		px, err := p.SpritePixels(id)
		if err != nil {
			return out, err
		}
		img := imagingNRGBA(px, size)
		data, err := imaging.Encode(img, format, imaging.Magenta)
		if err != nil {
			return out, err
		}
		path := filepath.Join(dir, fmt.Sprintf("%d.%s", id, format))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return out, err
		}
		out = append(out, path)
	}
	return out, nil
}

// AddPixels appends sprites from raw RGBA pixels (size*size*4 bytes each),
// for example cut by the slicer. Fully transparent sprites are skipped.
func (ss *SpriteService) AddPixels(sprites [][]byte) ([]uint32, error) {
	p, err := ss.s.Project()
	if err != nil {
		return nil, err
	}
	var pixels [][]byte
	for _, px := range sprites {
		if !isTransparent(px) {
			pixels = append(pixels, px)
		}
	}
	if len(pixels) == 0 {
		return []uint32{}, nil
	}
	ids, err := p.AddSprites(pixels)
	if err == nil {
		ss.s.Changed()
	}
	return ids, err
}

// Optimize removes duplicate, empty and unused sprites and renumbers the
// rest (see project.OptimizeSprites).
func (ss *SpriteService) Optimize(o project.OptimizeOptions) (project.OptimizeResult, error) {
	p, err := ss.s.Project()
	if err != nil {
		return project.OptimizeResult{}, err
	}
	res, err := p.OptimizeSprites(o)
	if err == nil && (res.Before != res.After || res.Things > 0) {
		ss.s.Changed()
	}
	return res, err
}

// Find returns the ids of sprites matching a filter (see project.FindSprites).
func (ss *SpriteService) Find(filter string) ([]uint32, error) {
	p, err := ss.s.Project()
	if err != nil {
		return nil, err
	}
	return p.FindSprites(filter)
}

// Users lists the objects that use a sprite.
func (ss *SpriteService) Users(id uint32) ([]project.ThingRef, error) {
	p, err := ss.s.Project()
	if err != nil {
		return nil, err
	}
	return p.SpriteUsers(id), nil
}

// ReplaceRefs points every use of the sprites in from at sprite to (0
// clears them) and returns the number of changed objects.
func (ss *SpriteService) ReplaceRefs(from []uint32, to uint32) (int, error) {
	p, err := ss.s.Project()
	if err != nil {
		return 0, err
	}
	n, err := p.ReplaceSpriteRefs(from, to)
	if err == nil && n > 0 {
		ss.s.Changed()
	}
	return n, err
}

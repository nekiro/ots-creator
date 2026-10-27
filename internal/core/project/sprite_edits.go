package project

import (
	"fmt"
	"slices"

	"github.com/nekiro/ots-creator/internal/formats/spr"
)

func (p *Project) SpritePixels(id uint32) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sprites.pixels(id)
}

func (p *Project) IsSpriteEmpty(id uint32) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	compressed, err := p.sprites.compressed(id)
	return len(compressed) == 0, err
}

func (p *Project) checkPixels(pixels []byte) error {
	want := p.features.SpriteSize * p.features.SpriteSize * 4
	if len(pixels) != want {
		return fmt.Errorf("sprite pixels have %d bytes, want %d", len(pixels), want)
	}
	return nil
}

func (p *Project) AddSprites(pixels [][]byte) ([]uint32, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, spritePixels := range pixels {
		if err := p.checkPixels(spritePixels); err != nil {
			return nil, err
		}
	}
	edit := p.record(fmt.Sprintf("Add %d sprite(s)", len(pixels)))
	ids := p.addSpritesLocked(edit, pixels)
	edit.commit()
	return ids, nil
}

func (p *Project) addSpritesLocked(edit *recorder, pixels [][]byte) []uint32 {
	ids := make([]uint32, len(pixels))
	for i, spritePixels := range pixels {
		id := p.sprites.count() + 1
		edit.setSpriteCount(id)
		edit.setSprite(id, spr.Compress(spritePixels, p.features.SpriteSize, p.features.Transparency))
		ids[i] = id
	}
	return ids
}

func (p *Project) ReplaceSprite(id uint32, pixels []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.checkPixels(pixels); err != nil {
		return err
	}
	if id == 0 || id > p.sprites.count() {
		return fmt.Errorf("sprite %d does not exist", id)
	}
	edit := p.record(fmt.Sprintf("Replace sprite %d", id))
	edit.setSprite(id, spr.Compress(pixels, p.features.SpriteSize, p.features.Transparency))
	edit.commit()
	return nil
}

// RemoveSprites clears sprites. The last sprite is deleted, others become
// empty so references stay valid.
func (p *Project) RemoveSprites(ids []uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	slices.Reverse(sorted)
	edit := p.record(fmt.Sprintf("Remove %d sprite(s)", len(ids)))
	for _, id := range sorted {
		if id == 0 || id > p.sprites.count() {
			return fmt.Errorf("sprite %d does not exist", id)
		}
		edit.setSprite(id, nil)
		if id == p.sprites.count() {
			edit.setSpriteCount(id - 1)
		}
	}
	edit.commit()
	return nil
}

// addSpritesDedup adds sprites, mapping fully transparent ones to id 0 and
// reusing identical sprites within the same import.
func (p *Project) addSpritesDedup(edit *recorder, pixels [][]byte) []uint32 {
	ids := make([]uint32, len(pixels))
	seen := map[string]uint32{}
	for i, spritePixels := range pixels {
		compressed := spr.Compress(spritePixels, p.features.SpriteSize, p.features.Transparency)
		if len(compressed) == 0 {
			continue
		}
		if id, ok := seen[string(compressed)]; ok {
			ids[i] = id
			continue
		}
		id := p.sprites.count() + 1
		edit.setSpriteCount(id)
		edit.setSprite(id, compressed)
		seen[string(compressed)] = id
		ids[i] = id
	}
	return ids
}

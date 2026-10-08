package client

// DefaultSpriteSize is the edge length of a classic sprite in pixels.
const DefaultSpriteSize = 32

// Features are the format switches that change how dat/spr files are encoded.
type Features struct {
	// Extended stores sprite ids and the sprite count as u32 instead of u16.
	Extended bool `json:"extended"`
	// Transparency stores an alpha channel for every colored sprite pixel.
	Transparency bool `json:"transparency"`
	// ImprovedAnimations stores per-frame durations, loop count and start frame.
	ImprovedAnimations bool `json:"improvedAnimations"`
	// FrameGroups stores several frame groups (idle, walking) for outfits.
	FrameGroups bool `json:"frameGroups"`
	// SpriteSize is the sprite edge length in pixels (32 for official clients).
	SpriteSize int `json:"spriteSize"`
}

// DefaultFeatures returns the features forced by a client version.
func DefaultFeatures(version uint16) Features {
	f := Features{SpriteSize: DefaultSpriteSize}
	f.ApplyVersionDefaults(version)
	return f
}

// ApplyVersionDefaults turns on the features that are mandatory for a version.
// It never turns a feature off.
func (f *Features) ApplyVersionDefaults(version uint16) {
	if version >= 960 {
		f.Extended = true
	}
	if version >= 1050 {
		f.ImprovedAnimations = true
	}
	if version >= 1057 {
		f.FrameGroups = true
	}
	if f.SpriteSize == 0 {
		f.SpriteSize = DefaultSpriteSize
	}
}

// MetadataFormat returns which flag table a client version uses (1..6).
func MetadataFormat(version uint16) int {
	switch {
	case version <= 730:
		return 1
	case version <= 750:
		return 2
	case version <= 772:
		return 3
	case version <= 854:
		return 4
	case version <= 986:
		return 5
	}
	return 6
}

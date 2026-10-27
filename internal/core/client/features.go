package client

const DefaultSpriteSize = 32

type Features struct {
	// Extended stores sprite ids and the sprite count as u32 instead of u16.
	Extended bool `json:"extended"`
	// Transparency stores an alpha channel for every colored sprite pixel.
	Transparency bool `json:"transparency"`
	// ImprovedAnimations stores per-frame durations, loop count and start frame.
	ImprovedAnimations bool `json:"improvedAnimations"`
	// FrameGroups stores several frame groups (idle, walking) for outfits.
	FrameGroups bool `json:"frameGroups"`
	SpriteSize  int  `json:"spriteSize"`
}

func DefaultFeatures(version uint16) Features {
	features := Features{SpriteSize: DefaultSpriteSize}
	features.ApplyVersionDefaults(version)
	return features
}

// First client versions whose files require a feature.
const (
	extendedVersion           = 960
	improvedAnimationsVersion = 1050
	frameGroupsVersion        = 1057
)

// ApplyVersionDefaults turns on the features that are mandatory for a version.
// It never turns a feature off.
func (f *Features) ApplyVersionDefaults(version uint16) {
	if version >= extendedVersion {
		f.Extended = true
	}
	if version >= improvedAnimationsVersion {
		f.ImprovedAnimations = true
	}
	if version >= frameGroupsVersion {
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

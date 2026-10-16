package market

import "math"

// Outfit previews are painted with the default colors of the editor
// (frontend/src/lib/render/outfit.ts): head 78, body 69, legs 58, feet 76.
var defaultColors = [4]int{78, 69, 58, 76}

// hsiToRGB converts an outfit palette index (0..132) to RGB, like the
// client's ColorUtils.HSItoRGB.
func hsiToRGB(color int) [3]uint8 {
	const steps, values = 19, 7
	if color < 0 || color >= steps*values {
		color = 0
	}
	var hue, saturation, intensity float64
	if color%steps == 0 {
		intensity = 1 - float64(color)/steps/values
	} else {
		hue = float64(color%steps) * (1.0 / 18)
		saturation, intensity = 1, 1
		switch color / steps {
		case 0:
			saturation, intensity = 0.25, 1
		case 1:
			saturation, intensity = 0.25, 0.75
		case 2:
			saturation, intensity = 0.5, 0.75
		case 3:
			saturation, intensity = 0.667, 0.75
		case 4:
			saturation, intensity = 1, 1
		case 5:
			saturation, intensity = 1, 0.75
		case 6:
			saturation, intensity = 1, 0.5
		}
	}
	if intensity == 0 {
		return [3]uint8{}
	}
	if saturation == 0 {
		gray := uint8(math.Floor(intensity * 0xff))
		return [3]uint8{gray, gray, gray}
	}
	var red, green, blue float64
	switch {
	case hue < 1.0/6:
		red, blue = intensity, intensity*(1-saturation)
		green = blue + (intensity-blue)*6*hue
	case hue < 2.0/6:
		green, blue = intensity, intensity*(1-saturation)
		red = green - (intensity-blue)*(6*hue-1)
	case hue < 3.0/6:
		green, red = intensity, intensity*(1-saturation)
		blue = red + (intensity-red)*(6*hue-2)
	case hue < 4.0/6:
		blue, red = intensity, intensity*(1-saturation)
		green = blue - (intensity-red)*(6*hue-3)
	case hue < 5.0/6:
		blue, green = intensity, intensity*(1-saturation)
		red = green + (intensity-green)*(6*hue-4)
	default:
		red, green = intensity, intensity*(1-saturation)
		blue = red - (intensity-green)*(6*hue-5)
	}
	toByte := func(gray float64) uint8 { return uint8(int(math.Floor(gray*0xff)) & 0xff) }
	return [3]uint8{toByte(red), toByte(green), toByte(blue)}
}

// colorize multiplies base pixels by the default outfit colors where the
// template (mask layer) marks them: yellow head, red body, green legs, blue
// feet. Both are RGBA buffers of the same size.
func colorize(base, template []byte) {
	var colors [4][3]uint8
	for part, paletteIndex := range defaultColors {
		colors[part] = hsiToRGB(paletteIndex)
	}
	for offset := 0; offset+3 < len(base); offset += 4 {
		if template[offset+3] == 0 || base[offset+3] == 0 {
			continue
		}
		red, green, blue := template[offset] > 0, template[offset+1] > 0, template[offset+2] > 0
		part := -1
		switch {
		case red && green && !blue:
			part = 0
		case red && !green && !blue:
			part = 1
		case !red && green && !blue:
			part = 2
		case !red && !green && blue:
			part = 3
		}
		if part < 0 {
			continue
		}
		for j := range 3 {
			base[offset+j] = uint8(int(base[offset+j]) * int(colors[part][j]) / 255)
		}
	}
}

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
	var h, s, i float64
	if color%steps == 0 {
		i = 1 - float64(color)/steps/values
	} else {
		h = float64(color%steps) * (1.0 / 18)
		s, i = 1, 1
		switch color / steps {
		case 0:
			s, i = 0.25, 1
		case 1:
			s, i = 0.25, 0.75
		case 2:
			s, i = 0.5, 0.75
		case 3:
			s, i = 0.667, 0.75
		case 4:
			s, i = 1, 1
		case 5:
			s, i = 1, 0.75
		case 6:
			s, i = 1, 0.5
		}
	}
	if i == 0 {
		return [3]uint8{}
	}
	if s == 0 {
		v := uint8(math.Floor(i * 0xff))
		return [3]uint8{v, v, v}
	}
	var r, g, b float64
	switch {
	case h < 1.0/6:
		r, b = i, i*(1-s)
		g = b + (i-b)*6*h
	case h < 2.0/6:
		g, b = i, i*(1-s)
		r = g - (i-b)*(6*h-1)
	case h < 3.0/6:
		g, r = i, i*(1-s)
		b = r + (i-r)*(6*h-2)
	case h < 4.0/6:
		b, r = i, i*(1-s)
		g = b - (i-r)*(6*h-3)
	case h < 5.0/6:
		b, g = i, i*(1-s)
		r = g + (i-g)*(6*h-4)
	default:
		r, g = i, i*(1-s)
		b = r - (i-g)*(6*h-5)
	}
	c := func(v float64) uint8 { return uint8(int(math.Floor(v*0xff)) & 0xff) }
	return [3]uint8{c(r), c(g), c(b)}
}

// colorize multiplies base pixels by the default outfit colors where the
// template (mask layer) marks them: yellow head, red body, green legs, blue
// feet. Both are RGBA buffers of the same size.
func colorize(base, template []byte) {
	var colors [4][3]uint8
	for k, c := range defaultColors {
		colors[k] = hsiToRGB(c)
	}
	for p := 0; p+3 < len(base); p += 4 {
		if template[p+3] == 0 || base[p+3] == 0 {
			continue
		}
		r, g, b := template[p] > 0, template[p+1] > 0, template[p+2] > 0
		k := -1
		switch {
		case r && g && !b:
			k = 0
		case r && !g && !b:
			k = 1
		case !r && g && !b:
			k = 2
		case !r && !g && b:
			k = 3
		}
		if k < 0 {
			continue
		}
		for j := range 3 {
			base[p+j] = uint8(int(base[p+j]) * int(colors[k][j]) / 255)
		}
	}
}

package app

import "image"

func imagingNRGBA(px []byte, size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	copy(img.Pix, px)
	return img
}

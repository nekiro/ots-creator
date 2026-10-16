package app

import "image"

// spriteImage wraps the RGBA pixels of a sprite of size x size pixels.
func spriteImage(pixels []byte, size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	copy(img.Pix, pixels)
	return img
}

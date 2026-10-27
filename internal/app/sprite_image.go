package app

import "image"

func spriteImage(pixels []byte, size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	copy(img.Pix, pixels)
	return img
}

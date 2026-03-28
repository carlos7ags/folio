// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"fmt"
	goimage "image"
	"image/color"
	"image/png"
	"os"
)

// NewPNG creates an Image from raw PNG data.
// It decodes the PNG to extract pixel data, then re-compresses with FlateDecode.
// Alpha channels are extracted into a separate SMask.
func NewPNG(data []byte) (*Image, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("png: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	hasAlpha := imageHasAlpha(img)

	if hasAlpha {
		return buildRGBA(img, w, h)
	}
	return buildRGB(img, w, h)
}

// LoadPNG reads a PNG file and creates an Image.
func LoadPNG(path string) (*Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewPNG(data)
}

// buildRGB extracts pixel data without alpha. Grayscale images are encoded
// as DeviceGray; all others are encoded as DeviceRGB.
func buildRGB(img goimage.Image, w, h int) (*Image, error) {
	bounds := img.Bounds()

	// Check if the image is grayscale before allocating buffers.
	if isGrayscale(img) {
		grayPixels := make([]byte, 0, w*h)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, _, _, _ := img.At(x, y).RGBA()
				grayPixels = append(grayPixels, byte(r>>8))
			}
		}
		return &Image{
			data:       grayPixels,
			width:      w,
			height:     h,
			colorSpace: "DeviceGray",
			bpc:        8,
			filter:     "FlateDecode",
		}, nil
	}

	pixels := make([]byte, 0, w*h*3)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			pixels = append(pixels, byte(r>>8), byte(g>>8), byte(b>>8))
		}
	}

	return &Image{
		data:       pixels,
		width:      w,
		height:     h,
		colorSpace: "DeviceRGB",
		bpc:        8,
		filter:     "FlateDecode",
	}, nil
}

// buildRGBA extracts RGB pixel data and alpha channel separately.
// PDF requires straight (non-premultiplied) alpha: the RGB bytes must
// contain the original colors, with alpha stored in a separate SMask.
// Go's color.RGBA() method returns premultiplied values, so we must
// use non-premultiplied access paths to get correct colors.
func buildRGBA(img goimage.Image, w, h int) (*Image, error) {
	bounds := img.Bounds()
	pixels := make([]byte, 0, w*h*3)
	alpha := make([]byte, 0, w*h)

	switch src := img.(type) {
	case *goimage.NRGBA:
		// NRGBA stores straight (non-premultiplied) RGBA in Pix.
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				off := (y-bounds.Min.Y)*src.Stride + (x-bounds.Min.X)*4
				pixels = append(pixels, src.Pix[off], src.Pix[off+1], src.Pix[off+2])
				alpha = append(alpha, src.Pix[off+3])
			}
		}
	default:
		// Generic path: convert each pixel to NRGBA to get straight alpha.
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				pixels = append(pixels, c.R, c.G, c.B)
				alpha = append(alpha, c.A)
			}
		}
	}

	return &Image{
		data:       pixels,
		width:      w,
		height:     h,
		colorSpace: "DeviceRGB",
		bpc:        8,
		filter:     "FlateDecode",
		smask:      alpha,
		smaskW:     w,
		smaskH:     h,
	}, nil
}

// imageHasAlpha reports whether any pixel in the image has non-opaque alpha.
func imageHasAlpha(img goimage.Image) bool {
	switch img.(type) {
	case *goimage.NRGBA, *goimage.NRGBA64, *goimage.RGBA, *goimage.RGBA64:
		// These types can have alpha. Check if any pixel is non-opaque.
		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				if a != 0xFFFF {
					return true
				}
			}
		}
	}
	return false
}

// isGrayscale reports whether the image uses a grayscale color model.
func isGrayscale(img goimage.Image) bool {
	switch img.ColorModel() {
	case color.GrayModel, color.Gray16Model:
		return true
	}
	return false
}

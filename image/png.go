// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"fmt"
	goimage "image"
	"image/color"
	"image/png"
)

// NewPNG creates an Image from raw PNG data. It decodes the PNG and
// re-encodes pixels for FlateDecode embedding. Alpha channels are
// extracted into a separate soft mask. Dimensions are validated against
// [MaxDimension] and [MaxPixels] before the pixel buffer is allocated.
func NewPNG(data []byte) (*Image, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("png: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if err := checkDimensions(w, h); err != nil {
		return nil, fmt.Errorf("png: %w", err)
	}

	if isGrayscale(img) {
		return buildGray(img, w, h)
	}
	return buildRGBMaybeAlpha(img, w, h)
}

// LoadPNG reads a PNG file from disk and creates an Image. Files larger
// than [MaxFileSize] are rejected with [ErrFileTooLarge].
func LoadPNG(path string) (*Image, error) {
	data, err := readLimited(path)
	if err != nil {
		return nil, err
	}
	return NewPNG(data)
}

// buildGray extracts pixel data for a grayscale image as DeviceGray.
func buildGray(img goimage.Image, w, h int) (*Image, error) {
	bounds := img.Bounds()
	pixels := make([]byte, 0, w*h)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			pixels = append(pixels, byte(r>>8))
		}
	}
	return &Image{
		data:       pixels,
		width:      w,
		height:     h,
		colorSpace: "DeviceGray",
		bpc:        8,
		filter:     "FlateDecode",
	}, nil
}

// buildRGBMaybeAlpha extracts RGB pixel data and, if any pixel is
// non-opaque, the straight (non-premultiplied) alpha channel.
//
// Detection and extraction happen in a single pass: the alpha buffer is
// always populated while iterating and discarded at the end if every
// pixel was opaque. This replaces the prior two-pass approach
// (imageHasAlpha + buildRGBA) which walked the entire image twice.
//
// Fast paths exist for *goimage.NRGBA (the stdlib PNG straight-alpha
// type) and *goimage.RGBA (premultiplied). The generic path uses
// [color.NRGBAModel.Convert] to obtain straight alpha for any other
// image type.
func buildRGBMaybeAlpha(img goimage.Image, w, h int) (*Image, error) {
	bounds := img.Bounds()
	pixels := make([]byte, 0, w*h*3)
	alpha := make([]byte, 0, w*h)
	hasAlpha := false

	switch src := img.(type) {
	case *goimage.NRGBA:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			row := src.Pix[(y-bounds.Min.Y)*src.Stride:]
			for x := range w {
				off := x * 4
				r, g, b, a := row[off], row[off+1], row[off+2], row[off+3]
				pixels = append(pixels, r, g, b)
				alpha = append(alpha, a)
				if a != 0xFF {
					hasAlpha = true
				}
			}
		}

	case *goimage.RGBA:
		// image.RGBA stores premultiplied values; un-premultiply when
		// partially transparent so the PDF RGB bytes contain straight
		// colors.
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			row := src.Pix[(y-bounds.Min.Y)*src.Stride:]
			for x := range w {
				off := x * 4
				r, g, b, a := row[off], row[off+1], row[off+2], row[off+3]
				if a > 0 && a < 255 {
					r = uint8(uint16(r) * 255 / uint16(a))
					g = uint8(uint16(g) * 255 / uint16(a))
					b = uint8(uint16(b) * 255 / uint16(a))
				} else if a == 0 {
					r, g, b = 0, 0, 0
				}
				pixels = append(pixels, r, g, b)
				alpha = append(alpha, a)
				if a != 0xFF {
					hasAlpha = true
				}
			}
		}

	default:
		// Generic path: convert to NRGBA via the color model.
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				pixels = append(pixels, c.R, c.G, c.B)
				alpha = append(alpha, c.A)
				if c.A != 0xFF {
					hasAlpha = true
				}
			}
		}
	}

	out := &Image{
		data:       pixels,
		width:      w,
		height:     h,
		colorSpace: "DeviceRGB",
		bpc:        8,
		filter:     "FlateDecode",
	}
	if hasAlpha {
		out.smask = alpha
		out.smaskW = w
		out.smaskH = h
	}
	return out, nil
}

// buildRGBOnly extracts RGB pixel data without alpha. Used by formats
// like TIFF where alpha is intentionally discarded. It is a simpler
// cousin of [buildRGBMaybeAlpha] that avoids allocating the alpha buffer.
func buildRGBOnly(img goimage.Image, w, h int) (*Image, error) {
	bounds := img.Bounds()
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

// isGrayscale reports whether the image uses a grayscale color model.
func isGrayscale(img goimage.Image) bool {
	switch img.ColorModel() {
	case color.GrayModel, color.Gray16Model:
		return true
	}
	return false
}

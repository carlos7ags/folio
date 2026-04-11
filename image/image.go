// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	goimage "image"

	"github.com/carlos7ags/folio/core"
)

// Image holds decoded image data ready for embedding in a PDF.
type Image struct {
	data       []byte // raw image data (JPEG: original bytes; PNG: decoded pixels)
	width      int
	height     int
	colorSpace string // "DeviceRGB", "DeviceGray", "DeviceCMYK"
	bpc        int    // bits per component (usually 8)
	filter     string // "DCTDecode" for JPEG, "FlateDecode" for PNG/TIFF
	smask      []byte // soft mask (alpha channel) for PNG with transparency
	smaskW     int    // smask width (same as image width for alpha)
	smaskH     int    // smask height
}

// Width returns the image width in pixels.
func (img *Image) Width() int { return img.width }

// Height returns the image height in pixels.
func (img *Image) Height() int { return img.height }

// AspectRatio returns the ratio of width to height.
func (img *Image) AspectRatio() float64 {
	if img.height == 0 {
		return 1
	}
	return float64(img.width) / float64(img.height)
}

// NewFromGoImage creates an Image from a Go image.RGBA. Pixel data is
// extracted as raw RGB bytes for FlateDecode embedding; if any pixel is
// non-opaque, a soft mask is generated.
//
// NewFromGoImage returns nil when:
//   - src is nil
//   - dimensions are non-positive
//   - the stride is smaller than width*4 (would read past the pixel buffer)
//   - dimensions exceed [MaxDimension] or the pixel count exceeds [MaxPixels]
func NewFromGoImage(src *goimage.RGBA) *Image {
	if src == nil {
		return nil
	}
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if err := checkDimensions(w, h); err != nil {
		return nil
	}

	// Validate stride: must be at least width*4 to safely access RGBA pixels.
	if src.Stride < w*4 {
		return nil
	}

	pixels := make([]byte, 0, w*h*3)
	var alpha []byte
	hasAlpha := false

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := (y-bounds.Min.Y)*src.Stride + (x-bounds.Min.X)*4
			r := src.Pix[off]
			g := src.Pix[off+1]
			b := src.Pix[off+2]
			a := src.Pix[off+3]
			// image.RGBA stores premultiplied values. PDF needs straight
			// (non-premultiplied) alpha, so un-premultiply when a < 255.
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

	img := &Image{
		data:       pixels,
		width:      w,
		height:     h,
		colorSpace: "DeviceRGB",
		bpc:        8,
		filter:     "FlateDecode",
	}
	if hasAlpha {
		img.smask = alpha
		img.smaskW = w
		img.smaskH = h
	}
	return img
}

// BuildXObject creates the PDF image XObject dictionary and stream.
// It returns the image XObject reference and, if the image has an alpha
// channel, a separate SMask XObject reference. The addObject callback
// registers each indirect object in the PDF file.
func (img *Image) BuildXObject(addObject func(core.PdfObject) *core.PdfIndirectReference) (*core.PdfIndirectReference, *core.PdfIndirectReference) {
	var stream *core.PdfStream
	if img.filter == "DCTDecode" {
		// JPEG: raw bytes go directly, no compression by us.
		stream = core.NewPdfStream(img.data)
		stream.SetCompress(false)
		stream.Dict.Set("Filter", core.NewPdfName("DCTDecode"))
	} else {
		// PNG: pixel data, compress with FlateDecode.
		stream = core.NewPdfStreamCompressed(img.data)
	}

	// Copy dict entries into stream dict.
	stream.Dict.Set("Type", core.NewPdfName("XObject"))
	stream.Dict.Set("Subtype", core.NewPdfName("Image"))
	stream.Dict.Set("Width", core.NewPdfInteger(img.width))
	stream.Dict.Set("Height", core.NewPdfInteger(img.height))
	stream.Dict.Set("ColorSpace", core.NewPdfName(img.colorSpace))
	stream.Dict.Set("BitsPerComponent", core.NewPdfInteger(img.bpc))

	// Handle SMask (alpha channel).
	var smaskRef *core.PdfIndirectReference
	if len(img.smask) > 0 {
		smaskStream := core.NewPdfStreamCompressed(img.smask)
		smaskStream.Dict.Set("Type", core.NewPdfName("XObject"))
		smaskStream.Dict.Set("Subtype", core.NewPdfName("Image"))
		smaskStream.Dict.Set("Width", core.NewPdfInteger(img.smaskW))
		smaskStream.Dict.Set("Height", core.NewPdfInteger(img.smaskH))
		smaskStream.Dict.Set("ColorSpace", core.NewPdfName("DeviceGray"))
		smaskStream.Dict.Set("BitsPerComponent", core.NewPdfInteger(8))
		smaskRef = addObject(smaskStream)
		stream.Dict.Set("SMask", smaskRef)
	}

	imgRef := addObject(stream)
	return imgRef, smaskRef
}

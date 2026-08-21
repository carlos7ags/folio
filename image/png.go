// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	goimage "image"
	"image/color"
	"image/png"
	"io"
	"math"
)

// NewPNG creates an Image from raw PNG data. It decodes the PNG and
// re-encodes pixels for FlateDecode embedding. Alpha channels are
// extracted into a separate soft mask. Dimensions are validated against
// [MaxDimension] and [MaxPixels] before the pixel buffer is allocated.
func NewPNG(data []byte) (*Image, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image: png: %w", err)
	}
	if err := checkDimensions(cfg.Width, cfg.Height); err != nil {
		return nil, fmt.Errorf("image: png: %w", err)
	}

	if img := pngPassthrough(data); img != nil {
		return img, nil
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image: png: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if err := checkDimensions(w, h); err != nil {
		return nil, fmt.Errorf("image: png: %w", err)
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

var pngSignature = [8]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// pngPassthrough attempts to reuse a PNG's IDAT stream verbatim as a
// FlateDecode image payload with a PNG predictor (ISO 32000-1 §7.4.4.4).
// It returns nil when the file is not eligible; ineligibility is never an
// error, the caller falls back to a full decode.
func pngPassthrough(data []byte) *Image {
	if len(data) < 8 || !bytes.Equal(data[:8], pngSignature[:]) {
		return nil
	}

	var (
		width, height                      int
		bitDepth, colorType                byte
		compression, filterMethod, interlc byte
		sawIHDR                            bool
		idat                               []byte
	)

	off := 8
	for {
		if off+8 > len(data) {
			return nil
		}
		length := binary.BigEndian.Uint32(data[off : off+4])
		if length > math.MaxInt32 {
			return nil
		}
		typeStart := off + 4
		payloadStart := typeStart + 4
		payloadEnd := payloadStart + int(length)
		crcEnd := payloadEnd + 4
		if payloadEnd < 0 || crcEnd < 0 || crcEnd > len(data) {
			return nil
		}
		chunkType := string(data[typeStart:payloadStart])
		payload := data[payloadStart:payloadEnd]
		storedCRC := binary.BigEndian.Uint32(data[payloadEnd:crcEnd])
		if crc32.ChecksumIEEE(data[typeStart:payloadEnd]) != storedCRC {
			return nil
		}

		switch chunkType {
		case "IHDR":
			if sawIHDR || len(payload) != 13 {
				return nil
			}
			sawIHDR = true
			width = int(binary.BigEndian.Uint32(payload[0:4]))
			height = int(binary.BigEndian.Uint32(payload[4:8]))
			bitDepth = payload[8]
			colorType = payload[9]
			compression = payload[10]
			filterMethod = payload[11]
			interlc = payload[12]

			if bitDepth != 8 {
				return nil
			}
			if colorType != 0 && colorType != 2 {
				return nil
			}
			if compression != 0 || filterMethod != 0 {
				return nil
			}
			if interlc != 0 {
				return nil
			}
		case "tRNS":
			return nil
		case "IDAT":
			if !sawIHDR {
				return nil
			}
			idat = append(idat, payload...)
		case "IEND":
			goto walked
		default:
			if !sawIHDR {
				return nil
			}
		}

		off = crcEnd
	}

walked:
	if !sawIHDR || len(idat) == 0 {
		return nil
	}
	if err := checkDimensions(width, height); err != nil {
		return nil
	}

	channels := 1
	colorSpace := "DeviceGray"
	if colorType == 2 {
		channels = 3
		colorSpace = "DeviceRGB"
	}
	rowBytes := width * channels
	wantLen := int64(height) * int64(1+rowBytes)

	zr, err := zlib.NewReader(bytes.NewReader(idat))
	if err != nil {
		return nil
	}
	n, err := io.Copy(io.Discard, zr)
	if err != nil {
		_ = zr.Close()
		return nil
	}
	if err := zr.Close(); err != nil {
		return nil
	}
	if n != wantLen {
		return nil
	}

	return &Image{
		data:            idat,
		width:           width,
		height:          height,
		colorSpace:      colorSpace,
		bpc:             8,
		filter:          "FlateDecode",
		preCompressed:   true,
		predictorColors: channels,
	}
}

// isGrayscale reports whether the image uses a grayscale color model.
func isGrayscale(img goimage.Image) bool {
	switch img.ColorModel() {
	case color.GrayModel, color.Gray16Model:
		return true
	}
	return false
}

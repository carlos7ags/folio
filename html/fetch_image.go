// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package html

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	folioimage "github.com/carlos7ags/folio/image"
)

// fetchImage downloads an image from a URL and returns a folio Image.
// Supports JPEG, PNG, and TIFF. Detects format from Content-Type header
// or file extension, falling back to content sniffing.
func (c *converter) fetchImage(url string) (*folioimage.Image, error) {
	if c.urlPolicy != nil {
		if err := c.urlPolicy(url); err != nil {
			return nil, err
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch image %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch image %s: HTTP %d", url, resp.StatusCode)
	}

	// Limit download size to 50MB.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return nil, fmt.Errorf("fetch image %s: %w", url, err)
	}

	// Detect format from Content-Type or URL extension.
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg"):
		return folioimage.NewJPEG(data)
	case strings.Contains(ct, "png"):
		return folioimage.NewPNG(data)
	case strings.Contains(ct, "tiff"):
		return folioimage.NewTIFF(data)
	}

	// Fallback: try by URL extension.
	ext := strings.ToLower(filepath.Ext(url))
	if idx := strings.IndexByte(ext, '?'); idx >= 0 {
		ext = ext[:idx]
	}
	switch ext {
	case ".jpg", ".jpeg":
		return folioimage.NewJPEG(data)
	case ".png":
		return folioimage.NewPNG(data)
	case ".tif", ".tiff":
		return folioimage.NewTIFF(data)
	}

	// Last resort: content sniffing.
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return folioimage.NewJPEG(data)
	}
	if len(data) >= 8 && string(data[:4]) == "\x89PNG" {
		return folioimage.NewPNG(data)
	}

	if img, err := folioimage.NewJPEG(data); err == nil {
		return img, nil
	}
	return folioimage.NewPNG(data)
}

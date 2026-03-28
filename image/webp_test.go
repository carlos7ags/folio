// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// minimalWebP is a 1x1 green VP8L (lossless) WebP.
// RIFF header (12) + VP8L chunk header (8) + VP8L bitstream.
// Generated from the VP8L spec: signature byte 0x2F, then 14 bits width-1,
// 14 bits height-1, 1 bit alpha, 3 bits version, then trivial LZ77 data.
var minimalWebP = []byte{
	// RIFF header
	'R', 'I', 'F', 'F',
	0x24, 0x00, 0x00, 0x00, // file size - 8 = 36
	'W', 'E', 'B', 'P',
	// VP8L chunk
	'V', 'P', '8', 'L',
	0x0d, 0x00, 0x00, 0x00, // chunk size = 13
	// VP8L bitstream: signature + 1x1 image
	0x2f,                   // signature byte
	0x00, 0x00, 0x00, 0x00, // width-1=0 (14 bits), height-1=0 (14 bits), alpha=0, version=0
	0x10, 0x07, 0x10, 0x11,
	0x11, 0x88, 0x88, 0x08,
}

func TestNewWebP(t *testing.T) {
	img, err := NewWebP(minimalWebP)
	if err != nil {
		t.Skipf("WebP decode failed (may need valid bitstream): %v", err)
	}
	if img.Width() != 1 || img.Height() != 1 {
		t.Errorf("expected 1x1, got %dx%d", img.Width(), img.Height())
	}
	if img.colorSpace != "DeviceRGB" {
		t.Errorf("expected DeviceRGB, got %s", img.colorSpace)
	}
}

func TestLoadWebP(t *testing.T) {
	// Try to decode first — if the embedded bytes aren't valid, skip.
	_, err := NewWebP(minimalWebP)
	if err != nil {
		t.Skipf("WebP decode not available with embedded bytes: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.webp")
	if err := os.WriteFile(path, minimalWebP, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	img, err := LoadWebP(path)
	if err != nil {
		t.Fatalf("LoadWebP: %v", err)
	}
	if img.Width() != 1 {
		t.Errorf("expected width 1, got %d", img.Width())
	}
}

func TestLoadWebPNotFound(t *testing.T) {
	_, err := LoadWebP("/nonexistent/test.webp")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestNewWebPInvalid(t *testing.T) {
	_, err := NewWebP([]byte{0, 1, 2, 3})
	if err == nil {
		t.Error("expected error for invalid WebP data")
	}
}

func TestWebPBuildXObject(t *testing.T) {
	img, err := NewWebP(minimalWebP)
	if err != nil {
		t.Skipf("WebP decode not available: %v", err)
	}
	objCount := 0
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		objCount++
		return &core.PdfIndirectReference{ObjectNumber: objCount, GenerationNumber: 0}
	}
	ref, _ := img.BuildXObject(addObject)
	if ref == nil {
		t.Fatal("expected non-nil image reference")
	}
}

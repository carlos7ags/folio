// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"errors"
	"io/fs"
	"os"
	"testing"
)

func TestParseFontEmpty(t *testing.T) {
	_, err := ParseFont([]byte{})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("expected ErrTruncated, got %v", err)
	}
}

func TestParseFontUnknownMagic(t *testing.T) {
	data := make([]byte, 16)
	_, err := ParseFont(data)
	if err == nil {
		t.Fatal("expected error for unknown magic")
	}
	if !errors.Is(err, ErrUnknownFormat) {
		t.Errorf("expected ErrUnknownFormat, got %v", err)
	}
}

func TestParseFontTTF(t *testing.T) {
	path := testFontPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	face, err := ParseFont(data)
	if err != nil {
		t.Fatalf("ParseFont returned error: %v", err)
	}
	if face == nil {
		t.Fatal("ParseFont returned nil face")
	}
	if face.PostScriptName() == "" {
		t.Error("expected non-empty PostScriptName")
	}
}

func TestParseFontWOFF(t *testing.T) {
	path := testFontPath(t)
	ttfData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	woffData := buildWOFF(t, ttfData)
	face, err := ParseFont(woffData)
	if err != nil {
		t.Fatalf("ParseFont(WOFF) failed: %v", err)
	}
	if face == nil {
		t.Fatal("ParseFont returned nil face")
	}
	if face.PostScriptName() == "" {
		t.Error("expected non-empty PostScriptName")
	}
}

func TestLoadFontMissingFile(t *testing.T) {
	_, err := LoadFont("/nonexistent/path/does-not-exist.ttf")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestLoadFontTTF(t *testing.T) {
	path := testFontPath(t)
	face, err := LoadFont(path)
	if err != nil {
		t.Fatalf("LoadFont(%s) failed: %v", path, err)
	}
	if face == nil {
		t.Fatal("LoadFont returned nil face")
	}
	if face.PostScriptName() == "" {
		t.Error("expected non-empty PostScriptName")
	}
}

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestURLPolicyBlocksImgSrc verifies that URLPolicy blocks <img src="http://...">
// fetches. When blocked, the image falls back to its alt text (a Paragraph).
func TestURLPolicyBlocksImgSrc(t *testing.T) {
	blocked := false
	elems, err := Convert(`<img src="http://localhost/photo.jpg"/>`, &Options{
		URLPolicy: func(url string) error {
			blocked = true
			return fmt.Errorf("blocked")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Error("URLPolicy was not called for <img src>")
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.Paragraph); !ok {
		t.Errorf("expected Paragraph fallback, got %T", elems[0])
	}
}

// TestURLPolicyBlocksBackgroundImage verifies that URLPolicy blocks
// background-image: url("http://...") fetches via the same hook.
func TestURLPolicyBlocksBackgroundImage(t *testing.T) {
	blocked := false
	elems, err := Convert(
		`<div style="width:100px; height:100px; background-image: url('http://localhost/bg.jpg')">text</div>`,
		&Options{
			URLPolicy: func(url string) error {
				blocked = true
				return fmt.Errorf("blocked")
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Error("URLPolicy was not called for background-image URL")
	}
	// The div should still render (just without the background image).
	if len(elems) == 0 {
		t.Error("expected elements despite blocked background image")
	}
}

// TestURLPolicyAllowsWhenNil verifies that a nil URLPolicy does not
// panic. Uses a data URI to avoid network calls.
func TestURLPolicyAllowsWhenNil(t *testing.T) {
	_, err := Convert(`<p>no remote URLs</p>`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// TestURLPolicyConvertFull verifies that URLPolicy works through the
// ConvertFull entry point, not just Convert.
func TestURLPolicyConvertFull(t *testing.T) {
	blocked := false
	_, err := ConvertFull(`<img src="http://localhost/photo.jpg"/>`, &Options{
		URLPolicy: func(url string) error {
			blocked = true
			return fmt.Errorf("blocked")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Error("URLPolicy was not called via ConvertFull")
	}
}

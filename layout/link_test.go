// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestLinkBasic(t *testing.T) {
	l := NewLink("Click here", "https://example.com", font.Helvetica, 12)
	lines := l.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least 1 line")
	}
	if lines[0].linkRef == nil {
		t.Fatal("expected linkRef")
	}
	if lines[0].linkRef.uri != "https://example.com" {
		t.Errorf("uri = %q, want https://example.com", lines[0].linkRef.uri)
	}
}

func TestLinkInternal(t *testing.T) {
	l := NewInternalLink("Go to section 2", "sec2", font.Helvetica, 12)
	lines := l.Layout(400)
	if lines[0].linkRef == nil {
		t.Fatal("expected linkRef")
	}
	if lines[0].linkRef.destName != "sec2" {
		t.Errorf("destName = %q, want sec2", lines[0].linkRef.destName)
	}
	if lines[0].linkRef.uri != "" {
		t.Error("external URI should be empty for internal link")
	}
}

func TestLinkWithStyling(t *testing.T) {
	l := NewLink("Styled link", "https://example.com", font.Helvetica, 12).
		SetColor(ColorBlue).
		SetUnderline()

	lines := l.Layout(400)
	if len(lines[0].Words) == 0 {
		t.Fatal("expected words")
	}
	word := lines[0].Words[0]
	if word.Color != ColorBlue {
		t.Error("expected blue color")
	}
	if word.Decoration&DecorationUnderline == 0 {
		t.Error("expected underline decoration")
	}
}

func TestLinkRendering(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewLink("Click me", "https://example.com", font.Helvetica, 12).
		SetColor(ColorBlue).
		SetUnderline())

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text content")
	}

	// Should have recorded a link area.
	if len(pages[0].Links) != 1 {
		t.Fatalf("expected 1 link area, got %d", len(pages[0].Links))
	}
	link := pages[0].Links[0]
	if link.URI != "https://example.com" {
		t.Errorf("link URI = %q, want https://example.com", link.URI)
	}
	if link.W <= 0 || link.H <= 0 {
		t.Error("link area should have positive dimensions")
	}
}

func TestLinkMultiLine(t *testing.T) {
	longText := strings.Repeat("Click this very long link text ", 10)
	l := NewLink(longText, "https://example.com", font.Helvetica, 12)

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(l)
	pages := r.Render()

	// Multi-line link should produce multiple link areas.
	if len(pages[0].Links) < 2 {
		t.Errorf("expected multiple link areas for wrapped text, got %d", len(pages[0].Links))
	}
	for _, la := range pages[0].Links {
		if la.URI != "https://example.com" {
			t.Errorf("all link areas should have same URI")
		}
	}
}

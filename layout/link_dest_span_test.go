// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

// TestLinkSpansSeparatesTargets pins that linkSpans keys on the whole link
// target, not just the URI. Adjacent words linking to different places —
// one internal, one external — must produce two annotations, and an
// internal target must reach LinkArea.DestName so the writer emits a
// destination rather than a /URI action.
func TestLinkSpansSeparatesTargets(t *testing.T) {
	words := []Word{
		{Text: "plain", Width: 30, SpaceAfter: 4},
		{Text: "inside", Width: 40, SpaceAfter: 4, LinkDest: "section-two"},
		{Text: "outside", Width: 50, SpaceAfter: 4, LinkURI: "https://example.com/"},
		{Text: "tail", Width: 20},
	}

	spans := linkSpans(words)
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2 (one internal, one external): %+v", len(spans), spans)
	}
	if spans[0].DestName != "section-two" || spans[0].URI != "" {
		t.Errorf("internal span = {URI:%q DestName:%q}, want {\"\" \"section-two\"}",
			spans[0].URI, spans[0].DestName)
	}
	if spans[1].URI != "https://example.com/" || spans[1].DestName != "" {
		t.Errorf("external span = {URI:%q DestName:%q}, want {\"https://example.com/\" \"\"}",
			spans[1].URI, spans[1].DestName)
	}
	if spans[0].W <= 0 || spans[1].W <= 0 {
		t.Errorf("spans must have positive width: %+v", spans)
	}
	// The two spans must not overlap — an overlapping rect would make one
	// link swallow the other's clickable area.
	if spans[0].X+spans[0].W > spans[1].X {
		t.Errorf("spans overlap: %+v", spans)
	}
}

// TestLinkSpansMergesContiguousDest keeps words of one internal link in a
// single annotation instead of one rect per word.
func TestLinkSpansMergesContiguousDest(t *testing.T) {
	words := []Word{
		{Text: "jump", Width: 30, SpaceAfter: 4, LinkDest: "target"},
		{Text: "here", Width: 30, LinkDest: "target"},
	}
	spans := linkSpans(words)
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1: %+v", len(spans), spans)
	}
	if spans[0].DestName != "target" {
		t.Errorf("DestName = %q, want target", spans[0].DestName)
	}
	if spans[0].W < 64 {
		t.Errorf("span width = %v, want at least the two words plus the gap (64)", spans[0].W)
	}
}

// TestInlineElementCarriesLinkTarget covers an image that is the whole
// content of a link: the measured word must keep the target so the link
// area survives into the page.
func TestInlineElementCarriesLinkTarget(t *testing.T) {
	img := NewDiv()
	img.SetWidth(34)
	img.SetMinHeight(34)

	p := NewStyledParagraph(
		TextRun{InlineElement: img, LinkDest: "target"},
		NewRun(" caption", font.Helvetica, 12),
	)
	plan := p.PlanLayout(LayoutArea{Width: 300, Height: 500})
	if len(plan.Blocks) == 0 {
		t.Fatal("paragraph produced no blocks")
	}

	var found bool
	for _, b := range plan.Blocks {
		for _, l := range b.Links {
			if l.DestName == "target" {
				found = true
			}
			if l.URI != "" {
				t.Errorf("inline element link emitted URI %q; an internal target must not become a URI", l.URI)
			}
		}
	}
	if !found {
		t.Error("an inline element inside a link produced no link area — the annotation would be missing")
	}
}

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// checkGolden compares got against the committed golden file, or writes it
// when UPDATE_GOLDEN is set. Regenerating goldens must never run in CI — a
// stale-golden failure there is the safety net working as intended.
func checkGolden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join("testdata", "style_golden", name+".golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s — run UPDATE_GOLDEN=1 go test to create: %v", path, err)
	}
	if string(want) != got {
		t.Errorf("golden mismatch for %s\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}

// f4 formats a float with fixed 4-decimal precision so goldens are stable
// across architectures.
func f4(v float64) string {
	return strconv.FormatFloat(v, 'f', 4, 64)
}

// colorStr renders a layout.Color deterministically regardless of color
// space.
func colorStr(c layout.Color) string {
	if c.Space == layout.ColorSpaceCMYK {
		return fmt.Sprintf("cmyk(%s,%s,%s,%s)", f4(c.C), f4(c.M), f4(c.Y), f4(c.K))
	}
	return fmt.Sprintf("rgb(%s,%s,%s)", f4(c.R), f4(c.G), f4(c.B))
}

// newGoldenConverter builds a converter the same way ConvertFullWithContext
// does (html/converter.go), minus font loading — the corpus uses no
// @font-face — so computeElementStyle sees the exact same converter state
// Convert would give it.
func newGoldenConverter(t *testing.T, htmlStr string) (*converter, *html.Node, computedStyle) {
	t.Helper()
	var opts *Options
	o := opts.defaults()

	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	budget := newAssetBudget(o.MaxTotalAssetBytes)
	ss := parseStyleBlocks(doc, o, budget, func(string, error) {})

	c := &converter{
		opts:           o,
		logger:         loggerOrDiscard(o.Logger),
		rootFontSize:   o.DefaultFontSize,
		sheet:          ss,
		embeddedFonts:  make(map[string]*font.EmbeddedFont),
		containerWidth: o.PageWidth,
		counters:       make(map[string][]int),
		budget:         budget,
	}

	root := defaultStyle()
	root.FontSize = o.DefaultFontSize
	return c, doc, root
}

// dumpStyles walks every element under <body>, threading each element's
// computed style as its children's parent the same way walkChildren does,
// and returns a deterministic text dump of the resolved styles.
//
// <html> and <body> are resolved (so a rule targeting them, or inheritance
// through them, is honored) but not dumped — only their descendants are,
// matching the corpus's focus on author content.
func dumpStyles(t *testing.T, htmlStr string) string {
	t.Helper()
	c, doc, root := newGoldenConverter(t, htmlStr)

	var htmlNode *html.Node
	for n := doc.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode && n.DataAtom == atom.Html {
			htmlNode = n
			break
		}
	}
	if htmlNode == nil {
		t.Fatal("no <html> element in parsed document")
	}
	htmlStyle := c.computeElementStyle(htmlNode, root)

	body := findBodyNode(doc)
	if body == nil {
		t.Fatal("no <body> element in parsed document")
	}
	bodyStyle := c.computeElementStyle(body, htmlStyle)

	var sb strings.Builder
	var walk func(n *html.Node, parent computedStyle, depth int)
	walk = func(n *html.Node, parent computedStyle, depth int) {
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type != html.ElementNode {
				continue
			}
			style := c.computeElementStyle(ch, parent)
			writeStyleLine(t, &sb, ch.Data, style, depth)
			walk(ch, style, depth+1)
		}
	}
	walk(body, bodyStyle, 0)
	return sb.String()
}

// writeStyleLine appends one deterministic dump line for style. Fields are
// grouped and pipe-delimited so a diff pinpoints which subsystem changed.
// A dumped value that is NaN/Inf, or a non-positive font size, is a product
// bug this corpus exists to catch — fail loudly instead of pinning it.
func writeStyleLine(t *testing.T, sb *strings.Builder, tag string, s computedStyle, depth int) {
	t.Helper()
	if s.FontSize <= 0 {
		t.Fatalf("%s: non-positive font size %v", tag, s.FontSize)
	}
	for _, v := range []float64{s.FontSize, s.LineHeight, s.LetterSpacing, s.TextIndent} {
		if v != v || v > 1e300 || v < -1e300 { // NaN or effectively Inf
			t.Fatalf("%s: non-finite value in dumped style: %v", tag, v)
		}
	}

	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(sb, "%s%s fontFamily=%s fontSize=%s weight=%d style=%s color=%s align=%d lineHeight=%s display=%s\n",
		indent, tag, s.FontFamily, f4(s.FontSize), s.FontWeight, s.FontStyle, colorStr(s.Color), s.TextAlign, f4(s.LineHeight), s.Display)
	fmt.Fprintf(sb, "%s  marginAt400=%s,%s,%s,%s paddingAt400=%s,%s,%s,%s borderW=%s,%s,%s,%s decoration=%d\n",
		indent,
		f4(s.MarginTopAt(400)), f4(s.MarginRightAt(400)), f4(s.MarginBottomAt(400)), f4(s.MarginLeftAt(400)),
		f4(s.PaddingTopAt(400)), f4(s.PaddingRightAt(400)), f4(s.PaddingBottomAt(400)), f4(s.PaddingLeftAt(400)),
		f4(s.BorderTopWidth), f4(s.BorderRightWidth), f4(s.BorderBottomWidth), f4(s.BorderLeftWidth),
		s.TextDecoration)
	fmt.Fprintf(sb, "%s  letterSpacing=%s textTransform=%s whiteSpace=%s textIndent=%s\n",
		indent, f4(s.LetterSpacing), s.TextTransform, s.WhiteSpace, f4(s.TextIndent))
	fmt.Fprintf(sb, "%s  flex: justify=%s alignItems=%s gap=%s\n",
		indent, s.JustifyContent, s.AlignItems, f4(s.Gap))
}

// TestStyleGolden pins html/converter_style.go's end-to-end "HTML+CSS
// snippet → computed style" behavior against a fixed corpus. This is the
// behavior-preservation net for refactors of converter_style.go:
// a golden diff here means observable style resolution changed.
//
// The At(400) columns pin the #269 lazy margin/padding resolution against
// a fixed 400pt container width. As of Phase 4 (see html/style.go) the
// legacy eagerly-resolved float fields no longer exist — only these
// resolved-at-consumer-time values are dumped.
func TestStyleGolden(t *testing.T) {
	cases := []struct {
		name string
		html string
	}{
		{
			name: "inline_style",
			html: `<p style="color: #ff0000; font-size: 18px">x</p>`,
		},
		{
			name: "class_selector",
			html: `<style>.a { font-weight: bold }</style><p class="a">x</p>`,
		},
		{
			name: "descendant_selector",
			html: `<style>div p { color: blue }</style><div><p>x</p></div><p>y</p>`,
		},
		{
			name: "specificity",
			html: `<style>
				p { color: green }
				.b { color: blue }
				#c { color: red }
			</style><p id="c" class="b">x</p>`,
		},
		{
			name: "inheritance",
			html: `<style>.parent { font-size: 20px; color: #123456; margin: 10px }</style>` +
				`<div class="parent"><p>child p</p><span>child span</span></div>`,
		},
		{
			// Pins the #269 lazy-resolution mid-state: legacy float fields
			// are gone, so the At(400) columns are the only observable
			// resolution of %, em, rem and calc().
			name: "relative_lengths",
			html: `<div style="font-size: 20px">` +
				`<p style="font-size: 2em; margin-left: 10%; padding-top: calc(10% + 5px)">x</p>` +
				`<p style="font-size: 1.5rem">y</p>` +
				`</div>`,
		},
		{
			name: "shorthands",
			html: `<p style="margin: 1px 2px 3px 4px; padding: 10px 20px; border: 1px solid red">x</p>`,
		},
		{
			name: "css_variables",
			html: `<style>:root { --c: green } p { color: var(--c) }</style><p>x</p>`,
		},
		{
			name: "tag_defaults",
			html: `<h1>a</h1><h2>b</h2><strong>c</strong><em>d</em><code>e</code>`,
		},
		{
			name: "display_flex",
			html: `<div style="display: flex; justify-content: space-between; align-items: center; gap: 8px">` +
				`<span>a</span><span>b</span></div>`,
		},
		{
			name: "text_properties",
			html: `<p style="text-align: center; text-decoration: underline; letter-spacing: 2px; ` +
				`text-transform: uppercase; white-space: pre">x</p>`,
		},
		{
			// Two <style> blocks setting the same property — cascade order
			// (not specificity) decides; the later declaration wins.
			name: "conflicting_last_wins",
			html: `<style>p { color: red }</style><style>p { color: blue }</style><p>x</p>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dumpStyles(t, tc.html)
			checkGolden(t, tc.name, got)
		})
	}
}

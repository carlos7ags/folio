// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
)

// styleByID resolves computed styles down the tree (threading each element's
// style as its children's parent, like walkChildren) and returns the style of
// the element whose id attribute equals id.
func styleByID(t *testing.T, htmlStr, id string) computedStyle {
	t.Helper()
	return styleByIDWithOptions(t, htmlStr, id, nil)
}

// styleByIDWithOptions is styleByID with caller-supplied Options.
func styleByIDWithOptions(t *testing.T, htmlStr, id string, opts *Options) computedStyle {
	t.Helper()
	c, doc, root := newGoldenConverterWithOptions(t, htmlStr, opts)
	var found *computedStyle
	var walk func(n *html.Node, parent computedStyle)
	walk = func(n *html.Node, parent computedStyle) {
		for ch := n.FirstChild; ch != nil && found == nil; ch = ch.NextSibling {
			if ch.Type != html.ElementNode {
				continue
			}
			s := c.computeElementStyle(ch, parent)
			if getAttr(ch, "id") == id {
				found = &s
				return
			}
			walk(ch, s)
		}
	}
	walk(doc, root)
	if found == nil {
		t.Fatalf("no element with id %q", id)
	}
	return *found
}

// usedLineHeight returns the used line height in points, or -1 for normal.
func usedLineHeight(s computedStyle) float64 {
	if s.LineHeight == layout.LeadingNormal {
		return -1
	}
	return s.LineHeight * s.FontSize
}

// TestLineHeightInheritance pins CSS Inline Layout L3 §4.3 inheritance: a
// <length>/<percentage>/em line-height computes to an absolute length that
// descendants inherit unchanged, while a unitless <number> inherits as a
// factor re-applied to each descendant's own font-size.
func TestLineHeightInheritance(t *testing.T) {
	tests := []struct {
		name string
		html string
		want float64 // used line height of #t in points; -1 = normal
	}{
		// 20px = 15pt; a 40px child keeps 15pt (was 20/12 × 30 = 37.5pt).
		{"px inherits absolute", `<div style="line-height:20px"><p id="t" style="font-size:40px">x</p></div>`, 15},
		// 150% of the div's 12pt = 18pt (was 1.5 × 30 = 45pt).
		{"percent inherits absolute", `<div style="line-height:150%"><p id="t" style="font-size:40px">x</p></div>`, 18},
		// 2em at the div's 10px (7.5pt) = 15pt.
		{"em inherits absolute", `<div style="font-size:10px;line-height:2em"><p id="t" style="font-size:40px">x</p></div>`, 15},
		{"length calc inherits absolute", `<div style="line-height:calc(10px + 10px)"><p id="t" style="font-size:40px">x</p></div>`, 15},
		// Through two levels.
		{"px inherits through grandchild", `<div style="line-height:20px"><section style="font-size:8px"><p id="t" style="font-size:40px">x</p></section></div>`, 15},
		// A unitless number inherits as a factor: 1.5 × 30pt.
		{"number inherits as factor", `<div style="line-height:1.5"><p id="t" style="font-size:40px">x</p></div>`, 45},
		{"dimensionless calc inherits as factor", `<div style="line-height:calc(1 + 0.5)"><p id="t" style="font-size:40px">x</p></div>`, 45},
		// A descendant number overrides the inherited absolute value and
		// its own descendants re-multiply: 2 × 15pt.
		{"number clears inherited absolute", `<div style="line-height:20px"><p style="line-height:2"><span id="t" style="font-size:20px">x</span></p></div>`, 30},
		{"normal clears inherited absolute", `<div style="line-height:20px"><p style="line-height:normal"><span id="t" style="font-size:20px">x</span></p></div>`, -1},
		// em resolves against the element's FINAL font-size regardless of
		// declaration order: 2em × 30pt = 60pt.
		{"em against own final font-size", `<p id="t" style="line-height:2em;font-size:40px">x</p>`, 60},
		// The font shorthand's slash line-height behaves the same way.
		{"font shorthand px inherits absolute", `<div style="font:12px/20px serif"><p id="t" style="font-size:40px">x</p></div>`, 15},
		{"font shorthand percent inherits absolute", `<div style="font:16px/150% serif"><p id="t" style="font-size:40px">x</p></div>`, 18},
		{"font shorthand number inherits as factor", `<div style="font:16px/1.5 serif"><p id="t" style="font-size:40px">x</p></div>`, 45},
		// A length slash part of the shorthand resolves against the
		// element's FINAL font-size, like the standalone property: a later
		// font-size (same or higher-specificity rule) wins.
		{"font shorthand percent with later font-size", `<style>p{font:10px/150% serif} .big{font-size:20px}</style><p id="t" class="big">x</p>`, 22.5},
		{"font shorthand em with later font-size", `<p id="t" style="font:10px/2em serif; font-size:20px">x</p>`, 30},
		{"font shorthand px with later font-size", `<p id="t" style="font:10px/20px serif; font-size:40px">x</p>`, 15},
		{"font shorthand number with later font-size", `<p id="t" style="font:10px/2 serif; font-size:20px">x</p>`, 30},
		// A later shorthand with a number clears an earlier length.
		{"font shorthand number overrides standalone length", `<p id="t" style="line-height:2em; font:10px/1.5 serif; font-size:20px">x</p>`, 22.5},
		// Stylesheet rules, not just inline styles.
		{"stylesheet px inherits absolute", `<style>.a{line-height:20px}.b{font-size:40px}</style><div class="a"><p id="t" class="b">x</p></div>`, 15},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := usedLineHeight(styleByID(t, tc.html, "t"))
			if math.Abs(got-tc.want) > 0.01 {
				t.Errorf("used line-height = %.4fpt, want %.4fpt", got, tc.want)
			}
		})
	}
}

// TestLineHeightCSSWideKeywords pins `initial`/`revert` as `normal` and
// `inherit`/`unset` as keeping the inherited value.
func TestLineHeightCSSWideKeywords(t *testing.T) {
	tests := []struct {
		name string
		html string
		want float64 // -1 = normal
	}{
		{"initial is normal", `<div style="line-height:2"><p id="t" style="line-height:initial">x</p></div>`, -1},
		{"revert is normal", `<div style="line-height:2"><p id="t" style="line-height:revert">x</p></div>`, -1},
		{"initial clears inherited absolute", `<div style="line-height:20px"><p id="t" style="line-height:initial">x</p></div>`, -1},
		{"inherit keeps parent factor", `<div style="line-height:2"><p id="t" style="line-height:inherit">x</p></div>`, 24},
		{"unset keeps parent absolute", `<div style="line-height:20px"><p id="t" style="line-height:unset;font-size:40px">x</p></div>`, 15},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := usedLineHeight(styleByID(t, tc.html, "t"))
			if math.Abs(got-tc.want) > 0.01 {
				t.Errorf("used line-height = %.4fpt, want %.4fpt", got, tc.want)
			}
		})
	}
}

// TestNegativeLineHeightIgnored pins that a negative line-height is invalid
// CSS and dropped, so the previous (cascaded or inherited) value stays — in
// particular -1 must not turn into the LeadingNormal sentinel.
func TestNegativeLineHeightIgnored(t *testing.T) {
	tests := []struct {
		name string
		html string
		want float64
	}{
		{"negative number keeps inherited factor", `<div style="line-height:2"><p id="t" style="line-height:-1">x</p></div>`, 24},
		{"negative length keeps inherited absolute", `<div style="line-height:20px"><p id="t" style="line-height:-10px;font-size:40px">x</p></div>`, 15},
		{"negative percent keeps same-element earlier value", `<p id="t" style="line-height:2;line-height:-50%">x</p>`, 24},
		{"negative stylesheet value loses to earlier rule", `<style>p{line-height:3}p{line-height:-1}</style><p id="t">x</p>`, 36},
		// A negative calc() result is clamped to 0 rather than dropped.
		{"negative calc clamps to zero", `<div style="line-height:2"><p id="t" style="line-height:calc(0.5 - 1)">x</p></div>`, 0},
		{"negative length calc clamps to zero", `<div style="line-height:2"><p id="t" style="line-height:calc(1em - 30px);font-size:20px">x</p></div>`, 0},
		// Re-resolved at the final font-size (positive at the default 12pt,
		// negative at 40px = 30pt): still clamps.
		{"negative calc after font-size clamps to zero", `<p id="t" style="line-height:calc(20pt - 1em);font-size:40px">x</p>`, 0},
		// An invalid slash part invalidates the whole shorthand, so the
		// inherited font-size and line-height both stay.
		{"font shorthand negative ignored", `<div style="line-height:2"><p id="t" style="font:16px/-1 serif">x</p></div>`, 24},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := usedLineHeight(styleByID(t, tc.html, "t"))
			if math.Abs(got-tc.want) > 0.01 {
				t.Errorf("used line-height = %.4fpt, want %.4fpt", got, tc.want)
			}
		})
	}
}

// planLines lays out the first converted element at width and returns the
// number of placed blocks (one per text line for paragraphs and lists).
func planLines(t *testing.T, src string, width float64) int {
	t.Helper()
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	return len(elems[0].PlanLayout(layout.LayoutArea{Width: width, Height: 1000}).Blocks)
}

// planHeight lays out the first converted element at width and returns the
// height it consumes.
func planHeight(t *testing.T, src string, width float64) float64 {
	t.Helper()
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	return elems[0].PlanLayout(layout.LayoutArea{Width: width, Height: 1000}).Consumed
}

// TestWhiteSpaceNoWrapLists pins that white-space:nowrap reaches list item
// text: the item must stay on one overflowing line in a 60pt column, i.e.
// be exactly as tall as a one-word item.
func TestWhiteSpaceNoWrapLists(t *testing.T) {
	const text = "alpha beta gamma delta epsilon"
	oneLine := planHeight(t, `<ul><li>alpha</li></ul>`, 60)
	if h := planHeight(t, `<ul><li>`+text+`</li></ul>`, 60); h <= oneLine*1.5 {
		t.Fatalf("control: wrapping list should take several lines at 60pt (height %.2f vs one line %.2f)", h, oneLine)
	}
	cases := []struct{ name, html, oneLine string }{
		{"nowrap on ul", `<ul style="white-space:nowrap"><li>` + text + `</li></ul>`, ""},
		{"nowrap on ol", `<ol style="white-space:nowrap"><li>` + text + `</li></ol>`, `<ol><li>alpha</li></ol>`},
		{"nowrap on li", `<ul><li style="white-space:nowrap">` + text + `</li></ul>`, ""},
		{"nowrap via stylesheet", `<style>ul{white-space:nowrap}</style><ul><li>` + text + `</li></ul>`, ""},
		{"nowrap inline span in li", `<ul><li><span style="white-space:nowrap">` + text + `</span></li></ul>`, ""},
		{"nowrap on ul, li with box", `<ul style="white-space:nowrap"><li style="background:#eee">` + text + `</li></ul>`, `<ul><li style="background:#eee">alpha</li></ul>`},
		{"nowrap on ul, nested list", `<ul style="white-space:nowrap"><li>alpha<ul><li>` + text + `</li></ul></li></ul>`, `<ul style="white-space:nowrap"><li>alpha<ul><li>alpha</li></ul></li></ul>`},
		{"pre on ul", `<ul style="white-space:pre"><li>` + text + `</li></ul>`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := oneLine
			if tc.oneLine != "" {
				want = planHeight(t, tc.oneLine, 60)
			}
			if h := planHeight(t, tc.html, 60); math.Abs(h-want) > 0.01 {
				t.Errorf("nowrap list height = %.2f, want one-line height %.2f", h, want)
			}
		})
	}
}

// TestWhiteSpaceNormalLiInNoWrapList pins that white-space is decided per
// item: a <li> declaring white-space:normal wraps even inside a nowrap list,
// while its siblings that only inherit nowrap stay on one line.
func TestWhiteSpaceNormalLiInNoWrapList(t *testing.T) {
	const text = "alpha beta gamma delta epsilon"
	oneLine := planHeight(t, `<ul><li>alpha</li></ul>`, 60)
	cases := []struct {
		name  string
		html  string
		wraps bool
	}{
		{"inline style", `<ul style="white-space:nowrap"><li style="white-space:normal">` + text + `</li></ul>`, true},
		{"stylesheet", `<style>ul{white-space:nowrap} li.w{white-space:normal}</style><ul><li class="w">` + text + `</li></ul>`, true},
		{"normal li with inline child", `<ul style="white-space:nowrap"><li style="white-space:normal"><b>` + text + `</b></li></ul>`, true},
		{"nested list inherits nested ul normal", `<ul style="white-space:nowrap"><li>alpha<ul style="white-space:normal"><li>` + text + `</li></ul></li></ul>`, true},
		{"nested list inherits outer nowrap", `<ul style="white-space:nowrap"><li>alpha<ul><li>` + text + `</li></ul></li></ul>`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := planHeight(t, tc.html, 60)
			// The nested cases carry an extra "alpha" line.
			lines := h / oneLine
			if strings.Contains(tc.name, "nested") {
				lines--
			}
			if got := lines > 1.5; got != tc.wraps {
				t.Errorf("wraps = %v (height %.2f, one line %.2f), want %v", got, h, oneLine, tc.wraps)
			}
		})
	}
	// Mixed siblings: only the normal item wraps.
	mixed := planHeight(t, `<ul style="white-space:nowrap"><li>`+text+`</li><li style="white-space:normal">`+text+`</li></ul>`, 60)
	wrapped := planHeight(t, `<ul><li>`+text+`</li></ul>`, 60)
	if math.Abs(mixed-(oneLine+wrapped)) > 0.01 {
		t.Errorf("mixed list height = %.2f, want nowrap line %.2f + wrapped item %.2f", mixed, oneLine, wrapped)
	}
}

// TestWhiteSpacePreDoesNotSoftWrap pins that white-space:pre suppresses soft
// wrapping (like nowrap) while still breaking at preserved newlines, and that
// pre-wrap / pre-line keep wrapping.
func TestWhiteSpacePreDoesNotSoftWrap(t *testing.T) {
	const text = "alpha beta gamma delta epsilon"
	tests := []struct {
		name  string
		html  string
		check func(n int) bool
		want  string
	}{
		{"pre on block", `<div style="white-space:pre">` + text + `</div>`, func(n int) bool { return n == 1 }, "1 line"},
		{"pre on p", `<p style="white-space:pre">` + text + `</p>`, func(n int) bool { return n == 1 }, "1 line"},
		{"pre on inline span", `<p><span style="white-space:pre">` + text + `</span></p>`, func(n int) bool { return n == 1 }, "1 line"},
		{"pre keeps newlines", "<div style=\"white-space:pre\">alpha beta gamma\ndelta epsilon</div>", func(n int) bool { return n == 2 }, "2 lines"},
		{"pre-wrap wraps", `<div style="white-space:pre-wrap">` + text + `</div>`, func(n int) bool { return n > 1 }, ">1 line"},
		{"pre-line wraps", `<div style="white-space:pre-line">` + text + `</div>`, func(n int) bool { return n > 1 }, ">1 line"},
		{"normal wraps", `<div>` + text + `</div>`, func(n int) bool { return n > 1 }, ">1 line"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if n := planLines(t, tc.html, 60); !tc.check(n) {
				t.Errorf("got %d lines, want %s", n, tc.want)
			}
		})
	}
}

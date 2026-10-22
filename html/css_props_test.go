// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestCSSPropertyRegistryWiring verifies that the registry contains
// the 8 pilot properties with their canonical names.
func TestCSSPropertyRegistryWiring(t *testing.T) {
	want := []string{
		"color",
		"letter-spacing",
		"word-spacing",
		"text-transform",
		"text-align",
		"white-space",
		"direction",
		"opacity",
	}
	for _, name := range want {
		if _, ok := cssPropByName[name]; !ok {
			t.Errorf("registry missing pilot property %q", name)
		}
	}
}

// TestCSSPropertyApply exercises every pilot property against a
// representative corpus of valid and invalid inputs, asserting the
// resulting field on computedStyle. These are the contract tests
// that lock in the migrated behavior — any future change to the
// registry entry that diverges from the pre-migration switch will
// fail one of these.
func TestCSSPropertyApply(t *testing.T) {
	// Sentinel values for the invalid-input baseline. Constructing the
	// baseline explicitly (rather than relying on Go's zero value)
	// protects the test against future renames or default-value shifts
	// in computedStyle.
	cases := []struct {
		property string
		value    string
		check    func(t *testing.T, s, before *computedStyle)
	}{
		// color — compare against parseColor's own output rather than
		// a hand-crafted RGB tuple, so the test stays in lock-step with
		// any future encoding change.
		{"color", "red", func(t *testing.T, s, _ *computedStyle) {
			want, ok := parseColor("red")
			if !ok {
				t.Fatal("test fixture: parseColor(red) failed")
			}
			if s.Color != want {
				t.Errorf("color red = %v, want %v", s.Color, want)
			}
		}},
		{"color", "#3366cc", func(t *testing.T, s, _ *computedStyle) {
			want, _ := parseColor("#3366cc")
			if s.Color != want {
				t.Errorf("color #3366cc = %v, want %v", s.Color, want)
			}
		}},
		{"color", "not-a-color", func(t *testing.T, s, before *computedStyle) {
			if s.Color != before.Color {
				t.Errorf("invalid color leaked: %v vs baseline %v", s.Color, before.Color)
			}
		}},

		// letter-spacing
		{"letter-spacing", "2px", func(t *testing.T, s, _ *computedStyle) {
			if s.LetterSpacing == 0 {
				t.Errorf("letter-spacing 2px = 0; expected non-zero")
			}
		}},
		{"letter-spacing", "normal", func(t *testing.T, s, _ *computedStyle) {
			if s.LetterSpacing != 0 {
				t.Errorf("letter-spacing normal = %v, want 0", s.LetterSpacing)
			}
		}},
		{"letter-spacing", "garbage", func(t *testing.T, s, before *computedStyle) {
			if s.LetterSpacing != before.LetterSpacing {
				t.Errorf("invalid letter-spacing leaked: %v vs baseline %v", s.LetterSpacing, before.LetterSpacing)
			}
		}},

		// word-spacing
		{"word-spacing", "4px", func(t *testing.T, s, _ *computedStyle) {
			if s.WordSpacing == 0 {
				t.Errorf("word-spacing 4px = 0; expected non-zero")
			}
		}},
		{"word-spacing", "normal", func(t *testing.T, s, _ *computedStyle) {
			if s.WordSpacing != 0 {
				t.Errorf("word-spacing normal = %v, want 0", s.WordSpacing)
			}
		}},

		// text-transform
		{"text-transform", "UPPERCASE", func(t *testing.T, s, _ *computedStyle) {
			if s.TextTransform != "uppercase" {
				t.Errorf("text-transform UPPERCASE = %q, want uppercase (lowercased)", s.TextTransform)
			}
		}},
		{"text-transform", "lowercase", func(t *testing.T, s, _ *computedStyle) {
			if s.TextTransform != "lowercase" {
				t.Errorf("text-transform = %q, want lowercase", s.TextTransform)
			}
		}},
		{"text-transform", "capitalize", func(t *testing.T, s, _ *computedStyle) {
			if s.TextTransform != "capitalize" {
				t.Errorf("text-transform = %q, want capitalize", s.TextTransform)
			}
		}},
		{"text-transform", "none", func(t *testing.T, s, _ *computedStyle) {
			if s.TextTransform != "none" {
				t.Errorf("text-transform = %q, want none", s.TextTransform)
			}
		}},
		{"text-transform", "shouty", func(t *testing.T, s, before *computedStyle) {
			if s.TextTransform != before.TextTransform {
				t.Errorf("invalid text-transform leaked: %q vs baseline %q", s.TextTransform, before.TextTransform)
			}
		}},

		// text-align
		{"text-align", "center", func(t *testing.T, s, _ *computedStyle) {
			if s.TextAlign != layout.AlignCenter {
				t.Errorf("text-align center = %v", s.TextAlign)
			}
			if !s.TextAlignSet {
				t.Error("TextAlignSet not flipped")
			}
		}},
		{"text-align", "garbage", func(t *testing.T, s, before *computedStyle) {
			if s.TextAlignSet != before.TextAlignSet {
				t.Error("invalid text-align flipped TextAlignSet")
			}
		}},

		// white-space
		{"white-space", "pre", func(t *testing.T, s, _ *computedStyle) {
			if s.WhiteSpace != "pre" {
				t.Errorf("white-space = %q, want pre", s.WhiteSpace)
			}
		}},
		{"white-space", "pre-wrap", func(t *testing.T, s, _ *computedStyle) {
			if s.WhiteSpace != "pre-wrap" {
				t.Errorf("white-space = %q, want pre-wrap", s.WhiteSpace)
			}
		}},
		{"white-space", "garbage", func(t *testing.T, s, before *computedStyle) {
			if s.WhiteSpace != before.WhiteSpace {
				t.Errorf("invalid white-space leaked: %q vs baseline %q", s.WhiteSpace, before.WhiteSpace)
			}
		}},

		// direction
		{"direction", "rtl", func(t *testing.T, s, _ *computedStyle) {
			if s.Direction != layout.DirectionRTL {
				t.Errorf("direction rtl = %v", s.Direction)
			}
		}},
		{"direction", "LTR", func(t *testing.T, s, _ *computedStyle) {
			if s.Direction != layout.DirectionLTR {
				t.Errorf("direction LTR (case-insensitive) = %v", s.Direction)
			}
		}},
		{"direction", "garbage", func(t *testing.T, s, before *computedStyle) {
			if s.Direction != before.Direction {
				t.Errorf("invalid direction leaked: %v vs baseline %v", s.Direction, before.Direction)
			}
		}},

		// opacity
		{"opacity", "0.5", func(t *testing.T, s, _ *computedStyle) {
			if s.Opacity != 0.5 {
				t.Errorf("opacity 0.5 = %v", s.Opacity)
			}
		}},
		{"opacity", "1.5", func(t *testing.T, s, _ *computedStyle) {
			if s.Opacity != 1.0 {
				t.Errorf("opacity 1.5 should clamp to 1.0, got %v", s.Opacity)
			}
		}},
		{"opacity", "-0.5", func(t *testing.T, s, _ *computedStyle) {
			if s.Opacity != 0.0 {
				t.Errorf("opacity -0.5 should clamp to 0.0, got %v", s.Opacity)
			}
		}},
		{"opacity", "abc", func(t *testing.T, s, before *computedStyle) {
			if s.Opacity != before.Opacity {
				t.Errorf("invalid opacity leaked: %v vs baseline %v", s.Opacity, before.Opacity)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.property+"/"+tc.value, func(t *testing.T) {
			s := &computedStyle{FontSize: 12}
			before := *s
			p, ok := cssPropByName[tc.property]
			if !ok {
				t.Fatalf("registry missing %q", tc.property)
			}
			p.Apply(s, tc.value)
			tc.check(t, s, &before)
		})
	}
}

// TestCSSPropertyDispatchInApplyProperty verifies that the
// applyProperty router actually routes pilot properties through the
// registry AND that unmigrated properties continue to dispatch to the
// legacy switch. The regression this guards: a future "registry
// always returns" bug would silently break unmigrated properties.
func TestCSSPropertyDispatchInApplyProperty(t *testing.T) {
	c := &converter{}

	t.Run("migrated/registry-handles", func(t *testing.T) {
		cases := []struct{ property, value string }{
			{"color", "red"},
			{"letter-spacing", "3px"},
			{"text-align", "right"},
			{"opacity", "0.7"},
		}
		for _, tc := range cases {
			t.Run(tc.property, func(t *testing.T) {
				viaRouter := &computedStyle{FontSize: 12}
				c.applyProperty(tc.property, tc.value, viaRouter)
				viaRegistry := &computedStyle{FontSize: 12}
				cssPropByName[tc.property].Apply(viaRegistry, tc.value)
				switch tc.property {
				case "color":
					if viaRouter.Color != viaRegistry.Color {
						t.Errorf("router color %v != registry color %v", viaRouter.Color, viaRegistry.Color)
					}
				case "letter-spacing":
					if viaRouter.LetterSpacing != viaRegistry.LetterSpacing {
						t.Errorf("router LS %v != registry LS %v", viaRouter.LetterSpacing, viaRegistry.LetterSpacing)
					}
				case "text-align":
					if viaRouter.TextAlign != viaRegistry.TextAlign {
						t.Errorf("router TA %v != registry TA %v", viaRouter.TextAlign, viaRegistry.TextAlign)
					}
				case "opacity":
					if viaRouter.Opacity != viaRegistry.Opacity {
						t.Errorf("router op %v != registry op %v", viaRouter.Opacity, viaRegistry.Opacity)
					}
				}
			})
		}
	})

	t.Run("unknown-property-is-ignored", func(t *testing.T) {
		// All known CSS properties are now in the registry; the legacy
		// switch is gone. An unknown property name must be silently
		// ignored (matching the original no-op fallthrough behavior),
		// not panic, not write to any field.
		s := &computedStyle{FontSize: 12}
		before := *s
		c.applyProperty("not-a-css-property", "anything", s)
		if !reflect.DeepEqual(*s, before) {
			t.Error("unknown property mutated computedStyle; should be a no-op")
		}
	})
}

// TestCSSPropertyMetadataComplete is a hygiene guard: every entry in
// cssProperties must have non-empty Name, Apply, Category, and Values
// fields. Notes is optional. The generated docs/CSS_SUPPORT.md depends
// on these fields being populated; an entry that ships with empty
// Values would produce an "—" cell in the doc, hiding the property's
// real accepted-values surface from readers.
//
// This test catches the case where a contributor adds a new property
// to the registry but forgets the documentation fields.
func TestCSSPropertyMetadataComplete(t *testing.T) {
	for _, p := range cssProperties {
		if p.Name == "" {
			t.Errorf("cssProperty entry has empty Name: %+v", p)
			continue
		}
		if p.Apply == nil {
			t.Errorf("%q: Apply is nil", p.Name)
		}
		if p.Category == "" {
			t.Errorf("%q: Category is empty", p.Name)
		}
		if len(p.Values) == 0 {
			t.Errorf("%q: Values is empty (every property needs at least one accepted value form for the generated doc)", p.Name)
		}
		// Notes are rendered into a markdown table cell. An unescaped
		// `|` would break the table; escape with `\|` if a pipe is
		// truly needed in the prose (none of the current entries do).
		if strings.ContainsRune(p.Notes, '|') {
			t.Errorf("%q: Notes contains an unescaped `|` which would break the markdown table; use `\\|`", p.Name)
		}
	}
}

// TestCSSPropertyAliasIndexing verifies the cssPropByName map handles
// the (currently zero) aliases correctly. Forward-protective: if an
// alias is added later, this test forces an explicit assertion.
func TestCSSPropertyAliasIndexing(t *testing.T) {
	for _, p := range cssProperties {
		got := cssPropByName[p.Name]
		if got != &cssProperties[mustIndex(t, p.Name)] {
			t.Errorf("byName[%q] does not point into cssProperties slice", p.Name)
		}
		for _, alias := range p.Aliases {
			if cssPropByName[alias] != got {
				t.Errorf("alias %q does not resolve to same descriptor as %q", alias, p.Name)
			}
		}
	}
}

func mustIndex(t *testing.T, name string) int {
	t.Helper()
	for i := range cssProperties {
		if cssProperties[i].Name == name {
			return i
		}
	}
	t.Fatalf("property %q not in cssProperties", name)
	return -1
}

// TestCSSDocsInSync is the CI guard: it asserts that the contents of
// docs/CSS_SUPPORT.md match what RenderCSSPropertiesMarkdown produces
// from the current registry. If a contributor adds, removes, or
// modifies a registry entry without running `go generate ./html/...`,
// this test fails immediately. The fix is always:
//
//	go generate ./html/...
//	git add docs/CSS_SUPPORT.md
func TestCSSDocsInSync(t *testing.T) {
	want := RenderCSSPropertiesMarkdown()

	// Walk up from the test cwd to find docs/CSS_SUPPORT.md. Tests
	// run from the package directory; docs/ is at the repo root.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	var docPath string
	for i := 0; i < 16; i++ {
		candidate := filepath.Join(dir, "docs", "CSS_SUPPORT.md")
		if _, err := os.Stat(candidate); err == nil {
			docPath = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if docPath == "" {
		t.Fatal("docs/CSS_SUPPORT.md not found; run `go generate ./html/...` to create it")
	}

	got, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	if string(got) != want {
		// Find the first line that differs to give a directly-actionable
		// error rather than a byte-count mystery.
		gotLines := strings.Split(string(got), "\n")
		wantLines := strings.Split(want, "\n")
		diffLine := -1
		var gotL, wantL string
		max := len(gotLines)
		if len(wantLines) > max {
			max = len(wantLines)
		}
		for i := 0; i < max; i++ {
			var g, w string
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if g != w {
				diffLine = i + 1
				gotL = g
				wantL = w
				break
			}
		}
		t.Errorf("docs/CSS_SUPPORT.md is stale. Run `go generate ./html/...` to regenerate.\n"+
			"first divergence at line %d:\n"+
			"  on disk: %q\n"+
			"  registry: %q\n"+
			"(file %d bytes, registry would write %d bytes)",
			diffLine, gotL, wantL, len(got), len(want))
	}
}

// TestNoSwitchRegistryOverlap is a FORWARD-GUARD static-analysis test:
// it parses converter_style.go with the Go AST package, finds every
// `case "X":` inside applyProperty, and asserts X is NOT in
// cssPropByName.
//
// As of the registry migration (this PR), applyProperty contains zero
// case clauses — the switch was removed entirely. So this test passes
// vacuously today. Its purpose is to fire IF a future contributor
// reintroduces a switch case that would shadow the registry: the
// dispatch order in applyProperty calls the registry first, so a
// reintroduced switch case for a registered property would be
// unreachable (a silent bug). This guard makes that conflict loud.
func TestNoSwitchRegistryOverlap(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "converter_style.go", nil, 0)
	if err != nil {
		t.Fatalf("parse converter_style.go: %v", err)
	}

	// Find applyProperty's body and walk every case clause inside it.
	var applyFn *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == "applyProperty" {
			applyFn = fn
			break
		}
	}
	if applyFn == nil {
		t.Fatal("applyProperty not found in converter_style.go")
	}

	overlaps := []string{}
	ast.Inspect(applyFn.Body, func(n ast.Node) bool {
		cc, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range cc.List {
			lit, ok := expr.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			// lit.Value is the quoted form, e.g. `"color"`.
			name := lit.Value
			if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
				name = name[1 : len(name)-1]
			}
			if _, exists := cssPropByName[name]; exists {
				overlaps = append(overlaps, name)
			}
		}
		return true
	})

	if len(overlaps) > 0 {
		t.Errorf("properties present in BOTH registry and legacy switch (registry shadows switch): %v", overlaps)
	}
}

// TestCSSPropertyParitySnapshot locks in the exact behavior of each
// migrated property's registry Apply against a snapshot of the
// pre-migration switch logic. Each test row was derived by reading
// the pre-migration case body and computing the expected style for a
// representative input. This is the artifact a reviewer wants to see
// for a "no behavior change" refactor: a small table of (input →
// expected style) tuples that cannot be satisfied by accident.
//
// If a future migration changes the registry entry's Apply in a way
// that diverges from the pre-migration switch (e.g. a typo, dropped
// validation), the matching row here fails immediately.
func TestCSSPropertyParitySnapshot(t *testing.T) {
	cases := []struct {
		name        string
		property    string
		value       string
		fontSize    float64
		expectField func(s *computedStyle) bool
	}{
		{
			name:     "color/red",
			property: "color", value: "red", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				want, _ := parseColor("red")
				return s.Color == want
			},
		},
		{
			name:     "letter-spacing/2px",
			property: "letter-spacing", value: "2px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				l := parseLength("2px")
				if l == nil {
					return false
				}
				return s.LetterSpacing == l.toPoints(0, 12)
			},
		},
		{
			name:     "letter-spacing/0.5em",
			property: "letter-spacing", value: "0.5em", fontSize: 16,
			expectField: func(s *computedStyle) bool {
				l := parseLength("0.5em")
				if l == nil {
					return false
				}
				return s.LetterSpacing == l.toPoints(0, 16)
			},
		},
		{
			name:     "letter-spacing/normal",
			property: "letter-spacing", value: "normal", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.LetterSpacing == 0 },
		},
		{
			name:     "word-spacing/4px",
			property: "word-spacing", value: "4px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				l := parseLength("4px")
				if l == nil {
					return false
				}
				return s.WordSpacing == l.toPoints(0, 12)
			},
		},
		{
			name:     "text-transform/uppercase",
			property: "text-transform", value: "uppercase", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.TextTransform == "uppercase" },
		},
		{
			name:     "text-align/center-set",
			property: "text-align", value: "center", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.TextAlign == layout.AlignCenter && s.TextAlignSet
			},
		},
		{
			name:     "white-space/pre-wrap",
			property: "white-space", value: "pre-wrap", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.WhiteSpace == "pre-wrap" },
		},
		{
			name:     "direction/rtl",
			property: "direction", value: "rtl", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Direction == layout.DirectionRTL },
		},
		{
			name:     "opacity/0.5",
			property: "opacity", value: "0.5", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Opacity == 0.5 },
		},
		{
			name:     "opacity/clamp-high",
			property: "opacity", value: "1.7", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Opacity == 1.0 },
		},
		{
			name:     "opacity/clamp-low",
			property: "opacity", value: "-0.3", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Opacity == 0.0 },
		},

		// --- Hard properties (Phase 2-3 parity) ---

		{
			// margin shorthand with auto in position 1 (top) — sets MarginTopAuto.
			name:     "margin/single-auto",
			property: "margin", value: "auto", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.MarginTopAuto && s.MarginLeftAuto && s.MarginRightAuto
			},
		},
		{
			// margin "10px auto" — top=10px, left/right auto.
			name:     "margin/10px-auto",
			property: "margin", value: "10px auto", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return !s.MarginTopAuto && s.MarginLeftAuto && s.MarginRightAuto
			},
		},
		{
			// border-radius with 2 values: TL=BR=v1, TR=BL=v2.
			name:     "border-radius/two-values",
			property: "border-radius", value: "5px 10px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				tl := parseLengthPt("5px", 12)
				br := parseLengthPt("10px", 12)
				return s.BorderRadiusTL == tl && s.BorderRadiusBR == tl &&
					s.BorderRadiusTR == br && s.BorderRadiusBL == br
			},
		},
		{
			// border-radius with 4 values — sets all corners independently.
			name:     "border-radius/four-values",
			property: "border-radius", value: "1px 2px 3px 4px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BorderRadiusTL == parseLengthPt("1px", 12) &&
					s.BorderRadiusTR == parseLengthPt("2px", 12) &&
					s.BorderRadiusBR == parseLengthPt("3px", 12) &&
					s.BorderRadiusBL == parseLengthPt("4px", 12)
			},
		},
		{
			// border-radius: 50% — percentage must be preserved as a
			// fraction on every corner (abs stays 0), so it can resolve
			// against the box at layout time instead of collapsing to 0.
			name:     "border-radius/percent-shorthand",
			property: "border-radius", value: "50%", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BorderRadiusTLPct == 0.5 && s.BorderRadiusTRPct == 0.5 &&
					s.BorderRadiusBRPct == 0.5 && s.BorderRadiusBLPct == 0.5 &&
					s.BorderRadiusTL == 0 && s.BorderRadiusTR == 0 &&
					s.BorderRadiusBR == 0 && s.BorderRadiusBL == 0
			},
		},
		{
			// Per-corner percentage longhand.
			name:     "border-top-left-radius/percent",
			property: "border-top-left-radius", value: "25%", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BorderRadiusTLPct == 0.25 && s.BorderRadiusTL == 0
			},
		},
		{
			// A length corner must leave the percentage fraction at 0 and
			// resolve to points as before.
			name:     "border-top-left-radius/length",
			property: "border-top-left-radius", value: "14pt", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BorderRadiusTLPct == 0 && s.BorderRadiusTL == parseLengthPt("14pt", 12)
			},
		},
		{
			// Mixed shorthand: percentage and length per corner.
			name:     "border-radius/mixed-four",
			property: "border-radius", value: "50% 4px 25% 8px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BorderRadiusTLPct == 0.5 && s.BorderRadiusTL == 0 &&
					s.BorderRadiusTRPct == 0 && s.BorderRadiusTR == parseLengthPt("4px", 12) &&
					s.BorderRadiusBRPct == 0.25 && s.BorderRadiusBR == 0 &&
					s.BorderRadiusBLPct == 0 && s.BorderRadiusBL == parseLengthPt("8px", 12)
			},
		},
		{
			// font shorthand sets style, weight, size, line-height, family.
			// "16px" parses to 12pt (CSS 1px = 0.75pt).
			name:     "font/full-shorthand",
			property: "font", value: "italic bold 16px/1.5 serif", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.FontStyle == "italic" && s.FontWeight == 700 &&
					s.FontSize == 12 && s.LineHeight == 1.5 && s.FontFamily == "serif"
			},
		},
		{
			// border shorthand sets all 12 fields on all 4 sides.
			name:     "border/shorthand",
			property: "border", value: "2px solid red", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				redClr, _ := parseColor("red")
				return s.BorderTopWidth > 0 && s.BorderRightWidth > 0 &&
					s.BorderBottomWidth > 0 && s.BorderLeftWidth > 0 &&
					s.BorderTopStyle == "solid" && s.BorderRightStyle == "solid" &&
					s.BorderBottomStyle == "solid" && s.BorderLeftStyle == "solid" &&
					s.BorderTopColor == redClr && s.BorderRightColor == redClr &&
					s.BorderBottomColor == redClr && s.BorderLeftColor == redClr
			},
		},
		{
			// gap with 2 values: row-gap then column-gap (= grid-gap).
			// 8px = 6pt, 16px = 12pt.
			name:     "gap/two-values",
			property: "gap", value: "8px 16px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.RowGap == 6 && s.GridColumnGap == 12 && s.Gap == 6
			},
		},
		{
			// gap with single value: applied to all three. 12px = 9pt.
			name:     "gap/single-value",
			property: "gap", value: "12px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.Gap == 9 && s.RowGap == 9 && s.GridColumnGap == 9
			},
		},
		{
			// border-spacing with 2 values: H then V.
			// 5px = 3.75pt, 10px = 7.5pt.
			name:     "border-spacing/two-values",
			property: "border-spacing", value: "5px 10px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BorderSpacingH == 3.75 && s.BorderSpacingV == 7.5
			},
		},
		{
			// vertical-align keyword: sets VerticalAlign, clears BaselineShiftSet.
			name:     "vertical-align/super-keyword",
			property: "vertical-align", value: "super", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.VerticalAlign == "super" && !s.BaselineShiftSet
			},
		},
		{
			// vertical-align with length: sets BaselineShiftValue + Set flag.
			// 5px = 3.75pt.
			name:     "vertical-align/5px-length",
			property: "vertical-align", value: "5px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BaselineShiftSet && s.BaselineShiftValue == 3.75
			},
		},
		{
			// baseline-shift "sub" alias.
			name:     "baseline-shift/sub",
			property: "baseline-shift", value: "sub", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.VerticalAlign == "sub" && !s.BaselineShiftSet
			},
		},
		{
			// background gradient routes to BackgroundImage, not BackgroundColor.
			name:     "background/linear-gradient",
			property: "background", value: "linear-gradient(red, blue)", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BackgroundImage == "linear-gradient(red, blue)" &&
					s.BackgroundColor == nil
			},
		},
		{
			// background plain color routes to BackgroundColor, not Image.
			name:     "background/plain-color",
			property: "background", value: "yellow", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BackgroundImage == "" && s.BackgroundColor != nil
			},
		},
		{
			// column-rule shorthand sets all three fields.
			name:     "column-rule/shorthand",
			property: "column-rule", value: "1px dashed blue", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				blueClr, _ := parseColor("blue")
				return s.ColumnRuleWidth > 0 && s.ColumnRuleStyle == "dashed" &&
					s.ColumnRuleColor == blueClr
			},
		},
		{
			// padding shorthand fans to 4 sides. Px → pt at 0.75 ratio
			// resolves through PaddingXxxAt; the stored *cssLength keeps
			// the unresolved px form. Drive PaddingXxxAt(0) since px is
			// container-independent.
			name:     "padding/four-values",
			property: "padding", value: "1px 2px 3px 4px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.PaddingTopAt(0) == 0.75 && s.PaddingRightAt(0) == 1.5 &&
					s.PaddingBottomAt(0) == 2.25 && s.PaddingLeftAt(0) == 3
			},
		},

		// --- Additional category coverage ---

		// Backgrounds
		{
			name:     "background-color/hex",
			property: "background-color", value: "#abcdef", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				want, _ := parseColor("#abcdef")
				return s.BackgroundColor != nil && *s.BackgroundColor == want
			},
		},
		{
			name:     "background-image/url",
			property: "background-image", value: "url(foo.png)", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BackgroundImage == "url(foo.png)"
			},
		},
		{
			name:     "background-repeat/no-repeat",
			property: "background-repeat", value: "no-repeat", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.BackgroundRepeat == "no-repeat" },
		},

		// Box model individual sides
		{
			name:     "width/100px",
			property: "width", value: "100px", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Width != nil },
		},
		{
			name:     "min-height/2em",
			property: "min-height", value: "2em", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.MinHeight != nil },
		},
		{
			name:     "aspect-ratio/16-9",
			property: "aspect-ratio", value: "16/9", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.AspectRatio > 0 },
		},
		{
			name:     "padding-top/8px",
			property: "padding-top", value: "8px", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.PaddingTopAt(0) == 6 },
		},
		{
			name:     "margin-top/auto",
			property: "margin-top", value: "auto", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.MarginTopAuto },
		},
		{
			name:     "margin-bottom/16px",
			property: "margin-bottom", value: "16px", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.MarginBottomAt(0) == 12 },
		},

		// Borders individual fields
		{
			name:     "border-top-width/3px",
			property: "border-top-width", value: "3px", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.BorderTopWidth == 2.25 },
		},
		{
			name:     "border-color/red",
			property: "border-color", value: "red", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				want, _ := parseColor("red")
				return s.BorderTopColor == want && s.BorderLeftColor == want
			},
		},
		{
			name:     "border-style/dashed",
			property: "border-style", value: "dashed", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BorderTopStyle == "dashed" && s.BorderRightStyle == "dashed"
			},
		},
		{
			name:     "border-top-left-radius/5px",
			property: "border-top-left-radius", value: "5px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BorderRadiusTL == parseLengthPt("5px", 12)
			},
		},
		{
			name:     "border-collapse/collapse",
			property: "border-collapse", value: "collapse", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.BorderCollapse == "collapse" },
		},

		// Layout
		{
			name:     "display/flex",
			property: "display", value: "flex", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Display == "flex" },
		},
		{
			name:     "position/absolute",
			property: "position", value: "absolute", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Position == "absolute" },
		},
		{
			name:     "z-index/5",
			property: "z-index", value: "5", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.ZIndex == 5 && s.ZIndexSet },
		},
		{
			name:     "overflow/hidden",
			property: "overflow", value: "hidden", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Overflow == "hidden" },
		},
		{
			name:     "float/right",
			property: "float", value: "right", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Float == "right" },
		},
		{
			name:     "box-sizing/border-box",
			property: "box-sizing", value: "border-box", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.BoxSizing == "border-box" },
		},
		{
			name:     "visibility/hidden",
			property: "visibility", value: "hidden", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Visibility == "hidden" },
		},

		// Flexbox
		{
			name:     "flex-direction/column",
			property: "flex-direction", value: "column", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.FlexDirection == "column" },
		},
		{
			name:     "justify-content/space-between",
			property: "justify-content", value: "space-between", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.JustifyContent == "space-between" },
		},
		{
			name:     "align-items/center",
			property: "align-items", value: "center", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.AlignItems == "center" },
		},
		{
			name:     "flex-grow/1",
			property: "flex-grow", value: "1", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.FlexGrow == 1 },
		},
		{
			name:     "order/-2",
			property: "order", value: "-2", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Order == -2 },
		},

		// Grid
		{
			name:     "grid-template-columns/3-1fr",
			property: "grid-template-columns", value: "1fr 1fr 1fr", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.GridTemplateColumns == "1fr 1fr 1fr" },
		},
		{
			name:     "grid-row-start/2",
			property: "grid-row-start", value: "2", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.GridRowStart == 2 },
		},

		// MultiColumn
		{
			name:     "column-count/3",
			property: "column-count", value: "3", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.ColumnCount == 3 },
		},
		{
			name:     "columns/2-200px",
			property: "columns", value: "2 200px", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.ColumnCount == 2 && s.ColumnWidth == 150 // 200px = 150pt
			},
		},

		// Pagination
		{
			name:     "page-break-before/always",
			property: "page-break-before", value: "always", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.PageBreakBefore == "always" },
		},
		{
			name:     "page-break-inside/avoid",
			property: "page-break-inside", value: "avoid", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.PageBreakInside == "avoid" },
		},
		{
			name:     "orphans/3",
			property: "orphans", value: "3", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Orphans == 3 },
		},

		// Lists
		{
			name:     "list-style-type/disc",
			property: "list-style-type", value: "disc", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.ListStyleType == "disc" },
		},

		// Effects
		{
			name:     "transform/translate",
			property: "transform", value: "translate(5px, 10px)", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Transform == "translate(5px, 10px)" },
		},
		{
			name:     "object-fit/cover",
			property: "object-fit", value: "cover", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.ObjectFit == "cover" },
		},
		{
			name:     "outline/composed",
			property: "outline", value: "2px solid blue", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				blueClr, _ := parseColor("blue")
				return s.OutlineWidth > 0 && s.OutlineStyle == "solid" && s.OutlineColor == blueClr
			},
		},
		{
			name:     "text-overflow/ellipsis",
			property: "text-overflow", value: "ellipsis", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.TextOverflow == "ellipsis" },
		},

		// PDF-specific
		{
			name:     "bookmark-level/2",
			property: "bookmark-level", value: "2", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BookmarkLevel == 2 && s.BookmarkLevelSet
			},
		},
		{
			name:     "bookmark-level/none",
			property: "bookmark-level", value: "none", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.BookmarkLevel == -1 && s.BookmarkLevelSet
			},
		},
		{
			name:     "bookmark-state/open",
			property: "bookmark-state", value: "open", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.BookmarkState == "open" },
		},
		{
			name:     "string-set/named",
			property: "string-set", value: "title content()", fontSize: 12,
			expectField: func(s *computedStyle) bool {
				return s.StringSetName == "title" && s.StringSetValue == "content()"
			},
		},

		// Typography extras
		{
			// "700" passes through to the 700 rung on the numeric ladder.
			name:     "font-weight/700",
			property: "font-weight", value: "700", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.FontWeight == 700 },
		},
		{
			name:     "font-style/italic",
			property: "font-style", value: "italic", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.FontStyle == "italic" },
		},
		{
			name:     "text-decoration/underline",
			property: "text-decoration", value: "underline", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.TextDecoration != 0 },
		},
		{
			name:     "word-break/break-all",
			property: "word-break", value: "break-all", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.WordBreak == "break-all" },
		},
		{
			name:     "hyphens/auto",
			property: "hyphens", value: "auto", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Hyphens == "auto" },
		},
		{
			name:     "hyphens/-webkit-alias",
			property: "-webkit-hyphens", value: "auto", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.Hyphens == "auto" },
		},
		{
			name:     "line-height/1.5",
			property: "line-height", value: "1.5", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.LineHeight == 1.5 },
		},
		{
			name:     "text-indent/2em",
			property: "text-indent", value: "2em", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.TextIndent == 24 },
		},
		{
			name:     "unicode-bidi/isolate",
			property: "unicode-bidi", value: "isolate", fontSize: 12,
			expectField: func(s *computedStyle) bool { return s.UnicodeBidi == "isolate" },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &computedStyle{FontSize: tc.fontSize}
			p, ok := cssPropByName[tc.property]
			if !ok {
				t.Fatalf("registry missing %q", tc.property)
			}
			p.Apply(s, tc.value)
			if !tc.expectField(s) {
				t.Errorf("parity snapshot drift on %s — registry produced %+v", tc.name, s)
			}
		})
	}
}

// TestAtRulesDocCoverage parses html/css.go and asserts that every
// at-rule literal recognized by parseCSS appears as a code-fenced
// reference in the rendered CSS_SUPPORT.md. The At-rules section is
// hand-written (no parallel registry — see css_props_doc.go), so this
// test is the drift guard: if a future contributor wires up a new
// at-rule in parseCSS without documenting it, the test fails.
//
// Margin-box names (top-center, etc.) are NOT prefixed with "@" in
// extractMarginBoxes — they're parsed as bare identifiers — so they're
// not auto-discovered and must be hand-listed in the doc.
func TestAtRulesDocCoverage(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "css.go", nil, 0)
	if err != nil {
		t.Fatalf("parse css.go: %v", err)
	}

	doc := RenderCSSPropertiesMarkdown()

	seen := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		val := lit.Value
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		if !strings.HasPrefix(val, "@") {
			return true
		}
		// Normalize "@page ", "@page:", "@page" → "@page". Take the
		// at-rule keyword up to the first space or ':'.
		name := val
		for i, c := range name {
			if c == ' ' || c == ':' {
				name = name[:i]
				break
			}
		}
		// Filter out empty "@" or anything that doesn't look like an
		// at-rule keyword.
		if len(name) < 2 {
			return true
		}
		seen[name] = true
		return true
	})

	for name := range seen {
		if !strings.Contains(doc, "`"+name+"`") {
			t.Errorf("at-rule %q is referenced in html/css.go but not documented in CSS_SUPPORT.md — add it to the At-rules section in html/css_props_doc.go", name)
		}
	}
}

// TestFunctionsDocCoverage is the drift guard for the Functions section
// of CSS_SUPPORT.md. The list below mirrors what the doc claims to
// support; the test asserts (a) each name appears as a code-fenced
// reference in the rendered doc and (b) for every category, at least
// one representative form is actually recognized by the relevant
// parser. Adding a new function value to a Folio parser without
// updating the doc requires also updating this list — that's the
// forcing function.
//
// Function-call dispatch in Folio is spread across half a dozen files
// (properties.go for color/math, converter_style_parsers.go for
// transform, css_props.go for gradients, page.go for page-counter,
// converter_style.go for var/counter), so a single AST walk like
// TestAtRulesDocCoverage isn't tractable. A static list is the
// pragmatic alternative.
func TestFunctionsDocCoverage(t *testing.T) {
	want := []string{
		// Math
		"calc()", "min()", "max()", "clamp()",
		// Color
		"rgb()", "rgba()", "hsl()", "hsla()", "cmyk()",
		// Gradients
		"linear-gradient()", "repeating-linear-gradient()",
		"radial-gradient()", "repeating-radial-gradient()",
		// Content / counters
		"var()", "attr()", "content()", "counter()", "counters()", "string()",
		// Transforms
		"translate()", "translateX()", "translateY()",
		"rotate()", "scale()", "scaleX()", "scaleY()",
		"skew()", "skewX()", "skewY()",
		// Other
		"url()",
	}

	doc := RenderCSSPropertiesMarkdown()
	for _, name := range want {
		if !strings.Contains(doc, "`"+name+"`") {
			t.Errorf("function %q is in the documented-functions list but not present in CSS_SUPPORT.md — add it to the Functions section in html/css_props_doc.go", name)
		}
	}

	// Behavioral smoke checks: one representative form per category to
	// catch the case where a function is documented but its parser was
	// removed. Per-function parity is covered exhaustively elsewhere
	// (parseLength, parseColor, parseTransform have their own test
	// suites); these assertions are a bare sanity net.
	if l := parseLength("calc(10px + 5px)"); l == nil {
		t.Error("parseLength rejected calc(10px + 5px) — Math section is documenting an unsupported form")
	}
	if l := parseLength("min(10px, 20px)"); l == nil {
		t.Error("parseLength rejected min(10px, 20px)")
	}
	if l := parseLength("max(10px, 20px)"); l == nil {
		t.Error("parseLength rejected max(10px, 20px)")
	}
	if l := parseLength("clamp(5px, 10px, 20px)"); l == nil {
		t.Error("parseLength rejected clamp(5px, 10px, 20px)")
	}
	if _, ok := parseColor("rgb(0, 0, 0)"); !ok {
		t.Error("parseColor rejected rgb(0, 0, 0)")
	}
	if _, ok := parseColor("hsl(0, 0%, 0%)"); !ok {
		t.Error("parseColor rejected hsl(0, 0%, 0%)")
	}
	if _, ok := parseColor("cmyk(0, 0, 0, 1)"); !ok {
		t.Error("parseColor rejected cmyk(0, 0, 0, 1)")
	}
	if ops := parseTransform("translate(5px, 10px) rotate(45deg)"); len(ops) != 2 {
		t.Errorf("parseTransform produced %d ops for translate+rotate; want 2", len(ops))
	}
}

// TestSelectorsDocCoverage is the drift guard for the Selectors
// section of CSS_SUPPORT.md. The static lists below mirror what the
// doc claims; the test asserts each name appears as a code-fenced
// reference in the rendered doc.
//
// Behavioral coverage for selector matching is in TestSelectorMatches
// (and friends) — this test focuses on the doc/parser surface area:
// if a contributor adds a new pseudo-class to pseudoMatches without
// also updating CSS_SUPPORT.md, this fails.
func TestSelectorsDocCoverage(t *testing.T) {
	combinators := []string{">", "+", "~"}
	attrOps := []string{"^=", "$=", "*=", "~=", "|="}
	pseudoClasses := []string{
		":root", ":empty",
		":first-child", ":last-child",
		":nth-child(", ":nth-last-child(",
		":first-of-type", ":last-of-type",
		":nth-of-type(", ":nth-last-of-type(",
		":not(", ":is(", ":where(",
	}
	pseudoElements := []string{"::before", "::after", "::marker", "::placeholder"}

	doc := RenderCSSPropertiesMarkdown()
	check := func(group string, names []string) {
		for _, name := range names {
			if !strings.Contains(doc, "`"+name) {
				t.Errorf("%s %q is in the documented-selectors list but not present in CSS_SUPPORT.md — add it to the Selectors section in html/css_props_doc.go", group, name)
			}
		}
	}
	check("combinator", combinators)
	check("attribute operator", attrOps)
	check("pseudo-class", pseudoClasses)
	check("pseudo-element", pseudoElements)
}

// TestGlossaryDriftGuard verifies that every angle-bracket placeholder
// used in cssProperties[].Values (e.g. "<length>", "<color>") is defined
// in the Value-form glossary section of CSS_SUPPORT.md. If a contributor
// adds a new property whose Values reference an undefined placeholder,
// this test fails.
func TestGlossaryDriftGuard(t *testing.T) {
	// Self-explanatory placeholders that don't need glossary entries.
	// They map 1:1 onto a sibling property name or are obvious from context.
	allowlist := map[string]bool{
		"<font-family>":      true, // maps to font-family property
		"<font-size>":        true, // maps to font-size property
		"<font-weight>":      true, // maps to font-weight property
		"<font-style>":       true, // maps to font-style property
		"<line-height>":      true, // maps to line-height property
		"<flex-direction>":   true, // maps to flex-direction property
		"<flex-wrap>":        true, // maps to flex-wrap property
		"<flex-grow>":        true, // maps to flex-grow property
		"<flex-shrink>":      true, // maps to flex-shrink property
		"<flex-basis>":       true, // maps to flex-basis property
		"<column-gap>":       true, // maps to gap property
		"<row-gap>":          true, // maps to gap property
		"<column-width>":     true, // maps to column-width property
		"<column-count>":     true, // maps to column-count property
		"<generic-family>":   true, // serif, sans-serif, monospace
		"<absolute-size>":    true, // small, medium, large, etc.
		"<relative-size>":    true, // smaller, larger
		"<family-name>":      true, // font family name
		"<content-list>":     true, // CSS content property values
		"<number 0..1>":      true, // range-qualified number
		"<integer 100..900>": true, // range-qualified integer (font-weight)
		"<1-4 of these>":     true, // descriptive text, not a placeholder
	}

	// 1. Extract all <...> placeholders used in cssProperties[].Values.
	used := map[string]bool{}
	for _, p := range cssProperties {
		for _, v := range p.Values {
			for _, tok := range extractPlaceholders(v) {
				if !allowlist[tok] {
					used[tok] = true
				}
			}
		}
	}

	// 2. Extract placeholders defined in the glossary.
	doc := RenderCSSPropertiesMarkdown()
	defined := extractGlossaryPlaceholders(doc)

	// 3. Check each used placeholder is defined.
	for ph := range used {
		if !defined[ph] {
			t.Errorf("placeholder %q used in cssProperties[].Values but not defined in Value-form glossary — add it to html/css_props_doc.go or the allowlist in this test", ph)
		}
	}
}

// extractPlaceholders returns all <...> tokens in s.
func extractPlaceholders(s string) []string {
	var result []string
	for {
		start := strings.Index(s, "<")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], ">")
		if end < 0 {
			break
		}
		result = append(result, s[start:start+end+1])
		s = s[start+end+1:]
	}
	return result
}

// extractGlossaryPlaceholders parses the glossary section of the doc
// and returns the set of defined placeholders.
func extractGlossaryPlaceholders(doc string) map[string]bool {
	defined := map[string]bool{}
	inGlossary := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.Contains(line, "## Value-form glossary") {
			inGlossary = true
			continue
		}
		if inGlossary && strings.HasPrefix(line, "## ") {
			break // next section
		}
		if inGlossary && strings.HasPrefix(line, "| `") {
			// Extract all <...> placeholders from the first column
			// (may be multiple, comma-separated)
			for _, ph := range extractPlaceholders(line) {
				defined[ph] = true
			}
		}
	}
	return defined
}

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"slices"
	"strings"
)

// cssWideKeyword describes how one property resolves the CSS-wide keywords
// `inherit`, `initial` and `unset` (CSS Cascading 4 §7.3). Property parsers
// reject these keywords as invalid values, so without this table a declared
// `color: inherit` was silently dropped and a less specific rule or a UA tag
// default (e.g. `a { color: blue }`, `<b>` bold, `<h1>` 24pt) stayed in
// effect. The keywords are cascaded values like any other and must win
// whenever their declaration wins.
type cssWideKeyword struct {
	// inherited reports whether the property is inherited, which decides
	// whether `unset` behaves as `inherit` or as `initial`.
	inherited bool
	// inheritAliases lists extra keywords (lowercase) that, on this
	// property only, compute to the inherited value.
	inheritAliases []string
	// inherit copies the parent's computed value into s.
	inherit func(s, parent *computedStyle)
	// initial resets s to the property's initial value, read from init:
	// the converter's root style (see converter.initialStyle), so values
	// configured through Options such as DefaultFontSize are honored.
	initial func(s, init *computedStyle)
}

// cssWideKeywords lists the properties whose CSS-wide keywords are resolved
// by applyCSSWideKeyword. Properties not listed keep their previous
// behavior: the keyword reaches the property parser, which ignores it.
var cssWideKeywords = map[string]cssWideKeyword{
	"color": {
		inherited: true,
		// On the color property itself, currentColor computes to the
		// inherited value (CSS Color 4 §6.4).
		inheritAliases: []string{"currentcolor"},
		inherit:        func(s, p *computedStyle) { s.Color = p.Color },
		initial:        func(s, i *computedStyle) { s.Color = i.Color },
	},
	"font-size": {
		inherited: true,
		inherit:   func(s, p *computedStyle) { s.FontSize = p.FontSize },
		initial:   func(s, i *computedStyle) { s.FontSize = i.FontSize },
	},
	"font-weight": {
		inherited: true,
		inherit:   func(s, p *computedStyle) { s.FontWeight = p.FontWeight },
		initial:   func(s, i *computedStyle) { s.FontWeight = i.FontWeight },
	},
	"font-style": {
		inherited: true,
		inherit:   func(s, p *computedStyle) { s.FontStyle = p.FontStyle },
		initial:   func(s, i *computedStyle) { s.FontStyle = i.FontStyle },
	},
	"font-family": {
		inherited: true,
		inherit:   func(s, p *computedStyle) { s.FontFamily = p.FontFamily },
		initial:   func(s, i *computedStyle) { s.FontFamily = i.FontFamily },
	},
	"line-height": {
		inherited: true,
		inherit: func(s, p *computedStyle) {
			// The parent's computed value: a multiplier, or an absolute
			// length that computeElementStyle re-expresses against this
			// element's final font-size. Clear lineHeightRaw so an earlier
			// line-height declaration on this element is not re-resolved.
			s.LineHeight = p.LineHeight
			s.lineHeightAbs = p.lineHeightAbs
			s.lineHeightAbsSet = p.lineHeightAbsSet
			s.lineHeightRaw = ""
		},
		initial: func(s, _ *computedStyle) {
			s.setLineHeight(lineHeightSpec{kind: lineHeightNormal})
			s.lineHeightRaw = ""
		},
	},
	"text-align": {
		inherited: true,
		inherit: func(s, p *computedStyle) {
			s.TextAlign = p.TextAlign
			s.TextAlignKeyword = p.TextAlignKeyword
			s.TextAlignSet = p.TextAlignSet
			// Declared on this element, so it outranks UA defaults such
			// as <th> centering.
			s.TextAlignSelfSet = true
		},
		initial: func(s, _ *computedStyle) {
			s.TextAlign, s.TextAlignKeyword, _ = parseTextAlign("start")
			s.TextAlignSet = true
			s.TextAlignSelfSet = true
		},
	},
	"text-transform": {
		inherited: true,
		inherit:   func(s, p *computedStyle) { s.TextTransform = p.TextTransform },
		initial:   func(s, _ *computedStyle) { s.TextTransform = "none" },
	},
	"white-space": {
		inherited: true,
		inherit:   func(s, p *computedStyle) { s.WhiteSpace = p.WhiteSpace },
		initial:   func(s, _ *computedStyle) { s.WhiteSpace = "normal" },
	},
	"letter-spacing": {
		inherited: true,
		inherit:   func(s, p *computedStyle) { s.LetterSpacing = p.LetterSpacing },
		initial:   func(s, _ *computedStyle) { s.LetterSpacing = 0 },
	},
	"word-spacing": {
		inherited: true,
		inherit:   func(s, p *computedStyle) { s.WordSpacing = p.WordSpacing },
		initial:   func(s, _ *computedStyle) { s.WordSpacing = 0 },
	},
	"text-indent": {
		inherited: true,
		inherit:   func(s, p *computedStyle) { s.TextIndent = p.TextIndent },
		initial:   func(s, _ *computedStyle) { s.TextIndent = 0 },
	},
	"background-color": {
		inherited: false,
		inherit: func(s, p *computedStyle) {
			if p.BackgroundColor == nil {
				s.BackgroundColor = nil
				return
			}
			c := *p.BackgroundColor
			s.BackgroundColor = &c
		},
		initial: func(s, _ *computedStyle) { s.BackgroundColor = nil },
	},
}

// initialStyle returns the style CSS initial values are read from:
// defaultStyle with the root font size configured through
// Options.DefaultFontSize, which is folio's `medium`.
func (c *converter) initialStyle() computedStyle {
	s := defaultStyle()
	if c.rootFontSize > 0 {
		s.FontSize = c.rootFontSize
	}
	return s
}

// applyCSSWideKeyword resolves a declaration whose value is a CSS-wide
// keyword (or one of the property's inheritAliases) for a property listed
// in cssWideKeywords. It reports whether the declaration was handled; when
// false the caller applies it normally.
func (c *converter) applyCSSWideKeyword(prop, value string, s, parent *computedStyle) bool {
	kw, ok := cssWideKeywords[prop]
	if !ok {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(value))
	switch {
	case v == "inherit", v == "unset" && kw.inherited, slices.Contains(kw.inheritAliases, v):
		kw.inherit(s, parent)
	case v == "initial", v == "unset":
		init := c.initialStyle()
		kw.initial(s, &init)
	default:
		return false
	}
	return true
}

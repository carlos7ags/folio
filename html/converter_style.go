// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// computeElementStyle resolves the style for an element node.
func (c *converter) computeElementStyle(n *html.Node, parent computedStyle) computedStyle {
	style := parent.inherit()

	// Apply tag defaults.
	c.applyTagDefaults(n, &style)

	// Apply stylesheet rules.
	if c.sheet != nil {
		for _, decl := range c.sheet.matchingDeclarations(n) {
			c.applyProperty(decl.property, decl.value, &style)
		}
	}

	// Apply inline style attribute (highest specificity).
	if attr := getAttr(n, "style"); attr != "" {
		c.applyInlineStyle(attr, &style)
	}

	return style
}

// applyTagDefaults sets browser-like defaults for known HTML elements.
func (c *converter) applyTagDefaults(n *html.Node, style *computedStyle) {
	switch n.DataAtom {
	case atom.H1:
		style.FontSize = 24 // 32px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 16.08 // 0.67em at 32px → 32*0.67*0.75
		style.MarginBottom = 16.08
	case atom.H2:
		style.FontSize = 18 // 24px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 14.94 // 0.83em at 24px → 24*0.83*0.75
		style.MarginBottom = 14.94
	case atom.H3:
		style.FontSize = 14.04 // 18.72px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 14.04 // 1em at 18.72px → 18.72*0.75
		style.MarginBottom = 14.04
	case atom.H4:
		style.FontSize = 12 // 16px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 16.02 // 1.33em at 16px → 16*1.33*0.75
		style.MarginBottom = 16.02
	case atom.H5:
		style.FontSize = 9.96 // 13.28px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 16.60 // 1.67em at 13.28px → 13.28*1.67*0.75
		style.MarginBottom = 16.60
	case atom.H6:
		style.FontSize = 8.01 // 10.72px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 18.62 // 2.33em at 10.72px → 10.72*2.33*0.75
		style.MarginBottom = 18.62
	case atom.P:
		style.MarginTop = 12 // 1em at 16px → 16*0.75
		style.MarginBottom = 12
	case atom.Strong, atom.B:
		style.FontWeight = "bold"
	case atom.Em, atom.I:
		style.FontStyle = "italic"
	case atom.U:
		style.TextDecoration |= layout.DecorationUnderline
	case atom.S, atom.Del:
		style.TextDecoration |= layout.DecorationStrikethrough
	case atom.Mark:
		// Browser default: yellow highlight background.
		bg := layout.RGB(1, 1, 0)
		style.BackgroundColor = &bg
	case atom.Small:
		style.FontSize = style.FontSize * 0.833
	case atom.Sub:
		style.FontSize = style.FontSize * 0.75
		style.VerticalAlign = "sub"
	case atom.Sup:
		style.FontSize = style.FontSize * 0.75
		style.VerticalAlign = "super"
	case atom.Code:
		style.FontFamily = "courier"
	case atom.Pre:
		style.FontFamily = "courier"
		style.WhiteSpace = "pre"
		style.MarginTop = 12
		style.MarginBottom = 12
	case atom.Hr:
		style.MarginTop = 6
		style.MarginBottom = 6
	case atom.A:
		style.Color = layout.RGB(0, 0, 0.933) // default link blue
		style.TextDecoration |= layout.DecorationUnderline
	case atom.Table:
		style.MarginTop = 12
		style.MarginBottom = 12
		style.BorderCollapse = "collapse"
	case atom.Ul, atom.Ol:
		style.MarginTop = 12
		style.MarginBottom = 12
	case atom.Blockquote:
		style.MarginTop = 12
		style.MarginBottom = 12
	case atom.Dl:
		style.MarginTop = 12
		style.MarginBottom = 12
	case atom.Dt:
		style.FontWeight = "bold"
	case atom.Dd:
		style.MarginLeft = 30 // browser default ~40px → 30pt
	case atom.Figure:
		style.MarginTop = 12
		style.MarginBottom = 12
	case atom.Figcaption:
		style.FontStyle = "italic"
		style.FontSize = style.FontSize * 0.9
	case atom.Fieldset:
		style.MarginTop = 9 // ~12px * 0.75
		style.MarginBottom = 9
		style.Display = "block"
	case atom.Legend:
		style.FontWeight = "bold"
	case atom.Button:
		style.Display = "inline"
	case atom.Input, atom.Select, atom.Textarea:
		style.Display = "inline"
	case atom.Label:
		style.Display = "inline"
	}
}

// applyInlineStyle parses a CSS style attribute and applies it to the style.
func (c *converter) applyInlineStyle(attr string, style *computedStyle) {
	for _, decl := range splitDeclarations(attr) {
		prop, val := splitDeclaration(decl)
		if prop == "" || val == "" {
			continue
		}
		c.applyProperty(prop, val, style)
	}
}

// resolveVars replaces var(--name) and var(--name, fallback) references in a
// CSS value string using the element's custom properties. Handles nested var()
// calls and multiple var() references in a single value.
func resolveVars(value string, style *computedStyle) string {
	for {
		idx := strings.Index(value, "var(")
		if idx < 0 {
			return value
		}
		// Find matching closing paren, accounting for nested parens.
		depth := 0
		end := -1
		for i := idx + 4; i < len(value); i++ {
			if value[i] == '(' {
				depth++
			}
			if value[i] == ')' {
				if depth == 0 {
					end = i
					break
				}
				depth--
			}
		}
		if end < 0 {
			return value // malformed, bail out
		}

		inner := value[idx+4 : end]
		// Split on first comma for fallback.
		name, fallback := inner, ""
		if ci := strings.IndexByte(inner, ','); ci >= 0 {
			name = strings.TrimSpace(inner[:ci])
			fallback = strings.TrimSpace(inner[ci+1:])
		} else {
			name = strings.TrimSpace(name)
		}

		resolved := fallback
		if style.CustomProperties != nil {
			if v, ok := style.CustomProperties[name]; ok {
				resolved = v
			}
		}
		value = value[:idx] + resolved + value[end+1:]
	}
}

// applyProperty applies a single CSS property to a computed style.
func (c *converter) applyProperty(prop, val string, style *computedStyle) {
	// Resolve var() references before any processing.
	if strings.Contains(val, "var(") {
		val = resolveVars(val, style)
	}

	// Store custom properties (CSS variables).
	if strings.HasPrefix(prop, "--") {
		if style.CustomProperties == nil {
			style.CustomProperties = make(map[string]string)
		}
		style.CustomProperties[prop] = val
		return
	}

	switch prop {
	case "color":
		if c, ok := parseColor(val); ok {
			style.Color = c
		}
	case "background-color":
		if c, ok := parseColor(val); ok {
			style.BackgroundColor = &c
		}
	case "background":
		// Background shorthand: handle gradients, urls, or plain colors.
		lower := strings.ToLower(strings.TrimSpace(val))
		if strings.HasPrefix(lower, "linear-gradient(") ||
			strings.HasPrefix(lower, "repeating-linear-gradient(") ||
			strings.HasPrefix(lower, "radial-gradient(") ||
			strings.HasPrefix(lower, "repeating-radial-gradient(") ||
			strings.HasPrefix(lower, "url(") {
			style.BackgroundImage = strings.TrimSpace(val)
		} else if clr, ok := parseColor(val); ok {
			style.BackgroundColor = &clr
		}
	case "background-image":
		style.BackgroundImage = strings.TrimSpace(val)
	case "background-size":
		style.BackgroundSize = strings.TrimSpace(strings.ToLower(val))
	case "background-position":
		style.BackgroundPosition = strings.TrimSpace(strings.ToLower(val))
	case "background-repeat":
		style.BackgroundRepeat = strings.TrimSpace(strings.ToLower(val))
	case "font-family":
		style.FontFamily = parseFontFamily(val)
	case "font-size":
		style.FontSize = parseFontSize(val, style.FontSize)
	case "font-weight":
		style.FontWeight = parseFontWeight(val)
	case "font-style":
		style.FontStyle = parseFontStyle(val)
	case "text-align":
		if a, ok := parseTextAlign(val); ok {
			style.TextAlign = a
		}
	case "text-align-last":
		if a, ok := parseTextAlign(val); ok {
			style.TextAlignLast = a
			style.TextAlignLastSet = true
		}
	case "text-decoration":
		style.TextDecoration = parseTextDecoration(val)
	case "text-transform":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "uppercase" || v == "lowercase" || v == "capitalize" || v == "none" {
			style.TextTransform = v
		}
	case "white-space":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "normal" || v == "nowrap" || v == "pre" || v == "pre-wrap" || v == "pre-line" {
			style.WhiteSpace = v
		}
	case "word-break":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "normal" || v == "break-all" || v == "keep-all" || v == "break-word" {
			style.WordBreak = v
		}
	case "hyphens", "-webkit-hyphens":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "none" || v == "manual" || v == "auto" {
			style.Hyphens = v
		}
	case "letter-spacing":
		if l := parseLength(val); l != nil {
			style.LetterSpacing = l.toPoints(0, style.FontSize)
		} else if strings.TrimSpace(strings.ToLower(val)) == "normal" {
			style.LetterSpacing = 0
		}
	case "word-spacing":
		if l := parseLength(val); l != nil {
			style.WordSpacing = l.toPoints(0, style.FontSize)
		} else if strings.TrimSpace(strings.ToLower(val)) == "normal" {
			style.WordSpacing = 0
		}
	case "text-indent":
		if l := parseLength(val); l != nil {
			style.TextIndent = l.toPoints(0, style.FontSize)
		}
	case "line-height":
		style.LineHeight = parseLineHeight(val, style.FontSize)
	case "display":
		style.Display = parseDisplay(val)
	case "margin":
		style.MarginTop, style.MarginRight, style.MarginBottom, style.MarginLeft =
			parseMarginShorthand(val, style.FontSize)
		// Detect auto keywords in margin shorthand.
		parts := strings.Fields(val)
		autoFlags := make([]bool, len(parts))
		for i, p := range parts {
			autoFlags[i] = strings.ToLower(p) == "auto"
		}
		switch len(parts) {
		case 1:
			if autoFlags[0] {
				style.MarginTopAuto = true
				style.MarginLeftAuto = true
				style.MarginRightAuto = true
			}
		case 2:
			if autoFlags[0] {
				style.MarginTopAuto = true
			}
			if autoFlags[1] {
				style.MarginLeftAuto = true
				style.MarginRightAuto = true
			}
		case 3:
			if autoFlags[0] {
				style.MarginTopAuto = true
			}
			if autoFlags[1] {
				style.MarginLeftAuto = true
				style.MarginRightAuto = true
			}
		case 4:
			if autoFlags[0] {
				style.MarginTopAuto = true
			}
			if autoFlags[1] {
				style.MarginRightAuto = true
			}
			if autoFlags[3] {
				style.MarginLeftAuto = true
			}
		}
	case "margin-top":
		if strings.TrimSpace(strings.ToLower(val)) == "auto" {
			style.MarginTopAuto = true
		} else {
			style.MarginTop = parseBoxSide(val, style.FontSize)
		}
	case "margin-right":
		if strings.TrimSpace(strings.ToLower(val)) == "auto" {
			style.MarginRightAuto = true
		} else {
			style.MarginRight = parseBoxSide(val, style.FontSize)
		}
	case "margin-bottom":
		style.MarginBottom = parseBoxSide(val, style.FontSize)
	case "margin-left":
		if strings.TrimSpace(strings.ToLower(val)) == "auto" {
			style.MarginLeftAuto = true
		} else {
			style.MarginLeft = parseBoxSide(val, style.FontSize)
		}
	case "padding":
		style.PaddingTop, style.PaddingRight, style.PaddingBottom, style.PaddingLeft =
			parseMarginShorthand(val, style.FontSize)
	case "padding-top":
		style.PaddingTop = parseBoxSide(val, style.FontSize)
	case "padding-right":
		style.PaddingRight = parseBoxSide(val, style.FontSize)
	case "padding-bottom":
		style.PaddingBottom = parseBoxSide(val, style.FontSize)
	case "padding-left":
		style.PaddingLeft = parseBoxSide(val, style.FontSize)
	case "width":
		style.Width = parseLength(val)
	case "max-width":
		style.MaxWidth = parseLength(val)
	case "min-width":
		style.MinWidth = parseLength(val)
	case "height":
		style.Height = parseLength(val)
	case "border":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.BorderTopWidth = w
		style.BorderRightWidth = w
		style.BorderBottomWidth = w
		style.BorderLeftWidth = w
		style.BorderTopStyle = s
		style.BorderRightStyle = s
		style.BorderBottomStyle = s
		style.BorderLeftStyle = s
		style.BorderTopColor = clr
		style.BorderRightColor = clr
		style.BorderBottomColor = clr
		style.BorderLeftColor = clr
	case "border-width":
		w := parseBoxSide(val, style.FontSize)
		style.BorderTopWidth = w
		style.BorderRightWidth = w
		style.BorderBottomWidth = w
		style.BorderLeftWidth = w
	case "border-top-width":
		style.BorderTopWidth = parseBoxSide(val, style.FontSize)
	case "border-right-width":
		style.BorderRightWidth = parseBoxSide(val, style.FontSize)
	case "border-bottom-width":
		style.BorderBottomWidth = parseBoxSide(val, style.FontSize)
	case "border-left-width":
		style.BorderLeftWidth = parseBoxSide(val, style.FontSize)
	case "border-color":
		if c, ok := parseColor(val); ok {
			style.BorderTopColor = c
			style.BorderRightColor = c
			style.BorderBottomColor = c
			style.BorderLeftColor = c
		}
	case "border-style":
		style.BorderTopStyle = val
		style.BorderRightStyle = val
		style.BorderBottomStyle = val
		style.BorderLeftStyle = val
	case "flex-direction":
		style.FlexDirection = strings.TrimSpace(strings.ToLower(val))
	case "justify-content":
		style.JustifyContent = strings.TrimSpace(strings.ToLower(val))
	case "align-items":
		style.AlignItems = strings.TrimSpace(strings.ToLower(val))
	case "align-self":
		style.AlignSelf = strings.TrimSpace(strings.ToLower(val))
	case "flex-wrap":
		style.FlexWrap = strings.TrimSpace(strings.ToLower(val))
	case "flex":
		parseFlexShorthand(val, style)
	case "flex-flow":
		parseFlexFlowShorthand(val, style)
	case "flex-grow":
		if v, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			style.FlexGrow = v
		}
	case "flex-shrink":
		if v, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			style.FlexShrink = v
		}
	case "flex-basis":
		style.FlexBasis = parseLength(val)
	case "gap", "grid-gap":
		parts := strings.Fields(strings.TrimSpace(val))
		if len(parts) == 1 {
			v := parseBoxSide(parts[0], style.FontSize)
			style.Gap = v
			style.RowGap = v
			style.GridColumnGap = v
		} else if len(parts) >= 2 {
			style.RowGap = parseBoxSide(parts[0], style.FontSize)
			style.GridColumnGap = parseBoxSide(parts[1], style.FontSize)
			style.Gap = style.RowGap // flex compat: use row-gap value
		}
	case "row-gap":
		style.RowGap = parseBoxSide(val, style.FontSize)
	case "grid-template-columns":
		style.GridTemplateColumns = strings.TrimSpace(val)
	case "grid-template-rows":
		style.GridTemplateRows = strings.TrimSpace(val)
	case "grid-column":
		style.GridColumnStart, style.GridColumnEnd = parseGridLine(val)
	case "grid-row":
		style.GridRowStart, style.GridRowEnd = parseGridLine(val)
	case "grid-column-start":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			style.GridColumnStart = v
		}
	case "grid-column-end":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			style.GridColumnEnd = v
		}
	case "grid-row-start":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			style.GridRowStart = v
		}
	case "grid-row-end":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			style.GridRowEnd = v
		}
	case "grid-auto-flow":
		style.GridAutoFlow = strings.TrimSpace(strings.ToLower(val))
	case "grid-auto-rows":
		style.GridAutoRows = strings.TrimSpace(val)
	case "grid-template-areas":
		style.GridTemplateAreas = parseGridTemplateAreas(val)
	case "grid-area":
		style.GridArea = strings.TrimSpace(val)
	case "align-content":
		style.AlignContent = strings.TrimSpace(strings.ToLower(val))
	case "justify-items":
		style.JustifyItems = strings.TrimSpace(strings.ToLower(val))
	case "page-break-before", "break-before":
		v := strings.TrimSpace(strings.ToLower(val))
		switch v {
		case "always", "page":
			style.PageBreakBefore = "always"
		case "avoid", "avoid-page":
			style.PageBreakBefore = "avoid"
		case "auto":
			style.PageBreakBefore = "auto"
		}
	case "page-break-after", "break-after":
		v := strings.TrimSpace(strings.ToLower(val))
		switch v {
		case "always", "page":
			style.PageBreakAfter = "always"
		case "avoid", "avoid-page":
			style.PageBreakAfter = "avoid"
		case "auto":
			style.PageBreakAfter = "auto"
		}
	case "page-break-inside", "break-inside":
		v := strings.TrimSpace(strings.ToLower(val))
		switch v {
		case "avoid", "avoid-page":
			style.PageBreakInside = "avoid"
		case "auto":
			style.PageBreakInside = "auto"
		}
	case "orphans":
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
			style.Orphans = n
		}
	case "widows":
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
			style.Widows = n
		}
	case "list-style-type", "list-style":
		v := strings.TrimSpace(strings.ToLower(val))
		// Extract type from shorthand (list-style: disc inside).
		if parts := strings.Fields(v); len(parts) > 0 {
			style.ListStyleType = parts[0]
		}
	case "border-collapse":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "collapse" || v == "separate" {
			style.BorderCollapse = v
		}
	case "border-spacing":
		// Supports: "5px" (both) or "5px 10px" (horizontal vertical).
		parts := strings.Fields(strings.TrimSpace(val))
		if len(parts) == 1 {
			if l := parseLength(parts[0]); l != nil {
				v := l.toPoints(0, style.FontSize)
				style.BorderSpacingH = v
				style.BorderSpacingV = v
			}
		} else if len(parts) >= 2 {
			if lh := parseLength(parts[0]); lh != nil {
				style.BorderSpacingH = lh.toPoints(0, style.FontSize)
			}
			if lv := parseLength(parts[1]); lv != nil {
				style.BorderSpacingV = lv.toPoints(0, style.FontSize)
			}
		}
	case "vertical-align":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "top" || v == "middle" || v == "bottom" || v == "super" || v == "sub" || v == "baseline" || v == "text-top" || v == "text-bottom" {
			style.VerticalAlign = v
		}
	case "border-top":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.BorderTopWidth = w
		style.BorderTopStyle = s
		style.BorderTopColor = clr
	case "border-right":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.BorderRightWidth = w
		style.BorderRightStyle = s
		style.BorderRightColor = clr
	case "border-bottom":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.BorderBottomWidth = w
		style.BorderBottomStyle = s
		style.BorderBottomColor = clr
	case "border-left":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.BorderLeftWidth = w
		style.BorderLeftStyle = s
		style.BorderLeftColor = clr
	case "font":
		fs, fw, sz, lh, ff := parseFontShorthand(val, style.FontSize)
		if fs != "" {
			style.FontStyle = fs
		}
		if fw != "" {
			style.FontWeight = fw
		}
		if sz > 0 {
			style.FontSize = sz
		}
		if lh > 0 {
			style.LineHeight = lh
		}
		if ff != "" {
			style.FontFamily = ff
		}
	case "border-radius":
		parts := strings.Fields(val)
		switch len(parts) {
		case 1:
			if l := parseLength(parts[0]); l != nil {
				v := l.toPoints(0, style.FontSize)
				style.BorderRadius = v
				style.BorderRadiusTL = v
				style.BorderRadiusTR = v
				style.BorderRadiusBR = v
				style.BorderRadiusBL = v
			}
		case 2:
			tl := parseLengthPt(parts[0], style.FontSize)
			br := parseLengthPt(parts[1], style.FontSize)
			style.BorderRadiusTL = tl
			style.BorderRadiusBR = tl
			style.BorderRadiusTR = br
			style.BorderRadiusBL = br
			style.BorderRadius = tl
		case 3:
			tl := parseLengthPt(parts[0], style.FontSize)
			tr := parseLengthPt(parts[1], style.FontSize)
			bl := parseLengthPt(parts[2], style.FontSize)
			style.BorderRadiusTL = tl
			style.BorderRadiusTR = tr
			style.BorderRadiusBR = bl
			style.BorderRadiusBL = tr
			style.BorderRadius = tl
		case 4:
			style.BorderRadiusTL = parseLengthPt(parts[0], style.FontSize)
			style.BorderRadiusTR = parseLengthPt(parts[1], style.FontSize)
			style.BorderRadiusBR = parseLengthPt(parts[2], style.FontSize)
			style.BorderRadiusBL = parseLengthPt(parts[3], style.FontSize)
			style.BorderRadius = style.BorderRadiusTL
		}
	case "border-top-left-radius":
		style.BorderRadiusTL = parseLengthPt(val, style.FontSize)
	case "border-top-right-radius":
		style.BorderRadiusTR = parseLengthPt(val, style.FontSize)
	case "border-bottom-right-radius":
		style.BorderRadiusBR = parseLengthPt(val, style.FontSize)
	case "border-bottom-left-radius":
		style.BorderRadiusBL = parseLengthPt(val, style.FontSize)
	case "opacity":
		if v, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			style.Opacity = v
		}
	case "overflow":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "hidden" || v == "visible" || v == "auto" || v == "scroll" {
			style.Overflow = v
		}
	case "float":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "left" || v == "right" || v == "none" {
			style.Float = v
		}
	case "clear":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "left" || v == "right" || v == "both" || v == "none" {
			style.Clear = v
		}
	case "box-sizing":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "content-box" || v == "border-box" {
			style.BoxSizing = v
		}
	case "text-shadow":
		style.TextShadow = parseBoxShadow(strings.TrimSpace(strings.ToLower(val)), style.FontSize)
	case "visibility":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "visible" || v == "hidden" || v == "collapse" {
			style.Visibility = v
		}
	case "min-height":
		style.MinHeight = parseLength(val)
	case "max-height":
		style.MaxHeight = parseLength(val)
	case "position":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "static" || v == "relative" || v == "absolute" || v == "fixed" {
			style.Position = v
		}
	case "top":
		style.Top = parseLength(val)
	case "left":
		style.Left = parseLength(val)
	case "right":
		style.Right = parseLength(val)
	case "bottom":
		style.Bottom = parseLength(val)
	case "z-index":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			style.ZIndex = v
			style.ZIndexSet = true
		}

	// Box shadow (supports comma-separated multiple shadows)
	case "box-shadow":
		style.BoxShadows = parseBoxShadows(val, style.FontSize)

	// Text overflow
	case "text-overflow":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "clip" || v == "ellipsis" {
			style.TextOverflow = v
		}

	// Outline
	case "outline":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.OutlineWidth = w
		style.OutlineStyle = s
		style.OutlineColor = clr
	case "outline-width":
		style.OutlineWidth = parseBoxSide(val, style.FontSize)
	case "outline-style":
		style.OutlineStyle = strings.TrimSpace(strings.ToLower(val))
	case "outline-color":
		if c, ok := parseColor(val); ok {
			style.OutlineColor = c
		}
	case "outline-offset":
		style.OutlineOffset = parseBoxSide(val, style.FontSize)

	// CSS Columns
	case "column-count":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && v > 0 {
			style.ColumnCount = v
		}
	case "column-gap":
		v := parseBoxSide(val, style.FontSize)
		style.ColumnGap = v
		style.GridColumnGap = v
	case "column-width":
		if l := parseLength(val); l != nil {
			style.ColumnWidth = l.toPoints(0, style.FontSize)
		}
	case "columns":
		parts := strings.Fields(strings.TrimSpace(val))
		for _, p := range parts {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				style.ColumnCount = v
			} else if l := parseLength(p); l != nil {
				// In the columns shorthand, a length is column-width, not gap.
				style.ColumnWidth = l.toPoints(0, style.FontSize)
			}
		}

	case "column-rule":
		style.ColumnRuleWidth, style.ColumnRuleStyle, style.ColumnRuleColor = parseColumnRule(val, style.FontSize)
	case "column-rule-width":
		if l := parseLength(val); l != nil {
			style.ColumnRuleWidth = l.toPoints(0, style.FontSize)
		}
	case "column-rule-style":
		style.ColumnRuleStyle = strings.TrimSpace(strings.ToLower(val))
	case "column-rule-color":
		if c, ok := parseColor(val); ok {
			style.ColumnRuleColor = c
		}

	// CSS transforms
	case "transform":
		style.Transform = strings.TrimSpace(val)
	case "transform-origin":
		style.TransformOrigin = strings.TrimSpace(val)

	// Text decoration extensions
	case "text-decoration-color":
		if c, ok := parseColor(val); ok {
			style.TextDecorationColor = &c
		}
	case "text-decoration-style":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "solid" || v == "dashed" || v == "dotted" || v == "double" || v == "wavy" {
			style.TextDecorationStyle = v
		}

	// CSS counters
	case "counter-reset":
		style.CounterReset = parseCounterEntries(val, 0)
	case "counter-increment":
		style.CounterIncrement = parseCounterEntries(val, 1)

	// Object fit/position (images)
	case "object-fit":
		v := strings.TrimSpace(strings.ToLower(val))
		switch v {
		case "contain", "cover", "fill", "none", "scale-down":
			style.ObjectFit = v
		}
	case "object-position":
		style.ObjectPosition = strings.TrimSpace(strings.ToLower(val))

	// CSS bookmark properties
	case "bookmark-level":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && v >= 0 && v <= 6 {
			style.BookmarkLevel = v
			style.BookmarkLevelSet = true
		}
	case "bookmark-label":
		style.BookmarkLabel = strings.Trim(strings.TrimSpace(val), `"'`)

	// CSS string-set for running headers.
	// Format: string-set: name content() | string-set: name "literal"
	case "string-set":
		parts := strings.Fields(strings.TrimSpace(val))
		if len(parts) >= 2 {
			style.StringSetName = parts[0]
			style.StringSetValue = strings.Join(parts[1:], " ")
		}
	}
}

// generatePseudoElement creates a text element for ::before or ::after content.
func (c *converter) generatePseudoElement(text string, style computedStyle) layout.Element {
	stdFont, embFont := c.resolveFontPair(style)
	run := layout.TextRun{
		Text:            text,
		Font:            stdFont,
		Embedded:        embFont,
		FontSize:        style.FontSize,
		Color:           style.Color,
		Decoration:      style.TextDecoration,
		DecorationColor: style.TextDecorationColor,
		DecorationStyle: style.TextDecorationStyle,
		LetterSpacing:   style.LetterSpacing,
		WordSpacing:     style.WordSpacing,
		BaselineShift:   baselineShiftFromStyle(style),
		TextShadow:      textShadowFromStyle(style),
	}
	p := layout.NewStyledParagraph(run)
	p.SetAlign(style.TextAlign)
	p.SetLeading(style.LineHeight)
	return p
}

// parsePseudoContent extracts the text from a CSS content property value.
// Supports quoted strings, counter(name), counters(name, separator), and
// concatenation of the above. Returns empty string for unsupported values.
func (c *converter) parsePseudoContent(decls []cssDecl) string {
	for _, d := range decls {
		if d.property == "content" {
			val := strings.TrimSpace(d.value)
			if val == "none" || val == "" {
				return ""
			}
			return c.resolveContentValue(val)
		}
	}
	return ""
}

// resolveContentValue parses a CSS content value, resolving quoted strings,
// counter() and counters() function calls.
func (c *converter) resolveContentValue(val string) string {
	var result strings.Builder
	remaining := val
	for len(remaining) > 0 {
		remaining = strings.TrimSpace(remaining)
		if len(remaining) == 0 {
			break
		}
		// Quoted string.
		if remaining[0] == '"' || remaining[0] == '\'' {
			quote := remaining[0]
			end := strings.IndexByte(remaining[1:], quote)
			if end >= 0 {
				result.WriteString(remaining[1 : end+1])
				remaining = remaining[end+2:]
				continue
			}
			// Malformed quote — treat rest as literal.
			result.WriteString(remaining[1:])
			break
		}
		// counters() function — must check before counter() to avoid prefix match.
		if strings.HasPrefix(remaining, "counters(") {
			closeIdx := strings.IndexByte(remaining, ')')
			if closeIdx >= 0 {
				inner := remaining[len("counters("):closeIdx]
				parts := strings.SplitN(inner, ",", 2)
				name := strings.TrimSpace(parts[0])
				sep := "."
				if len(parts) > 1 {
					sep = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
				stack := c.counters[name]
				strs := make([]string, len(stack))
				for i, v := range stack {
					strs[i] = strconv.Itoa(v)
				}
				result.WriteString(strings.Join(strs, sep))
				remaining = remaining[closeIdx+1:]
				continue
			}
		}
		// counter() function.
		if strings.HasPrefix(remaining, "counter(") {
			closeIdx := strings.IndexByte(remaining, ')')
			if closeIdx >= 0 {
				name := strings.TrimSpace(remaining[len("counter("):closeIdx])
				result.WriteString(strconv.Itoa(c.getCounter(name)))
				remaining = remaining[closeIdx+1:]
				continue
			}
		}
		// Skip unknown token.
		spIdx := strings.IndexByte(remaining, ' ')
		if spIdx >= 0 {
			remaining = remaining[spIdx+1:]
		} else {
			break
		}
	}
	return result.String()
}

// parseCounterEntries parses a counter-reset or counter-increment value.
// defaultVal is the default value when no integer follows a name (0 for reset, 1 for increment).
func parseCounterEntries(val string, defaultVal int) []counterEntry {
	parts := strings.Fields(val)
	var entries []counterEntry
	for i := 0; i < len(parts); i++ {
		name := parts[i]
		if name == "none" {
			return nil
		}
		value := defaultVal
		if i+1 < len(parts) {
			if v, err := strconv.Atoi(parts[i+1]); err == nil {
				value = v
				i++ // skip the number
			}
		}
		entries = append(entries, counterEntry{Name: name, Value: value})
	}
	return entries
}

// resetCounter pushes a new counter value onto the stack for the given name.
func (c *converter) resetCounter(name string, value int) {
	c.counters[name] = append(c.counters[name], value)
}

// popCounter removes the most recently pushed counter for the given name.
// Called when leaving an element that did counter-reset to restore nesting.
func (c *converter) popCounter(name string) {
	stack := c.counters[name]
	if len(stack) > 0 {
		c.counters[name] = stack[:len(stack)-1]
	}
}

// incrementCounter adds value to the innermost counter for the given name.
// If no counter exists, auto-instantiates one at the document root per CSS spec.
func (c *converter) incrementCounter(name string, value int) {
	stack := c.counters[name]
	if len(stack) == 0 {
		// Auto-instantiate at document root per CSS spec.
		c.counters[name] = []int{value}
		return
	}
	stack[len(stack)-1] += value
}

// getCounter returns the current (innermost) value of the named counter.
func (c *converter) getCounter(name string) int {
	stack := c.counters[name]
	if len(stack) == 0 {
		return 0
	}
	return stack[len(stack)-1]
}

// parseTransform parses a CSS transform value like "rotate(45deg) scale(1.5)"
// into a slice of layout.TransformOp.

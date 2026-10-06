// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"sort"
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

	// Apply the HTML dir attribute. Per HTML spec, dir is a presentational
	// hint that maps to the CSS direction property. It can be overridden
	// by an explicit CSS direction declaration in the cascade below.
	if dir := getAttr(n, "dir"); dir != "" {
		switch strings.ToLower(dir) {
		case "rtl":
			style.Direction = layout.DirectionRTL
		case "ltr":
			style.Direction = layout.DirectionLTR
		case "auto":
			style.Direction = layout.DirectionAuto
		}
	}

	// Collect all CSS declarations from the matched stylesheet rules and
	// the element's inline style, then apply them in cascade tier order.
	// Per CSS Cascading Level 4 §6.4.4, the origin-and-importance tiers
	// for author-level declarations are, from lowest to highest precedence:
	//
	//   tier 0: author-normal (stylesheet normal)
	//   tier 1: author-inline-normal (inline normal — style="..." attribute)
	//   tier 2: author-important (stylesheet !important)
	//   tier 3: author-inline-important (inline !important)
	//
	// A stylesheet rule marked `!important` therefore beats a non-important
	// inline declaration, which is the opposite of the naive "inline always
	// wins" rule the converter used before #137. Within each tier, stylesheet
	// decls stay in the selector-specificity order matchingDeclarations
	// already produced, and inline decls stay in source order.
	//
	// Declarations are applied in two passes so that var() references
	// resolve against the fully-cascaded custom properties (CSS spec:
	// var() substitution happens at computed-value time). Without the
	// two-pass split, a stylesheet rule like
	//   .row { align-items: var(--ai); }
	// would resolve var(--ai) at apply-time using only the variables
	// known so far, silently ignoring a later inline --ai override.
	type pendingDecl struct {
		property  string
		value     string
		important bool
		inline    bool
	}
	var decls []pendingDecl
	if c.sheet != nil {
		for _, decl := range c.sheet.matchingDeclarations(n) {
			decls = append(decls, pendingDecl{
				property:  decl.property,
				value:     decl.value,
				important: decl.important,
				inline:    false,
			})
		}
	}
	if attr := getAttr(n, "style"); attr != "" {
		for _, decl := range splitDeclarations(attr) {
			prop, val, imp := splitDeclarationWithImportant(decl)
			if prop == "" || val == "" {
				continue
			}
			decls = append(decls, pendingDecl{
				property:  prop,
				value:     val,
				important: imp,
				inline:    true,
			})
		}
	}

	// Partition into the four cascade tiers. sort.SliceStable keeps the
	// within-tier order intact, so stylesheet specificity and inline
	// source order are preserved.
	tier := func(d pendingDecl) int {
		switch {
		case !d.important && !d.inline:
			return 0
		case !d.important && d.inline:
			return 1
		case d.important && !d.inline:
			return 2
		default:
			return 3
		}
	}
	sort.SliceStable(decls, func(i, j int) bool {
		return tier(decls[i]) < tier(decls[j])
	})

	// Pass 1: custom property declarations only. Their values populate
	// style.CustomProperties so subsequent var() lookups in pass 2 can
	// see them, regardless of where in the cascade they were declared.
	for _, d := range decls {
		if strings.HasPrefix(d.property, "--") {
			c.applyProperty(d.property, d.value, &style)
		}
	}
	// Pass 2: regular declarations. var() references are now resolved
	// against the fully-cascaded custom properties.
	for _, d := range decls {
		if !strings.HasPrefix(d.property, "--") {
			c.applyProperty(d.property, d.value, &style)
		}
	}

	// Re-resolve a standalone `line-height` against the element's FINAL
	// font-size. line-height:<length> is eagerly turned into a font-size
	// multiplier when its declaration is applied; if `font-size` is declared
	// after `line-height` in the same rule, that multiplier used a stale
	// font-size (e.g. `line-height:20px; font-size:11px` produced 1.25× — the
	// default 16px basis — instead of 20/11 = 1.82×, so the glyph rode high in
	// a line box that was too short). Resolving here makes the used line
	// height independent of declaration order. Multiplier/percentage/em/normal
	// values are font-size-independent, so this is a no-op for them.
	if style.lineHeightRaw != "" {
		style.LineHeight = parseLineHeight(style.lineHeightRaw, style.FontSize)
	}

	return style
}

// applyTagDefaults sets browser-like defaults for known HTML elements.
func (c *converter) applyTagDefaults(n *html.Node, style *computedStyle) {
	switch n.DataAtom {
	case atom.H1:
		style.FontSize = 24 // 32px * 0.75
		style.FontWeight = 700
		style.MarginTopLength = &cssLength{Value: 16.08, Unit: "pt"} // 0.67em at 32px → 32*0.67*0.75
		style.MarginBottomLength = &cssLength{Value: 16.08, Unit: "pt"}
	case atom.H2:
		style.FontSize = 18 // 24px * 0.75
		style.FontWeight = 700
		style.MarginTopLength = &cssLength{Value: 14.94, Unit: "pt"} // 0.83em at 24px → 24*0.83*0.75
		style.MarginBottomLength = &cssLength{Value: 14.94, Unit: "pt"}
	case atom.H3:
		style.FontSize = 14.04 // 18.72px * 0.75
		style.FontWeight = 700
		style.MarginTopLength = &cssLength{Value: 14.04, Unit: "pt"} // 1em at 18.72px → 18.72*0.75
		style.MarginBottomLength = &cssLength{Value: 14.04, Unit: "pt"}
	case atom.H4:
		style.FontSize = 12 // 16px * 0.75
		style.FontWeight = 700
		style.MarginTopLength = &cssLength{Value: 16.02, Unit: "pt"} // 1.33em at 16px → 16*1.33*0.75
		style.MarginBottomLength = &cssLength{Value: 16.02, Unit: "pt"}
	case atom.H5:
		style.FontSize = 9.96 // 13.28px * 0.75
		style.FontWeight = 700
		style.MarginTopLength = &cssLength{Value: 16.60, Unit: "pt"} // 1.67em at 13.28px → 13.28*1.67*0.75
		style.MarginBottomLength = &cssLength{Value: 16.60, Unit: "pt"}
	case atom.H6:
		style.FontSize = 8.01 // 10.72px * 0.75
		style.FontWeight = 700
		style.MarginTopLength = &cssLength{Value: 18.62, Unit: "pt"} // 2.33em at 10.72px → 10.72*2.33*0.75
		style.MarginBottomLength = &cssLength{Value: 18.62, Unit: "pt"}
	case atom.P:
		style.MarginTopLength = &cssLength{Value: 12, Unit: "pt"} // 1em at 16px → 16*0.75
		style.MarginBottomLength = &cssLength{Value: 12, Unit: "pt"}
	case atom.Span:
		// CSS 2.1 §9.2.2: <span> is inline by default. Without this,
		// the inherited Display="block" leaks through and walkChildren
		// treats <span> as a block sibling, producing one paragraph
		// per element instead of grouping text and inline elements
		// into a single anonymous block box.
		style.Display = "inline"
	case atom.Strong, atom.B:
		style.FontWeight = 700
		style.Display = "inline"
	case atom.Em, atom.I:
		style.FontStyle = "italic"
		style.Display = "inline"
	case atom.U:
		style.TextDecoration |= layout.DecorationUnderline
		style.Display = "inline"
	case atom.S, atom.Del:
		style.TextDecoration |= layout.DecorationStrikethrough
		style.Display = "inline"
	case atom.Mark:
		// Browser default: yellow highlight background.
		bg := layout.RGB(1, 1, 0)
		style.BackgroundColor = &bg
		style.Display = "inline"
	case atom.Small:
		style.FontSize = style.FontSize * 0.833
		style.Display = "inline"
	case atom.Sub:
		style.FontSize = style.FontSize * 0.75
		style.VerticalAlign = "sub"
		style.Display = "inline"
	case atom.Sup:
		style.FontSize = style.FontSize * 0.75
		style.VerticalAlign = "super"
		style.Display = "inline"
	case atom.Code:
		style.FontFamily = "courier"
		style.Display = "inline"
	case atom.Pre:
		style.FontFamily = "courier"
		style.WhiteSpace = "pre"
		style.MarginTopLength = &cssLength{Value: 12, Unit: "pt"}
		style.MarginBottomLength = &cssLength{Value: 12, Unit: "pt"}
	case atom.Hr:
		style.MarginTopLength = &cssLength{Value: 6, Unit: "pt"}
		style.MarginBottomLength = &cssLength{Value: 6, Unit: "pt"}
	case atom.A:
		style.Color = layout.RGB(0, 0, 0.933) // default link blue
		style.TextDecoration |= layout.DecorationUnderline
		style.Display = "inline"
	case atom.Table:
		// Browser UA defaults: no margins, separate borders, 2px spacing.
		// CSS 2.1 §17.6: border-collapse initial value is "separate".
		style.BorderSpacingH = 1.5 // 2px * 0.75
		style.BorderSpacingV = 1.5
	case atom.Ul, atom.Ol:
		style.MarginTopLength = &cssLength{Value: 12, Unit: "pt"}
		style.MarginBottomLength = &cssLength{Value: 12, Unit: "pt"}
	case atom.Blockquote:
		style.MarginTopLength = &cssLength{Value: 12, Unit: "pt"}
		style.MarginBottomLength = &cssLength{Value: 12, Unit: "pt"}
	case atom.Dl:
		style.MarginTopLength = &cssLength{Value: 12, Unit: "pt"}
		style.MarginBottomLength = &cssLength{Value: 12, Unit: "pt"}
	case atom.Dt:
		style.FontWeight = 700
	case atom.Dd:
		style.MarginLeftLength = &cssLength{Value: 30, Unit: "pt"} // browser default ~40px → 30pt
	case atom.Figure:
		style.MarginTopLength = &cssLength{Value: 12, Unit: "pt"}
		style.MarginBottomLength = &cssLength{Value: 12, Unit: "pt"}
	case atom.Figcaption:
		style.FontStyle = "italic"
		style.FontSize = style.FontSize * 0.9
	case atom.Fieldset:
		style.MarginTopLength = &cssLength{Value: 9, Unit: "pt"} // ~12px * 0.75
		style.MarginBottomLength = &cssLength{Value: 9, Unit: "pt"}
		style.Display = "block"
	case atom.Legend:
		style.FontWeight = 700
	case atom.Button:
		style.Display = "inline"
	case atom.Input, atom.Select, atom.Textarea:
		style.Display = "inline"
	case atom.Label:
		style.Display = "inline"
	case atom.Img, atom.Svg:
		// Replaced elements default to inline-level in browsers. Without
		// this override they inherit Display="block" from a parent <p>
		// (see inherit()), and collectRuns skips block-display children
		// — silently dropping inline <img>/<svg> from paragraph flow.
		// The top-level converter dispatch for Img/Svg runs before any
		// Display check, so the block-level path is unaffected.
		style.Display = "inline"
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
	// Custom properties (CSS variables) are stored as raw token
	// values. Their var() references are resolved lazily when read
	// by a non-custom property in pass 2 of computeElementStyle.
	// Storing raw values lets forward references like
	//   --b: var(--a);
	//   --a: blue;
	// resolve correctly: resolveVars is iterative and transitively
	// expands nested var() in the stored values. Eager resolution
	// here would freeze --b to the empty fallback because --a wasn't
	// declared yet at apply time.
	if strings.HasPrefix(prop, "--") {
		if style.CustomProperties == nil {
			style.CustomProperties = make(map[string]string)
		}
		style.CustomProperties[prop] = val
		return
	}

	// Resolve var() references on non-custom properties against the
	// fully-cascaded CustomProperties map.
	if strings.Contains(val, "var(") {
		val = resolveVars(val, style)
	}

	// All CSS properties are now handled via the registry. Unknown
	// properties are silently ignored, matching the legacy switch's
	// fallthrough behavior.
	if p, ok := cssPropByName[prop]; ok {
		p.Apply(style, val)
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
	p.SetAlign(resolveTextAlign(style))
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
				// Split on top-level commas only: the separator argument is a
				// quoted string that may itself contain commas (e.g.
				// counters(item, ", ")), so a naive split would corrupt it.
				parts := splitCounterArgs(inner)
				name := strings.TrimSpace(parts[0])
				sep := "."
				if len(parts) > 1 {
					sep = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
				style := ""
				if len(parts) > 2 {
					style = strings.TrimSpace(parts[2])
				}
				stack := c.counters[name]
				strs := make([]string, len(stack))
				for i, v := range stack {
					strs[i] = formatCounterValue(v, style)
				}
				result.WriteString(strings.Join(strs, sep))
				remaining = remaining[closeIdx+1:]
				continue
			}
		}
		// counter() function. Supports an optional list-style argument:
		// counter(<name>) or counter(<name>, <list-style>).
		if strings.HasPrefix(remaining, "counter(") {
			closeIdx := strings.IndexByte(remaining, ')')
			if closeIdx >= 0 {
				inner := remaining[len("counter("):closeIdx]
				name, style := inner, ""
				if comma := strings.IndexByte(inner, ','); comma >= 0 {
					name = inner[:comma]
					style = strings.TrimSpace(inner[comma+1:])
				}
				name = strings.TrimSpace(name)
				// `page` and `pages` are reserved counters whose value is
				// only known once pagination runs. Emit the same deferred
				// placeholder used by @page margin boxes; `layout` resolves
				// it during content-stream emission, applying the style.
				switch name {
				case "page":
					result.WriteString(layout.CounterPlaceholder("page", style))
				case "pages":
					result.WriteString(layout.CounterPlaceholder("pages", style))
				default:
					result.WriteString(formatCounterValue(c.getCounter(name), style))
				}
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

// splitCounterArgs splits a counters() argument list on top-level commas,
// ignoring commas inside single- or double-quoted strings so a quoted
// separator like ", " is preserved intact.
func splitCounterArgs(inner string) []string {
	var args []string
	var buf strings.Builder
	var quote byte
	for i := 0; i < len(inner); i++ {
		ch := inner[i]
		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			}
			buf.WriteByte(ch)
		case ch == '"' || ch == '\'':
			quote = ch
			buf.WriteByte(ch)
		case ch == ',':
			args = append(args, buf.String())
			buf.Reset()
		default:
			buf.WriteByte(ch)
		}
	}
	args = append(args, buf.String())
	return args
}

// formatCounterValue renders a counter value using a CSS list-style keyword.
// The style is matched case-insensitively; unknown keywords fall back to
// decimal. Roman/alpha conversion is shared with the layout package so list
// markers and counter() output stay consistent.
func formatCounterValue(n int, style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "", "decimal":
		return strconv.Itoa(n)
	case "decimal-leading-zero":
		if n >= 0 && n < 10 {
			return "0" + strconv.Itoa(n)
		}
		return strconv.Itoa(n)
	case "lower-roman":
		return layout.ToRoman(n, false)
	case "upper-roman":
		return layout.ToRoman(n, true)
	case "lower-alpha", "lower-latin":
		return layout.ToAlpha(n, 'a')
	case "upper-alpha", "upper-latin":
		return layout.ToAlpha(n, 'A')
	case "none":
		return ""
	default:
		return strconv.Itoa(n)
	}
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

// withImplicitListItemReset returns entries with an implicit "list-item"
// counter-reset of 0 prepended, matching the CSS Lists 3 §6.1 user-agent
// default (`ol, ul { counter-reset: list-item }`) that makes each list root
// number its own items independently, including nested lists restarting at
// 1. If the author already declared an explicit reset for "list-item",
// theirs wins and no implicit entry is added.
func withImplicitListItemReset(entries []counterEntry) []counterEntry {
	for _, e := range entries {
		if e.Name == "list-item" {
			return entries
		}
	}
	return append([]counterEntry{{Name: "list-item", Value: 0}}, entries...)
}

// withImplicitListItemIncrement returns entries with an implicit
// "list-item" counter-increment of 1 prepended, matching the CSS Lists 3
// §6.1 user-agent default (`li { counter-increment: list-item }`) — this is
// what makes `content: counter(list-item)` in `::marker` number items
// without any author counter-increment declaration. If the author already
// declared an explicit increment for "list-item", theirs wins.
func withImplicitListItemIncrement(entries []counterEntry) []counterEntry {
	for _, e := range entries {
		if e.Name == "list-item" {
			return entries
		}
	}
	return append([]counterEntry{{Name: "list-item", Value: 1}}, entries...)
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

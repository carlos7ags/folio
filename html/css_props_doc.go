// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"sort"
	"strings"
)

// RenderCSSPropertiesMarkdown produces the contents of docs/CSS_SUPPORT.md
// from the cssProperties registry. The output is grouped by Category in
// a fixed order, then alphabetized within each category. Properties in
// the registry are the ground truth — any change to cssProperties
// regenerates the doc on the next `go generate`.
//
// The function is exported so that internal/gen-css-docs/main.go can
// invoke it without importing unexported internals. It is otherwise not
// part of Folio's public API contract — callers outside the doc
// generator should not rely on its output format.
func RenderCSSPropertiesMarkdown() string {
	var b strings.Builder

	b.WriteString("# Folio CSS support\n\n")
	b.WriteString("> Auto-generated from `html/css_props.go`, `html/css.go`, `html/css_selectors.go`, and the function parsers in `html/`. Do not edit by hand.\n")
	b.WriteString("> Run `go generate ./html/...` to regenerate after changing the registry.\n\n")
	b.WriteString("Folio's HTML-to-PDF converter recognizes the CSS properties listed below.\n")
	b.WriteString("Properties not in this document are silently ignored at render time.\n\n")

	// Orientation paragraph: tells a first-time evaluator how to read
	// the rest of the document.
	b.WriteString("## How to read this document\n\n")
	b.WriteString("Each per-category table lists the property name, any alternative names\n")
	b.WriteString("(aliases) accepted by the parser, the value forms that are recognized,\n")
	b.WriteString("and any notes about parsing or interactions with other properties.\n\n")
	b.WriteString("Value forms are written in CSS spec shorthand: `<length>` means a\n")
	b.WriteString("length value (e.g. `12px`, `1em`, `0.5in`); `<color>` means any\n")
	b.WriteString("supported color form (named, hex, rgb/rgba, hsl/hsla, cmyk); and so on.\n")
	b.WriteString("See [Value-form glossary](#value-form-glossary) below for the full list.\n\n")
	b.WriteString("If you don't see a property here, Folio's parser silently ignores it\n")
	b.WriteString("at render time — there is no warning. Use\n")
	b.WriteString("[`html.Options.StrictAssets`](../html) to escalate certain asset failures,\n")
	b.WriteString("but unknown CSS properties are always silent.\n\n")

	// Sort properties by Category, then by Name. Categories appear in a
	// fixed display order so the doc is stable across runs.
	categoryOrder := []string{
		"Typography",
		"Color",
		"Backgrounds",
		"BoxModel",
		"Borders",
		"Layout",
		"Flexbox",
		"Grid",
		"MultiColumn",
		"Tables",
		"Pagination",
		"Lists",
		"Effects",
		"PDF",
	}

	byCat := make(map[string][]cssProperty)
	for _, p := range cssProperties {
		byCat[p.Category] = append(byCat[p.Category], p)
	}
	for cat := range byCat {
		sort.Slice(byCat[cat], func(i, j int) bool {
			return byCat[cat][i].Name < byCat[cat][j].Name
		})
	}

	// Summary count.
	b.WriteString("## At a glance\n\n")
	b.WriteString("| Category | Properties |\n")
	b.WriteString("|---|---:|\n")
	totalCount := 0
	for _, cat := range categoryOrder {
		entries := byCat[cat]
		if len(entries) == 0 {
			continue
		}
		fmt.Fprintf(&b, "| %s | %d |\n", cat, len(entries))
		totalCount += len(entries)
	}
	fmt.Fprintf(&b, "| **Total** | **%d** |\n\n", totalCount)

	// Value-form glossary — disambiguates the angle-bracket placeholders
	// that appear in per-property "Accepted values" columns.
	b.WriteString("## Value-form glossary\n\n")
	b.WriteString("Angle-bracket placeholders used in the per-property tables below.\n\n")
	b.WriteString("| Placeholder | Meaning |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `<length>` | A CSS length: `<number><unit>` where unit is `px`, `pt`, `em`, `rem`, `cm`, `mm`, or `in`. Examples: `12px`, `1.5em`, `0.5in`. Distinct from `<percentage>`, which is listed separately as an alternative in per-property tables. |\n")
	b.WriteString("| `<percentage>` | A `<number>%`. Resolves against the containing context (line-height, parent dimension, etc.). |\n")
	b.WriteString("| `<number>` | A unitless real number, e.g. `1.5`, `0.7`, `-2`. |\n")
	b.WriteString("| `<integer>` | A whole number, e.g. `0`, `5`, `-1`. Range constraints (e.g. `<integer 1..6>`) are listed in the per-property table. |\n")
	b.WriteString("| `<string>` | A quoted text literal, e.g. `\"My Title\"` or `'caption'`. |\n")
	b.WriteString("| `<color>` | Any of: `<named>` (`red`, `transparent`), `<hex>` (`#abc`, `#aabbcc`), `rgb()`, `rgba()`, `hsl()`, `hsla()`, `cmyk()`. Folio renders sRGB only — `oklch()` and `color-mix()` are not supported. |\n")
	b.WriteString("| `<named>`, `<hex>` | Component forms of `<color>`: `<named>` is a CSS named color (`red`, `aliceblue`, etc.); `<hex>` is `#RGB`, `#RGBA`, `#RRGGBB`, or `#RRGGBBAA`. |\n")
	b.WriteString("| `<line-width>` | A `<length>` or one of the keywords `thin`, `medium`, `thick`. Used in border/outline shorthands. |\n")
	b.WriteString("| `<line-style>` | One of `solid`, `dashed`, `dotted`, `double`, `none`. |\n")
	b.WriteString("| `<position>` | A 1- or 2-component position keyword/length. Examples: `center`, `top right`, `50% 25%`, `10px 20px`. Applies to `background-position`, `object-position`, `transform-origin`. |\n")
	b.WriteString("| `<grid-line>` | A grid line reference: an integer (e.g. `2`), a `span` keyword (`span 3`), or a named line (rare; line names not yet supported). |\n")
	b.WriteString("| `<track-list>` | A space-separated list of track sizes for `grid-template-columns`/`-rows`. Examples: `1fr 1fr`, `100px auto`, `repeat(3, 1fr)`. |\n")
	b.WriteString("| `<track-size>` | A single grid track size: `<length>`, `<percentage>`, `<number>fr`, `auto`, `min-content`, `max-content`. |\n")
	b.WriteString("| `<ratio>` | An aspect ratio expressed as `<number>/<number>` or a single `<number>`. Example: `16/9`. |\n")
	b.WriteString("| `<gradient>` | `linear-gradient(...)`, `repeating-linear-gradient(...)`, `radial-gradient(...)`, or `repeating-radial-gradient(...)`. |\n")
	b.WriteString("| `<transform-function>` | A CSS transform: `translate()`, `translateX()`/`Y()`, `rotate()`, `scale()`/`X()`/`Y()`, `skew()`/`X()`/`Y()`. |\n")
	b.WriteString("| `<offset-x>`, `<offset-y>`, `<blur>`, `<spread>` | Component lengths in shadow shorthands (`box-shadow`, `text-shadow`). All are `<length>`; spread accepts negatives to inset the shadow. |\n")
	b.WriteString("| `<identifier>` | A custom name, e.g. for `counter-reset` or `string-set`. |\n")
	b.WriteString("| `<absolute-size>` | A CSS font absolute size keyword: `xx-small`, `x-small`, `small`, `medium`, `large`, `x-large`, `xx-large`. |\n")
	b.WriteString("| `<relative-size>` | A CSS font relative size keyword: `smaller`, `larger`. Resolves relative to the parent's font size. |\n")
	b.WriteString("| `<font-size>` | A `<length>` or `<percentage>` for font-size. Percentages resolve against the parent's font size. |\n")
	b.WriteString("| `<font-weight>` | A `<number>` (100–900) or keyword: `normal` (400), `bold` (700), `lighter`, `bolder`. |\n")
	b.WriteString("| `<font-family>` | A font family name, optionally quoted. Multiple families can be comma-separated. |\n")
	b.WriteString("| `<line-height>` | A `<number>`, `<length>`, or `<percentage>` for line-height. Unitless values are multiplied by font-size. |\n")
	b.WriteString("| `<column-gap>` | A `<length>` or `<percentage>` for column-gap/row-gap. `normal` uses the browser default (1em for multi-column). |\n")
	b.WriteString("| `<flex-basis>` | A flex-basis value: `<length>`, `<percentage>`, or `auto`. |\n")
	b.WriteString("| `<flex-shrink>` | A unitless `<number>` for flex-shrink (default 1). |\n")
	b.WriteString("| `<integer 100..900>` | A font-weight integer in the range 100–900. |\n")
	b.WriteString("\n")
	b.WriteString("**`calc()`, `min()`, `max()`, `clamp()`** are accepted everywhere a `<length>` or `<percentage>` is. The parser preserves them as single tokens through shorthand splitting.\n\n")

	// Box-alignment cross-context callout. align-items / justify-content
	// etc. are listed under Flexbox in the per-category tables, but in
	// CSS 3 Box Alignment they apply to Grid containers too. Note this
	// once at the top of the doc to avoid confusion.
	b.WriteString("## Box-alignment properties\n\n")
	b.WriteString("`justify-content`, `align-items`, `align-self`, and `align-content` are\n")
	b.WriteString("listed under Flexbox or Grid in the per-category tables for grouping,\n")
	b.WriteString("but per CSS Box Alignment Level 3 they apply to BOTH flex and grid\n")
	b.WriteString("containers. Folio honors them in either context.\n\n")
	b.WriteString("Similarly, `gap` (and its alias `grid-gap`) is grouped under Grid but\n")
	b.WriteString("also takes effect on flex containers as the gap between items.\n\n")

	// Per-category tables.
	for _, cat := range categoryOrder {
		entries := byCat[cat]
		if len(entries) == 0 {
			continue
		}
		b.WriteString("## ")
		b.WriteString(cat)
		b.WriteString("\n\n")
		b.WriteString("| Property | Aliases | Accepted values | Notes |\n")
		b.WriteString("|---|---|---|---|\n")
		for _, p := range entries {
			aliases := "—"
			if len(p.Aliases) > 0 {
				aliases = "`" + strings.Join(p.Aliases, "`, `") + "`"
			}
			values := "—"
			if len(p.Values) > 0 {
				escaped := make([]string, len(p.Values))
				for i, v := range p.Values {
					escaped[i] = "`" + v + "`"
				}
				values = strings.Join(escaped, ", ")
			}
			notes := p.Notes
			if notes == "" {
				notes = "—"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", p.Name, aliases, values, notes)
		}
		b.WriteString("\n")
	}

	// Selectors — hand-curated section. ~33 entries already centralized
	// in css_selectors.go; a registry-driven version was considered, but
	// for a doc-only payoff a hand-written section + drift-guard test
	// is the lower-friction option. TestSelectorsDocCoverage maintains
	// the static expected list.
	b.WriteString("## Selectors\n\n")
	b.WriteString("CSS selectors recognized by Folio's stylesheet parser. Selectors not\n")
	b.WriteString("listed here are silently dropped at parse time — the rule's declarations\n")
	b.WriteString("never apply to any element.\n\n")
	b.WriteString("### Combinators\n\n")
	b.WriteString("| Combinator | Example | Meaning |\n")
	b.WriteString("|---|---|---|\n")
	b.WriteString("| descendant (space) | `article p` | `p` anywhere inside `article`. |\n")
	b.WriteString("| `>` | `ul > li` | Direct child only. |\n")
	b.WriteString("| `+` | `h2 + p` | Immediately-following sibling. |\n")
	b.WriteString("| `~` | `h2 ~ p` | Any later sibling. |\n\n")
	b.WriteString("### Simple selectors\n\n")
	b.WriteString("| Selector | Example | Notes |\n")
	b.WriteString("|---|---|---|\n")
	b.WriteString("| Type | `p`, `h1` | Element name match. |\n")
	b.WriteString("| Class | `.note` | Matches elements whose `class` attribute contains the name. Multiple classes can be chained: `.note.warning`. |\n")
	b.WriteString("| ID | `#title` | Matches the element with the given `id`. |\n")
	b.WriteString("| Universal | `*` | Matches every element. |\n")
	b.WriteString("| Attribute | `[lang]`, `[lang=\"en\"]` | See attribute operators below. |\n\n")
	b.WriteString("Selectors compose: `article.featured > p.lead` matches a `p` with class `lead` that is a direct child of an `article` with class `featured`.\n\n")
	b.WriteString("### Attribute operators\n\n")
	b.WriteString("| Operator | Example | Matches when... |\n")
	b.WriteString("|---|---|---|\n")
	b.WriteString("| presence | `[hidden]` | Attribute is present (any value, including empty). |\n")
	b.WriteString("| `=` | `[type=\"submit\"]` | Attribute value equals the operand exactly. |\n")
	b.WriteString("| `^=` | `[href^=\"https://\"]` | Value starts with the operand. |\n")
	b.WriteString("| `$=` | `[src$=\".pdf\"]` | Value ends with the operand. |\n")
	b.WriteString("| `*=` | `[class*=\"btn\"]` | Value contains the operand as a substring. |\n")
	b.WriteString("| `~=` | `[rel~=\"author\"]` | Value, treated as a whitespace-separated list, contains the operand as a whole word. |\n")
	b.WriteString("| `|=` | `[lang|=\"en\"]` | Value equals the operand or starts with `operand-`. |\n\n")
	b.WriteString("Case-sensitivity flags (`[lang=\"EN\" i]`) are not parsed.\n\n")
	b.WriteString("### Pseudo-classes\n\n")
	b.WriteString("| Pseudo-class | Notes |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `:root` | The document root (`<html>`). |\n")
	b.WriteString("| `:empty` | Element with no element children and no non-empty text nodes. |\n")
	b.WriteString("| `:first-child` | First child of its parent. |\n")
	b.WriteString("| `:last-child` | Last child of its parent. |\n")
	b.WriteString("| `:nth-child(<expr>)` | Position match. `<expr>` accepts `odd`, `even`, an integer, or `An+B` form (e.g. `2n+1`, `3n`, `-n+3`). |\n")
	b.WriteString("| `:nth-last-child(<expr>)` | Same as `:nth-child` but counted from the end. |\n")
	b.WriteString("| `:first-of-type` | First element of its tag type among siblings. |\n")
	b.WriteString("| `:last-of-type` | Last element of its tag type among siblings. |\n")
	b.WriteString("| `:nth-of-type(<expr>)` | Position match restricted to the element's tag type. |\n")
	b.WriteString("| `:nth-last-of-type(<expr>)` | Same, counted from the end. |\n")
	b.WriteString("| `:not(<simple>)` | Negation. Argument is a single simple selector — selector lists inside `:not()` are not parsed. |\n")
	b.WriteString("| `:is(<list>)` | Matches if any selector in the comma-separated list matches. Specificity follows the highest-specificity argument. |\n")
	b.WriteString("| `:where(<list>)` | Same matching as `:is()` but contributes zero specificity. |\n\n")
	b.WriteString("Interaction-state pseudo-classes (`:hover`, `:focus`, `:active`, `:visited`, `:target`, `:checked`, `:disabled`) are not supported — PDFs are static.\n\n")
	b.WriteString("### Pseudo-elements\n\n")
	b.WriteString("| Pseudo-element | Notes |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `::before` | Inserts generated content before the element. Driven by the `content` declaration. |\n")
	b.WriteString("| `::after` | Inserts generated content after the element. |\n")
	b.WriteString("| `::marker` | Styles the list marker on `<li>` elements (`color`, `font-size`, etc.). |\n")
	b.WriteString("| `::placeholder` | Styles the placeholder text on form fields. |\n\n")
	b.WriteString("The double-colon form is required — single-colon legacy forms (`:before`, `:after`) are not recognized. `::first-letter`, `::first-line`, `::selection`, `::backdrop` are not supported.\n\n")

	// At-rules — hand-curated section. The set of @-rules Folio
	// recognizes is small and changes rarely; a parallel registry would
	// be more bookkeeping than the doc payoff justifies. The
	// TestAtRulesDocCoverage CI guard parses html/css.go and asserts
	// every @-prefixed string literal in parseCSS is mentioned here, so
	// new at-rule support cannot land without updating this section.
	b.WriteString("## At-rules\n\n")
	b.WriteString("CSS at-rules recognized by Folio's stylesheet parser. Anything not listed here\n")
	b.WriteString("is silently dropped during parsing — there is no warning.\n\n")
	b.WriteString("| Rule | Selectors / context | Notes |\n")
	b.WriteString("|---|---|---|\n")
	b.WriteString("| `@font-face` | — | Declares a custom font face. Recognized descriptors: `font-family`, `src`, `font-weight`, `font-style`. The `format()` annotation in `src` is advisory; Folio inspects the URL contents to determine format (WOFF1, TTF, TTC). WOFF2 is not supported. |\n")
	b.WriteString("| `@page` | `:first`, `:left`, `:right`, no selector | Page-level styling: page size, margins, and nested margin boxes. Pseudo-selectors target the first page or left/right pages in a duplex flow. |\n")
	b.WriteString("| `@page` margin boxes | `@top-left`, `@top-center`, `@top-right`, `@bottom-left`, `@bottom-center`, `@bottom-right` | Running headers/footers, declared inside an `@page` block. Populate via static `content`, `string()`, or `counter(page)`. The four corner boxes (`@top-left-corner`, etc.) and the `@left-*` / `@right-*` boxes are not interpreted. |\n")
	b.WriteString("| `@supports` | `(<property>: <value>)`, `not (...)`, `and`, `or` | Feature query. Inner rules are parsed only if the condition evaluates true against Folio's actual support — useful for shipping fallbacks alongside Folio-specific styling. |\n")
	b.WriteString("| `@media print` | — | Treated as unconditional (PDF is a print medium). Inner rules are parsed as if at the top level. Other `@media` queries are silently discarded; see below. |\n\n")
	b.WriteString("### Silently ignored at-rules\n\n")
	b.WriteString("Listed for evaluators migrating from a browser-based renderer. None of\n")
	b.WriteString("these produce a warning — the rule and its body are dropped during parsing.\n\n")
	b.WriteString("| Rule | Why |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `@media screen`, `@media (max-width: ...)`, etc. | Only `@media print` is interpreted; PDF output has fixed page geometry, so viewport breakpoints have no analogue. |\n")
	b.WriteString("| `@import` | External stylesheet imports are not followed during CSS parsing. Use `<link rel=\"stylesheet\">` in the HTML instead — those are loaded through the asset resolver. |\n")
	b.WriteString("| `@keyframes`, `@-webkit-keyframes` | PDF has no animation timeline. |\n")
	b.WriteString("| `@counter-style` | Custom list counter styles are not parsed; only the keywords listed under `list-style-type` are recognized. |\n")
	b.WriteString("| `@namespace`, `@charset` | Not interpreted. |\n")
	b.WriteString("| `@layer`, `@scope`, `@container`, `@property` | Newer CSS spec features; not interpreted. |\n\n")

	// Functions — hand-curated section, same rationale as At-rules. The
	// set of functional values Folio recognizes is small (~30 across
	// math/color/gradients/content/transforms/url) and spread across
	// half a dozen parsers, so a function-dispatch registry would be a
	// substantial refactor for a doc-only payoff. TestFunctionsDocCoverage
	// is the drift guard: it maintains a static list of the function
	// names that this section claims to document and asserts each is
	// referenced here AND that the relevant parser actually recognizes
	// the form.
	b.WriteString("## Functions\n\n")
	b.WriteString("CSS functional values recognized by Folio's parsers, grouped by category.\n")
	b.WriteString("Functions not listed here pass through as opaque text and almost always\n")
	b.WriteString("cause the containing declaration to be discarded.\n\n")
	b.WriteString("### Math\n\n")
	b.WriteString("Accepted everywhere a `<length>` or `<percentage>` is expected.\n")
	b.WriteString("Folio's parser preserves these as single tokens through shorthand splitting,\n")
	b.WriteString("so they survive inside `margin`, `padding`, `flex`, `transform()`, etc.\n\n")
	b.WriteString("| Function | Notes |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `calc()` | Supports `+`, `-`, `*`, `/` with operator precedence and nested parentheses. Mixed units (e.g. `calc(100% - 20px)`) resolve at layout time. |\n")
	b.WriteString("| `min()` | Comma-separated argument list. Returns the smallest resolved value. |\n")
	b.WriteString("| `max()` | Comma-separated argument list. Returns the largest resolved value. |\n")
	b.WriteString("| `clamp()` | `clamp(<min>, <preferred>, <max>)`. |\n\n")
	b.WriteString("Known limitations: `calc()` does not yet expand inside `rotate()`, `scale()`, `skew()`, `background-position`, or `linear-gradient()` color stops — see issues #265, #266, #274, #275.\n\n")
	b.WriteString("### Color\n\n")
	b.WriteString("Accepted everywhere a `<color>` is expected. Output is sRGB regardless of input form.\n\n")
	b.WriteString("| Function | Notes |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `rgb()` | `rgb(R, G, B)` or `rgb(R G B)`. Components are 0-255 integers or 0-100% percentages. |\n")
	b.WriteString("| `rgba()` | `rgba(R, G, B, A)`. Alpha is 0-1 or 0-100%. |\n")
	b.WriteString("| `hsl()` | `hsl(H, S%, L%)`. Hue in degrees. |\n")
	b.WriteString("| `hsla()` | `hsla(H, S%, L%, A)`. |\n")
	b.WriteString("| `cmyk()` / `device-cmyk()` | `cmyk(C, M, Y, K)` with components as 0-1 or 0-100%. Folio converts to sRGB for raster compositing; the original CMYK is preserved in the PDF color space for print pipelines. |\n\n")
	b.WriteString("Known unsupported color functions: `oklch()`, `oklab()`, `lch()`, `lab()`, `color-mix()`, `color()` — see [Known unsupported features](#known-unsupported-features) for workarounds.\n\n")
	b.WriteString("### Gradients\n\n")
	b.WriteString("Accepted as `background-image` values.\n\n")
	b.WriteString("| Function | Notes |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `linear-gradient()` | Direction (`to right`, `45deg`, etc.) plus 2+ `<color>` stops. |\n")
	b.WriteString("| `repeating-linear-gradient()` | Same syntax; tiles the gradient pattern. |\n")
	b.WriteString("| `radial-gradient()` | Shape (`circle`, `ellipse`), size, and `<color>` stops. |\n")
	b.WriteString("| `repeating-radial-gradient()` | Same syntax; tiles the gradient pattern. |\n\n")
	b.WriteString("`conic-gradient()` is not supported.\n\n")
	b.WriteString("### Content and counters\n\n")
	b.WriteString("Used in `string-set`, `bookmark-label`, `content`, and `@page` margin boxes.\n\n")
	b.WriteString("| Function | Notes |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `var()` | CSS custom property reference. Supports a fallback as the second argument: `var(--c, #000)`. Resolved BEFORE per-property dispatch, so functions and gradients receive resolved values. |\n")
	b.WriteString("| `attr()` | Reads an HTML attribute. Used in `bookmark-label`. |\n")
	b.WriteString("| `content()` | Substitutes the element's text content. Used in `string-set` and `bookmark-label`. |\n")
	b.WriteString("| `counter()` | `counter(<name>)` or `counter(<name>, <list-style>)`. Page counter `counter(page)` is supported in `@page` margin boxes. |\n")
	b.WriteString("| `counters()` | `counters(<name>, <separator>)` for nested counter chains. |\n")
	b.WriteString("| `string()` | Reads the latest value of a named string set via `string-set` (used in running headers). |\n\n")
	b.WriteString("Known unsupported: `target-counter()` for cross-references — tracked as #222.\n\n")
	b.WriteString("### Transform\n\n")
	b.WriteString("Used in `transform`. Multiple functions compose in the listed order.\n\n")
	b.WriteString("| Function | Notes |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `translate()` | `translate(<tx>)` or `translate(<tx>, <ty>)`. Lengths in any supported unit; bare numbers treated as px. |\n")
	b.WriteString("| `translateX()` | Single `<length>` argument. |\n")
	b.WriteString("| `translateY()` | Single `<length>` argument. |\n")
	b.WriteString("| `rotate()` | Single `<angle>`: `deg`, `rad`, `grad`, `turn`, or bare number (degrees). |\n")
	b.WriteString("| `scale()` | `scale(<s>)` (uniform) or `scale(<sx>, <sy>)`. |\n")
	b.WriteString("| `scaleX()` | Single `<number>` argument. |\n")
	b.WriteString("| `scaleY()` | Single `<number>` argument. |\n")
	b.WriteString("| `skew()` | `skew(<ax>)` or `skew(<ax>, <ay>)`. |\n")
	b.WriteString("| `skewX()` | Single `<angle>` argument. |\n")
	b.WriteString("| `skewY()` | Single `<angle>` argument. |\n\n")
	b.WriteString("Known unsupported: `matrix()`, `matrix3d()`, `translate3d()`, `rotate3d()`, `scale3d()`, `perspective()` — Folio renders 2D only.\n\n")
	b.WriteString("### Other\n\n")
	b.WriteString("| Function | Notes |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `url()` | Used in `background-image`, `@font-face` `src`, and asset references. Resolves through Folio's asset loader (BaseFS or HTTP via Client, subject to `Options.URLPolicy`). |\n\n")

	// Known unsupported list — hardcoded for now; future work could
	// derive this from a separate registry.
	b.WriteString("## Known unsupported features\n\n")
	b.WriteString("These properties / values are commonly requested but NOT recognized by Folio.\n")
	b.WriteString("Folio silently ignores unknown property names, so a stylesheet that uses\n")
	b.WriteString("any of these will render — just without the styling those declarations\n")
	b.WriteString("would have applied in a browser.\n\n")
	b.WriteString("| Feature | Why | Workaround |\n")
	b.WriteString("|---|---|---|\n")
	b.WriteString("| `oklch()`, `oklab()`, `lch()`, `lab()` color | Folio renders sRGB only; no ICC profile support. | Precompute the sRGB equivalent and use `#hex` or `rgb()`. |\n")
	b.WriteString("| `color-mix()` | Folio's parser doesn't expand the function. | Precompute the mixed color, or assign it to a CSS variable: `--btn-tint: #c44;`. |\n")
	b.WriteString("| `-webkit-line-clamp` / `line-clamp` | PDFs are paginated, not scrollable; the property has no analogue. | Truncate before HTML emission, or use `layout.Paragraph.SplitAfterLine` for first-N-lines-plus-appendix flows. |\n")
	b.WriteString("| `text-wrap: pretty` / `text-wrap: balance` | Browser-only line-break heuristic; cosmetic. | Render without it. |\n")
	b.WriteString("| `filter`, `backdrop-filter`, `mix-blend-mode` | PDF lacks an analogue for screen-compositing. | Pre-bake effects into images. |\n")
	b.WriteString("| `:hover`, `:focus`, `:active` | PDF has no interaction state. | Style the static state directly. |\n")
	b.WriteString("| Custom HTML elements / Web Components | Folio's HTML parser handles a fixed element set. | Pre-render to a known element (`<div>` / `<span>`) before passing to Folio. |\n")
	b.WriteString("| `position: sticky` | Has no analogue in paginated layout. | Use `@page` running headers/footers via margin boxes. |\n")
	b.WriteString("| ICC profiles for color management | Folio is sRGB-only. | Use sRGB-correct hex values; convert assets to sRGB before embedding. |\n")
	b.WriteString("\n")

	// Final orientation: how to extend the registry.
	b.WriteString("## Adding a new CSS property\n\n")
	b.WriteString("1. Append a `cssProperty` entry to `cssProperties` in `html/css_props.go`.\n")
	b.WriteString("   Required: `Name` and `Apply`. Recommended: `Category`, `Values`, `Notes`.\n")
	b.WriteString("2. Run `go generate ./html/...` to regenerate this document.\n")
	b.WriteString("3. Add at least one row to `TestCSSPropertyParitySnapshot` in\n")
	b.WriteString("   `html/css_props_test.go` asserting the new property's behavior.\n")
	b.WriteString("4. CI guards: `TestCSSDocsInSync` ensures the doc matches the registry,\n")
	b.WriteString("   and `TestNoSwitchRegistryOverlap` ensures no legacy switch case is\n")
	b.WriteString("   reintroduced for a registered property.\n")

	return b.String()
}

//go:generate go run ../internal/gen-css-docs

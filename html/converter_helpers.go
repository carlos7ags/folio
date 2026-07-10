// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// resolveFont maps a computedStyle's family/weight/style to a standard PDF font.
//
// Standard PDF-14 families ship Regular and Bold variants only — there
// is no SemiBold or Light. Per CSS Fonts L4 §5.2's synthetic-bolding
// guidance for missing weights, weights ≥ 600 round to Bold and
// weights < 600 round to Regular. Documents that need finer granularity
// must declare an `@font-face` for the desired weight; that path uses
// nearest-weight matching in resolveFontPair.
func resolveFont(style computedStyle) *font.Standard {
	weight := style.FontWeight
	if weight == 0 {
		weight = 400
	}
	bold := weight >= 600
	italic := style.FontStyle == "italic"

	switch mapToStandardFamily(style.FontFamily) {
	case "courier":
		switch {
		case bold && italic:
			return font.CourierBoldOblique
		case bold:
			return font.CourierBold
		case italic:
			return font.CourierOblique
		default:
			return font.Courier
		}
	case "times":
		switch {
		case bold && italic:
			return font.TimesBoldItalic
		case bold:
			return font.TimesBold
		case italic:
			return font.TimesItalic
		default:
			return font.TimesRoman
		}
	default: // "helvetica"
		switch {
		case bold && italic:
			return font.HelveticaBoldOblique
		case bold:
			return font.HelveticaBold
		case italic:
			return font.HelveticaOblique
		default:
			return font.Helvetica
		}
	}
}

// resolveFontPair returns either a standard font or an embedded font for
// the given style. When the requested family has one or more matching
// @font-face declarations, the closest face is selected per CSS Fonts L4
// §5.2's nearest-weight algorithm; otherwise resolveFont returns the
// standard PDF-14 fallback.
func (c *converter) resolveFontPair(style computedStyle) (*font.Standard, *font.EmbeddedFont) {
	if len(c.embeddedFonts) > 0 {
		family := strings.ToLower(style.FontFamily)
		fontStyle := style.FontStyle
		if fontStyle == "" {
			fontStyle = "normal"
		}
		desired := style.FontWeight
		if desired == 0 {
			desired = 400
		}
		if ef := c.matchEmbeddedFont(family, fontStyle, desired); ef != nil {
			return nil, ef
		}
		// Style mismatch fallback: try the same family at the requested
		// weight but with the opposite style. Better to render in
		// regular Inter than fall through to Helvetica when the author
		// asked for italic Inter.
		altStyle := "normal"
		if fontStyle == "normal" {
			altStyle = "italic"
		}
		if ef := c.matchEmbeddedFont(family, altStyle, desired); ef != nil {
			return nil, ef
		}
	}
	return resolveFont(style), nil
}

// resolveMarginBoxFont resolves the font for one margin box's own
// declarations, reusing exactly the same nearest-weight/style matching
// body text uses (resolveFontPair) so margin-box and body-text font
// selection cannot silently diverge.
//
// When the box declares no font-style/font-weight/font-family of its
// own, it keeps issue #328's behavior verbatim: it inherits bodyEmbedded,
// the document's default body embedded font instance, with no standard
// fallback. Otherwise its own declared properties are resolved against a
// plain Helvetica/normal/400 base (unset properties default to plain,
// not to <body>'s style), which may return either a standard PDF-14 font
// or an @font-face-matched embedded font.
func (c *converter) resolveMarginBoxFont(mbc MarginBoxContent, bodyEmbedded *font.EmbeddedFont) (*font.Standard, *font.EmbeddedFont) {
	if mbc.FontFamily == "" && mbc.FontStyle == "" && mbc.FontWeight == 0 {
		return nil, bodyEmbedded
	}
	style := computedStyle{FontFamily: "helvetica", FontWeight: 400, FontStyle: "normal"}
	if mbc.FontFamily != "" {
		style.FontFamily = mbc.FontFamily
	}
	if mbc.FontWeight != 0 {
		style.FontWeight = mbc.FontWeight
	}
	if mbc.FontStyle != "" {
		style.FontStyle = mbc.FontStyle
	}
	return c.resolveFontPair(style)
}

// matchEmbeddedFont returns the @font-face whose declared weight is the
// nearest match for `desired` within the given family + style, per
// CSS Fonts L4 §5.2:
//
//   - exact match wins;
//   - desired = 400: try 500 first, then walk down (300 → 200 → 100),
//     then walk up (600 → 700 → 800 → 900);
//   - desired = 500: try 400 first, then walk down (300 → 200 → 100),
//     then walk up (600 → 700 → 800 → 900);
//   - desired ≤ 500 (and not 400/500): walk down first, then up;
//   - desired ≥ 500 (and not 500): walk up first, then down.
//
// Returns nil if no face for (family, style) is registered.
func (c *converter) matchEmbeddedFont(family, fontStyle string, desired int) *font.EmbeddedFont {
	if ef, ok := c.embeddedFonts[family+"|"+strconv.Itoa(desired)+"|"+fontStyle]; ok {
		return ef
	}
	// Collect every weight registered for this (family, style).
	prefix := family + "|"
	suffix := "|" + fontStyle
	weights := make([]int, 0, 9)
	for k := range c.embeddedFonts {
		if !strings.HasPrefix(k, prefix) || !strings.HasSuffix(k, suffix) {
			continue
		}
		w, err := strconv.Atoi(k[len(prefix) : len(k)-len(suffix)])
		if err != nil {
			continue
		}
		weights = append(weights, w)
	}
	if len(weights) == 0 {
		return nil
	}
	sort.Ints(weights)
	picked := pickNearestWeight(desired, weights)
	if picked == 0 {
		return nil
	}
	return c.embeddedFonts[family+"|"+strconv.Itoa(picked)+"|"+fontStyle]
}

// pickNearestWeight implements the CSS Fonts L4 §5.2 ladder walk
// against a sorted ascending slice of available weights:
//
//   - exact match always wins;
//   - desired in [400, 500]: scan available weights in (desired, 500]
//     ascending, then below desired descending, then above 500 ascending;
//   - desired < 400: scan below desired descending, then above ascending;
//   - desired > 500: scan above desired ascending, then below descending.
//
// The [400, 500] arm is the spec's only non-symmetric window — it
// reflects the historical convention that 500 is "Medium, treated as
// Regular" so authors writing 400 should prefer 500 over jumping up
// to 700 when 400 isn't shipped, and vice versa for 500.
func pickNearestWeight(desired int, weights []int) int {
	for _, w := range weights {
		if w == desired {
			return w
		}
	}
	if desired >= 400 && desired <= 500 {
		// Ascending in (desired, 500].
		for _, w := range weights {
			if w > desired && w <= 500 {
				return w
			}
		}
		// Descending below desired.
		if w := highestBelow(desired, weights); w != 0 {
			return w
		}
		// Ascending above 500.
		for _, w := range weights {
			if w > 500 {
				return w
			}
		}
		return 0
	}
	if desired < 400 {
		if w := highestBelow(desired, weights); w != 0 {
			return w
		}
		return lowestAbove(desired, weights)
	}
	// desired > 500
	if w := lowestAbove(desired, weights); w != 0 {
		return w
	}
	return highestBelow(desired, weights)
}

func highestBelow(desired int, weights []int) int {
	out := 0
	for _, w := range weights {
		if w < desired {
			out = w
		}
	}
	return out
}

func lowestAbove(desired int, weights []int) int {
	for _, w := range weights {
		if w > desired {
			return w
		}
	}
	return 0
}

// collectText recursively collects all text content from a node.
func collectText(n *html.Node) string {
	var sb strings.Builder
	collectTextInto(n, &sb)
	return collapseWhitespace(sb.String())
}

// collectTextInto appends all text content from n and its descendants to sb.
func collectTextInto(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		return
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		collectTextInto(child, sb)
	}
}

// collectRawText preserves whitespace (for <pre> elements).
func collectRawText(n *html.Node) string {
	var sb strings.Builder
	collectRawTextInto(n, &sb)
	return sb.String()
}

// collectRawTextInto appends raw text from n and its descendants to sb, preserving whitespace.
func collectRawTextInto(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		return
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		collectRawTextInto(child, sb)
	}
}

// findNestedList finds the first <ul> or <ol> child of a node.
func findNestedList(n *html.Node) *html.Node {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode &&
			(child.DataAtom == atom.Ul || child.DataAtom == atom.Ol) {
			return child
		}
	}
	return nil
}

// collapseWhitespace collapses runs of whitespace into single spaces and trims.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// collapseWhitespaceInline collapses runs of whitespace into single spaces
// while preserving a leading and/or trailing space if the original had one.
// Per CSS Text Module Level 3 §4.1.1 (Phase I), whitespace collapsing
// operates across inline element boundaries and does NOT strip
// leading/trailing spaces from individual text nodes. Use this variant
// in inline formatting contexts (collectRuns) where boundary whitespace
// signals inter-word spacing between adjacent styled runs.
func collapseWhitespaceInline(s string) string {
	collapsed := strings.Join(strings.Fields(s), " ")
	if collapsed == "" {
		// Whitespace-only text node: preserve as a single space so it
		// maintains inter-element spacing (e.g. "<b>bold</b> <i>italic</i>").
		if len(s) > 0 {
			return " "
		}
		return ""
	}
	hasLeading := s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r' || s[0] == '\f'
	hasTrailing := s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == '\f'
	if hasLeading {
		collapsed = " " + collapsed
	}
	if hasTrailing {
		collapsed = collapsed + " "
	}
	return collapsed
}

// applyTextTransform applies a CSS text-transform value to a string.
func applyTextTransform(s, transform string) string {
	switch transform {
	case "uppercase":
		return strings.ToUpper(s)
	case "lowercase":
		return strings.ToLower(s)
	case "capitalize":
		return capitalizeWords(s)
	default:
		return s
	}
}

// capitalizeWords capitalizes the first letter of each word.
func capitalizeWords(s string) string {
	var sb strings.Builder
	prevSpace := true
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			prevSpace = true
			sb.WriteRune(r)
		} else if prevSpace {
			sb.WriteRune(toUpperRune(r))
			prevSpace = false
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// toUpperRune converts a single rune to uppercase.
func toUpperRune(r rune) rune {
	s := strings.ToUpper(string(r))
	for _, c := range s {
		return c
	}
	return r
}

// processWhitespace handles whitespace according to the white-space CSS property.
func processWhitespace(s, whiteSpace string) string {
	switch whiteSpace {
	case "pre", "pre-wrap":
		// Preserve whitespace and line breaks.
		return s
	case "pre-line":
		// Collapse spaces/tabs but preserve line breaks.
		var sb strings.Builder
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(strings.Join(strings.Fields(line), " "))
		}
		return strings.TrimSpace(sb.String())
	default: // "normal", "nowrap"
		return collapseWhitespace(s)
	}
}

// textContent returns the concatenated text of all descendant text nodes.
func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var s string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += textContent(c)
	}
	return strings.TrimSpace(s)
}

// extractMeta extracts metadata from a <meta> element.
func (c *converter) extractMeta(n *html.Node) {
	name := strings.ToLower(getAttr(n, "name"))
	content := getAttr(n, "content")
	if content == "" {
		return
	}
	switch name {
	case "author":
		c.metadata.Author = content
	case "description":
		c.metadata.Description = content
	case "keywords":
		c.metadata.Keywords = content
	case "generator":
		c.metadata.Creator = content
	case "subject":
		c.metadata.Subject = content
	}
}

// getAttr returns the value of the named attribute on n, or the empty string.
func getAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// findHTMLLang walks the parsed document tree and returns the value of
// the `lang` attribute on the document's root <html> element, or "" if
// the attribute is absent. Called once at converter setup so the value
// is available to loadFontFaces before any @font-face rule is parsed —
// font.ParseFontForLanguage uses it to pick the appropriate face from
// pan-CJK TTCs (#280).
//
// The walk is a simple depth-first search rather than a strict
// "first child of DocumentNode" lookup because golang.org/x/net/html
// inserts implicit nodes (Doctype, comments, errata <html> wrappers)
// in shapes that are not always uniform across input. The first
// ElementNode with atom.Html wins; subsequent ones (extremely rare —
// malformed fragments) are ignored.
func findHTMLLang(doc *html.Node) string {
	var walk func(*html.Node) string
	walk = func(n *html.Node) string {
		if n == nil {
			return ""
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Html {
			return getAttr(n, "lang")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if v := walk(c); v != "" {
				return v
			}
		}
		return ""
	}
	return walk(doc)
}

// splitDeclarations splits a CSS style string into individual declarations.
func splitDeclarations(style string) []string {
	return strings.Split(style, ";")
}

// splitDeclarationWithImportant splits "property: value" into
// (property, value, important). The "!important" suffix is recognized
// case-insensitively and stripped from the returned value so downstream
// property parsers never see it.
func splitDeclarationWithImportant(decl string) (string, string, bool) {
	idx := strings.IndexByte(decl, ':')
	if idx < 0 {
		return "", "", false
	}
	prop := strings.TrimSpace(decl[:idx])
	val := strings.TrimSpace(decl[idx+1:])
	important := false
	if strings.HasSuffix(strings.ToLower(val), "!important") {
		important = true
		val = strings.TrimSpace(val[:len(val)-len("!important")])
	}
	return strings.ToLower(prop), val, important
}

// parseInt parses a string to int, returning 0 on failure.
func parseInt(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

// parseAttrFloat parses an HTML attribute value as float64 (for width/height attrs).
func parseAttrFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseBackgroundImage parses the CSS background-image value and returns
// the kind ("url", "linear-gradient", "radial-gradient") and the inner value.
func parseBackgroundImage(val string) (kind string, inner string) {
	val = strings.TrimSpace(val)
	lower := strings.ToLower(val)

	if strings.HasPrefix(lower, "url(") {
		inner := extractFunctionArgs(val)
		// Remove surrounding quotes.
		inner = strings.Trim(inner, `"'`)
		return "url", inner
	}
	if strings.HasPrefix(lower, "linear-gradient(") || strings.HasPrefix(lower, "repeating-linear-gradient(") {
		return "linear-gradient", extractFunctionArgs(val)
	}
	if strings.HasPrefix(lower, "radial-gradient(") || strings.HasPrefix(lower, "repeating-radial-gradient(") {
		return "radial-gradient", extractFunctionArgs(val)
	}
	return "", val
}

// extractFunctionArgs extracts the content between the outermost parentheses.
func extractFunctionArgs(val string) string {
	start := strings.IndexByte(val, '(')
	if start < 0 {
		return val
	}
	// Find matching close paren.
	depth := 0
	for i := start; i < len(val); i++ {
		switch val[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return val[start+1 : i]
			}
		}
	}
	return val[start+1:]
}

// parseLinearGradient parses CSS linear-gradient arguments.
// Returns the angle in degrees and the color stops.
func parseLinearGradient(args string) (float64, []layout.GradientStop) {
	// Split on commas, but respect nested parentheses (e.g., rgb()).
	parts := splitGradientArgs(args)
	if len(parts) < 2 {
		return 180, nil
	}

	angle := 180.0 // default: to bottom
	startIdx := 0

	// Check if first part is a direction.
	first := strings.TrimSpace(strings.ToLower(parts[0]))
	if strings.HasPrefix(first, "to ") {
		angle = parseGradientDirection(first)
		startIdx = 1
	} else if strings.HasSuffix(first, "deg") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(first, "deg"), 64); err == nil {
			angle = v
		}
		startIdx = 1
	} else if strings.HasSuffix(first, "rad") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(first, "rad"), 64); err == nil {
			angle = v * 180 / math.Pi
		}
		startIdx = 1
	}

	colorParts := parts[startIdx:]
	stops := parseGradientStops(colorParts)

	return angle, stops
}

// parseRadialGradient parses CSS radial-gradient arguments.
// Returns the color stops (center ellipse is assumed).
func parseRadialGradient(args string) []layout.GradientStop {
	parts := splitGradientArgs(args)
	if len(parts) < 2 {
		return nil
	}

	startIdx := 0
	// Skip shape/size keywords.
	first := strings.TrimSpace(strings.ToLower(parts[0]))
	if first == "circle" || first == "ellipse" ||
		strings.HasPrefix(first, "circle ") || strings.HasPrefix(first, "ellipse ") ||
		strings.Contains(first, "closest") || strings.Contains(first, "farthest") {
		startIdx = 1
	}

	return parseGradientStops(parts[startIdx:])
}

// splitGradientArgs splits a gradient argument string on commas,
// respecting nested parentheses (e.g., rgb(1,2,3)).
func splitGradientArgs(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, strings.TrimSpace(s[start:]))
	}
	return parts
}

// parseGradientDirection converts "to right", "to bottom left", etc. to degrees.
func parseGradientDirection(dir string) float64 {
	dir = strings.TrimPrefix(dir, "to ")
	dir = strings.TrimSpace(dir)
	switch dir {
	case "top":
		return 0
	case "right":
		return 90
	case "bottom":
		return 180
	case "left":
		return 270
	case "top right":
		return 45
	case "top left":
		return 315
	case "bottom right":
		return 135
	case "bottom left":
		return 225
	default:
		return 180
	}
}

// parseGradientStops parses a slice of "color [position]" strings into
// GradientStops.
//
// Calc support level is "restricted calc": a position token may be a
// plain percent ("50%") or a calc/min/max/clamp tree whose leaves are
// all percent or dimensionless (e.g. calc(50% - 10%), min(40%, 60%)).
// Mixed-unit calc such as calc(50% + 10px) is currently rejected —
// reducing it to a fraction requires the gradient line length, which is
// not known until render time. The same gap applies to plain length
// stop positions such as "blue 100px" or "red 1em": they fall back to
// the default position (0) instead of being resolved against the
// gradient line. Lazy resolution against the gradient line is the
// deferred fix (option (c) in issue #265).
func parseGradientStops(parts []string) []layout.GradientStop {
	var stops []layout.GradientStop
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Split on whitespace at paren depth 0 so calc/min/max/clamp
		// values stay a single token (e.g. "calc(50% - 10%)" with its
		// internal spaces is one token, not three).
		stop := layout.GradientStop{}
		tokens := splitTopLevelFields(p)

		if len(tokens) >= 2 {
			last := tokens[len(tokens)-1]
			positionMatched := false

			// Plain "<num>%" first — keeps the fast path for the
			// common case and avoids parseLength's calc dispatch.
			if strings.HasSuffix(last, "%") {
				if v, err := strconv.ParseFloat(strings.TrimSuffix(last, "%"), 64); err == nil {
					stop.Position = v / 100
					positionMatched = true
				}
			}

			// Calc/min/max/clamp: parse the token and accept only
			// percent-only trees. Mixed-unit trees fall through and
			// the whole field is treated as a color, matching the
			// pre-calc behaviour.
			if !positionMatched {
				if l := parseLength(last); l != nil {
					if frac, ok := percentFraction(l); ok {
						stop.Position = frac
						positionMatched = true
					}
				}
			}

			if positionMatched {
				colorStr := strings.Join(tokens[:len(tokens)-1], " ")
				if clr, ok := parseColor(colorStr); ok {
					stop.Color = clr
				}
			} else {
				// All tokens are the color.
				if clr, ok := parseColor(p); ok {
					stop.Color = clr
				}
			}
		} else {
			if clr, ok := parseColor(p); ok {
				stop.Color = clr
			}
		}

		stops = append(stops, stop)
	}

	return stops
}

// bgPosZero / bgPosHalf / bgPosFull are the percent constants that back
// the background-position keywords (left/top -> 0%, center -> 50%,
// right/bottom -> 100%). Storing them as cssLength preserves the percent
// semantics at draw time: 100% on a 200pt container with a 100pt image
// resolves to (200-100) * 1.0 = 100, which places the image's right
// edge against the container's right edge, matching the CSS "100% 100%"
// spec.
var (
	bgPosZero = &cssLength{Value: 0, Unit: "%"}
	bgPosHalf = &cssLength{Value: 50, Unit: "%"}
	bgPosFull = &cssLength{Value: 100, Unit: "%"}
)

// parseBgPosition parses the CSS background-position value into a pair
// of layout.ResolvableLength values. Resolution against the background
// box (and any em/rem inputs against font-size) happens lazily at draw
// time so plain lengths and mixed-unit calc — e.g. "100px 50%" or
// "calc(50% + 10px)" — produce the correct offset without needing the
// box dimensions at parse time.
//
// Each axis may be a keyword (left/center/right/top/bottom), a plain
// percent, a plain length, or a calc/min/max/clamp tree. Single-axis
// inputs default the missing axis to center. An unrecognised token
// drops both axes to their default (matching the prior fraction return
// of [0, 0] / [center, center] in the single-keyword case).
//
// The same lazy-resolution refactor is tracked for gradient stops in
// issue #318; gradient stops still parse to fractions because the
// gradient line length lives behind a different rendering boundary.
func parseBgPosition(val string) [2]layout.ResolvableLength {
	val = strings.TrimSpace(strings.ToLower(val))
	def := [2]layout.ResolvableLength{bgPosZero, bgPosZero}
	if val == "" {
		return def
	}

	// splitTopLevelFields keeps calc()/min()/max()/clamp() values as a
	// single token even when they contain internal whitespace.
	parts := splitTopLevelFields(val)

	// parseAxis maps a single axis token to a *cssLength. Keywords map
	// to the bgPosZero/Half/Full percent constants; everything else is
	// routed through parseLength which already understands plain
	// lengths, percents, and the calc/min/max/clamp family.
	parseAxis := func(s string) (*cssLength, bool) {
		switch s {
		case "left", "top":
			return bgPosZero, true
		case "center":
			return bgPosHalf, true
		case "right", "bottom":
			return bgPosFull, true
		}
		if l := parseLength(s); l != nil {
			return l, true
		}
		return nil, false
	}

	if len(parts) == 1 {
		if parts[0] == "center" {
			return [2]layout.ResolvableLength{bgPosHalf, bgPosHalf}
		}
		if l, ok := parseAxis(parts[0]); ok {
			// Single keyword/value: "top"/"bottom" target the y-axis
			// (x defaults to center); everything else targets x
			// (y defaults to center).
			switch parts[0] {
			case "top", "bottom":
				return [2]layout.ResolvableLength{bgPosHalf, l}
			default:
				return [2]layout.ResolvableLength{l, bgPosHalf}
			}
		}
		return def
	}

	x, y := layout.ResolvableLength(bgPosZero), layout.ResolvableLength(bgPosZero)
	if l, ok := parseAxis(parts[0]); ok {
		x = l
	}
	if l, ok := parseAxis(parts[1]); ok {
		y = l
	}
	return [2]layout.ResolvableLength{x, y}
}

// resolveBackgroundImage resolves a background-image CSS value into a layout.BackgroundImage.
// Returns nil if the value cannot be resolved.
func (c *converter) resolveBackgroundImage(style computedStyle) *layout.BackgroundImage {
	if style.BackgroundImage == "" {
		return nil
	}

	kind, inner := parseBackgroundImage(style.BackgroundImage)
	var img *folioimage.Image

	switch kind {
	case "url":
		imgPath := inner
		loaded, err := c.loadImageAsset(imgPath)
		if err != nil {
			c.reportAssetError("background-image", err, "src", imgPath)
			return nil
		}
		img = loaded

	case "linear-gradient":
		angle, stops := parseLinearGradient(inner)
		if len(stops) < 2 {
			return nil
		}
		// Render at a reasonable resolution.
		w, h := 200, 200
		rgba := layout.RenderLinearGradient(w, h, angle, stops)
		img = folioimage.NewFromGoImage(rgba)

	case "radial-gradient":
		stops := parseRadialGradient(inner)
		if len(stops) < 2 {
			return nil
		}
		w, h := 200, 200
		rgba := layout.RenderRadialGradient(w, h, stops)
		img = folioimage.NewFromGoImage(rgba)

	default:
		return nil
	}

	if img == nil {
		return nil
	}

	// Gradients fill the entire background area by default (CSS spec):
	// they don't tile and stretch to cover the element. Images tile.
	isGradient := kind == "linear-gradient" || kind == "radial-gradient"
	repeat := style.BackgroundRepeat
	if repeat == "" && isGradient {
		repeat = "no-repeat"
	}
	size := style.BackgroundSize
	if size == "" && isGradient {
		size = "cover"
	}

	bgImg := &layout.BackgroundImage{
		Image:    img,
		Size:     size,
		Position: parseBgPosition(style.BackgroundPosition),
		FontSize: style.FontSize,
		Repeat:   repeat,
	}

	// Parse explicit size values.
	if style.BackgroundSize != "" && style.BackgroundSize != "cover" && style.BackgroundSize != "contain" && style.BackgroundSize != "auto" {
		// splitTopLevelFields keeps calc()/min()/max()/clamp() values as
		// a single token even when they contain internal whitespace.
		parts := splitTopLevelFields(style.BackgroundSize)
		if len(parts) >= 1 {
			if l := parseLength(parts[0]); l != nil {
				bgImg.SizeW = l.toPoints(0, style.FontSize)
			}
		}
		if len(parts) >= 2 {
			if l := parseLength(parts[1]); l != nil {
				bgImg.SizeH = l.toPoints(0, style.FontSize)
			}
		}
	}

	if bgImg.Repeat == "" {
		bgImg.Repeat = "repeat"
	}

	return bgImg
}

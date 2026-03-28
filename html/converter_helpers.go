// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// resolveFont maps a computedStyle's family/weight/style to a standard PDF font.
func resolveFont(style computedStyle) *font.Standard {
	bold := style.FontWeight == "bold"
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

// resolveFontPair returns either a standard font or an embedded font for the
// given style. If the font family matches an @font-face rule, the embedded
// font is returned; otherwise the standard font is returned.
func (c *converter) resolveFontPair(style computedStyle) (*font.Standard, *font.EmbeddedFont) {
	if len(c.embeddedFonts) > 0 {
		family := strings.ToLower(style.FontFamily)
		key := family + "|" + style.FontWeight + "|" + style.FontStyle
		if ef, ok := c.embeddedFonts[key]; ok {
			return nil, ef
		}
		// Try without specific weight/style.
		keyBase := family + "|normal|normal"
		if ef, ok := c.embeddedFonts[keyBase]; ok {
			return nil, ef
		}
	}
	return resolveFont(style), nil
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

// splitDeclarations splits a CSS style string into individual declarations.
func splitDeclarations(style string) []string {
	return strings.Split(style, ";")
}

// splitDeclaration splits "property: value" into (property, value).
func splitDeclaration(decl string) (string, string) {
	idx := strings.IndexByte(decl, ':')
	if idx < 0 {
		return "", ""
	}
	prop := strings.TrimSpace(decl[:idx])
	val := strings.TrimSpace(decl[idx+1:])
	return strings.ToLower(prop), val
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

// parseGradientStops parses a slice of "color [position]" strings into GradientStops.
func parseGradientStops(parts []string) []layout.GradientStop {
	var stops []layout.GradientStop
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Try to split into color + position.
		// The position is the last token if it ends with %.
		stop := layout.GradientStop{}
		tokens := strings.Fields(p)

		if len(tokens) >= 2 {
			last := tokens[len(tokens)-1]
			if strings.HasSuffix(last, "%") {
				if v, err := strconv.ParseFloat(strings.TrimSuffix(last, "%"), 64); err == nil {
					stop.Position = v / 100
				}
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

// parseBgPosition converts CSS background-position keywords to [x, y]
// fractions in [0, 1].
func parseBgPosition(val string) [2]float64 {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "" {
		return [2]float64{0, 0}
	}

	parts := strings.Fields(val)

	toFrac := func(s string) (float64, bool) {
		switch s {
		case "left":
			return 0, true
		case "center":
			return 0.5, true
		case "right":
			return 1, true
		case "top":
			return 0, true
		case "bottom":
			return 1, true
		}
		if strings.HasSuffix(s, "%") {
			if v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64); err == nil {
				return v / 100, true
			}
		}
		return 0, false
	}

	if len(parts) == 1 {
		if parts[0] == "center" {
			return [2]float64{0.5, 0.5}
		}
		if f, ok := toFrac(parts[0]); ok {
			// Single keyword: "left" = 0, 0.5; "top" = 0.5, 0
			switch parts[0] {
			case "top", "bottom":
				return [2]float64{0.5, f}
			default:
				return [2]float64{f, 0.5}
			}
		}
		return [2]float64{0, 0}
	}

	x, y := 0.0, 0.0
	if f, ok := toFrac(parts[0]); ok {
		x = f
	}
	if f, ok := toFrac(parts[1]); ok {
		y = f
	}
	return [2]float64{x, y}
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
		if strings.HasPrefix(imgPath, "http://") || strings.HasPrefix(imgPath, "https://") {
			loaded, err := c.fetchImage(imgPath)
			if err != nil {
				return nil
			}
			img = loaded
		} else {
			if !filepath.IsAbs(imgPath) && c.opts.BasePath != "" {
				imgPath = filepath.Join(c.opts.BasePath, imgPath)
			}
			loaded, err := loadImage(imgPath)
			if err != nil {
				return nil
			}
			img = loaded
		}

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
		Repeat:   repeat,
	}

	// Parse explicit size values.
	if style.BackgroundSize != "" && style.BackgroundSize != "cover" && style.BackgroundSize != "contain" && style.BackgroundSize != "auto" {
		parts := strings.Fields(style.BackgroundSize)
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

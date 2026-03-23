// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// fontFaceRule holds a parsed @font-face declaration.
type fontFaceRule struct {
	family string
	src    string
	weight string
	style  string
}

// pageRule holds parsed @page declarations.
type pageRule struct {
	selector     string // "", "first", "left", "right"
	declarations []cssDecl
	marginBoxes  map[string][]cssDecl // e.g. "top-center" → declarations
}

// styleSheet holds parsed CSS rules from <style> blocks.
type styleSheet struct {
	rules     []cssRule
	fontFaces []fontFaceRule
	pageRules []pageRule // @page declarations
}

// cssRule is a single CSS rule: selector(s) + declarations.
type cssRule struct {
	selectors    []cssSelector
	declarations []cssDecl
}

// cssSelector is a parsed CSS selector.
type cssSelector struct {
	parts       []selectorPart // for descendant combinators: "div p" → [{tag:"div"}, {tag:"p"}]
	specificity int            // higher = more specific
}

// selectorPart is a single simple selector (tag, .class, or #id)
// with an optional combinator describing its relationship to the previous part.
type selectorPart struct {
	combinator    string // "", " " (descendant), ">" (child), "+" (adjacent sibling), "~" (general sibling)
	tag           string // e.g. "p", "h1", "*" (empty if class/id only)
	class         string // e.g. "highlight"
	id            string // e.g. "title"
	classes       []string
	pseudo        string // e.g. "first-child", "nth-child(2)"
	pseudoElement string // e.g. "before", "after"
	attrSelectors []attrSelector
}

// attrSelector represents a CSS attribute selector like [attr], [attr=value], etc.
type attrSelector struct {
	name  string // attribute name
	op    string // "", "=", "^=", "$=", "*=", "~=", "|="
	value string // expected value (empty for presence-only [attr])
}

// cssDecl is a CSS property: value pair.
type cssDecl struct {
	property  string
	value     string
	important bool
}

// parseStyleBlocks finds all <link rel="stylesheet"> and <style> elements in the
// document and parses their CSS. Linked stylesheets are processed before <style>
// blocks so that inline styles override external ones by source order.
func parseStyleBlocks(doc *html.Node, basePath string) *styleSheet {
	ss := &styleSheet{}

	// First pass: collect <link rel="stylesheet"> elements and load them.
	var walkLinks func(*html.Node)
	walkLinks = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Link {
			rel := ""
			href := ""
			for _, a := range n.Attr {
				switch a.Key {
				case "rel":
					rel = strings.ToLower(strings.TrimSpace(a.Val))
				case "href":
					href = strings.TrimSpace(a.Val)
				}
			}
			if rel == "stylesheet" && href != "" {
				path := href
				if !filepath.IsAbs(path) && basePath != "" {
					path = filepath.Join(basePath, path)
				}
				data, err := os.ReadFile(path)
				if err == nil {
					ss.parseCSS(string(data))
				}
				// Silently skip if file can't be read.
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walkLinks(child)
		}
	}
	walkLinks(doc)

	// Second pass: collect <style> blocks (override linked stylesheets by source order).
	var walkStyles func(*html.Node)
	walkStyles = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Style {
			// Collect text content of the <style> element.
			var sb strings.Builder
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					sb.WriteString(child.Data)
				}
			}
			ss.parseCSS(sb.String())
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walkStyles(child)
		}
	}
	walkStyles(doc)

	return ss
}

// parseCSS parses a CSS string into rules.
func (ss *styleSheet) parseCSS(css string) {
	// Strip CSS comments.
	css = stripComments(css)

	// Extract @media print blocks first — include their rules directly
	// since PDF is a print medium.
	css = ss.extractMediaPrint(css)

	// Split on closing braces to find rules, handling nested braces.
	remaining := css
	for {
		openIdx := strings.IndexByte(remaining, '{')
		if openIdx < 0 {
			break
		}
		closeIdx := findMatchingBrace(remaining, openIdx)
		if closeIdx < 0 {
			break
		}

		selectorStr := strings.TrimSpace(remaining[:openIdx])
		declStr := strings.TrimSpace(remaining[openIdx+1 : closeIdx])
		remaining = remaining[closeIdx+1:]

		if selectorStr == "" {
			continue
		}

		// Parse @font-face rules.
		if selectorStr == "@font-face" {
			decls := parseDeclarations(declStr)
			ff := fontFaceRule{weight: "normal", style: "normal"}
			for _, d := range decls {
				switch d.property {
				case "font-family":
					ff.family = strings.ToLower(strings.Trim(strings.TrimSpace(d.value), `"'`))
				case "src":
					ff.src = parseFontFaceSrc(d.value)
				case "font-weight":
					ff.weight = strings.TrimSpace(strings.ToLower(d.value))
				case "font-style":
					ff.style = strings.TrimSpace(strings.ToLower(d.value))
				}
			}
			if ff.family != "" && ff.src != "" {
				ss.fontFaces = append(ss.fontFaces, ff)
			}
			continue
		}
		// Parse @page rules (with optional pseudo-selector like :first, :left, :right).
		if selectorStr == "@page" || strings.HasPrefix(selectorStr, "@page ") || strings.HasPrefix(selectorStr, "@page:") {
			sel := ""
			rest := strings.TrimPrefix(selectorStr, "@page")
			rest = strings.TrimSpace(rest)
			if strings.HasPrefix(rest, ":") {
				sel = strings.TrimPrefix(rest, ":")
				sel = strings.TrimSpace(sel)
			}

			// Extract nested margin box rules (e.g. @top-center { ... }) from declStr.
			marginBoxes := make(map[string][]cssDecl)
			cleanedDecls := extractMarginBoxes(declStr, marginBoxes)

			decls := parseDeclarations(cleanedDecls)
			ss.pageRules = append(ss.pageRules, pageRule{
				selector:     sel,
				declarations: decls,
				marginBoxes:  marginBoxes,
			})
			continue
		}

		// Skip other @-rules (e.g. @media screen).
		if strings.HasPrefix(selectorStr, "@") {
			continue
		}

		selectors := parseSelectors(selectorStr)
		decls := parseDeclarations(declStr)
		if len(selectors) > 0 && len(decls) > 0 {
			ss.rules = append(ss.rules, cssRule{
				selectors:    selectors,
				declarations: decls,
			})
		}
	}
}

// extractMediaPrint finds @media print { ... } blocks, parses them as
// regular rules, and removes them from the input CSS so they don't
// interfere with normal rule parsing. Returns the CSS with @media print
// blocks replaced by their inner content.
func (ss *styleSheet) extractMediaPrint(css string) string {
	var result strings.Builder
	remaining := css
	for {
		idx := strings.Index(remaining, "@media")
		if idx < 0 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:idx])
		remaining = remaining[idx:]

		// Find the opening brace of the @media block.
		openIdx := strings.IndexByte(remaining, '{')
		if openIdx < 0 {
			result.WriteString(remaining)
			break
		}

		mediaQuery := strings.TrimSpace(remaining[6:openIdx]) // after "@media"
		remaining = remaining[openIdx+1:]

		// Find matching closing brace (handle nesting).
		depth := 1
		end := 0
		for i := 0; i < len(remaining); i++ {
			if remaining[i] == '{' {
				depth++
			} else if remaining[i] == '}' {
				depth--
				if depth == 0 {
					end = i
					break
				}
			}
		}
		if depth != 0 {
			// Malformed — skip.
			result.WriteString(remaining)
			break
		}

		innerCSS := remaining[:end]
		remaining = remaining[end+1:]

		// Include @media print rules (PDF is a print medium).
		if strings.Contains(mediaQuery, "print") {
			result.WriteString(innerCSS)
		}
		// Other @media blocks are discarded.
	}
	return result.String()
}

// extractMarginBoxes extracts nested @-rules (margin boxes) from a @page declaration
// string. Returns the remaining declarations with margin boxes removed.
// Supported margin boxes: @top-left, @top-center, @top-right,
// @bottom-left, @bottom-center, @bottom-right.
func extractMarginBoxes(declStr string, boxes map[string][]cssDecl) string {
	var clean strings.Builder
	remaining := declStr
	for {
		atIdx := strings.IndexByte(remaining, '@')
		if atIdx < 0 {
			clean.WriteString(remaining)
			break
		}
		// Write everything before the @
		clean.WriteString(remaining[:atIdx])

		// Find the name (e.g. "top-center")
		rest := remaining[atIdx+1:]
		openIdx := strings.IndexByte(rest, '{')
		if openIdx < 0 {
			clean.WriteString(remaining[atIdx:])
			break
		}
		name := strings.TrimSpace(rest[:openIdx])

		// Find matching close brace
		fullStr := remaining[atIdx:]
		braceStart := strings.IndexByte(fullStr, '{')
		closeIdx := findMatchingBrace(fullStr, braceStart)
		if closeIdx < 0 {
			clean.WriteString(remaining[atIdx:])
			break
		}

		boxDecls := strings.TrimSpace(fullStr[braceStart+1 : closeIdx])
		boxes[name] = parseDeclarations(boxDecls)

		remaining = remaining[atIdx+closeIdx+1:]
	}
	return clean.String()
}

// findMatchingBrace finds the closing '}' that matches the opening '{' at openIdx,
// correctly handling nested braces.
func findMatchingBrace(s string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// stripComments removes /* ... */ comments from CSS.
func stripComments(css string) string {
	var sb strings.Builder
	for {
		start := strings.Index(css, "/*")
		if start < 0 {
			sb.WriteString(css)
			break
		}
		sb.WriteString(css[:start])
		end := strings.Index(css[start+2:], "*/")
		if end < 0 {
			break
		}
		css = css[start+2+end+2:]
	}
	return sb.String()
}

// parseSelectors parses a comma-separated selector list like "h1, h2, .title".
func parseSelectors(s string) []cssSelector {
	parts := strings.Split(s, ",")
	var selectors []cssSelector
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		sel := parseSelector(p)
		selectors = append(selectors, sel)
	}
	return selectors
}

// parseSelector parses a single selector like "div > p.highlight + span".
func parseSelector(s string) cssSelector {
	// Normalize combinators: add spaces around >, +, ~ (but not inside parens).
	s = normalizeCombinators(s)
	tokens := strings.Fields(s)

	var parts []selectorPart
	spec := 0
	nextCombinator := "" // combinator for the next part

	for _, tok := range tokens {
		// Check for combinator tokens.
		if tok == ">" || tok == "+" || tok == "~" {
			nextCombinator = tok
			continue
		}

		part := parseSelectorPart(tok)

		// Set combinator — first part has "", subsequent default to " " (descendant).
		if len(parts) > 0 && nextCombinator == "" {
			part.combinator = " " // descendant
		} else {
			part.combinator = nextCombinator
		}
		nextCombinator = ""

		if part.id != "" {
			spec += 100
		}
		if part.class != "" {
			spec += 10
		}
		for range part.classes {
			spec += 10
		}
		// Universal selector (*) adds no specificity.
		if part.tag != "" && part.tag != "*" {
			spec += 1
		}
		if part.pseudo != "" {
			spec += 10 // pseudo-classes have class-level specificity
		}
		for range part.attrSelectors {
			spec += 10 // attribute selectors have class-level specificity
		}
		if part.pseudoElement != "" {
			spec += 1 // pseudo-elements have element-level specificity
		}
		parts = append(parts, part)
	}
	return cssSelector{parts: parts, specificity: spec}
}

// normalizeCombinators inserts spaces around >, +, ~ combinators
// so they become separate tokens, but not inside parentheses (e.g. :nth-child(2n+1)).
func normalizeCombinators(s string) string {
	var sb strings.Builder
	depth := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 && (ch == '>' || ch == '+' || ch == '~') {
			sb.WriteByte(' ')
			sb.WriteByte(ch)
			sb.WriteByte(' ')
		} else {
			sb.WriteByte(ch)
		}
	}
	return sb.String()
}

// parseSelectorPart parses a simple selector like "p", ".class", "#id", "p.class", or "p:first-child".
func parseSelectorPart(s string) selectorPart {
	var part selectorPart

	// Extract attribute selectors [attr], [attr=value], etc. before other parsing.
	for {
		openBrk := strings.IndexByte(s, '[')
		if openBrk < 0 {
			break
		}
		closeBrk := strings.IndexByte(s[openBrk:], ']')
		if closeBrk < 0 {
			break
		}
		closeBrk += openBrk
		attrContent := s[openBrk+1 : closeBrk]
		s = s[:openBrk] + s[closeBrk+1:]

		as := parseAttrSelector(attrContent)
		part.attrSelectors = append(part.attrSelectors, as)
	}

	// Handle pseudo-class (e.g. ":first-child", ":nth-child(2)").
	// Must be extracted before class/id parsing.
	if colonIdx := strings.Index(s, ":"); colonIdx >= 0 {
		pseudo := s[colonIdx+1:]
		s = s[:colonIdx]
		// Handle double colon (::pseudo-element).
		if strings.HasPrefix(pseudo, ":") {
			pe := strings.ToLower(pseudo[1:])
			if pe == "before" || pe == "after" {
				part.pseudoElement = pe
			}
		} else {
			part.pseudo = strings.ToLower(pseudo)
		}
	}

	// Handle #id.
	if idx := strings.IndexByte(s, '#'); idx >= 0 {
		rest := s[idx+1:]
		// ID may be followed by . for class.
		if dotIdx := strings.IndexByte(rest, '.'); dotIdx >= 0 {
			part.id = rest[:dotIdx]
			rest = rest[dotIdx:]
		} else {
			part.id = rest
			rest = ""
		}
		if idx > 0 {
			part.tag = strings.ToLower(s[:idx])
		}
		s = rest
	}

	// Handle .class (possibly multiple).
	for {
		dotIdx := strings.IndexByte(s, '.')
		if dotIdx < 0 {
			if s != "" && part.tag == "" {
				part.tag = strings.ToLower(s)
			}
			break
		}
		if dotIdx > 0 && part.tag == "" {
			part.tag = strings.ToLower(s[:dotIdx])
		}
		s = s[dotIdx+1:]
		nextDot := strings.IndexByte(s, '.')
		if nextDot < 0 {
			cls := strings.ToLower(s)
			if part.class == "" {
				part.class = cls
			} else {
				part.classes = append(part.classes, cls)
			}
			break
		}
		cls := strings.ToLower(s[:nextDot])
		if part.class == "" {
			part.class = cls
		} else {
			part.classes = append(part.classes, cls)
		}
		s = s[nextDot:]
	}

	return part
}

// parseAttrSelector parses the content inside [...] into an attrSelector.
func parseAttrSelector(content string) attrSelector {
	content = strings.TrimSpace(content)
	// Try each multi-char operator first.
	for _, op := range []string{"^=", "$=", "*=", "~=", "|="} {
		if idx := strings.Index(content, op); idx >= 0 {
			name := strings.TrimSpace(content[:idx])
			val := strings.TrimSpace(content[idx+len(op):])
			val = strings.Trim(val, `"'`)
			return attrSelector{name: strings.ToLower(name), op: op, value: val}
		}
	}
	// Simple equality.
	if idx := strings.IndexByte(content, '='); idx >= 0 {
		name := strings.TrimSpace(content[:idx])
		val := strings.TrimSpace(content[idx+1:])
		val = strings.Trim(val, `"'`)
		return attrSelector{name: strings.ToLower(name), op: "=", value: val}
	}
	// Presence only.
	return attrSelector{name: strings.ToLower(content)}
}

// parseDeclarations parses "color: red; font-size: 12px" into key-value pairs.
func parseDeclarations(s string) []cssDecl {
	var decls []cssDecl
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.IndexByte(part, ':')
		if idx < 0 {
			continue
		}
		prop := strings.TrimSpace(strings.ToLower(part[:idx]))
		val := strings.TrimSpace(part[idx+1:])

		// Detect !important.
		imp := false
		if strings.HasSuffix(strings.ToLower(val), "!important") {
			imp = true
			val = strings.TrimSpace(val[:len(val)-len("!important")])
		}

		if prop != "" && val != "" {
			decls = append(decls, cssDecl{property: prop, value: val, important: imp})
		}
	}
	return decls
}

// parseFontFaceSrc extracts the font file path from a CSS src value.
// Supports url("path"), url('path'), and url(path).
func parseFontFaceSrc(val string) string {
	if idx := strings.Index(val, "url("); idx >= 0 {
		rest := val[idx+4:]
		end := strings.IndexByte(rest, ')')
		if end < 0 {
			return ""
		}
		path := strings.TrimSpace(rest[:end])
		path = strings.Trim(path, `"'`)
		return path
	}
	return ""
}

// matchingDeclarations returns all CSS declarations that match a node,
// ordered by specificity (lowest first, so later entries override).
// !important declarations are returned after normal ones (specificity boosted by 1000).
// Rules with pseudo-elements (::before, ::after) are excluded.
func (ss *styleSheet) matchingDeclarations(n *html.Node) []cssDecl {
	if ss == nil || len(ss.rules) == 0 {
		return nil
	}

	type match struct {
		specificity int
		decl        cssDecl
	}
	var matches []match

	for _, rule := range ss.rules {
		for _, sel := range rule.selectors {
			// Skip selectors with pseudo-elements — those are for ::before/::after.
			if len(sel.parts) > 0 && sel.parts[len(sel.parts)-1].pseudoElement != "" {
				continue
			}
			if selectorMatches(sel, n) {
				for _, d := range rule.declarations {
					spec := sel.specificity
					if d.important {
						spec += 1000
					}
					matches = append(matches, match{specificity: spec, decl: d})
				}
				break
			}
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort by specificity (stable, lower first).
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].specificity < matches[j-1].specificity; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}

	var result []cssDecl
	for _, m := range matches {
		result = append(result, m.decl)
	}
	return result
}

// matchingPseudoElementDeclarations returns CSS declarations for a pseudo-element
// (e.g. "before" or "after") that matches a given node. The pseudo parameter
// should be "before" or "after" (without the :: prefix).
func (ss *styleSheet) matchingPseudoElementDeclarations(n *html.Node, pseudo string) []cssDecl {
	if ss == nil || len(ss.rules) == 0 {
		return nil
	}

	type match struct {
		specificity int
		decl        cssDecl
	}
	var matches []match

	for _, rule := range ss.rules {
		for _, sel := range rule.selectors {
			if len(sel.parts) == 0 {
				continue
			}
			last := sel.parts[len(sel.parts)-1]
			if last.pseudoElement != pseudo {
				continue
			}
			// Match the selector against the node (ignoring the pseudoElement field in partMatches).
			if selectorMatches(sel, n) {
				for _, d := range rule.declarations {
					spec := sel.specificity
					if d.important {
						spec += 1000
					}
					matches = append(matches, match{specificity: spec, decl: d})
				}
				break
			}
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort by specificity (stable, lower first).
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].specificity < matches[j-1].specificity; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}

	var result []cssDecl
	for _, m := range matches {
		result = append(result, m.decl)
	}
	return result
}

// selectorMatches checks if a selector matches a node.
func selectorMatches(sel cssSelector, n *html.Node) bool {
	if len(sel.parts) == 0 {
		return false
	}

	// Match from right to left (last part must match the node).
	if !partMatches(sel.parts[len(sel.parts)-1], n) {
		return false
	}

	// Walk backwards through remaining parts, respecting combinators.
	current := n
	for i := len(sel.parts) - 2; i >= 0; i-- {
		comb := sel.parts[i+1].combinator

		switch comb {
		case ">": // Child combinator: parent must match.
			current = current.Parent
			if current == nil || !partMatches(sel.parts[i], current) {
				return false
			}
		case "+": // Adjacent sibling: previous element sibling must match.
			prev := prevElementSibling(current)
			if prev == nil || !partMatches(sel.parts[i], prev) {
				return false
			}
			current = prev
		case "~": // General sibling: any preceding element sibling must match.
			found := false
			for sib := prevElementSibling(current); sib != nil; sib = prevElementSibling(sib) {
				if partMatches(sel.parts[i], sib) {
					current = sib
					found = true
					break
				}
			}
			if !found {
				return false
			}
		default: // Descendant (space): any ancestor must match.
			found := false
			for p := current.Parent; p != nil; p = p.Parent {
				if partMatches(sel.parts[i], p) {
					current = p
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

// prevElementSibling returns the previous element sibling of n, or nil.
func prevElementSibling(n *html.Node) *html.Node {
	for sib := n.PrevSibling; sib != nil; sib = sib.PrevSibling {
		if sib.Type == html.ElementNode {
			return sib
		}
	}
	return nil
}

// partMatches checks if a simple selector part matches a node.
// pseudoElement is intentionally not checked here — it is used at a higher level.
func partMatches(part selectorPart, n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	if part.tag != "" && part.tag != "*" && strings.ToLower(n.Data) != part.tag {
		return false
	}

	if part.id != "" {
		nodeID := nodeAttr(n, "id")
		if nodeID != part.id {
			return false
		}
	}

	if part.class != "" {
		classes := nodeClasses(n)
		if !containsClass(classes, part.class) {
			return false
		}
		for _, cls := range part.classes {
			if !containsClass(classes, cls) {
				return false
			}
		}
	}

	if part.pseudo != "" && !pseudoMatches(part.pseudo, n) {
		return false
	}

	// Check attribute selectors.
	for _, as := range part.attrSelectors {
		if !attrSelectorMatches(as, n) {
			return false
		}
	}

	return true
}

// attrSelectorMatches checks if an attribute selector matches a node.
func attrSelectorMatches(as attrSelector, n *html.Node) bool {
	val := nodeAttr(n, as.name)
	switch as.op {
	case "": // presence only [attr]
		for _, a := range n.Attr {
			if a.Key == as.name {
				return true
			}
		}
		return false
	case "=": // exact match
		return val == as.value
	case "^=": // starts with
		return as.value != "" && strings.HasPrefix(val, as.value)
	case "$=": // ends with
		return as.value != "" && strings.HasSuffix(val, as.value)
	case "*=": // contains
		return as.value != "" && strings.Contains(val, as.value)
	case "~=": // space-separated word list contains
		for _, w := range strings.Fields(val) {
			if w == as.value {
				return true
			}
		}
		return false
	case "|=": // exact match or prefix followed by "-"
		return val == as.value || strings.HasPrefix(val, as.value+"-")
	}
	return false
}

// pseudoMatches checks if a pseudo-class matches a node.
func pseudoMatches(pseudo string, n *html.Node) bool {
	switch {
	case pseudo == "first-child":
		return isNthChild(n, 1)
	case pseudo == "last-child":
		return isLastChild(n)
	case strings.HasPrefix(pseudo, "nth-child(") && strings.HasSuffix(pseudo, ")"):
		inner := pseudo[len("nth-child(") : len(pseudo)-1]
		inner = strings.TrimSpace(inner)
		if inner == "odd" {
			pos := childIndex(n)
			return pos > 0 && pos%2 == 1
		}
		if inner == "even" {
			pos := childIndex(n)
			return pos > 0 && pos%2 == 0
		}
		if num, err := strconv.Atoi(inner); err == nil {
			return isNthChild(n, num)
		}
		return false
	case strings.HasPrefix(pseudo, "not(") && strings.HasSuffix(pseudo, ")"):
		inner := pseudo[len("not(") : len(pseudo)-1]
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return false
		}
		innerPart := parseSelectorPart(inner)
		return !partMatches(innerPart, n)
	default:
		return false
	}
}

// childIndex returns the 1-based index of n among its parent's element children.
func childIndex(n *html.Node) int {
	if n.Parent == nil {
		return 0
	}
	idx := 0
	for sib := n.Parent.FirstChild; sib != nil; sib = sib.NextSibling {
		if sib.Type == html.ElementNode {
			idx++
			if sib == n {
				return idx
			}
		}
	}
	return 0
}

// isNthChild checks if n is the nth element child (1-based).
func isNthChild(n *html.Node, pos int) bool {
	return childIndex(n) == pos
}

// isLastChild checks if n is the last element child of its parent.
func isLastChild(n *html.Node) bool {
	if n.Parent == nil {
		return false
	}
	for sib := n.NextSibling; sib != nil; sib = sib.NextSibling {
		if sib.Type == html.ElementNode {
			return false
		}
	}
	return true
}

// nodeAttr returns the value of an attribute on a node.
func nodeAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// nodeClasses returns the space-separated class list from a node.
func nodeClasses(n *html.Node) []string {
	cls := nodeAttr(n, "class")
	if cls == "" {
		return nil
	}
	return strings.Fields(cls)
}

// containsClass checks if a class list contains a class name.
// The comparison is case-insensitive because CSS class selectors are
// lowercased during parsing, and HTML class names are conventionally
// treated as case-insensitive by browsers.
func containsClass(classes []string, name string) bool {
	for _, c := range classes {
		if strings.EqualFold(c, name) {
			return true
		}
	}
	return false
}

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"sort"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// fontFaceRule holds a parsed @font-face declaration.
type fontFaceRule struct {
	family string
	src    string
	style  string
	// origin is the href of the stylesheet this rule was parsed from. "" for
	// inline <style> blocks (or any rule with no enclosing stylesheet origin),
	// in which case src is resolved from the BaseFS root. For linked
	// stylesheets, src is resolved relative to path.Dir(origin) — either
	// inside BaseFS or via Client when origin is an http(s) URL.
	origin string
	weight int // CSS Fonts L4 numeric ladder; default 400 (normal).
}

// pageRule holds parsed @page declarations.
type pageRule struct {
	marginBoxes  map[string][]cssDecl // e.g. "top-center" → declarations
	selector     string               // "", "first", "left", "right"
	declarations []cssDecl
}

// styleSheet holds parsed CSS rules from <style> blocks.
type styleSheet struct {
	index     *ruleIndex // lazily built selector index, see buildIndex
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

// selectorRef locates one selector inside styleSheet.rules.
type selectorRef struct {
	ruleIdx int
	selIdx  int
}

// ruleIndex buckets selectors by their rightmost simple selector so
// matching a node probes only candidate rules instead of scanning all of
// them. A bucket key is a necessary (not sufficient) condition for a
// match: selectors whose last part has an id go in byID, else a class in
// byClass (lowercased), else a concrete tag in byTag, else universal
// (covers "*", pseudo-class-only, and attribute-only selectors).
//
// If a future selector feature relaxes matching of id/class/tag on the
// last part (e.g. an attribute-flag case-insensitive id), buildIndex and
// candidateRefs must be updated to match — see partMatches in
// css_selectors.go for the ground truth these buckets approximate.
type ruleIndex struct {
	byID      map[string][]selectorRef
	byClass   map[string][]selectorRef
	byTag     map[string][]selectorRef
	universal []selectorRef
	ruleCount int // len(ss.rules) when built; rebuilt if rules were appended
}

// buildIndex populates ss.index from ss.rules. Not safe for concurrent use;
// styleSheet matching is single-goroutine per conversion, same as the rest
// of the converter.
func (ss *styleSheet) buildIndex() {
	idx := &ruleIndex{
		byID:      map[string][]selectorRef{},
		byClass:   map[string][]selectorRef{},
		byTag:     map[string][]selectorRef{},
		ruleCount: len(ss.rules),
	}
	for ri, rule := range ss.rules {
		for si, sel := range rule.selectors {
			ref := selectorRef{ruleIdx: ri, selIdx: si}
			if len(sel.parts) == 0 {
				idx.universal = append(idx.universal, ref)
				continue
			}
			last := sel.parts[len(sel.parts)-1]
			switch {
			case last.id != "":
				idx.byID[last.id] = append(idx.byID[last.id], ref)
			case last.class != "":
				key := strings.ToLower(last.class)
				idx.byClass[key] = append(idx.byClass[key], ref)
			case last.tag != "" && last.tag != "*":
				idx.byTag[last.tag] = append(idx.byTag[last.tag], ref)
			default:
				idx.universal = append(idx.universal, ref)
			}
		}
	}
	ss.index = idx
}

// candidateRefs returns the selector refs whose bucket keys are consistent
// with n, sorted by (ruleIdx, selIdx) with duplicates removed. Bucket keys
// are necessary conditions for a match, so every selector that could match
// n is included — filtering candidates with selectorMatches is equivalent
// to scanning all rules.
func (ss *styleSheet) candidateRefs(n *html.Node) []selectorRef {
	if ss.index == nil || ss.index.ruleCount != len(ss.rules) {
		ss.buildIndex()
	}
	idx := ss.index
	var refs []selectorRef
	if id := nodeAttr(n, "id"); id != "" {
		refs = append(refs, idx.byID[id]...)
	}
	for _, cls := range nodeClasses(n) {
		refs = append(refs, idx.byClass[strings.ToLower(cls)]...)
	}
	refs = append(refs, idx.byTag[strings.ToLower(n.Data)]...)
	refs = append(refs, idx.universal...)
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ruleIdx != refs[j].ruleIdx {
			return refs[i].ruleIdx < refs[j].ruleIdx
		}
		return refs[i].selIdx < refs[j].selIdx
	})
	// Dedupe: duplicate node classes (class="a a") probe a bucket twice.
	out := refs[:0]
	for i, r := range refs {
		if i > 0 && r == refs[i-1] {
			continue
		}
		out = append(out, r)
	}
	return out
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
// Stylesheet hrefs route through the unified [resolveLocalAsset] contract
// (10MB HTTP cap matching the historical CSS fetch limit). onLoadError, if
// non-nil, is invoked for stylesheet loads that fail (used to surface the
// failure to Options.Logger without aborting the conversion).
func parseStyleBlocks(doc *html.Node, opts Options, budget *assetBudget, onLoadError func(href string, err error)) *styleSheet {
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
				data, err := resolveLocalAsset(opts, opts.URLPolicy, budget, "", href, 10<<20)
				if err == nil {
					ss.parseCSS(string(data), href)
				} else if onLoadError != nil {
					onLoadError(href, err)
				}
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
			ss.parseCSS(sb.String(), "")
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walkStyles(child)
		}
	}
	walkStyles(doc)

	return ss
}

// parseCSS parses a CSS string into rules. origin is the href the CSS was
// loaded from ("" for inline <style>); it is recorded on @font-face rules so
// their src url() can be resolved relative to the stylesheet rather than the
// document root.
func (ss *styleSheet) parseCSS(css string, origin string) {
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
			ff := fontFaceRule{weight: 400, style: "normal", origin: origin}
			for _, d := range decls {
				switch d.property {
				case "font-family":
					ff.family = strings.ToLower(strings.Trim(strings.TrimSpace(d.value), `"'`))
				case "src":
					ff.src = parseFontFaceSrc(d.value)
				case "font-weight":
					// @font-face descriptors don't inherit, so the
					// "inherited" arg is unused; pass 400.
					ff.weight = parseFontWeight(d.value, 400)
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
			rest := strings.TrimPrefix(selectorStr, "@page")
			rest = strings.TrimSpace(rest)

			// Determine the page selector. Only a bare `@page {}` (empty
			// rest) or a recognised pseudo (:first/:left/:right) are honoured.
			//
			// A NAMED page (`@page narrow {}`) or an UNSUPPORTED pseudo
			// (`@page :blank {}`) must NOT pollute the global default @page {}
			// rule: a named page has rest="narrow" (no leading ':'), and an
			// unsupported pseudo has sel="blank" with no case in page.go. Both
			// previously fell through to the default rule (selector ""), which
			// applied their margins to every page. We now skip them entirely.
			sel := ""
			if rest != "" {
				if !strings.HasPrefix(rest, ":") {
					// Named page (e.g. "@page narrow") — drop it.
					continue
				}
				// CSS page pseudo names are case-insensitive.
				sel = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(rest, ":")))
				switch sel {
				case "first", "left", "right":
					// Recognised pseudo — honour it.
				default:
					// Unsupported pseudo (e.g. ":blank") — drop it so it
					// does not become the default rule.
					continue
				}
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

		// Handle @supports feature queries.
		if strings.HasPrefix(selectorStr, "@supports") {
			condition := strings.TrimSpace(strings.TrimPrefix(selectorStr, "@supports"))
			if evaluateSupports(condition) {
				ss.parseCSS(declStr, origin)
			}
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

		// Find the name (e.g. "top-center"). CSS at-rule (margin-box)
		// names are case-insensitive, so normalize to lower case here;
		// the renderer switches on lower-case literals.
		rest := remaining[atIdx+1:]
		openIdx := strings.IndexByte(rest, '{')
		if openIdx < 0 {
			clean.WriteString(remaining[atIdx:])
			break
		}
		name := strings.ToLower(strings.TrimSpace(rest[:openIdx]))

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

// unescapeCSS removes CSS escape sequences: \X → X for single-character
// escapes. This allows selectors like \.class or \#id to match literal
// dots and hashes. Full 6-digit hex escapes (\0000XX) are not supported.
func unescapeCSS(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			sb.WriteByte(s[i])
		} else {
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// parseSelectorPart parses a simple selector like "p", ".class", "#id", "p.class", or "p:first-child".
func parseSelectorPart(s string) selectorPart {
	var part selectorPart

	// Extract top-level attribute selectors [attr], [attr=value], etc.
	// Brackets inside parentheses belong to pseudo-classes like :not([hidden])
	// and must not be extracted here.
	{
		i := 0
		for i < len(s) {
			openBrk := strings.IndexByte(s[i:], '[')
			if openBrk < 0 {
				break
			}
			openBrk += i

			// Check if this bracket is inside parentheses.
			parenDepth := 0
			for j := 0; j < openBrk; j++ {
				switch s[j] {
				case '(':
					parenDepth++
				case ')':
					parenDepth--
				}
			}
			if parenDepth > 0 {
				// Skip past this bracket pair — it belongs to a pseudo-class.
				closeBrk := strings.IndexByte(s[openBrk:], ']')
				if closeBrk < 0 {
					break
				}
				i = openBrk + closeBrk + 1
				continue
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
			// Don't advance i — the string was shortened, next '[' is at same position.
		}
	}

	// Handle pseudo-class (e.g. ":first-child", ":nth-child(2)").
	// Must be extracted before class/id parsing.
	if colonIdx := strings.Index(s, ":"); colonIdx >= 0 {
		pseudo := s[colonIdx+1:]
		s = s[:colonIdx]
		// Handle double colon (::pseudo-element).
		if strings.HasPrefix(pseudo, ":") {
			pe := strings.ToLower(pseudo[1:])
			if pe == "before" || pe == "after" || pe == "marker" || pe == "placeholder" {
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
			part.id = unescapeCSS(rest[:dotIdx])
			rest = rest[dotIdx:]
		} else {
			part.id = unescapeCSS(rest)
			rest = ""
		}
		if idx > 0 {
			part.tag = unescapeCSS(strings.ToLower(s[:idx]))
		}
		s = rest
	}

	// Handle .class (possibly multiple).
	for {
		dotIdx := strings.IndexByte(s, '.')
		if dotIdx < 0 {
			if s != "" && part.tag == "" {
				part.tag = unescapeCSS(strings.ToLower(s))
			}
			break
		}
		if dotIdx > 0 && part.tag == "" {
			part.tag = unescapeCSS(strings.ToLower(s[:dotIdx]))
		}
		s = s[dotIdx+1:]
		nextDot := strings.IndexByte(s, '.')
		if nextDot < 0 {
			cls := unescapeCSS(strings.ToLower(s))
			if part.class == "" {
				part.class = cls
			} else {
				part.classes = append(part.classes, cls)
			}
			break
		}
		cls := unescapeCSS(strings.ToLower(s[:nextDot]))
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
			return attrSelector{name: unescapeCSS(strings.ToLower(name)), op: op, value: unescapeCSS(val)}
		}
	}
	// Simple equality.
	if idx := strings.IndexByte(content, '='); idx >= 0 {
		name := strings.TrimSpace(content[:idx])
		val := strings.TrimSpace(content[idx+1:])
		val = strings.Trim(val, `"'`)
		return attrSelector{name: unescapeCSS(strings.ToLower(name)), op: "=", value: unescapeCSS(val)}
	}
	// Presence only.
	return attrSelector{name: unescapeCSS(strings.ToLower(content))}
}

// parseDeclarations parses "color: red; font-size: 12px" into key-value pairs.
func parseDeclarations(s string) []cssDecl {
	var decls []cssDecl
	for _, part := range splitDeclarationsCSS(s) {
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

// splitDeclarationsCSS splits a CSS declaration block on semicolons, but
// skips semicolons that appear inside url(...) or quoted strings. This
// prevents data URIs like url(data:font/truetype;base64,...) from being
// split at the ";base64" semicolon.
func splitDeclarationsCSS(s string) []string {
	var parts []string
	start := 0
	parenDepth := 0
	inSingle := false
	inDouble := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		// Skip backslash-escaped characters inside strings so that
		// \" and \' don't toggle the quote state.
		if ch == '\\' && (inSingle || inDouble) && i+1 < len(s) {
			i++ // skip the escaped character
			continue
		}
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '(' && !inSingle && !inDouble:
			parenDepth++
		case ch == ')' && !inSingle && !inDouble:
			if parenDepth > 0 {
				parenDepth--
			}
		case ch == ';' && !inSingle && !inDouble && parenDepth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
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
		decl        cssDecl
		specificity int
	}
	var matches []match

	// candidateRefs are sorted by (ruleIdx, selIdx), i.e. document order,
	// and include every selector that could possibly match n (bucket keys
	// are necessary conditions). So the first matching candidate of a rule
	// is the same selector a full linear scan would match first, and
	// skipRule reproduces the original scan's "break on first match".
	refs := ss.candidateRefs(n)
	skipRule := -1
	for _, ref := range refs {
		if ref.ruleIdx == skipRule {
			continue
		}
		rule := &ss.rules[ref.ruleIdx]
		sel := rule.selectors[ref.selIdx]
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
			skipRule = ref.ruleIdx
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
		decl        cssDecl
		specificity int
	}
	var matches []match

	refs := ss.candidateRefs(n)
	skipRule := -1
	for _, ref := range refs {
		if ref.ruleIdx == skipRule {
			continue
		}
		rule := &ss.rules[ref.ruleIdx]
		sel := rule.selectors[ref.selIdx]
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
			skipRule = ref.ruleIdx
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

// evaluateSupports evaluates a CSS @supports condition.
// Supports: "(property: value)", "not (condition)", "(c1) and (c2)", "(c1) or (c2)".
func evaluateSupports(condition string) bool {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return false
	}

	// Handle "not" prefix.
	if strings.HasPrefix(condition, "not ") || strings.HasPrefix(condition, "not(") {
		inner := strings.TrimPrefix(condition, "not")
		inner = strings.TrimSpace(inner)
		inner = strings.TrimPrefix(inner, "(")
		inner = strings.TrimSuffix(inner, ")")
		return !evaluateSupports(inner)
	}

	// Handle "and" / "or" combinations.
	// Simple split: find top-level "and" or "or" keywords.
	if parts := splitSupportsOperator(condition, " and "); len(parts) > 1 {
		for _, part := range parts {
			if !evaluateSupports(part) {
				return false
			}
		}
		return true
	}
	if parts := splitSupportsOperator(condition, " or "); len(parts) > 1 {
		for _, part := range parts {
			if evaluateSupports(part) {
				return true
			}
		}
		return false
	}

	// Strip outer parentheses: "(display: flex)" → "display: flex"
	condition = strings.TrimPrefix(condition, "(")
	condition = strings.TrimSuffix(condition, ")")
	condition = strings.TrimSpace(condition)

	// Extract property name (before the colon).
	colonIdx := strings.IndexByte(condition, ':')
	if colonIdx < 0 {
		return false
	}
	prop := strings.TrimSpace(condition[:colonIdx])
	return isSupportedCSSProperty(prop)
}

// splitSupportsOperator splits a @supports condition on a top-level operator,
// respecting parentheses nesting.
func splitSupportsOperator(s, op string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && i+len(op) <= len(s) && s[i:i+len(op)] == op {
			parts = append(parts, strings.TrimSpace(s[start:i]))
			start = i + len(op)
			i += len(op) - 1
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}

// isSupportedCSSProperty returns true if Folio handles the given CSS property.
func isSupportedCSSProperty(prop string) bool {
	switch strings.ToLower(strings.TrimSpace(prop)) {
	case "display", "color", "background-color", "background", "background-image",
		"font-family", "font-size", "font-weight", "font-style",
		"text-align", "text-align-last", "text-decoration", "text-transform",
		"text-indent", "text-overflow", "text-shadow",
		"line-height", "letter-spacing", "word-spacing", "word-break",
		"white-space", "hyphens",
		"margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
		"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
		"border", "border-top", "border-right", "border-bottom", "border-left",
		"border-width", "border-style", "border-color", "border-radius",
		"border-top-left-radius", "border-top-right-radius",
		"border-bottom-right-radius", "border-bottom-left-radius",
		"width", "height", "aspect-ratio", "min-width", "max-width", "min-height", "max-height",
		"opacity", "overflow", "visibility",
		"position", "top", "right", "bottom", "left", "z-index",
		"float", "clear",
		"flex", "flex-direction", "flex-wrap", "flex-grow", "flex-shrink", "flex-basis",
		"justify-content", "align-items", "align-content", "align-self",
		"gap", "row-gap", "column-gap",
		"grid-template-columns", "grid-template-rows", "grid-column", "grid-row",
		"column-count", "column-rule",
		"box-shadow", "box-sizing",
		"transform", "transform-origin",
		"page-break-before", "page-break-after", "page-break-inside",
		"orphans", "widows",
		"list-style-type", "counter-reset", "counter-increment",
		"object-fit", "object-position",
		"vertical-align":
		return true
	}
	return false
}

// Selector matching, pseudo-class evaluation, and DOM traversal helpers
// are in css_selectors.go.

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

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
		return matchesNth(childIndex(n), inner)
	case pseudo == "root":
		// :root matches the document element (<html>).
		return n.Parent != nil && n.Parent.Type == html.DocumentNode
	case pseudo == "first-of-type":
		return typeIndex(n) == 1
	case pseudo == "last-of-type":
		return isLastOfType(n)
	case strings.HasPrefix(pseudo, "nth-of-type(") && strings.HasSuffix(pseudo, ")"):
		inner := pseudo[len("nth-of-type(") : len(pseudo)-1]
		inner = strings.TrimSpace(inner)
		return matchesNth(typeIndex(n), inner)
	case strings.HasPrefix(pseudo, "nth-last-child(") && strings.HasSuffix(pseudo, ")"):
		inner := pseudo[len("nth-last-child(") : len(pseudo)-1]
		inner = strings.TrimSpace(inner)
		return matchesNth(lastChildIndex(n), inner)
	case strings.HasPrefix(pseudo, "nth-last-of-type(") && strings.HasSuffix(pseudo, ")"):
		inner := pseudo[len("nth-last-of-type(") : len(pseudo)-1]
		inner = strings.TrimSpace(inner)
		return matchesNth(lastTypeIndex(n), inner)
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

// typeIndex returns the 1-based index of n among siblings with the same tag name.
func typeIndex(n *html.Node) int {
	if n.Parent == nil || n.Type != html.ElementNode {
		return 0
	}
	idx := 0
	for sib := n.Parent.FirstChild; sib != nil; sib = sib.NextSibling {
		if sib.Type == html.ElementNode && sib.Data == n.Data {
			idx++
			if sib == n {
				return idx
			}
		}
	}
	return 0
}

// isLastOfType checks if n is the last sibling with the same tag name.
func isLastOfType(n *html.Node) bool {
	if n.Parent == nil || n.Type != html.ElementNode {
		return false
	}
	for sib := n.NextSibling; sib != nil; sib = sib.NextSibling {
		if sib.Type == html.ElementNode && sib.Data == n.Data {
			return false
		}
	}
	return true
}

// lastChildIndex returns the 1-based index of n counting from the last element child.
func lastChildIndex(n *html.Node) int {
	if n.Parent == nil {
		return 0
	}
	idx := 0
	for sib := n.Parent.LastChild; sib != nil; sib = sib.PrevSibling {
		if sib.Type == html.ElementNode {
			idx++
			if sib == n {
				return idx
			}
		}
	}
	return 0
}

// lastTypeIndex returns the 1-based index of n counting from the last sibling of the same type.
func lastTypeIndex(n *html.Node) int {
	if n.Parent == nil || n.Type != html.ElementNode {
		return 0
	}
	idx := 0
	for sib := n.Parent.LastChild; sib != nil; sib = sib.PrevSibling {
		if sib.Type == html.ElementNode && sib.Data == n.Data {
			idx++
			if sib == n {
				return idx
			}
		}
	}
	return 0
}

// matchesNth checks if a 1-based position matches an :nth-*() expression.
// Supports: "odd", "even", integer, "An+B" (e.g. "2n+1", "3n", "-n+3").
func matchesNth(pos int, expr string) bool {
	if pos <= 0 {
		return false
	}
	switch expr {
	case "odd":
		return pos%2 == 1
	case "even":
		return pos%2 == 0
	}
	// Try simple integer.
	if num, err := strconv.Atoi(expr); err == nil {
		return pos == num
	}
	// Parse An+B form.
	a, b := parseAnPlusB(expr)
	if a == 0 {
		return pos == b
	}
	// pos = a*n + b → n = (pos - b) / a, n >= 0 (or >= 1 if a > 0)
	diff := pos - b
	if diff%a != 0 {
		return false
	}
	n := diff / a
	return n >= 0
}

// parseAnPlusB parses "An+B" expressions like "2n+1", "3n", "-n+3", "n".
func parseAnPlusB(s string) (a, b int) {
	s = strings.ReplaceAll(s, " ", "")
	nIdx := strings.Index(s, "n")
	if nIdx < 0 {
		// No "n" — treat as pure B.
		b, _ = strconv.Atoi(s)
		return 0, b
	}
	// Parse A (before "n").
	aStr := s[:nIdx]
	switch aStr {
	case "", "+":
		a = 1
	case "-":
		a = -1
	default:
		a, _ = strconv.Atoi(aStr)
	}
	// Parse B (after "n").
	rest := s[nIdx+1:]
	if rest == "" {
		return a, 0
	}
	b, _ = strconv.Atoi(rest)
	return a, b
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

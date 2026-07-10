// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"reflect"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

// referenceMatchingDeclarations is a copy of matchingDeclarations' original
// linear scan, kept as a reference to prove the indexed version produces
// identical output for every node.
func referenceMatchingDeclarations(ss *styleSheet, n *xhtml.Node) []cssDecl {
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

// referenceMatchingPseudoElementDeclarations mirrors
// matchingPseudoElementDeclarations' original linear scan.
func referenceMatchingPseudoElementDeclarations(ss *styleSheet, n *xhtml.Node, pseudo string) []cssDecl {
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

// walkElements calls fn for every element node in the tree.
func walkElements(n *xhtml.Node, fn func(*xhtml.Node)) {
	if n.Type == xhtml.ElementNode {
		fn(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkElements(c, fn)
	}
}

// TestMatchingDeclarationsIndexEquivalence covers the selector shapes and
// cascade edge cases the rightmost-selector index must not disturb: bucket
// kinds (id/class/tag/universal), combinators, first-selector-wins within a
// rule, document-order tie-breaks, !important, pseudo-elements, case
// sensitivity, and duplicate node classes.
func TestMatchingDeclarationsIndexEquivalence(t *testing.T) {
	css := `
		#title { color: red; }
		.highlight { background: yellow; }
		p { margin: 1px; }
		* { box-sizing: border-box; }
		p.note#x { padding: 2px; }
		div p { color: green; }
		div > p { color: blue; }
		h1 + p { color: purple; }
		h1 ~ p { color: orange; }
		p, .highlight { font-weight: bold; }
		span.second-only { color: pink; }
		p.tie { border: 1px solid black; }
		p.tie2 { border: 2px solid black; }
		p.important-case { color: black !important; }
		:first-child { text-decoration: underline; }
		[data-x] { outline: 1px dotted; }
		p::before { content: "before"; }
		.a { color: cyan; }
	`
	docHTML := `<!DOCTYPE html><html><body>
		<p id="title" class="note x">title</p>
		<p id="x" class="note">note</p>
		<div><p>nested</p></div>
		<div><p class="direct">direct child</p></div>
		<h1>heading</h1>
		<p>after h1</p>
		<p class="highlight">both selectors match</p>
		<span class="second-only">second selector only</span>
		<p class="tie tie2">tie break</p>
		<P CLASS="Highlight" id="Title">case test</P>
		<div id="empty"></div>
		<p data-x="1">attr only</p>
		<p class="a a">duplicate class</p>
	</body></html>`

	doc, err := xhtml.Parse(strings.NewReader(docHTML))
	if err != nil {
		t.Fatalf("parse doc: %v", err)
	}

	ss := &styleSheet{}
	ss.parseCSS(css, "")

	var checked int
	walkElements(doc, func(n *xhtml.Node) {
		checked++
		got := ss.matchingDeclarations(n)
		want := referenceMatchingDeclarations(ss, n)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("matchingDeclarations mismatch for <%s id=%q class=%q>:\n got:  %+v\n want: %+v",
				n.Data, nodeAttr(n, "id"), nodeAttr(n, "class"), got, want)
		}

		for _, pseudo := range []string{"before", "after"} {
			gotPE := ss.matchingPseudoElementDeclarations(n, pseudo)
			wantPE := referenceMatchingPseudoElementDeclarations(ss, n, pseudo)
			if !reflect.DeepEqual(gotPE, wantPE) {
				t.Errorf("matchingPseudoElementDeclarations(%q) mismatch for <%s>:\n got:  %+v\n want: %+v",
					pseudo, n.Data, gotPE, wantPE)
			}
		}
	})
	if checked == 0 {
		t.Fatal("no element nodes walked")
	}
}

// TestMatchingDeclarationsPseudoElementExcluded locks that a ::before rule
// is excluded from matchingDeclarations and returned only by
// matchingPseudoElementDeclarations(n, "before").
func TestMatchingDeclarationsPseudoElementExcluded(t *testing.T) {
	ss := &styleSheet{}
	ss.parseCSS(`p::before { content: "x"; } p { color: red; }`, "")

	doc, err := xhtml.Parse(strings.NewReader(`<p>text</p>`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var p *xhtml.Node
	walkElements(doc, func(n *xhtml.Node) {
		if n.Data == "p" {
			p = n
		}
	})
	if p == nil {
		t.Fatal("no <p> found")
	}

	decls := ss.matchingDeclarations(p)
	for _, d := range decls {
		if d.property == "content" {
			t.Errorf("matchingDeclarations must not include ::before declarations, got %+v", decls)
		}
	}

	before := ss.matchingPseudoElementDeclarations(p, "before")
	if len(before) != 1 || before[0].property != "content" {
		t.Errorf("matchingPseudoElementDeclarations(before) = %+v, want [content]", before)
	}
}

// TestMatchingDeclarationsFirstSelectorWins locks that within a single
// rule, the first written selector that matches contributes the
// declarations — the winning selector's own specificity is used even
// though a later selector in the same rule would also match.
func TestMatchingDeclarationsFirstSelectorWins(t *testing.T) {
	ss := &styleSheet{}
	ss.parseCSS(`p, .highlight { font-weight: bold; }`, "")
	doc, err := xhtml.Parse(strings.NewReader(`<p class="highlight">x</p>`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var p *xhtml.Node
	walkElements(doc, func(n *xhtml.Node) {
		if n.Data == "p" {
			p = n
		}
	})
	got := ss.matchingDeclarations(p)
	want := referenceMatchingDeclarations(ss, p)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 declaration (rule matched once), got %d: %+v", len(got), got)
	}
}

// TestMatchingDeclarationsIndexRebuildsAfterAppend locks the lazy rebuild
// guard: rules appended to ss.rules after a first query must be visible to
// subsequent queries.
func TestMatchingDeclarationsIndexRebuildsAfterAppend(t *testing.T) {
	ss := &styleSheet{}
	ss.parseCSS(`p { color: red; }`, "")

	doc, err := xhtml.Parse(strings.NewReader(`<p>x</p>`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var p *xhtml.Node
	walkElements(doc, func(n *xhtml.Node) {
		if n.Data == "p" {
			p = n
		}
	})

	first := ss.matchingDeclarations(p)
	if len(first) != 1 {
		t.Fatalf("expected 1 declaration before append, got %d", len(first))
	}

	extra := &styleSheet{}
	extra.parseCSS(`p { font-size: 12px; }`, "")
	ss.rules = append(ss.rules, extra.rules...)

	second := ss.matchingDeclarations(p)
	if len(second) != 2 {
		t.Fatalf("expected 2 declarations after append, got %d: %+v", len(second), second)
	}
}

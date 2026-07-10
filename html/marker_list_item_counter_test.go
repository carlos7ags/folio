// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
	"testing"
)

// The implicit "list-item" counter (CSS Lists 3 §6.1): every <li>
// increments it by 1 and every <ul>/<ol> resets it to 0, so
// `content: counter(list-item)` numbers items without any author
// counter-reset/counter-increment declaration.

// Regression test for issue #378 gap D: counter(list-item) previously always
// resolved to 0 because the implicit reset/increment was never applied.
func TestMarkerListItemCounterBasic(t *testing.T) {
	htmlStr := `<style>
		ul.notes li::marker { content: counter(list-item) ". "; }
	</style>
	<ul class="notes"><li>Alpha</li><li>Beta</li><li>Gamma</li></ul>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	for _, want := range []string{"(1.) Tj", "(2.) Tj", "(3.) Tj", "(Alpha) Tj", "(Beta) Tj", "(Gamma) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing %q\nstream:\n%s", want, stream)
		}
	}
	if strings.Contains(stream, "(0.) Tj") {
		t.Errorf("marker still reads 0 — implicit list-item counter not applied\nstream:\n%s", stream)
	}
}

// A nested list must restart list-item at 1 rather than continuing the
// outer list's count, proving the implicit reset scopes per list root.
func TestMarkerListItemCounterNestedRestarts(t *testing.T) {
	htmlStr := `<style>
		li::marker { content: counter(list-item) ". " }
	</style>
	<ol><li>Alpha<ol><li>Beta</li><li>Gamma</li></ol></li><li>Delta</li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	for _, want := range []string{"(1.) Tj", "(2.) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing %q\nstream:\n%s", want, stream)
		}
	}
	if strings.Contains(stream, "(3.) Tj") || strings.Contains(stream, "(4.) Tj") {
		t.Errorf("nested list did not restart list-item at 1\nstream:\n%s", stream)
	}
}

// counters(list-item, ".") composes the built-in counter across nesting
// levels the same way an author-declared counter does.
func TestMarkerListItemCounterMultiLevel(t *testing.T) {
	htmlStr := `<style>
		li::marker { content: counters(list-item, ".") ". " }
	</style>
	<ol><li>Alpha<ol><li>Beta</li><li>Gamma</li></ol></li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	for _, want := range []string{"(1.) Tj", "(1.1) Tj", "(1.2) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing %q\nstream:\n%s", want, stream)
		}
	}
}

// An explicit author counter-increment for "list-item" must win over the
// implicit +1 — no double counting.
func TestMarkerListItemCounterExplicitIncrementWins(t *testing.T) {
	htmlStr := `<style>
		li { counter-increment: list-item 5 }
		li::marker { content: counter(list-item) ". " }
	</style>
	<ol><li>A</li><li>B</li><li>C</li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	for _, want := range []string{"(5.) Tj", "(10.) Tj", "(15.) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing %q\nstream:\n%s", want, stream)
		}
	}
	if strings.Contains(stream, "(1.) Tj") || strings.Contains(stream, "(6.) Tj") {
		t.Errorf("implicit +1 appears to have also applied (double counting)\nstream:\n%s", stream)
	}
}

// An explicit author counter-reset for "list-item" on the list root must
// win over the implicit reset to 0.
func TestMarkerListItemCounterExplicitResetWins(t *testing.T) {
	htmlStr := `<style>
		ol { counter-reset: list-item 10 }
		li::marker { content: counter(list-item) ". " }
	</style>
	<ol><li>A</li><li>B</li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	if !strings.Contains(stream, "(11.) Tj") {
		t.Errorf("expected first marker to be 11. (10 + implicit +1)\nstream:\n%s", stream)
	}
	if strings.Contains(stream, "(1.) Tj") {
		t.Errorf("explicit counter-reset: list-item 10 did not win over the implicit reset\nstream:\n%s", stream)
	}
}

// Pin the nested-list fast path specifically: a plain inline <li> containing
// a nested list with no box-model styles (the AddItemRunsWithSubList branch
// in populateList). This is the path most likely to silently regress since
// it bypasses convertElement entirely.
func TestMarkerListItemCounterNestedFastPath(t *testing.T) {
	htmlStr := `<style>
		li::marker { content: counter(list-item) ". " }
	</style>
	<ul><li>Alpha<ul><li>Beta</li><li>Gamma</li></ul></li></ul>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	for _, want := range []string{"(1.) Tj", "(2.) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing %q\nstream:\n%s", want, stream)
		}
	}
	if strings.Contains(stream, "(3.) Tj") {
		t.Errorf("nested fast-path list did not restart list-item at 1\nstream:\n%s", stream)
	}
}

// The element path (an <li> with box-model styles, forcing addElementListItem
// via a block child) must also number correctly.
func TestMarkerListItemCounterElementPath(t *testing.T) {
	htmlStr := `<style>
		li::marker { content: counter(list-item) ") " }
	</style>
	<ol><li><p>First</p></li><li><p>Second</p></li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	for _, want := range []string{`(1\)) Tj`, `(2\)) Tj`, "(First) Tj", "(Second) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing %q\nstream:\n%s", want, stream)
		}
	}
}

// Without any ::marker content, the implicit list-item counter must be
// inert — the style-derived marker numbering (List.marker()/l.start) is
// unaffected by the new counter plumbing.
func TestMarkerListItemCounterInertWithoutMarkerContent(t *testing.T) {
	htmlStr := `<ol><li>A</li><li>B</li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	for _, want := range []string{"(1.) Tj", "(2.) Tj", "(A) Tj", "(B) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing %q\nstream:\n%s", want, stream)
		}
	}
}

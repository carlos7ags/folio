// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"
)

// TestFindKashidaCandidatesEmpty verifies the empty-string base case.
func TestFindKashidaCandidatesEmpty(t *testing.T) {
	if got := FindKashidaCandidates(""); got != nil {
		t.Errorf("empty input: got %v, want nil", got)
	}
}

// TestFindKashidaCandidatesLatin verifies that pure Latin text yields no
// candidates: no rune has a join side, so every boundary is rejected.
func TestFindKashidaCandidatesLatin(t *testing.T) {
	if got := FindKashidaCandidates("hello world"); len(got) != 0 {
		t.Errorf("latin text: got %d candidates, want 0", len(got))
	}
}

// TestFindKashidaCandidatesSalaam exercises the canonical example: the
// Arabic word "سلام" (salaam) contains a seen-family letter immediately
// followed by a dual-joining letter, which must produce at least one
// candidate at the high-priority "after seen" site.
func TestFindKashidaCandidatesSalaam(t *testing.T) {
	got := FindKashidaCandidates("سلام")
	if len(got) == 0 {
		t.Fatal("expected at least one candidate in 'سلام'")
	}
	sawSeenPriority := false
	for _, c := range got {
		if c.Priority == kashidaPriorityAfterSeen {
			sawSeenPriority = true
			break
		}
	}
	if !sawSeenPriority {
		t.Errorf("expected an after-seen priority candidate; got %+v", got)
	}
}

// TestFindKashidaCandidatesBayt verifies that "بيت" (bayt, "house") — three
// dual-joining letters — produces at least one candidate at a basic or
// medial-pair priority site.
func TestFindKashidaCandidatesBayt(t *testing.T) {
	got := FindKashidaCandidates("بيت")
	if len(got) == 0 {
		t.Fatal("expected at least one candidate in 'بيت'")
	}
	for _, c := range got {
		if c.Priority < kashidaPriorityBasic || c.Priority > kashidaPriorityAfterSeen {
			t.Errorf("candidate %+v has out-of-range priority", c)
		}
	}
}

// TestInsertKashidasSalaamOne verifies that inserting one tatweel into
// "سلام" produces a string with exactly one U+0640 in it, and that the
// length grows by exactly one rune.
func TestInsertKashidasSalaamOne(t *testing.T) {
	in := "سلام"
	out := InsertKashidas(in, 1)
	if out == in {
		t.Fatal("expected insertion to change the string")
	}
	if got := strings.Count(out, string(kashidaTatweel)); got != 1 {
		t.Errorf("tatweel count: got %d, want 1", got)
	}
	if len([]rune(out)) != len([]rune(in))+1 {
		t.Errorf("rune length: got %d, want %d", len([]rune(out)), len([]rune(in))+1)
	}
}

// TestInsertKashidasMoreThanSites checks that requesting more tatweels
// than the word has legal sites caps at the available count rather than
// piling duplicates onto a single site.
func TestInsertKashidasMoreThanSites(t *testing.T) {
	in := "سلام"
	sites := len(FindKashidaCandidates(in))
	out := InsertKashidas(in, sites+10)
	got := strings.Count(out, string(kashidaTatweel))
	if got > sites {
		t.Errorf("inserted %d tatweels but only %d legal sites exist", got, sites)
	}
	if got != sites {
		t.Errorf("expected to fill all %d sites; got %d", sites, got)
	}
}

// TestInsertKashidasRoundTrip verifies that the result of an insertion
// is itself a valid carrier for further kashida sites — tatweel is
// dual-joining so each insertion creates two new candidate boundaries
// around it. The post-insertion candidate count should be at least the
// pre-insertion count.
func TestInsertKashidasRoundTrip(t *testing.T) {
	in := "سلام"
	before := len(FindKashidaCandidates(in))
	out := InsertKashidas(in, 1)
	after := len(FindKashidaCandidates(out))
	if after < before {
		t.Errorf("post-insertion candidate count shrank: before=%d after=%d", before, after)
	}
}

// TestInsertKashidasLatinNoOp verifies that pure Latin strings are
// returned unchanged regardless of the requested count.
func TestInsertKashidasLatinNoOp(t *testing.T) {
	in := "hello world"
	out := InsertKashidas(in, 5)
	if out != in {
		t.Errorf("latin string was mutated: got %q want %q", out, in)
	}
}

// TestInsertKashidasZeroCount verifies that count=0 is a no-op even on
// Arabic input.
func TestInsertKashidasZeroCount(t *testing.T) {
	in := "سلام"
	out := InsertKashidas(in, 0)
	if out != in {
		t.Errorf("count=0 mutated the string: got %q want %q", out, in)
	}
}

// TestInsertKashidasNegativeCount verifies the precondition guard:
// negative counts are treated as zero, not as a panic.
func TestInsertKashidasNegativeCount(t *testing.T) {
	in := "سلام"
	out := InsertKashidas(in, -3)
	if out != in {
		t.Errorf("negative count mutated the string: got %q want %q", out, in)
	}
}

// TestKashidaCandidatesPostShape verifies that the candidate finder also
// works on already-shaped Presentation Forms-B text. We shape "سلام"
// first and then look for candidates in the shaped string. The shaped
// glyphs are PFB codepoints and must still classify correctly via the
// PFB reverse-lookup table.
func TestKashidaCandidatesPostShape(t *testing.T) {
	shaped := ShapeArabic("سلام")
	got := FindKashidaCandidates(shaped)
	if len(got) == 0 {
		t.Fatalf("expected candidates in shaped 'سلام' (%q)", shaped)
	}
}

// TestSpreadPickAlternates verifies that spreadPick alternates between
// the front and back of the bucket so equal-priority sites are spread
// across the word rather than clustered.
func TestSpreadPickAlternates(t *testing.T) {
	bucket := []int{1, 2, 3, 4, 5}
	got := spreadPick(bucket, 3)
	if len(got) != 3 {
		t.Fatalf("len: got %d want 3", len(got))
	}
	// Expected order: front (1), back (5), front+1 (2).
	want := []int{1, 5, 2}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("spreadPick[%d]: got %d want %d", i, got[i], v)
		}
	}
}

// TestPickKashidaSitesPriorityOrder verifies that higher-priority sites
// are picked before lower-priority ones.
func TestPickKashidaSitesPriorityOrder(t *testing.T) {
	cands := []KashidaCandidate{
		{Position: 10, Priority: kashidaPriorityBasic},
		{Position: 20, Priority: kashidaPriorityAfterSeen},
		{Position: 30, Priority: kashidaPriorityBasic},
	}
	got := pickKashidaSites(cands, 1)
	if len(got) != 1 || got[0] != 20 {
		t.Errorf("expected highest-priority site at 20; got %v", got)
	}
}

// TestInsertKashidasInsertionAtBoundary asserts that insertion happens at
// rune boundaries (no UTF-8 corruption). The result must be valid UTF-8.
func TestInsertKashidasInsertionAtBoundary(t *testing.T) {
	in := "سلام"
	out := InsertKashidas(in, 2)
	if !isValidUTF8(out) {
		t.Errorf("result is not valid UTF-8: %q", out)
	}
}

// isValidUTF8 reports whether s contains only well-formed UTF-8 sequences.
func isValidUTF8(s string) bool {
	for i, r := range s {
		if r == 0xFFFD && len(s[i:]) > 0 && s[i] != 0xEF {
			return false
		}
	}
	return true
}

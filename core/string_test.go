// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

func TestEscapeLiteralStringControlCharNull(t *testing.T) {
	got := EscapeLiteralString(string([]byte{0x00}))
	expected := `\000`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestEscapeLiteralStringControlCharSOH(t *testing.T) {
	got := EscapeLiteralString(string([]byte{0x01}))
	expected := `\001`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestEscapeLiteralStringControlCharUS(t *testing.T) {
	// 0x1F is the last control character before space (0x20)
	got := EscapeLiteralString(string([]byte{0x1F}))
	expected := `\037`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestEscapeLiteralStringMixedControlChars(t *testing.T) {
	// Mix of a normal char, a control char, and a newline
	input := "A" + string([]byte{0x00}) + "\n" + "B"
	got := EscapeLiteralString(input)
	expected := `A\000\nB`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

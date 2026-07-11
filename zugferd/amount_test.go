// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package zugferd

import "testing"

func TestAmountString(t *testing.T) {
	tests := []struct {
		amount Amount
		want   string
	}{
		{NewAmount(401, 63), "401.63"},
		{NewAmount(0, 0), "0.00"},
		{NewAmount(5, 0), "5.00"},
		{NewAmount(-12, 50), "-12.50"},
	}
	for _, tt := range tests {
		if got := tt.amount.String(); got != tt.want {
			t.Errorf("Amount(%d).String() = %q, want %q", int64(tt.amount), got, tt.want)
		}
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		in   string
		want Amount
	}{
		{"401.63", NewAmount(401, 63)},
		{"50", NewAmount(50, 0)},
		{"12.5", NewAmount(12, 50)},
		{"-12.50", NewAmount(-12, 50)},
		{"0.00", 0},
	}
	for _, tt := range tests {
		got, err := ParseAmount(tt.in)
		if err != nil {
			t.Errorf("ParseAmount(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseAmount(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseAmountInvalid(t *testing.T) {
	tests := []string{"", ".", "1.234", "abc", "1.2.3", "1e10", "-", "1."}
	for _, in := range tests {
		if _, err := ParseAmount(in); err == nil {
			t.Errorf("ParseAmount(%q) = nil error, want error", in)
		}
	}
}

func TestAmountRoundTrip(t *testing.T) {
	for _, s := range []string{"401.63", "0.00", "999999.99", "-42.10"} {
		a, err := ParseAmount(s)
		if err != nil {
			t.Fatalf("ParseAmount(%q): %v", s, err)
		}
		if got := a.String(); got != s {
			t.Errorf("ParseAmount(%q).String() = %q, want %q", s, got, s)
		}
	}
}

func TestAmountAdd(t *testing.T) {
	a := NewAmount(337, 50)
	b := NewAmount(64, 13)
	if got, want := a.Add(b), NewAmount(401, 63); got != want {
		t.Errorf("Add = %v, want %v", got, want)
	}
}

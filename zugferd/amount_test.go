// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package zugferd

import (
	"math"
	"testing"
)

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

func TestParseAmountOverflow(t *testing.T) {
	tests := []string{
		"4611686018427387903.99",
		"92233720368547758.08",
		"99999999999999999999.00",
		"-92233720368547758.08",
	}
	for _, in := range tests {
		got, err := ParseAmount(in)
		if err == nil {
			t.Errorf("ParseAmount(%q) = %d, nil, want error", in, got)
		}
		if got != 0 {
			t.Errorf("ParseAmount(%q) amount = %d, want 0 on error", in, got)
		}
	}
}

func TestParseAmountBoundary(t *testing.T) {
	got, err := ParseAmount("92233720368547758.07")
	if err != nil {
		t.Fatalf("ParseAmount(max) error: %v", err)
	}
	if got != Amount(math.MaxInt64) {
		t.Errorf("ParseAmount(max) = %d, want %d", got, int64(math.MaxInt64))
	}
}

func TestAmountStringMinInt64(t *testing.T) {
	got := Amount(math.MinInt64).String()
	want := "-92233720368547758.08"
	if got != want {
		t.Errorf("Amount(MinInt64).String() = %q, want %q", got, want)
	}
}

func TestAddChecked(t *testing.T) {
	if sum, ok := Amount(1).AddChecked(Amount(2)); !ok || sum != 3 {
		t.Errorf("AddChecked(1,2) = %d, %v, want 3, true", sum, ok)
	}
	if _, ok := Amount(math.MaxInt64).AddChecked(Amount(1)); ok {
		t.Errorf("AddChecked(MaxInt64, 1) ok = true, want false")
	}
	if _, ok := Amount(math.MinInt64).AddChecked(Amount(-1)); ok {
		t.Errorf("AddChecked(MinInt64, -1) ok = true, want false")
	}
}

func TestNewAmountChecked(t *testing.T) {
	if _, err := NewAmountChecked(math.MaxInt64/100+1, 0); err == nil {
		t.Errorf("NewAmountChecked overflow = nil error, want error")
	}
}

func TestNewAmountPanicsOnOverflow(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("NewAmount overflow did not panic")
		}
	}()
	NewAmount(math.MaxInt64/100+1, 0)
}

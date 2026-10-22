// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package zugferd

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Amount is a monetary value stored as an integer number of minor
// units (cents), never a float, so invoice totals stay exact and
// XML output stays byte-stable across runs. It assumes a two-decimal
// currency (EUR, USD, GBP, ...); currencies with a different minor-unit
// scale (e.g. JPY, BHD) are not supported by this spike.
type Amount int64

// NewAmount builds an Amount from a whole-unit and cents part, e.g.
// NewAmount(401, 63) is 401.63. Panics if the result would overflow
// int64; NewAmount arguments are almost always literals, so an overflow
// here is a programmer error. For data-driven input, use
// NewAmountChecked instead.
func NewAmount(units, cents int64) Amount {
	a, err := NewAmountChecked(units, cents)
	if err != nil {
		panic("zugferd: NewAmount overflow")
	}
	return a
}

// NewAmountChecked builds an Amount from a whole-unit and cents part,
// reporting an error instead of overflowing when units/cents come from
// untrusted or data-driven input.
func NewAmountChecked(units, cents int64) (Amount, error) {
	if units < 0 {
		if units < -(math.MaxInt64-cents)/100 {
			return 0, fmt.Errorf("zugferd: amount overflows: units=%d cents=%d", units, cents)
		}
		return Amount(units*100 - cents), nil
	}
	if units > (math.MaxInt64-cents)/100 {
		return 0, fmt.Errorf("zugferd: amount overflows: units=%d cents=%d", units, cents)
	}
	return Amount(units*100 + cents), nil
}

// ParseAmount parses a fixed-point decimal string (e.g. "401.63",
// "50", "-12.5") into an Amount. It rejects scientific notation and
// more than two fractional digits.
func ParseAmount(s string) (Amount, error) {
	neg := strings.HasPrefix(s, "-")
	unsigned := strings.TrimPrefix(s, "-")
	whole, frac, hasFrac := strings.Cut(unsigned, ".")
	if whole == "" || (hasFrac && frac == "") || len(frac) > 2 {
		return 0, fmt.Errorf("zugferd: invalid amount %q", s)
	}
	for _, r := range whole + frac {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("zugferd: invalid amount %q", s)
		}
	}
	for len(frac) < 2 {
		frac += "0"
	}
	// int64 max minor units is 92233720368547758.07, i.e. 17 whole
	// digits; reject anything longer before it reaches ParseInt/the
	// multiply below.
	if len(strings.TrimLeft(whole, "0")) > 17 {
		return 0, fmt.Errorf("zugferd: invalid amount %q", s)
	}
	w, err := strconv.ParseInt(whole, 10, 63)
	if err != nil {
		return 0, fmt.Errorf("zugferd: invalid amount %q: %w", s, err)
	}
	c, err := strconv.ParseInt(frac, 10, 63)
	if err != nil {
		return 0, fmt.Errorf("zugferd: invalid amount %q: %w", s, err)
	}
	if w > (math.MaxInt64-c)/100 {
		return 0, fmt.Errorf("zugferd: amount %q overflows", s)
	}
	minor := w*100 + c
	if neg {
		minor = -minor
	}
	return Amount(minor), nil
}

// Add returns a + b. It wraps silently on overflow; callers that need
// overflow detection (e.g. invoice validation) should use AddChecked.
func (a Amount) Add(b Amount) Amount {
	return a + b
}

// AddChecked returns a + b, reporting ok=false if the sum overflows int64.
func (a Amount) AddChecked(b Amount) (Amount, bool) {
	s := a + b
	if (b > 0 && s < a) || (b < 0 && s > a) {
		return 0, false
	}
	return s, true
}

// String renders the amount as a fixed two-decimal string (e.g. "401.63"),
// the format CII expects for ram:*Amount elements.
func (a Amount) String() string {
	v := int64(a)
	if v >= 0 {
		return fmt.Sprintf("%d.%02d", v/100, v%100)
	}
	// v is negative; negating v directly would overflow when
	// v == math.MinInt64 (its positive counterpart isn't representable
	// as int64). Instead split first: Go truncates division toward
	// zero, so whole and cents are both <= 0 and representable, and
	// only then do we negate the (safe) parts.
	whole, cents := v/100, v%100
	return fmt.Sprintf("-%d.%02d", -whole, -cents)
}

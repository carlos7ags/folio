// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package zugferd

import (
	"fmt"
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
// NewAmount(401, 63) is 401.63.
func NewAmount(units, cents int64) Amount {
	if units < 0 {
		return Amount(units*100 - cents)
	}
	return Amount(units*100 + cents)
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
	w, err := strconv.ParseInt(whole, 10, 63)
	if err != nil {
		return 0, fmt.Errorf("zugferd: invalid amount %q: %w", s, err)
	}
	c, err := strconv.ParseInt(frac, 10, 63)
	if err != nil {
		return 0, fmt.Errorf("zugferd: invalid amount %q: %w", s, err)
	}
	minor := w*100 + c
	if neg {
		minor = -minor
	}
	return Amount(minor), nil
}

// Add returns a + b.
func (a Amount) Add(b Amount) Amount {
	return a + b
}

// String renders the amount as a fixed two-decimal string (e.g. "401.63"),
// the format CII expects for ram:*Amount elements.
func (a Amount) String() string {
	neg := a < 0
	v := int64(a)
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%d.%02d", v/100, v%100)
	if neg {
		s = "-" + s
	}
	return s
}

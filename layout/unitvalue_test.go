// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"
	"testing"
)

func TestPt(t *testing.T) {
	u := Pt(72)
	if u.Unit != UnitPoint {
		t.Errorf("Unit = %d, want UnitPoint", u.Unit)
	}
	if u.Value != 72 {
		t.Errorf("Value = %.1f, want 72", u.Value)
	}
	if got := u.Resolve(500); got != 72 {
		t.Errorf("Resolve(500) = %.1f, want 72", got)
	}
}

func TestPct(t *testing.T) {
	u := Pct(50)
	if u.Unit != UnitPercent {
		t.Errorf("Unit = %d, want UnitPercent", u.Unit)
	}
	if got := u.Resolve(400); got != 200 {
		t.Errorf("Resolve(400) = %.1f, want 200", got)
	}
}

func TestResolveAll(t *testing.T) {
	values := []UnitValue{Pct(30), Pct(70)}
	result := ResolveAll(values, 500)
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if math.Abs(result[0]-150) > 0.01 {
		t.Errorf("result[0] = %.2f, want 150", result[0])
	}
	if math.Abs(result[1]-350) > 0.01 {
		t.Errorf("result[1] = %.2f, want 350", result[1])
	}
}

func TestResolveAllMixed(t *testing.T) {
	// 100pt fixed + 50% of remaining
	values := []UnitValue{Pt(100), Pct(50)}
	result := ResolveAll(values, 400)
	if result[0] != 100 {
		t.Errorf("result[0] = %.1f, want 100", result[0])
	}
	// 50% of 400 = 200 (percentage is of total width)
	if result[1] != 200 {
		t.Errorf("result[1] = %.1f, want 200", result[1])
	}
}

func TestPctZero(t *testing.T) {
	u := Pct(0)
	if got := u.Resolve(500); got != 0 {
		t.Errorf("Pct(0).Resolve(500) = %.1f, want 0", got)
	}
}

func TestPctFull(t *testing.T) {
	u := Pct(100)
	if got := u.Resolve(468); got != 468 {
		t.Errorf("Pct(100).Resolve(468) = %.1f, want 468", got)
	}
}

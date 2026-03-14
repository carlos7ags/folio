// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"
)

func TestCalcSimpleSubtract(t *testing.T) {
	// calc(100% - 40px) at 500pt available width
	l := parseLength("calc(100% - 40px)")
	if l == nil {
		t.Fatal("parseLength returned nil for calc")
	}
	if l.calc == nil {
		t.Fatal("expected calc expression")
	}
	// 100% of 500 = 500, 40px = 30pt, result = 470
	got := l.toPoints(500, 12)
	if math.Abs(got-470) > 0.1 {
		t.Errorf("calc(100%% - 40px) at 500pt = %.1f, want 470", got)
	}
}

func TestCalcSimpleAdd(t *testing.T) {
	l := parseLength("calc(50% + 20px)")
	if l == nil || l.calc == nil {
		t.Fatal("expected calc")
	}
	// 50% of 400 = 200, 20px = 15pt, result = 215
	got := l.toPoints(400, 12)
	if math.Abs(got-215) > 0.1 {
		t.Errorf("calc(50%% + 20px) at 400pt = %.1f, want 215", got)
	}
}

func TestCalcMultiply(t *testing.T) {
	l := parseLength("calc(2 * 50px)")
	if l == nil || l.calc == nil {
		t.Fatal("expected calc")
	}
	// 2 * 50px = 2 * 37.5pt = 75pt
	got := l.toPoints(0, 12)
	if math.Abs(got-75) > 0.1 {
		t.Errorf("calc(2 * 50px) = %.1f, want 75", got)
	}
}

func TestCalcDivide(t *testing.T) {
	l := parseLength("calc(100% / 3)")
	if l == nil || l.calc == nil {
		t.Fatal("expected calc")
	}
	// 100% of 600 / 3 = 200
	got := l.toPoints(600, 12)
	if math.Abs(got-200) > 0.1 {
		t.Errorf("calc(100%% / 3) at 600pt = %.1f, want 200", got)
	}
}

func TestCalcWithEm(t *testing.T) {
	l := parseLength("calc(100% - 2em)")
	if l == nil || l.calc == nil {
		t.Fatal("expected calc")
	}
	// 100% of 500 = 500, 2em at 14pt = 28, result = 472
	got := l.toPoints(500, 14)
	if math.Abs(got-472) > 0.1 {
		t.Errorf("calc(100%% - 2em) at 500pt/14pt = %.1f, want 472", got)
	}
}

func TestCalcWithPt(t *testing.T) {
	l := parseLength("calc(100% - 72pt)")
	if l == nil || l.calc == nil {
		t.Fatal("expected calc")
	}
	// 100% of 612 = 612, 72pt = 72, result = 540
	got := l.toPoints(612, 12)
	if math.Abs(got-540) > 0.1 {
		t.Errorf("calc(100%% - 72pt) at 612pt = %.1f, want 540", got)
	}
}

func TestCalcNotCalc(t *testing.T) {
	// Regular length should still work.
	l := parseLength("100px")
	if l == nil {
		t.Fatal("expected length")
	}
	if l.calc != nil {
		t.Error("plain length should not have calc")
	}
	got := l.toPoints(0, 12)
	if math.Abs(got-75) > 0.1 {
		t.Errorf("100px = %.1f, want 75", got)
	}
}

func TestCalcInvalid(t *testing.T) {
	l := parseLength("calc()")
	if l != nil {
		t.Error("empty calc should return nil")
	}

	l = parseLength("calc(nonsense)")
	if l != nil {
		t.Error("invalid calc should return nil")
	}
}

func TestCalcDivideByZero(t *testing.T) {
	l := parseLength("calc(100px / 0)")
	if l == nil || l.calc == nil {
		t.Fatal("expected calc")
	}
	got := l.toPoints(0, 12)
	if got != 0 {
		t.Errorf("divide by zero should return 0, got %.1f", got)
	}
}

func TestCalcComplexExpression(t *testing.T) {
	// calc(100% - 40px - 40px) = 100% - 80px
	l := parseLength("calc(100% - 40px - 40px)")
	if l == nil || l.calc == nil {
		t.Fatal("expected calc")
	}
	// 100% of 500 = 500, 80px = 60pt, result = 440
	got := l.toPoints(500, 12)
	if math.Abs(got-440) > 0.1 {
		t.Errorf("calc(100%% - 40px - 40px) at 500pt = %.1f, want 440", got)
	}
}

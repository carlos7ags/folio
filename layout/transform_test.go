// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"
	"testing"
)

func approxEq(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

func TestComputeTransformMatrixIdentity(t *testing.T) {
	a, b, c, d, e, f := ComputeTransformMatrix(nil)
	if a != 1 || b != 0 || c != 0 || d != 1 || e != 0 || f != 0 {
		t.Errorf("identity: got [%f %f %f %f %f %f]", a, b, c, d, e, f)
	}
}

func TestComputeTransformMatrixRotate(t *testing.T) {
	ops := []TransformOp{{Type: "rotate", Values: [2]float64{90, 0}}}
	a, b, c, d, _, _ := ComputeTransformMatrix(ops)
	// cos(90°) ≈ 0, sin(90°) ≈ 1
	if !approxEq(a, 0, 1e-9) || !approxEq(b, 1, 1e-9) || !approxEq(c, -1, 1e-9) || !approxEq(d, 0, 1e-9) {
		t.Errorf("rotate(90): got [%f %f %f %f]", a, b, c, d)
	}
}

func TestComputeTransformMatrixScale(t *testing.T) {
	ops := []TransformOp{{Type: "scale", Values: [2]float64{2, 3}}}
	a, b, c, d, e, f := ComputeTransformMatrix(ops)
	if a != 2 || b != 0 || c != 0 || d != 3 || e != 0 || f != 0 {
		t.Errorf("scale(2,3): got [%f %f %f %f %f %f]", a, b, c, d, e, f)
	}
}

func TestComputeTransformMatrixTranslate(t *testing.T) {
	ops := []TransformOp{{Type: "translate", Values: [2]float64{10, 20}}}
	a, b, c, d, e, f := ComputeTransformMatrix(ops)
	if a != 1 || b != 0 || c != 0 || d != 1 || e != 10 || f != 20 {
		t.Errorf("translate(10,20): got [%f %f %f %f %f %f]", a, b, c, d, e, f)
	}
}

func TestComputeTransformMatrixSkewX(t *testing.T) {
	ops := []TransformOp{{Type: "skewX", Values: [2]float64{45, 0}}}
	a, b, c, d, _, _ := ComputeTransformMatrix(ops)
	// tan(45°) = 1
	if !approxEq(a, 1, 1e-9) || !approxEq(b, 0, 1e-9) || !approxEq(c, 1, 1e-9) || !approxEq(d, 1, 1e-9) {
		t.Errorf("skewX(45): got [%f %f %f %f]", a, b, c, d)
	}
}

func TestComputeTransformMatrixSkewY(t *testing.T) {
	ops := []TransformOp{{Type: "skewY", Values: [2]float64{45, 0}}}
	a, b, c, d, _, _ := ComputeTransformMatrix(ops)
	// tan(45°) = 1
	if !approxEq(a, 1, 1e-9) || !approxEq(b, 1, 1e-9) || !approxEq(c, 0, 1e-9) || !approxEq(d, 1, 1e-9) {
		t.Errorf("skewY(45): got [%f %f %f %f]", a, b, c, d)
	}
}

func TestComputeTransformMatrixMultiple(t *testing.T) {
	// rotate(90) then scale(2,2)
	ops := []TransformOp{
		{Type: "rotate", Values: [2]float64{90, 0}},
		{Type: "scale", Values: [2]float64{2, 2}},
	}
	a, b, c, d, _, _ := ComputeTransformMatrix(ops)
	// rotate(90): [0 1 -1 0] * scale(2,2): [2 0 0 2]
	// result: [0*2+1*0, 0*0+1*2, -1*2+0*0, -1*0+0*2] = [0 2 -2 0]
	if !approxEq(a, 0, 1e-9) || !approxEq(b, 2, 1e-9) || !approxEq(c, -2, 1e-9) || !approxEq(d, 0, 1e-9) {
		t.Errorf("rotate(90)+scale(2,2): got [%f %f %f %f]", a, b, c, d)
	}
}

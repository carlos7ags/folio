// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "math"

// TransformOp represents a single CSS transform function.
type TransformOp struct {
	Type   string     // "rotate", "scale", "translate", "skewX", "skewY"
	Values [2]float64 // up to 2 parameters (e.g. scaleX, scaleY)
}

// ComputeTransformMatrix multiplies all transform operations in order
// and returns the combined 2D affine matrix [a b c d e f].
//
// The PDF transformation matrix maps to:
//
//	| a  b  0 |
//	| c  d  0 |
//	| e  f  1 |
func ComputeTransformMatrix(ops []TransformOp) (a, b, c, d, e, f float64) {
	// Start with identity matrix.
	a, b, c, d, e, f = 1, 0, 0, 1, 0, 0

	for _, op := range ops {
		var oa, ob, oc, od, oe, of float64
		switch op.Type {
		case "rotate":
			rad := op.Values[0] * math.Pi / 180
			cos := math.Cos(rad)
			sin := math.Sin(rad)
			oa, ob, oc, od, oe, of = cos, sin, -sin, cos, 0, 0
		case "scale":
			sx := op.Values[0]
			sy := op.Values[1]
			oa, ob, oc, od, oe, of = sx, 0, 0, sy, 0, 0
		case "translate":
			tx := op.Values[0]
			ty := op.Values[1]
			oa, ob, oc, od, oe, of = 1, 0, 0, 1, tx, ty
		case "skewX":
			rad := op.Values[0] * math.Pi / 180
			oa, ob, oc, od, oe, of = 1, 0, math.Tan(rad), 1, 0, 0
		case "skewY":
			rad := op.Values[0] * math.Pi / 180
			oa, ob, oc, od, oe, of = 1, math.Tan(rad), 0, 1, 0, 0
		default:
			continue
		}
		// Multiply: result = current * op
		// [a b 0]   [oa ob 0]
		// [c d 0] x [oc od 0]
		// [e f 1]   [oe of 1]
		na := a*oa + b*oc
		nb := a*ob + b*od
		nc := c*oa + d*oc
		nd := c*ob + d*od
		ne := e*oa + f*oc + oe
		nf := e*ob + f*od + of
		a, b, c, d, e, f = na, nb, nc, nd, ne, nf
	}
	return
}

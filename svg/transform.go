// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Matrix is a 2D affine transformation matrix [a b c d e f].
// Maps to PDF cm operator: [a b c d e f] means:
//
//	x' = a*x + c*y + e
//	y' = b*x + d*y + f
type Matrix struct {
	A, B, C, D, E, F float64
}

// Identity returns the identity matrix.
func identity() Matrix { return Matrix{A: 1, D: 1} }

// Translate returns a translation matrix.
func Translate(tx, ty float64) Matrix {
	return Matrix{A: 1, D: 1, E: tx, F: ty}
}

// Scale returns a scaling matrix.
func Scale(sx, sy float64) Matrix {
	return Matrix{A: sx, D: sy}
}

// Rotate returns a rotation matrix for the given angle in degrees.
func Rotate(angleDeg float64) Matrix {
	rad := angleDeg * math.Pi / 180
	cos := math.Cos(rad)
	sin := math.Sin(rad)
	return Matrix{A: cos, B: sin, C: -sin, D: cos}
}

// SkewX returns a skew-X matrix for the given angle in degrees.
func SkewX(angleDeg float64) Matrix {
	rad := angleDeg * math.Pi / 180
	return Matrix{A: 1, C: math.Tan(rad), D: 1}
}

// SkewY returns a skew-Y matrix for the given angle in degrees.
func SkewY(angleDeg float64) Matrix {
	rad := angleDeg * math.Pi / 180
	return Matrix{A: 1, B: math.Tan(rad), D: 1}
}

// Multiply returns the product m * n, applying n first then m.
func (m Matrix) Multiply(n Matrix) Matrix {
	return Matrix{
		A: m.A*n.A + m.C*n.B,
		B: m.B*n.A + m.D*n.B,
		C: m.A*n.C + m.C*n.D,
		D: m.B*n.C + m.D*n.D,
		E: m.A*n.E + m.C*n.F + m.E,
		F: m.B*n.E + m.D*n.F + m.F,
	}
}

// ParseTransform parses an SVG transform attribute string like
// "translate(10,20) rotate(45) scale(2)" into a combined matrix.
// Supported functions: translate, scale, rotate, skewX, skewY, matrix.
func parseTransform(s string) Matrix {
	result := identity()
	s = strings.TrimSpace(s)
	for len(s) > 0 {
		// skip whitespace and commas
		s = strings.TrimLeftFunc(s, func(r rune) bool {
			return unicode.IsSpace(r) || r == ','
		})
		if len(s) == 0 {
			break
		}

		// find function name
		parenIdx := strings.Index(s, "(")
		if parenIdx < 0 {
			break
		}
		fname := strings.TrimSpace(s[:parenIdx])

		// find closing paren
		closeIdx := strings.Index(s[parenIdx:], ")")
		if closeIdx < 0 {
			break
		}
		closeIdx += parenIdx

		argStr := s[parenIdx+1 : closeIdx]
		args := parseArgs(argStr)
		s = s[closeIdx+1:]

		m := applyTransformFunc(fname, args)
		result = result.Multiply(m)
	}
	return result
}

// parseArgs splits a comma/space-separated argument list into float64 values.
func parseArgs(s string) []float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// replace commas with spaces, then split on whitespace
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	var result []float64
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			continue
		}
		result = append(result, v)
	}
	return result
}

// applyTransformFunc builds a Matrix from a named SVG transform function and its arguments.
func applyTransformFunc(name string, args []float64) Matrix {
	switch name {
	case "translate":
		tx := 0.0
		ty := 0.0
		if len(args) >= 1 {
			tx = args[0]
		}
		if len(args) >= 2 {
			ty = args[1]
		}
		return Translate(tx, ty)

	case "scale":
		sx := 1.0
		sy := 1.0
		if len(args) >= 1 {
			sx = args[0]
			sy = args[0] // uniform scale if only one arg
		}
		if len(args) >= 2 {
			sy = args[1]
		}
		return Scale(sx, sy)

	case "rotate":
		if len(args) == 0 {
			return identity()
		}
		angle := args[0]
		if len(args) >= 3 {
			// rotate(angle, cx, cy) = translate(cx,cy) * rotate(angle) * translate(-cx,-cy)
			cx, cy := args[1], args[2]
			return Translate(cx, cy).Multiply(Rotate(angle)).Multiply(Translate(-cx, -cy))
		}
		return Rotate(angle)

	case "skewX":
		if len(args) >= 1 {
			return SkewX(args[0])
		}
		return identity()

	case "skewY":
		if len(args) >= 1 {
			return SkewY(args[0])
		}
		return identity()

	case "matrix":
		if len(args) >= 6 {
			return Matrix{A: args[0], B: args[1], C: args[2], D: args[3], E: args[4], F: args[5]}
		}
		return identity()

	default:
		return identity()
	}
}

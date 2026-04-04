// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

// ParseCSSLength converts a CSS length string to PDF points.
// Supported units: px, pt, em, rem, %, mm, cm, in.
// Also supports calc(), min(), max(), clamp() expressions.
//
// fontSize is used to resolve em units. rem assumes a 16px (12pt)
// root font size. relativeTo is used for percentage values.
// Returns 0 for unparseable or empty values.
//
//	ParseCSSLength("1in", 12, 0)     // 72.0
//	ParseCSSLength("25.4mm", 12, 0)  // 72.0
//	ParseCSSLength("16px", 12, 0)    // 12.0 (16 * 0.75)
//	ParseCSSLength("2em", 12, 0)     // 24.0
//	ParseCSSLength("50%", 12, 200)   // 100.0
func ParseCSSLength(s string, fontSize, relativeTo float64) float64 {
	l := parseLength(s)
	if l == nil {
		return 0
	}
	return l.toPoints(relativeTo, fontSize)
}

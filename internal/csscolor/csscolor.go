// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package csscolor parses CSS color values shared by the html and svg
// packages: named colors, hex, rgb()/rgba(), hsl()/hsla(), and
// cmyk()/device-cmyk(). It has no opinion on the caller-specific
// keywords "none", "currentcolor", "transparent", "inherit", or
// "initial" — those stay in the calling package.
package csscolor

import (
	"math"
	"strconv"
	"strings"
)

// finite reports whether v is a usable number (not NaN or ±Inf).
func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// Color is a parsed CSS color. R, G, B, and A are in [0, 1]. CMYK is
// non-nil only when the input was a cmyk()/device-cmyk() function;
// callers that don't support CMYK should treat a non-nil CMYK as
// unsupported.
type Color struct {
	R, G, B, A float64
	CMYK       *[4]float64
}

// Parse parses a CSS color value. Supports named colors, #RGB, #RGBA,
// #RRGGBB, #RRGGBBAA, rgb()/rgba() (comma and CSS Color 4 space-separated
// forms), hsl()/hsla() (comma and space forms), and cmyk()/device-cmyk().
// Alpha defaults to 1.0 for formats that don't include it.
func Parse(s string) (Color, bool) {
	value := strings.TrimSpace(strings.ToLower(s))

	// Named color.
	if r, g, b, ok := Lookup(value); ok {
		return Color{R: r, G: g, B: b, A: 1}, true
	}

	// Hex color: #RGB, #RGBA, #RRGGBB, #RRGGBBAA.
	if strings.HasPrefix(value, "#") {
		return parseHex(value[1:])
	}

	// rgb(r, g, b) / rgba(r, g, b, a)
	// Also supports CSS Color Level 4 space-separated form:
	//   rgb(255 0 0) / rgb(255 0 0 / 0.5)
	if strings.HasPrefix(value, "rgb") {
		inner, ok := extractFuncArgs(value, "rgba(")
		if !ok {
			inner, ok = extractFuncArgs(value, "rgb(")
		}
		if ok {
			if strings.ContainsRune(inner, ',') {
				parts := strings.Split(inner, ",")
				if len(parts) >= 3 {
					r, ok1 := parseColorComponent(strings.TrimSpace(parts[0]))
					g, ok2 := parseColorComponent(strings.TrimSpace(parts[1]))
					b, ok3 := parseColorComponent(strings.TrimSpace(parts[2]))
					a := 1.0
					okA := true
					if len(parts) >= 4 {
						v, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
						if err != nil || !finite(v) {
							okA = false
						} else {
							a = clamp01(v)
						}
					}
					if ok1 && ok2 && ok3 && okA {
						return Color{R: r, G: g, B: b, A: a}, true
					}
				}
			} else {
				r, g, b, a, ok := parseSpaceColorArgs(inner)
				if ok {
					return Color{R: r, G: g, B: b, A: a}, true
				}
			}
		}
		return Color{}, false
	}

	// cmyk(c, m, y, k) / device-cmyk(c, m, y, k)
	if strings.HasPrefix(value, "cmyk(") || strings.HasPrefix(value, "device-cmyk(") {
		prefix := "cmyk("
		if strings.HasPrefix(value, "device-cmyk(") {
			prefix = "device-cmyk("
		}
		inner, ok := extractFuncArgs(value, prefix)
		if ok {
			parts := strings.Split(inner, ",")
			if len(parts) >= 4 {
				c, ok1 := parseCMYKComponent(strings.TrimSpace(parts[0]))
				m, ok2 := parseCMYKComponent(strings.TrimSpace(parts[1]))
				y, ok3 := parseCMYKComponent(strings.TrimSpace(parts[2]))
				k, ok4 := parseCMYKComponent(strings.TrimSpace(parts[3]))
				if ok1 && ok2 && ok3 && ok4 {
					return Color{A: 1, CMYK: &[4]float64{c, m, y, k}}, true
				}
			}
		}
		return Color{}, false
	}

	// hsl(h, s%, l%) / hsla(h, s%, l%, a)
	// Also supports CSS Color Level 4 space-separated form:
	//   hsl(120 100% 50%) / hsl(120 100% 50% / 0.5)
	if strings.HasPrefix(value, "hsl") {
		inner, ok := extractFuncArgs(value, "hsla(")
		if !ok {
			inner, ok = extractFuncArgs(value, "hsl(")
		}
		if ok {
			if strings.ContainsRune(inner, ',') {
				parts := strings.Split(inner, ",")
				if len(parts) >= 3 {
					h, ok1 := parseHue(strings.TrimSpace(parts[0]))
					s, ok2 := parsePercent(strings.TrimSpace(parts[1]))
					l, ok3 := parsePercent(strings.TrimSpace(parts[2]))
					a := 1.0
					okA := true
					if len(parts) >= 4 {
						v, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
						if err != nil || !finite(v) {
							okA = false
						} else {
							a = clamp01(v)
						}
					}
					if ok1 && ok2 && ok3 && okA {
						r, g, b := hslToRGB(h, s, l)
						return Color{R: r, G: g, B: b, A: a}, true
					}
				}
			} else {
				alpha, okA, parts := splitSlashAlpha(inner)
				if okA && len(parts) >= 3 {
					h, ok1 := parseHue(parts[0])
					s, ok2 := parsePercent(parts[1])
					l, ok3 := parsePercent(parts[2])
					if ok1 && ok2 && ok3 {
						r, g, b := hslToRGB(h, s, l)
						return Color{R: r, G: g, B: b, A: alpha}, true
					}
				}
			}
		}
		return Color{}, false
	}

	return Color{}, false
}

// parseHex parses a hex color body (without the leading '#') of length
// 3, 4, 6, or 8 into a Color.
func parseHex(hex string) (Color, bool) {
	switch len(hex) {
	case 3:
		r, ok1 := hexByte(hex[0], hex[0])
		g, ok2 := hexByte(hex[1], hex[1])
		b, ok3 := hexByte(hex[2], hex[2])
		if !ok1 || !ok2 || !ok3 {
			return Color{}, false
		}
		return Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: 1}, true
	case 4:
		r, ok1 := hexByte(hex[0], hex[0])
		g, ok2 := hexByte(hex[1], hex[1])
		b, ok3 := hexByte(hex[2], hex[2])
		a, ok4 := hexByte(hex[3], hex[3])
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return Color{}, false
		}
		return Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: float64(a) / 255}, true
	case 6:
		r, ok1 := hexByte(hex[0], hex[1])
		g, ok2 := hexByte(hex[2], hex[3])
		b, ok3 := hexByte(hex[4], hex[5])
		if !ok1 || !ok2 || !ok3 {
			return Color{}, false
		}
		return Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: 1}, true
	case 8:
		r, ok1 := hexByte(hex[0], hex[1])
		g, ok2 := hexByte(hex[2], hex[3])
		b, ok3 := hexByte(hex[4], hex[5])
		a, ok4 := hexByte(hex[6], hex[7])
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return Color{}, false
		}
		return Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: float64(a) / 255}, true
	}
	return Color{}, false
}

// splitSlashAlpha splits "R G B / A" into (alpha, ok, [R, G, B]).
// If no slash, alpha defaults to 1.0 and ok is true. If a slash is present
// but the alpha is malformed or non-finite, ok is false and the whole
// color must fail (a malformed alpha is a malformed color, not a fallback
// to alpha=1). The returned parts are trimmed strings.
func splitSlashAlpha(inner string) (alpha float64, ok bool, parts []string) {
	alpha = 1.0
	colorPart := inner
	if slashIdx := strings.IndexByte(inner, '/'); slashIdx >= 0 {
		colorPart = strings.TrimSpace(inner[:slashIdx])
		alphaStr := strings.TrimSpace(inner[slashIdx+1:])
		// Alpha can be a number (0.5) or percentage (50%).
		if strings.HasSuffix(alphaStr, "%") {
			v, err := strconv.ParseFloat(alphaStr[:len(alphaStr)-1], 64)
			if err != nil || !finite(v) {
				return 0, false, nil
			}
			alpha = clamp01(v / 100)
		} else {
			v, err := strconv.ParseFloat(alphaStr, 64)
			if err != nil || !finite(v) {
				return 0, false, nil
			}
			alpha = clamp01(v)
		}
	}
	return alpha, true, strings.Fields(colorPart)
}

// parseSpaceColorArgs parses space-separated RGB args with optional / alpha.
// Handles: "255 0 0", "255 0 0 / 0.5", "100% 0% 50%", "100% 0% 50% / 0.8"
func parseSpaceColorArgs(inner string) (r, g, b, a float64, ok bool) {
	a, okA, parts := splitSlashAlpha(inner)
	if !okA || len(parts) < 3 {
		return 0, 0, 0, 0, false
	}
	r, ok1 := parseColorComponent(parts[0])
	g, ok2 := parseColorComponent(parts[1])
	b, ok3 := parseColorComponent(parts[2])
	if !ok1 || !ok2 || !ok3 {
		return 0, 0, 0, 0, false
	}
	return r, g, b, a, true
}

// extractFuncArgs extracts the content inside a CSS function like "rgb(...)" or "rgba(...)".
func extractFuncArgs(value, prefix string) (string, bool) {
	if strings.HasPrefix(value, prefix) && strings.HasSuffix(value, ")") {
		return value[len(prefix) : len(value)-1], true
	}
	return "", false
}

// parseColorComponent parses an RGB color component (0-255 or percentage).
// The result is clamped to [0, 1]. ok is false on a malformed or
// non-finite (NaN/Inf) number.
func parseColorComponent(s string) (float64, bool) {
	var v float64
	var err error
	if strings.HasSuffix(s, "%") {
		v, err = strconv.ParseFloat(s[:len(s)-1], 64)
		v /= 100
	} else {
		v, err = strconv.ParseFloat(s, 64)
		v /= 255
	}
	if err != nil || !finite(v) {
		return 0, false
	}
	return clamp01(v), true
}

// parseHue parses a CSS hue value (degrees, 0-360). ok is false on a
// malformed or non-finite (NaN/Inf) number.
func parseHue(s string) (float64, bool) {
	s = strings.TrimSuffix(s, "deg")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || !finite(v) {
		return 0, false
	}
	// Normalize to 0-360.
	v = v - float64(int(v/360))*360
	if v < 0 {
		v += 360
	}
	return v / 360, true // return as 0-1
}

// parsePercent parses a percentage value like "50%". ok is false on a
// malformed or non-finite (NaN/Inf) number.
func parsePercent(s string) (float64, bool) {
	s = strings.TrimSuffix(s, "%")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || !finite(v) {
		return 0, false
	}
	return v / 100, true
}

// hexVal returns the numeric value of a hex digit and whether c is a
// valid hex digit.
func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// hexByte combines two hex digits (hi, lo) into a byte, or ok=false if
// either digit is invalid.
func hexByte(hi, lo byte) (byte, bool) {
	h, ok1 := hexVal(hi)
	l, ok2 := hexVal(lo)
	if !ok1 || !ok2 {
		return 0, false
	}
	return h*16 + l, true
}

// parseCMYKComponent parses a CMYK color component (0-1 or percentage).
// ok is false on a malformed or non-finite (NaN/Inf) number.
func parseCMYKComponent(s string) (float64, bool) {
	if strings.HasSuffix(s, "%") {
		v, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil || !finite(v) {
			return 0, false
		}
		return clamp01(v / 100), true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || !finite(v) {
		return 0, false
	}
	return clamp01(v), true
}

// hslToRGB converts HSL values (each 0-1) to RGB values (each 0-1).
func hslToRGB(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		return l, l, l
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	r = hueToRGB(p, q, h+1.0/3.0)
	g = hueToRGB(p, q, h)
	b = hueToRGB(p, q, h-1.0/3.0)
	return r, g, b
}

// hueToRGB is a helper for hslToRGB.
func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

// clamp01 clamps v to the range [0, 1]. NaN maps to 0 (defense in depth
// even though all callers already reject non-finite values themselves).
func clamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

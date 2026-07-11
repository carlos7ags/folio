// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package csscolor parses CSS color values shared by the html and svg
// packages: named colors, hex, rgb()/rgba(), hsl()/hsla(), and
// cmyk()/device-cmyk(). It has no opinion on the caller-specific
// keywords "none", "currentcolor", "transparent", "inherit", or
// "initial" — those stay in the calling package.
package csscolor

import (
	"strconv"
	"strings"
)

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
					r := parseColorComponent(strings.TrimSpace(parts[0]))
					g := parseColorComponent(strings.TrimSpace(parts[1]))
					b := parseColorComponent(strings.TrimSpace(parts[2]))
					a := 1.0
					if len(parts) >= 4 {
						if v, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err == nil {
							a = clamp01(v)
						}
					}
					return Color{R: r, G: g, B: b, A: a}, true
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
				c := parseCMYKComponent(strings.TrimSpace(parts[0]))
				m := parseCMYKComponent(strings.TrimSpace(parts[1]))
				y := parseCMYKComponent(strings.TrimSpace(parts[2]))
				k := parseCMYKComponent(strings.TrimSpace(parts[3]))
				return Color{A: 1, CMYK: &[4]float64{c, m, y, k}}, true
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
					h := parseHue(strings.TrimSpace(parts[0]))
					s := parsePercent(strings.TrimSpace(parts[1]))
					l := parsePercent(strings.TrimSpace(parts[2]))
					a := 1.0
					if len(parts) >= 4 {
						if v, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err == nil {
							a = clamp01(v)
						}
					}
					r, g, b := hslToRGB(h, s, l)
					return Color{R: r, G: g, B: b, A: a}, true
				}
			} else {
				alpha, parts := splitSlashAlpha(inner)
				if len(parts) >= 3 {
					h := parseHue(parts[0])
					s := parsePercent(parts[1])
					l := parsePercent(parts[2])
					r, g, b := hslToRGB(h, s, l)
					return Color{R: r, G: g, B: b, A: alpha}, true
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
		r := hexVal(hex[0])*16 + hexVal(hex[0])
		g := hexVal(hex[1])*16 + hexVal(hex[1])
		b := hexVal(hex[2])*16 + hexVal(hex[2])
		return Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: 1}, true
	case 4:
		r := hexVal(hex[0])*16 + hexVal(hex[0])
		g := hexVal(hex[1])*16 + hexVal(hex[1])
		b := hexVal(hex[2])*16 + hexVal(hex[2])
		a := hexVal(hex[3])*16 + hexVal(hex[3])
		return Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: float64(a) / 255}, true
	case 6:
		r := hexVal(hex[0])*16 + hexVal(hex[1])
		g := hexVal(hex[2])*16 + hexVal(hex[3])
		b := hexVal(hex[4])*16 + hexVal(hex[5])
		return Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: 1}, true
	case 8:
		r := hexVal(hex[0])*16 + hexVal(hex[1])
		g := hexVal(hex[2])*16 + hexVal(hex[3])
		b := hexVal(hex[4])*16 + hexVal(hex[5])
		a := hexVal(hex[6])*16 + hexVal(hex[7])
		return Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: float64(a) / 255}, true
	}
	return Color{}, false
}

// splitSlashAlpha splits "R G B / A" into (alpha, [R, G, B]).
// If no slash, alpha defaults to 1.0. The returned parts are trimmed strings.
func splitSlashAlpha(inner string) (float64, []string) {
	alpha := 1.0
	colorPart := inner
	if slashIdx := strings.IndexByte(inner, '/'); slashIdx >= 0 {
		colorPart = strings.TrimSpace(inner[:slashIdx])
		alphaStr := strings.TrimSpace(inner[slashIdx+1:])
		// Alpha can be a number (0.5) or percentage (50%).
		if strings.HasSuffix(alphaStr, "%") {
			if v, err := strconv.ParseFloat(alphaStr[:len(alphaStr)-1], 64); err == nil {
				alpha = clamp01(v / 100)
			}
		} else if v, err := strconv.ParseFloat(alphaStr, 64); err == nil {
			alpha = clamp01(v)
		}
	}
	parts := strings.Fields(colorPart)
	return alpha, parts
}

// parseSpaceColorArgs parses space-separated RGB args with optional / alpha.
// Handles: "255 0 0", "255 0 0 / 0.5", "100% 0% 50%", "100% 0% 50% / 0.8"
func parseSpaceColorArgs(inner string) (r, g, b, a float64, ok bool) {
	a, parts := splitSlashAlpha(inner)
	if len(parts) < 3 {
		return 0, 0, 0, 0, false
	}
	return parseColorComponent(parts[0]), parseColorComponent(parts[1]),
		parseColorComponent(parts[2]), a, true
}

// extractFuncArgs extracts the content inside a CSS function like "rgb(...)" or "rgba(...)".
func extractFuncArgs(value, prefix string) (string, bool) {
	if strings.HasPrefix(value, prefix) && strings.HasSuffix(value, ")") {
		return value[len(prefix) : len(value)-1], true
	}
	return "", false
}

// parseColorComponent parses an RGB color component (0-255 or percentage).
// The result is clamped to [0, 1].
func parseColorComponent(s string) float64 {
	var v float64
	if strings.HasSuffix(s, "%") {
		v, _ = strconv.ParseFloat(s[:len(s)-1], 64)
		v /= 100
	} else {
		v, _ = strconv.ParseFloat(s, 64)
		v /= 255
	}
	return clamp01(v)
}

// parseHue parses a CSS hue value (degrees, 0-360).
func parseHue(s string) float64 {
	s = strings.TrimSuffix(s, "deg")
	v, _ := strconv.ParseFloat(s, 64)
	// Normalize to 0-360.
	v = v - float64(int(v/360))*360
	if v < 0 {
		v += 360
	}
	return v / 360 // return as 0-1
}

// parsePercent parses a percentage value like "50%".
func parsePercent(s string) float64 {
	s = strings.TrimSuffix(s, "%")
	v, _ := strconv.ParseFloat(s, 64)
	return v / 100
}

// hexVal returns the numeric value of a hex digit.
func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// parseCMYKComponent parses a CMYK color component (0-1 or percentage).
func parseCMYKComponent(s string) float64 {
	if strings.HasSuffix(s, "%") {
		v, _ := strconv.ParseFloat(s[:len(s)-1], 64)
		return clamp01(v / 100)
	}
	v, _ := strconv.ParseFloat(s, 64)
	return clamp01(v)
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

// clamp01 clamps v to the range [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

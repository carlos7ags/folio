// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

// PathCommand represents a single SVG path command with absolute coordinates.
type PathCommand struct {
	Type byte      // 'M', 'L', 'C', 'Q', 'A', 'Z' (always uppercase/absolute after parsing)
	Args []float64 // coordinate arguments
}

// ParsePathData parses an SVG path d attribute into a slice of absolute-coordinate commands.
// All relative commands are converted to absolute.
// H/V are converted to L.
// S is converted to C (with reflected control point).
// T is converted to Q (with reflected control point).
func parsePathData(d string) ([]PathCommand, error) {
	tokens, err := tokenizePath(d)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, nil
	}

	var commands []PathCommand

	// Current point.
	var cx, cy float64
	// Start of current subpath (for Z).
	var sx, sy float64
	// Last control point for S/T reflection.
	var lastCPX, lastCPY float64
	var lastCmd byte

	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if !tok.isCommand {
			return nil, fmt.Errorf("svg: expected command at position %d, got %q", i, tok.value)
		}
		cmd := tok.value[0]
		i++

		argCount := commandArgCount(cmd)
		isRelative := cmd >= 'a' && cmd <= 'z'
		absCmd := toUpper(cmd)

		// Z takes no arguments.
		if absCmd == 'Z' {
			commands = append(commands, PathCommand{Type: 'Z'})
			cx, cy = sx, sy
			lastCPX, lastCPY = cx, cy
			lastCmd = 'Z'
			continue
		}

		if argCount == 0 {
			continue
		}

		// Process repeated implicit commands.
		first := true
		for i < len(tokens) && !tokens[i].isCommand {
			args, newI, err := consumeArgs(tokens, i, argCount)
			if err != nil {
				return nil, fmt.Errorf("svg: error parsing args for %c: %w", cmd, err)
			}
			i = newI

			// Convert relative to absolute.
			if isRelative {
				makeAbsolute(absCmd, args, cx, cy)
			}

			// Convert shorthand commands and emit.
			switch absCmd {
			case 'M':
				if first {
					commands = append(commands, PathCommand{Type: 'M', Args: []float64{args[0], args[1]}})
					sx, sy = args[0], args[1]
				} else {
					// Implicit lineto after first M pair.
					commands = append(commands, PathCommand{Type: 'L', Args: []float64{args[0], args[1]}})
				}
				cx, cy = args[0], args[1]
				lastCPX, lastCPY = cx, cy

			case 'L':
				commands = append(commands, PathCommand{Type: 'L', Args: []float64{args[0], args[1]}})
				cx, cy = args[0], args[1]
				lastCPX, lastCPY = cx, cy

			case 'H':
				commands = append(commands, PathCommand{Type: 'L', Args: []float64{args[0], cy}})
				cx = args[0]
				lastCPX, lastCPY = cx, cy

			case 'V':
				commands = append(commands, PathCommand{Type: 'L', Args: []float64{cx, args[0]}})
				cy = args[0]
				lastCPX, lastCPY = cx, cy

			case 'C':
				commands = append(commands, PathCommand{Type: 'C', Args: []float64{
					args[0], args[1], args[2], args[3], args[4], args[5],
				}})
				lastCPX, lastCPY = args[2], args[3]
				cx, cy = args[4], args[5]

			case 'S':
				// Reflect last control point of previous C/S.
				rx, ry := cx, cy
				if lastCmd == 'C' || lastCmd == 'S' {
					rx = 2*cx - lastCPX
					ry = 2*cy - lastCPY
				}
				commands = append(commands, PathCommand{Type: 'C', Args: []float64{
					rx, ry, args[0], args[1], args[2], args[3],
				}})
				lastCPX, lastCPY = args[0], args[1]
				cx, cy = args[2], args[3]

			case 'Q':
				commands = append(commands, PathCommand{Type: 'Q', Args: []float64{
					args[0], args[1], args[2], args[3],
				}})
				lastCPX, lastCPY = args[0], args[1]
				cx, cy = args[2], args[3]

			case 'T':
				// Reflect last control point of previous Q/T.
				rx, ry := cx, cy
				if lastCmd == 'Q' || lastCmd == 'T' {
					rx = 2*cx - lastCPX
					ry = 2*cy - lastCPY
				}
				commands = append(commands, PathCommand{Type: 'Q', Args: []float64{
					rx, ry, args[0], args[1],
				}})
				lastCPX, lastCPY = rx, ry
				cx, cy = args[0], args[1]

			case 'A':
				commands = append(commands, PathCommand{Type: 'A', Args: []float64{
					args[0], args[1], args[2], args[3], args[4], args[5], args[6],
				}})
				cx, cy = args[5], args[6]
				lastCPX, lastCPY = cx, cy
			}

			lastCmd = absCmd
			first = false
		}
	}

	return commands, nil
}

// ArcToCubics converts an SVG arc command to a series of cubic Bezier commands.
// Parameters match SVG A command: rx, ry, xAxisRotation (degrees), largeArc (0/1),
// sweep (0/1), endX, endY, and the current point (startX, startY).
// Returns a slice of PathCommand with Type='C'.
func arcToCubics(startX, startY, rx, ry, xAxisRotation float64, largeArc, sweep bool, endX, endY float64) []PathCommand {
	// Degenerate: same start and end point — nothing to draw.
	if startX == endX && startY == endY {
		return nil
	}

	// Degenerate: zero radius — treat as line.
	if rx == 0 || ry == 0 {
		return []PathCommand{{Type: 'C', Args: []float64{startX, startY, endX, endY, endX, endY}}}
	}

	rx = math.Abs(rx)
	ry = math.Abs(ry)

	phi := xAxisRotation * math.Pi / 180.0
	cosPhi := math.Cos(phi)
	sinPhi := math.Sin(phi)

	// Step 1: Compute (x1', y1') — SVG spec F.6.5.1
	dx := (startX - endX) / 2
	dy := (startY - endY) / 2
	x1p := cosPhi*dx + sinPhi*dy
	y1p := -sinPhi*dx + cosPhi*dy

	// Step 2: Ensure radii are large enough — SVG spec F.6.6.2/F.6.6.3
	x1pSq := x1p * x1p
	y1pSq := y1p * y1p
	rxSq := rx * rx
	rySq := ry * ry

	lambda := x1pSq/rxSq + y1pSq/rySq
	if lambda > 1 {
		scale := math.Sqrt(lambda)
		rx *= scale
		ry *= scale
		rxSq = rx * rx
		rySq = ry * ry
	}

	// Step 3: Compute (cx', cy') — SVG spec F.6.5.2
	num := rxSq*rySq - rxSq*y1pSq - rySq*x1pSq
	den := rxSq*y1pSq + rySq*x1pSq
	sq := 0.0
	if den > 0 {
		sq = math.Sqrt(math.Max(0, num/den))
	}
	if largeArc == sweep {
		sq = -sq
	}
	cxp := sq * rx * y1p / ry
	cyp := -sq * ry * x1p / rx

	// Step 4: Compute (cx, cy) from (cx', cy') — SVG spec F.6.5.3
	cx := cosPhi*cxp - sinPhi*cyp + (startX+endX)/2
	cy := sinPhi*cxp + cosPhi*cyp + (startY+endY)/2

	// Step 5: Compute theta1 and dtheta — SVG spec F.6.5.5/F.6.5.6
	theta1 := vecAngle(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	dtheta := vecAngle((x1p-cxp)/rx, (y1p-cyp)/ry, (-x1p-cxp)/rx, (-y1p-cyp)/ry)

	if !sweep && dtheta > 0 {
		dtheta -= 2 * math.Pi
	} else if sweep && dtheta < 0 {
		dtheta += 2 * math.Pi
	}

	// Step 6: Split into segments of at most pi/2.
	segments := int(math.Ceil(math.Abs(dtheta) / (math.Pi / 2)))
	segments = max(segments, 1)
	segAngle := dtheta / float64(segments)

	var result []PathCommand
	for i := range segments {
		t1 := theta1 + float64(i)*segAngle
		t2 := t1 + segAngle
		cubics := arcSegmentToCubic(cx, cy, rx, ry, phi, t1, t2)
		result = append(result, cubics)
	}
	return result
}

// arcSegmentToCubic approximates a single arc segment (at most pi/2) as a cubic Bezier.
func arcSegmentToCubic(cx, cy, rx, ry, phi, theta1, theta2 float64) PathCommand {
	alpha := math.Sin(theta2-theta1) * (math.Sqrt(4+3*math.Pow(math.Tan((theta2-theta1)/2), 2)) - 1) / 3

	cos1 := math.Cos(theta1)
	sin1 := math.Sin(theta1)
	cos2 := math.Cos(theta2)
	sin2 := math.Sin(theta2)
	cosPhi := math.Cos(phi)
	sinPhi := math.Sin(phi)

	// Point on ellipse at angle theta, rotated and translated.
	px := func(cosT, sinT float64) float64 {
		return cosPhi*rx*cosT - sinPhi*ry*sinT + cx
	}
	py := func(cosT, sinT float64) float64 {
		return sinPhi*rx*cosT + cosPhi*ry*sinT + cy
	}

	// Derivative of point on ellipse.
	dpx := func(cosT, sinT float64) float64 {
		return -cosPhi*rx*sinT - sinPhi*ry*cosT
	}
	dpy := func(cosT, sinT float64) float64 {
		return -sinPhi*rx*sinT + cosPhi*ry*cosT
	}

	x1 := px(cos1, sin1)
	y1 := py(cos1, sin1)
	dx1 := dpx(cos1, sin1)
	dy1 := dpy(cos1, sin1)

	x2 := px(cos2, sin2)
	y2 := py(cos2, sin2)
	dx2 := dpx(cos2, sin2)
	dy2 := dpy(cos2, sin2)

	_ = x1
	_ = y1

	return PathCommand{
		Type: 'C',
		Args: []float64{
			x1 + alpha*dx1, y1 + alpha*dy1,
			x2 - alpha*dx2, y2 - alpha*dy2,
			x2, y2,
		},
	}
}

// vecAngle computes the angle between vectors (ux,uy) and (vx,vy).
func vecAngle(ux, uy, vx, vy float64) float64 {
	dot := ux*vx + uy*vy
	lenU := math.Sqrt(ux*ux + uy*uy)
	lenV := math.Sqrt(vx*vx + vy*vy)
	cos := dot / (lenU * lenV)
	// Clamp to [-1, 1] for numerical stability.
	cos = math.Max(-1, math.Min(1, cos))
	angle := math.Acos(cos)
	if ux*vy-uy*vx < 0 {
		angle = -angle
	}
	return angle
}

// --- Tokenizer ---

// pathToken represents a single token from SVG path data: either a command
// letter or a numeric value.
type pathToken struct {
	value     string
	isCommand bool
}

// tokenizePath splits SVG path data into command letters and numeric values.
func tokenizePath(d string) ([]pathToken, error) {
	var tokens []pathToken
	r := strings.NewReader(d)

	for {
		ch, _, err := r.ReadRune()
		if err != nil {
			break
		}

		if unicode.IsSpace(ch) || ch == ',' {
			continue
		}

		if isPathCommand(ch) {
			tokens = append(tokens, pathToken{value: string(ch), isCommand: true})
			continue
		}

		if ch == '+' || ch == '-' || ch == '.' || (ch >= '0' && ch <= '9') {
			num := readNumber(r, ch)
			tokens = append(tokens, pathToken{value: num, isCommand: false})
			continue
		}

		return nil, fmt.Errorf("svg: unexpected character %q in path data", ch)
	}
	return tokens, nil
}

// readNumber reads a complete number starting with the given rune.
func readNumber(r *strings.Reader, first rune) string {
	var buf strings.Builder
	buf.WriteRune(first)

	hasDot := first == '.'
	hasE := false

	for {
		ch, _, err := r.ReadRune()
		if err != nil {
			break
		}

		if ch >= '0' && ch <= '9' {
			buf.WriteRune(ch)
			continue
		}

		if ch == '.' && !hasDot && !hasE {
			hasDot = true
			buf.WriteRune(ch)
			continue
		}

		if (ch == 'e' || ch == 'E') && !hasE {
			hasE = true
			buf.WriteRune(ch)
			// Peek for sign after exponent.
			next, _, err := r.ReadRune()
			if err != nil {
				break
			}
			if next == '+' || next == '-' {
				buf.WriteRune(next)
			} else {
				_ = r.UnreadRune()
			}
			continue
		}

		// Not part of this number; put it back.
		_ = r.UnreadRune()
		break
	}
	return buf.String()
}

// isPathCommand returns true if the rune is an SVG path command letter.
func isPathCommand(ch rune) bool {
	switch ch {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v',
		'C', 'c', 'S', 's', 'Q', 'q', 'T', 't',
		'A', 'a', 'Z', 'z':
		return true
	}
	return false
}

// commandArgCount returns the number of numeric arguments expected per repetition
// for the given command (uppercase).
func commandArgCount(cmd byte) int {
	switch toUpper(cmd) {
	case 'M', 'L', 'T':
		return 2
	case 'H', 'V':
		return 1
	case 'C':
		return 6
	case 'S', 'Q':
		return 4
	case 'A':
		return 7
	case 'Z':
		return 0
	}
	return 0
}

// toUpper converts a lowercase ASCII letter to uppercase.
func toUpper(cmd byte) byte {
	if cmd >= 'a' && cmd <= 'z' {
		return cmd - 32
	}
	return cmd
}

// consumeArgs reads argCount float64 values from the token list starting at pos.
func consumeArgs(tokens []pathToken, pos, argCount int) ([]float64, int, error) {
	args := make([]float64, argCount)
	for j := range argCount {
		if pos >= len(tokens) {
			return nil, pos, fmt.Errorf("expected %d args, got %d", argCount, j)
		}
		if tokens[pos].isCommand {
			return nil, pos, fmt.Errorf("expected number, got command %q", tokens[pos].value)
		}
		v, err := parseFloat(tokens[pos].value)
		if err != nil {
			return nil, pos, fmt.Errorf("invalid number %q: %w", tokens[pos].value, err)
		}
		args[j] = v
		pos++
	}
	return args, pos, nil
}

// parseFloat parses a float64 from a string, handling the common SVG path cases.
func parseFloat(s string) (float64, error) {
	var result float64
	_, err := fmt.Sscanf(s, "%g", &result)
	return result, err
}

// makeAbsolute converts relative arguments to absolute, modifying args in place.
func makeAbsolute(absCmd byte, args []float64, cx, cy float64) {
	switch absCmd {
	case 'M', 'L', 'T':
		args[0] += cx
		args[1] += cy
	case 'H':
		args[0] += cx
	case 'V':
		args[0] += cy
	case 'C':
		args[0] += cx
		args[1] += cy
		args[2] += cx
		args[3] += cy
		args[4] += cx
		args[5] += cy
	case 'S', 'Q':
		args[0] += cx
		args[1] += cy
		args[2] += cx
		args[3] += cy
	case 'A':
		// For arcs, only the endpoint is relative.
		args[5] += cx
		args[6] += cy
	}
}

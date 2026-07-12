// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// UnitType specifies how a UnitValue is interpreted.
type UnitType int

const (
	UnitPoint   UnitType = iota // absolute value in PDF points
	UnitPercent                 // percentage of available width
	UnitCalc                    // expression resolved against available width at layout time
)

// UnitValue represents a measurement that can be either an absolute
// point value, a percentage of available space, or a calc()-style
// expression resolved lazily at layout time.
type UnitValue struct {
	// Calc resolves the value against the available width. Only used
	// when Unit == UnitCalc; nil otherwise. Lets calc() expressions
	// containing percentages be resolved against the actual layout area
	// rather than an eagerly-captured container width.
	Calc  func(available float64) float64
	Value float64
	Unit  UnitType
}

// Pt creates a UnitValue in PDF points.
func Pt(v float64) UnitValue {
	return UnitValue{Value: v, Unit: UnitPoint}
}

// Pct creates a UnitValue as a percentage (0–100) of available space.
func Pct(v float64) UnitValue {
	return UnitValue{Value: v, Unit: UnitPercent}
}

// CalcUnit creates a UnitValue whose resolution is deferred to layout time.
// Pass a closure that, given the available width, returns points. Used for
// CSS calc()/min()/max()/clamp() expressions that contain percentage parts.
func CalcUnit(fn func(available float64) float64) UnitValue {
	return UnitValue{Unit: UnitCalc, Calc: fn}
}

// Resolve converts a UnitValue to points given the available width.
func (u UnitValue) Resolve(available float64) float64 {
	switch u.Unit {
	case UnitPercent:
		return available * u.Value / 100
	case UnitCalc:
		if u.Calc != nil {
			return u.Calc(available)
		}
		return 0
	}
	return u.Value
}

// ResolveAll converts a slice of UnitValues to point widths.
// Percentages are resolved against totalWidth. If the values
// don't sum to totalWidth, the remainder is unaccounted for
// (callers should validate or normalize as needed).
func ResolveAll(values []UnitValue, totalWidth float64) []float64 {
	result := make([]float64, len(values))
	for i, v := range values {
		result[i] = v.Resolve(totalWidth)
	}
	return result
}

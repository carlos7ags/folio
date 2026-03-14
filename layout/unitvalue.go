// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// UnitType specifies how a UnitValue is interpreted.
type UnitType int

const (
	UnitPoint   UnitType = iota // absolute value in PDF points
	UnitPercent                 // percentage of available width
)

// UnitValue represents a measurement that can be either an absolute
// point value or a percentage of available space.
type UnitValue struct {
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

// Resolve converts a UnitValue to points given the available width.
func (u UnitValue) Resolve(available float64) float64 {
	if u.Unit == UnitPercent {
		return available * u.Value / 100
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

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "github.com/carlos7ags/folio/font"

// resolveMeasurer returns the text measurer for an embedded/standard font
// pair: embedded wins, else the standard font, else fallback.
//
// The fallback makes each caller's nil policy explicit. Callers that test
// the result against nil must pass an untyped nil fallback; callers that
// historically returned their possibly-nil *font.Standard field unchecked
// pass that same field as the fallback, preserving the typed-nil interface
// they returned before consolidation.
func resolveMeasurer(embedded *font.EmbeddedFont, std *font.Standard, fallback font.TextMeasurer) font.TextMeasurer {
	if embedded != nil {
		return embedded
	}
	if std != nil {
		return std
	}
	return fallback
}

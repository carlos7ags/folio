// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

// Node represents a parsed SVG element.
type Node struct {
	Attrs     map[string]string
	Tag       string
	Text      string // text content for <text> elements
	Children  []*Node
	Style     Style
	Transform Matrix
}

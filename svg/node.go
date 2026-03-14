// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

// Node represents a parsed SVG element.
type Node struct {
	Tag       string
	Attrs     map[string]string
	Style     Style
	Transform Matrix
	Children  []*Node
	Text      string // text content for <text> elements
}

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package core implements the eight fundamental PDF object types
// defined in ISO 32000 §7.3, plus indirect references.
package core

import "io"

// ObjectType enumerates the PDF object types.
type ObjectType int

const (
	ObjectTypeBoolean    ObjectType = iota
	ObjectTypeNumber                // integer or real
	ObjectTypeString                // literal or hexadecimal
	ObjectTypeName                  // /Name
	ObjectTypeArray                 // [...]
	ObjectTypeDictionary            // << ... >>
	ObjectTypeStream                // dictionary + byte sequence
	ObjectTypeNull                  // null
	ObjectTypeReference             // indirect reference (e.g. 1 0 R)
)

// PdfObject is the interface satisfied by every PDF object type.
type PdfObject interface {
	// WriteTo serializes the object in PDF syntax to w.
	WriteTo(w io.Writer) (int64, error)

	// Type returns the object's PDF type.
	Type() ObjectType
}

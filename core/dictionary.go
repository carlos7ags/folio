// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"io"
)

// DictEntry is a key-value pair in a PdfDictionary.
// We use a slice of entries rather than a map to preserve insertion order,
// which produces deterministic PDF output (important for testing and
// byte-level reproducibility).
type DictEntry struct {
	Key   *PdfName
	Value PdfObject
}

// PdfDictionary represents a PDF dictionary object (ISO 32000 §7.3.7).
// Keys are PdfName objects; values can be any PdfObject.
type PdfDictionary struct {
	Entries []DictEntry
}

// NewPdfDictionary creates a new empty PdfDictionary.
func NewPdfDictionary() *PdfDictionary {
	return &PdfDictionary{}
}

// Type returns ObjectTypeDictionary.
func (d *PdfDictionary) Type() ObjectType { return ObjectTypeDictionary }

// Set adds or updates an entry. If the key already exists, its value is replaced.
// Panics if value is nil.
func (d *PdfDictionary) Set(key string, value PdfObject) {
	if value == nil {
		panic("core.PdfDictionary.Set: nil value for key " + key)
	}
	for i, e := range d.Entries {
		if e.Key.Value == key {
			d.Entries[i].Value = value
			return
		}
	}
	d.Entries = append(d.Entries, DictEntry{
		Key:   NewPdfName(key),
		Value: value,
	})
}

// Get retrieves a value by key name. Returns nil if not found.
func (d *PdfDictionary) Get(key string) PdfObject {
	for _, e := range d.Entries {
		if e.Key.Value == key {
			return e.Value
		}
	}
	return nil
}

// Remove deletes an entry by key name. Does nothing if the key does not exist.
func (d *PdfDictionary) Remove(key string) {
	for i, e := range d.Entries {
		if e.Key.Value == key {
			d.Entries = append(d.Entries[:i], d.Entries[i+1:]...)
			return
		}
	}
}

// WriteTo serializes the dictionary in PDF syntax to w.
func (d *PdfDictionary) WriteTo(w io.Writer) (int64, error) {
	cw := &countingWriter{w: w}

	if _, err := cw.WriteString("<<"); err != nil {
		return cw.n, err
	}

	for _, entry := range d.Entries {
		// Space before each key
		if _, err := cw.WriteString(" "); err != nil {
			return cw.n, err
		}
		if _, err := entry.Key.WriteTo(cw); err != nil {
			return cw.n, err
		}
		if _, err := cw.WriteString(" "); err != nil {
			return cw.n, err
		}
		if _, err := entry.Value.WriteTo(cw); err != nil {
			return cw.n, err
		}
	}

	if _, err := cw.WriteString(" >>"); err != nil {
		return cw.n, err
	}

	return cw.n, nil
}

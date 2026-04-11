// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"io"
	"iter"
	"slices"
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
//
// Insertion order is preserved to produce deterministic PDF output.
// A lazily-built map index accelerates key lookups from O(n) to O(1).
type PdfDictionary struct {
	// Entries is the ordered list of key-value pairs.
	//
	// Deprecated: direct field access is retained for backward compatibility.
	// Prefer [PdfDictionary.All] or [PdfDictionary.Get] in new code so that
	// the internal index stays consistent with any mutations.
	Entries []DictEntry

	// index maps key name → position in Entries. It is lazily built on
	// first use and rebuilt after any operation that reorders or removes
	// entries. A nil index means "not yet built" and is safe to use —
	// ensureIndex populates it on demand.
	index map[string]int
}

// NewPdfDictionary creates a new empty PdfDictionary.
func NewPdfDictionary() *PdfDictionary {
	return &PdfDictionary{}
}

// Type returns ObjectTypeDictionary.
func (d *PdfDictionary) Type() ObjectType { return ObjectTypeDictionary }

// ensureIndex lazily populates the key→position map. Called by every
// mutator and accessor so the index is always consistent with Entries.
//
// The index is rebuilt if its length disagrees with Entries, which
// catches the common case where external code assigns to Entries
// directly (e.g., slicing it to remove an element). Same-length
// mutations are not detected — direct Entries access is deprecated
// for this reason.
func (d *PdfDictionary) ensureIndex() {
	if d.index != nil && len(d.index) == len(d.Entries) {
		return
	}
	d.index = make(map[string]int, len(d.Entries))
	for i, e := range d.Entries {
		d.index[e.Key.Value] = i
	}
}

// Set adds or updates an entry. If the key already exists, its value is replaced.
// Panics if value is nil.
func (d *PdfDictionary) Set(key string, value PdfObject) {
	if value == nil {
		panic("core.PdfDictionary.Set: nil value for key " + key)
	}
	d.ensureIndex()
	if i, ok := d.index[key]; ok {
		d.Entries[i].Value = value
		return
	}
	d.index[key] = len(d.Entries)
	d.Entries = append(d.Entries, DictEntry{
		Key:   NewPdfName(key),
		Value: value,
	})
}

// Get retrieves a value by key name. Returns nil if not found.
func (d *PdfDictionary) Get(key string) PdfObject {
	d.ensureIndex()
	if i, ok := d.index[key]; ok {
		return d.Entries[i].Value
	}
	return nil
}

// Remove deletes an entry by key name. Does nothing if the key does not exist.
func (d *PdfDictionary) Remove(key string) {
	d.ensureIndex()
	i, ok := d.index[key]
	if !ok {
		return
	}
	d.Entries = slices.Delete(d.Entries, i, i+1)
	delete(d.index, key)
	// Indexes for entries after the removed one have shifted down by one.
	for j := i; j < len(d.Entries); j++ {
		d.index[d.Entries[j].Key.Value] = j
	}
}

// Len returns the number of entries.
func (d *PdfDictionary) Len() int {
	return len(d.Entries)
}

// All returns an iterator over (key, value) pairs in insertion order.
// Using this iterator insulates callers from the underlying representation
// of [PdfDictionary.Entries].
func (d *PdfDictionary) All() iter.Seq2[string, PdfObject] {
	return func(yield func(string, PdfObject) bool) {
		for _, e := range d.Entries {
			if !yield(e.Key.Value, e.Value) {
				return
			}
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

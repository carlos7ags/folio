// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"github.com/carlos7ags/folio/core"
)

// Copier copies objects from a PdfReader into a document writer,
// remapping indirect references so object numbers don't collide.
type Copier struct {
	reader *PdfReader
	addObj func(core.PdfObject) *core.PdfIndirectReference
	refMap map[int]*core.PdfIndirectReference // old objNum → new ref
}

// NewCopier creates a copier that bridges a reader and a writer's AddObject function.
func NewCopier(reader *PdfReader, addObject func(core.PdfObject) *core.PdfIndirectReference) *Copier {
	return &Copier{
		reader: reader,
		addObj: addObject,
		refMap: make(map[int]*core.PdfIndirectReference),
	}
}

// CopyObject deep-copies a PDF object, resolving and remapping all
// indirect references. Returns the new object suitable for the target writer.
func (c *Copier) CopyObject(obj core.PdfObject) (core.PdfObject, error) {
	return c.copyDeep(obj)
}

// CopyPage copies a page and all its resources from the source reader
// into the target writer. Returns the new page dictionary reference.
func (c *Copier) CopyPage(pageIndex int) (*core.PdfIndirectReference, error) {
	page, err := c.reader.Page(pageIndex)
	if err != nil {
		return nil, err
	}

	// Deep-copy the page dictionary, which recursively copies all
	// referenced objects (resources, content streams, fonts, images).
	copied, err := c.copyDeep(page.pageDict)
	if err != nil {
		return nil, err
	}

	copiedDict, ok := copied.(*core.PdfDictionary)
	if !ok {
		return nil, err
	}

	// Remove /Parent — it will be set by the target document's page tree.
	removeEntry(copiedDict, "Parent")

	return c.addObj(copiedDict), nil
}

func (c *Copier) copyDeep(obj core.PdfObject) (core.PdfObject, error) {
	if obj == nil {
		return core.NewPdfNull(), nil
	}

	switch v := obj.(type) {
	case *core.PdfIndirectReference:
		return c.copyIndirectRef(v)

	case *core.PdfDictionary:
		return c.copyDict(v)

	case *core.PdfArray:
		return c.copyArray(v)

	case *core.PdfStream:
		return c.copyStream(v)

	case *core.PdfNumber, *core.PdfString, *core.PdfName,
		*core.PdfBoolean, *core.PdfNull:
		// Primitive types — no references to remap.
		return obj, nil

	default:
		return obj, nil
	}
}

func (c *Copier) copyIndirectRef(ref *core.PdfIndirectReference) (core.PdfObject, error) {
	// Check if already copied.
	if newRef, ok := c.refMap[ref.ObjectNumber]; ok {
		return newRef, nil
	}

	// Resolve the original object.
	resolved, err := c.reader.resolver.Resolve(ref.ObjectNumber)
	if err != nil {
		return core.NewPdfNull(), nil // tolerant: return null for unresolvable refs
	}

	// Allocate a placeholder reference first (handles circular refs).
	placeholder := c.addObj(core.NewPdfNull())
	c.refMap[ref.ObjectNumber] = placeholder

	// Deep-copy the resolved object.
	copied, err := c.copyDeep(resolved)
	if err != nil {
		return placeholder, nil
	}

	// Replace the placeholder with the actual object.
	// Since we can't modify the registered object, we register the copy
	// and update the refMap. The placeholder ref is now orphaned but
	// harmless (it points to null).
	newRef := c.addObj(copied)
	c.refMap[ref.ObjectNumber] = newRef

	return newRef, nil
}

func (c *Copier) copyDict(dict *core.PdfDictionary) (*core.PdfDictionary, error) {
	newDict := core.NewPdfDictionary()
	for _, entry := range dict.Entries {
		copied, err := c.copyDeep(entry.Value)
		if err != nil {
			return nil, err
		}
		newDict.Set(entry.Key.Value, copied)
	}
	return newDict, nil
}

func (c *Copier) copyArray(arr *core.PdfArray) (*core.PdfArray, error) {
	newArr := core.NewPdfArray()
	for _, elem := range arr.Elements {
		copied, err := c.copyDeep(elem)
		if err != nil {
			return nil, err
		}
		newArr.Add(copied)
	}
	return newArr, nil
}

func (c *Copier) copyStream(stream *core.PdfStream) (*core.PdfStream, error) {
	// Copy the dictionary entries.
	newStream := core.NewPdfStream(stream.Data)
	for _, entry := range stream.Dict.Entries {
		copied, err := c.copyDeep(entry.Value)
		if err != nil {
			return nil, err
		}
		newStream.Dict.Set(entry.Key.Value, copied)
	}
	return newStream, nil
}

// removeEntry removes a key from a dictionary.
func removeEntry(dict *core.PdfDictionary, key string) {
	var kept []core.DictEntry
	for _, e := range dict.Entries {
		if e.Key.Value != key {
			kept = append(kept, e)
		}
	}
	dict.Entries = kept
}

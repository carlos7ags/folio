// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"
import (
	"fmt"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// runList accumulates TextRuns for passing to SetRuns / AddItemRuns.
type runList struct {
	runs []layout.TextRun
}

// folio_run_list_new creates a new empty run list for building styled text.
//
//export folio_run_list_new
func folio_run_list_new() C.uint64_t {
	return C.uint64_t(ht.store(&runList{}))
}

// folio_run_list_add adds a text run with a standard font to the list.
//
//export folio_run_list_add
func folio_run_list_add(rlH C.uint64_t, text *C.char, fontH C.uint64_t, fontSize C.double,
	r, g, b C.double) C.int32_t {
	rl, errCode := loadRunList(rlH)
	if errCode != errOK {
		return errCode
	}
	f, errCode := loadStandardFont(fontH)
	if errCode != errOK {
		return errCode
	}
	rl.runs = append(rl.runs, layout.TextRun{
		Text:     C.GoString(text),
		Font:     f,
		FontSize: float64(fontSize),
		Color:    layout.RGB(float64(r), float64(g), float64(b)),
	})
	return errOK
}

// folio_run_list_add_embedded adds a text run with an embedded font to the list.
//
//export folio_run_list_add_embedded
func folio_run_list_add_embedded(rlH C.uint64_t, text *C.char, fontH C.uint64_t, fontSize C.double,
	r, g, b C.double) C.int32_t {
	rl, errCode := loadRunList(rlH)
	if errCode != errOK {
		return errCode
	}
	ef, errCode := loadEmbeddedFont(fontH)
	if errCode != errOK {
		return errCode
	}
	rl.runs = append(rl.runs, layout.TextRun{
		Text:     C.GoString(text),
		Embedded: ef,
		FontSize: float64(fontSize),
		Color:    layout.RGB(float64(r), float64(g), float64(b)),
	})
	return errOK
}

// folio_run_list_add_link adds a text run that is a hyperlink.
//
//export folio_run_list_add_link
func folio_run_list_add_link(rlH C.uint64_t, text *C.char, fontH C.uint64_t, fontSize C.double,
	r, g, b C.double, uri *C.char, underline C.int32_t) C.int32_t {
	rl, errCode := loadRunList(rlH)
	if errCode != errOK {
		return errCode
	}
	run := layout.TextRun{
		Text:     C.GoString(text),
		FontSize: float64(fontSize),
		Color:    layout.RGB(float64(r), float64(g), float64(b)),
		LinkURI:  C.GoString(uri),
	}
	if underline != 0 {
		run.Decoration = layout.DecorationUnderline
	}
	// Determine font type from handle.
	v := ht.load(uint64(fontH))
	if v == nil {
		setLastError("invalid font handle")
		return errInvalidHandle
	}
	switch f := v.(type) {
	case *font.Standard:
		run.Font = f
	case *font.EmbeddedFont:
		run.Embedded = f
	default:
		setLastError(fmt.Sprintf("handle %d is not a font (type %T)", uint64(fontH), v))
		return errTypeMismatch
	}
	rl.runs = append(rl.runs, run)
	return errOK
}

// folio_run_list_last_set_underline sets underline decoration on the last added run.
//
//export folio_run_list_last_set_underline
func folio_run_list_last_set_underline(rlH C.uint64_t) C.int32_t {
	rl, errCode := loadRunList(rlH)
	if errCode != errOK {
		return errCode
	}
	if len(rl.runs) == 0 {
		setLastError("run list is empty")
		return errInvalidArg
	}
	rl.runs[len(rl.runs)-1].Decoration = layout.DecorationUnderline
	return errOK
}

// folio_run_list_last_set_strikethrough sets strikethrough decoration on the last added run.
//
//export folio_run_list_last_set_strikethrough
func folio_run_list_last_set_strikethrough(rlH C.uint64_t) C.int32_t {
	rl, errCode := loadRunList(rlH)
	if errCode != errOK {
		return errCode
	}
	if len(rl.runs) == 0 {
		setLastError("run list is empty")
		return errInvalidArg
	}
	rl.runs[len(rl.runs)-1].Decoration = layout.DecorationStrikethrough
	return errOK
}

// folio_run_list_last_set_letter_spacing sets letter-spacing on the last added run.
//
//export folio_run_list_last_set_letter_spacing
func folio_run_list_last_set_letter_spacing(rlH C.uint64_t, spacing C.double) C.int32_t {
	rl, errCode := loadRunList(rlH)
	if errCode != errOK {
		return errCode
	}
	if len(rl.runs) == 0 {
		setLastError("run list is empty")
		return errInvalidArg
	}
	rl.runs[len(rl.runs)-1].LetterSpacing = float64(spacing)
	return errOK
}

// folio_run_list_free removes a run list handle.
//
//export folio_run_list_free
func folio_run_list_free(rlH C.uint64_t) {
	ht.delete(uint64(rlH))
}

// ── Apply run list to elements ─────────────────────────────────────

// folio_heading_set_runs replaces the heading's text with the given run list.
//
//export folio_heading_set_runs
func folio_heading_set_runs(hH C.uint64_t, rlH C.uint64_t) C.int32_t {
	h, errCode := loadHeading(hH)
	if errCode != errOK {
		return errCode
	}
	rl, errCode := loadRunList(rlH)
	if errCode != errOK {
		return errCode
	}
	h.SetRuns(rl.runs)
	return errOK
}

// folio_list_add_item_runs adds a list item with styled text runs.
//
//export folio_list_add_item_runs
func folio_list_add_item_runs(listH C.uint64_t, rlH C.uint64_t) C.int32_t {
	l, errCode := loadList(listH)
	if errCode != errOK {
		return errCode
	}
	rl, errCode := loadRunList(rlH)
	if errCode != errOK {
		return errCode
	}
	l.AddItemRuns(rl.runs)
	return errOK
}

// folio_list_add_item_runs_with_sublist adds a list item with styled runs and returns a nested sub-list.
//
//export folio_list_add_item_runs_with_sublist
func folio_list_add_item_runs_with_sublist(listH C.uint64_t, rlH C.uint64_t) C.uint64_t {
	l, errCode := loadList(listH)
	if errCode != errOK {
		return 0
	}
	rl, errCode := loadRunList(rlH)
	if errCode != errOK {
		return 0
	}
	sub := l.AddItemRunsWithSubList(rl.runs)
	return C.uint64_t(ht.store(sub))
}

func loadRunList(h C.uint64_t) (*runList, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid run list handle")
		return nil, errInvalidHandle
	}
	rl, ok := v.(*runList)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a run list (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return rl, errOK
}

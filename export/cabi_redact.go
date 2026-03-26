// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"
import (
	"regexp"
	"unsafe"

	"github.com/carlos7ags/folio/reader"
)

// folio_redact_text redacts all occurrences of each target string from a PDF.
// targets is an array of C strings, count is the number of targets.
// Returns a modifier handle on success, 0 on failure.
//
//export folio_redact_text
func folio_redact_text(rH C.uint64_t, targets **C.char, count C.int32_t, optsH C.uint64_t) C.uint64_t {
	r, errCode := loadReader(rH)
	if errCode != errOK {
		return 0
	}
	n := int(count)
	if n <= 0 || targets == nil {
		setLastError("redact_text requires at least one target")
		return 0
	}
	cTargets := (*[1 << 20]*C.char)(unsafe.Pointer(targets))[:n:n]
	goTargets := make([]string, n)
	for i := 0; i < n; i++ {
		goTargets[i] = C.GoString(cTargets[i])
	}
	opts := loadRedactOpts(optsH)
	m, err := reader.RedactText(r, goTargets, opts)
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(m))
}

// folio_redact_pattern redacts all matches of a regex pattern from a PDF.
// Returns a modifier handle on success, 0 on failure.
//
//export folio_redact_pattern
func folio_redact_pattern(rH C.uint64_t, pattern *C.char, optsH C.uint64_t) C.uint64_t {
	r, errCode := loadReader(rH)
	if errCode != errOK {
		return 0
	}
	re, err := regexp.Compile(C.GoString(pattern))
	if err != nil {
		setLastError("invalid regex: " + err.Error())
		return 0
	}
	opts := loadRedactOpts(optsH)
	m, err := reader.RedactPattern(r, re, opts)
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(m))
}

// folio_redact_regions redacts specified rectangular areas from a PDF.
// pages, x1s, y1s, x2s, y2s are parallel arrays defining the marks.
// Returns a modifier handle on success, 0 on failure.
//
//export folio_redact_regions
func folio_redact_regions(rH C.uint64_t, pages *C.int32_t, x1s, y1s, x2s, y2s *C.double, count C.int32_t, optsH C.uint64_t) C.uint64_t {
	r, errCode := loadReader(rH)
	if errCode != errOK {
		return 0
	}
	n := int(count)
	if n <= 0 {
		setLastError("redact_regions requires at least one mark")
		return 0
	}
	cPages := (*[1 << 20]C.int32_t)(unsafe.Pointer(pages))[:n:n]
	cX1 := (*[1 << 20]C.double)(unsafe.Pointer(x1s))[:n:n]
	cY1 := (*[1 << 20]C.double)(unsafe.Pointer(y1s))[:n:n]
	cX2 := (*[1 << 20]C.double)(unsafe.Pointer(x2s))[:n:n]
	cY2 := (*[1 << 20]C.double)(unsafe.Pointer(y2s))[:n:n]

	marks := make([]reader.RedactionMark, n)
	for i := 0; i < n; i++ {
		marks[i] = reader.RedactionMark{
			Page: int(cPages[i]),
			Rect: reader.Box{
				X1: float64(cX1[i]), Y1: float64(cY1[i]),
				X2: float64(cX2[i]), Y2: float64(cY2[i]),
			},
		}
	}
	opts := loadRedactOpts(optsH)
	m, err := reader.RedactRegions(r, marks, opts)
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(m))
}

// folio_redact_opts_new creates a new RedactOptions handle with defaults.
//
//export folio_redact_opts_new
func folio_redact_opts_new() C.uint64_t {
	opts := &reader.RedactOptions{}
	return C.uint64_t(ht.store(opts))
}

// folio_redact_opts_free releases a RedactOptions handle.
//
//export folio_redact_opts_free
func folio_redact_opts_free(h C.uint64_t) {
	ht.delete(uint64(h))
}

// folio_redact_opts_set_fill_color sets the redaction box fill color (RGB 0-1).
//
//export folio_redact_opts_set_fill_color
func folio_redact_opts_set_fill_color(h C.uint64_t, r, g, b C.double) C.int32_t {
	opts, errCode := loadRedactOptsRequired(h)
	if errCode != errOK {
		return errCode
	}
	opts.FillColor = [3]float64{float64(r), float64(g), float64(b)}
	return errOK
}

// folio_redact_opts_set_overlay sets overlay text, font size, and color.
//
//export folio_redact_opts_set_overlay
func folio_redact_opts_set_overlay(h C.uint64_t, text *C.char, fontSize C.double, r, g, b C.double) C.int32_t {
	opts, errCode := loadRedactOptsRequired(h)
	if errCode != errOK {
		return errCode
	}
	opts.OverlayText = C.GoString(text)
	opts.OverlayFontSize = float64(fontSize)
	opts.OverlayColor = [3]float64{float64(r), float64(g), float64(b)}
	return errOK
}

// folio_redact_opts_set_strip_metadata enables or disables metadata stripping.
//
//export folio_redact_opts_set_strip_metadata
func folio_redact_opts_set_strip_metadata(h C.uint64_t, strip C.int32_t) C.int32_t {
	opts, errCode := loadRedactOptsRequired(h)
	if errCode != errOK {
		return errCode
	}
	opts.StripMetadata = strip != 0
	return errOK
}

// loadRedactOpts returns nil if h is 0, or the RedactOptions from the handle.
func loadRedactOpts(h C.uint64_t) *reader.RedactOptions {
	if h == 0 {
		return nil
	}
	v := ht.load(uint64(h))
	if v == nil {
		return nil
	}
	opts, _ := v.(*reader.RedactOptions)
	return opts
}

// loadRedactOptsRequired loads a required RedactOptions handle.
func loadRedactOptsRequired(h C.uint64_t) (*reader.RedactOptions, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid redact options handle")
		return nil, errInvalidHandle
	}
	opts, ok := v.(*reader.RedactOptions)
	if !ok {
		setLastError("handle is not a redact options")
		return nil, errTypeMismatch
	}
	return opts, errOK
}

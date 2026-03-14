// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"fmt"
	"time"

	"github.com/carlos7ags/folio/core"
)

// Info holds document metadata written to the PDF trailer's /Info dictionary
// (ISO 32000 §14.3.3).
type Info struct {
	Title        string
	Author       string
	Subject      string
	Keywords     string
	Creator      string // application that created the original content
	Producer     string // application that produced the PDF
	CreationDate time.Time
	ModDate      time.Time
}

// toDict converts Info to a PdfDictionary. Only non-zero fields are included.
func (info *Info) toDict() *core.PdfDictionary {
	d := core.NewPdfDictionary()

	if info.Title != "" {
		d.Set("Title", core.NewPdfLiteralString(info.Title))
	}
	if info.Author != "" {
		d.Set("Author", core.NewPdfLiteralString(info.Author))
	}
	if info.Subject != "" {
		d.Set("Subject", core.NewPdfLiteralString(info.Subject))
	}
	if info.Keywords != "" {
		d.Set("Keywords", core.NewPdfLiteralString(info.Keywords))
	}
	if info.Creator != "" {
		d.Set("Creator", core.NewPdfLiteralString(info.Creator))
	}
	if info.Producer != "" {
		d.Set("Producer", core.NewPdfLiteralString(info.Producer))
	}
	if !info.CreationDate.IsZero() {
		d.Set("CreationDate", core.NewPdfLiteralString(formatPdfDate(info.CreationDate)))
	}
	if !info.ModDate.IsZero() {
		d.Set("ModDate", core.NewPdfLiteralString(formatPdfDate(info.ModDate)))
	}

	return d
}

// isEmpty reports whether all fields are zero-valued.
func (info *Info) isEmpty() bool {
	return info.Title == "" && info.Author == "" && info.Subject == "" &&
		info.Keywords == "" && info.Creator == "" && info.Producer == "" &&
		info.CreationDate.IsZero() && info.ModDate.IsZero()
}

// formatPdfDate formats a time.Time as a PDF date string.
// PDF date format: D:YYYYMMDDHHmmSS+HH'mm' (ISO 32000 §7.9.4).
func formatPdfDate(t time.Time) string {
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	return fmt.Sprintf("D:%04d%02d%02d%02d%02d%02d%s%02d'%02d'",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second(),
		sign, hours, minutes)
}

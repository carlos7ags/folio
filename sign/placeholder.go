// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"fmt"
	"io"
	"strings"

	"github.com/carlos7ags/folio/core"
)

// contentsPlaceholderLen is the byte length of the hex-encoded /Contents value.
// This accommodates CMS signatures up to ~16KB (32768 hex chars = 16384 raw bytes).
// The actual signature is typically 2-8KB; we pad with zeroes.
const contentsPlaceholderLen = 32768

// byteRangeWidth is the fixed width for each integer in the /ByteRange array.
// Using fixed width avoids offset shifts when patching the real values.
const byteRangeWidth = 10

// signaturePlaceholder holds position information for patching the signature.
type signaturePlaceholder struct {
	// SigDictObjNum is the object number assigned to the signature dictionary.
	SigDictObjNum int

	// ByteRangeOffset is the byte position in the output where /ByteRange starts.
	ByteRangeOffset int

	// ContentsOffset is the byte position of the opening '<' of the /Contents hex string.
	ContentsOffset int

	// ContentsLen is the length of the hex placeholder including delimiters (< ... >).
	ContentsLen int
}

// byteRangePlaceholder is the fixed-width /ByteRange value before patching.
// Format: [0000000000 0000000000 0000000000 0000000000]
var byteRangePlaceholder = fmt.Sprintf("[%s %s %s %s]",
	strings.Repeat("0", byteRangeWidth),
	strings.Repeat("0", byteRangeWidth),
	strings.Repeat("0", byteRangeWidth),
	strings.Repeat("0", byteRangeWidth),
)

// contentsPlaceholder is the hex string placeholder: < 00...00 >
var contentsPlaceholder = "<" + strings.Repeat("0", contentsPlaceholderLen) + ">"

// buildSigDict creates a PAdES signature dictionary with placeholder values.
// The dictionary uses /SubFilter /ETSI.CAdES.detached for PAdES compliance.
//
// The returned dictionary serializes /ByteRange and /Contents as literal strings
// so we can find and patch them by byte offset.
func buildSigDict(name, location, reason, contactInfo string) *sigDictionary {
	return &sigDictionary{
		filter:      "Adobe.PPKLite",
		subFilter:   "ETSI.CAdES.detached",
		name:        name,
		location:    location,
		reason:      reason,
		contactInfo: contactInfo,
	}
}

// sigDictionary is a custom PdfObject that serializes with fixed-width
// /ByteRange and /Contents placeholders for later patching.
type sigDictionary struct {
	filter      string
	subFilter   string
	name        string
	location    string
	reason      string
	contactInfo string
}

func (s *sigDictionary) Type() core.ObjectType { return core.ObjectTypeDictionary }

func (s *sigDictionary) WriteTo(w io.Writer) (int64, error) {
	var total int64
	write := func(str string) error {
		n, err := w.Write([]byte(str))
		total += int64(n)
		return err
	}

	if err := write("<< /Type /Sig"); err != nil {
		return total, err
	}
	if err := write(fmt.Sprintf(" /Filter /%s", s.filter)); err != nil {
		return total, err
	}
	if err := write(fmt.Sprintf(" /SubFilter /%s", s.subFilter)); err != nil {
		return total, err
	}
	if s.name != "" {
		if err := write(fmt.Sprintf(" /Name (%s)", escapePdfString(s.name))); err != nil {
			return total, err
		}
	}
	if s.location != "" {
		if err := write(fmt.Sprintf(" /Location (%s)", escapePdfString(s.location))); err != nil {
			return total, err
		}
	}
	if s.reason != "" {
		if err := write(fmt.Sprintf(" /Reason (%s)", escapePdfString(s.reason))); err != nil {
			return total, err
		}
	}
	if s.contactInfo != "" {
		if err := write(fmt.Sprintf(" /ContactInfo (%s)", escapePdfString(s.contactInfo))); err != nil {
			return total, err
		}
	}

	// /ByteRange — fixed-width placeholder
	if err := write(" /ByteRange "); err != nil {
		return total, err
	}
	if err := write(byteRangePlaceholder); err != nil {
		return total, err
	}

	// /Contents — hex string placeholder
	if err := write(" /Contents "); err != nil {
		return total, err
	}
	if err := write(contentsPlaceholder); err != nil {
		return total, err
	}

	if err := write(" >>"); err != nil {
		return total, err
	}
	return total, nil
}

// patchByteRange writes the real ByteRange values into the placeholder.
// The ByteRange is [0, contentsStart, contentsEnd, endOffset-contentsEnd]
// where contentsStart..contentsEnd is the hex /Contents value.
func patchByteRange(pdf []byte, ph signaturePlaceholder) {
	fileLen := len(pdf)
	contentsStart := ph.ContentsOffset
	contentsEnd := ph.ContentsOffset + ph.ContentsLen

	br := fmt.Sprintf("[%0*d %0*d %0*d %0*d]",
		byteRangeWidth, 0,
		byteRangeWidth, contentsStart,
		byteRangeWidth, contentsEnd,
		byteRangeWidth, fileLen-contentsEnd,
	)
	copy(pdf[ph.ByteRangeOffset:], []byte(br))
}

// patchContents writes the DER-encoded CMS signature into the /Contents placeholder.
func patchContents(pdf []byte, ph signaturePlaceholder, sig []byte) error {
	maxSigLen := contentsPlaceholderLen / 2
	if len(sig) > maxSigLen {
		return fmt.Errorf("sign: signature too large (%d bytes, max %d)", len(sig), maxSigLen)
	}

	// Encode as hex string.
	hex := fmt.Sprintf("%X", sig)
	// Pad with trailing zeroes to fill the placeholder.
	hex += strings.Repeat("0", contentsPlaceholderLen-len(hex))

	// Write into the placeholder (between < and >).
	copy(pdf[ph.ContentsOffset+1:], []byte(hex))
	return nil
}

// escapePdfString escapes special characters for PDF literal strings.
func escapePdfString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
}

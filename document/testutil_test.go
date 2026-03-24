// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// streamRe matches FlateDecode stream objects in raw PDF bytes.
var streamRe = regexp.MustCompile(`(?s)/Filter\s+/FlateDecode[^>]*>>\s*\nstream\r?\n(.*?)endstream`)

// decompressedContentStreams extracts and decompresses all FlateDecode streams
// from a PDF. This is used by tests that need to inspect PDF operators after
// FlateDecode compression was enabled. It also includes any uncompressed text
// from the raw PDF for operators that are outside compressed streams.
func decompressedContentStreams(t *testing.T, pdfBytes []byte) string {
	t.Helper()
	var sb strings.Builder

	matches := streamRe.FindAllSubmatch(pdfBytes, -1)
	for _, m := range matches {
		data := m[1]
		r, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			continue // not a valid zlib stream, skip
		}
		decompressed, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			continue
		}
		sb.Write(decompressed)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// extractAllTextDoc extracts text by decompressing all content streams and
// pulling out text between BT/ET blocks. This is a simplified extractor for
// tests that just need to check for text presence in the output.
func extractAllTextDoc(t *testing.T, pdfBytes []byte) string {
	t.Helper()
	cs := decompressedContentStreams(t, pdfBytes)
	// Extract text string operands from Tj and TJ operators.
	var sb strings.Builder
	// Match literal strings like (text) Tj
	tjRe := regexp.MustCompile(`\(([^)]*)\)\s*Tj`)
	for _, m := range tjRe.FindAllStringSubmatch(cs, -1) {
		sb.WriteString(m[1])
		sb.WriteString(" ")
	}
	// Match TJ array elements: [(text) kern (text) ...] TJ
	tjArrayRe := regexp.MustCompile(`\[([^\]]*)\]\s*TJ`)
	parenRe := regexp.MustCompile(`\(([^)]*)\)`)
	for _, m := range tjArrayRe.FindAllStringSubmatch(cs, -1) {
		for _, pm := range parenRe.FindAllStringSubmatch(m[1], -1) {
			sb.WriteString(pm[1])
		}
		sb.WriteString(" ")
	}
	return sb.String()
}

// runQpdfCheck validates PDF bytes using qpdf --check.
// Skips if qpdf is not installed.
func runQpdfCheck(t *testing.T, pdfBytes []byte) {
	t.Helper()
	qpdfPath, err := exec.LookPath("qpdf")
	if err != nil {
		t.Skip("qpdf not installed, skipping validation")
	}
	tmpFile := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(tmpFile, pdfBytes, 0644); err != nil {
		t.Fatalf("write temp PDF: %v", err)
	}
	cmd := exec.Command(qpdfPath, "--check", tmpFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("qpdf --check failed: %v\n%s", err, output)
	}
}

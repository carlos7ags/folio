// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func encryptedDoc(t *testing.T, alg EncryptionAlgorithm, userPwd, ownerPwd string, perms core.Permission) []byte {
	t.Helper()
	doc := NewDocument(PageSizeA4)
	doc.SetEncryption(EncryptionConfig{
		Algorithm:     alg,
		UserPassword:  userPwd,
		OwnerPassword: ownerPwd,
		Permissions:   perms,
	})
	p := layout.NewParagraph("Hello, encrypted world!", font.Helvetica, 12)
	doc.Add(p)
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return buf.Bytes()
}

func TestEncryptionAES256(t *testing.T) {
	pdf := encryptedDoc(t, EncryptAES256, "user123", "owner456", core.PermPrint|core.PermExtract)
	if len(pdf) == 0 {
		t.Fatal("empty PDF output")
	}
	// Verify the PDF contains an /Encrypt reference.
	if !bytes.Contains(pdf, []byte("/Encrypt")) {
		t.Error("PDF missing /Encrypt in trailer")
	}
	runQpdfCheckEncrypted(t, pdf, "user123")
}

func TestEncryptionAES128(t *testing.T) {
	pdf := encryptedDoc(t, EncryptAES128, "pass", "admin", core.PermAll)
	if !bytes.Contains(pdf, []byte("/Encrypt")) {
		t.Error("PDF missing /Encrypt")
	}
	runQpdfCheckEncrypted(t, pdf, "pass")
}

func TestEncryptionRC4128(t *testing.T) {
	pdf := encryptedDoc(t, EncryptRC4128, "rc4test", "rc4owner", core.PermPrint)
	if !bytes.Contains(pdf, []byte("/Encrypt")) {
		t.Error("PDF missing /Encrypt")
	}
	runQpdfCheckEncrypted(t, pdf, "rc4test")
}

func TestEncryptionEmptyPassword(t *testing.T) {
	// Empty user password: anyone can open, owner password protects permissions.
	pdf := encryptedDoc(t, EncryptAES256, "", "owner", core.PermPrint)
	if !bytes.Contains(pdf, []byte("/Encrypt")) {
		t.Error("PDF missing /Encrypt")
	}
	runQpdfCheckEncrypted(t, pdf, "")
}

func TestEncryptionNoPermissions(t *testing.T) {
	pdf := encryptedDoc(t, EncryptAES256, "user", "owner", 0)
	if !bytes.Contains(pdf, []byte("/Encrypt")) {
		t.Error("PDF missing /Encrypt")
	}
	runQpdfCheckEncrypted(t, pdf, "user")
}

func TestEncryptionAllPermissions(t *testing.T) {
	pdf := encryptedDoc(t, EncryptAES256, "user", "owner", core.PermAll)
	runQpdfCheckEncrypted(t, pdf, "user")
}

func TestEncryptionHasIDArray(t *testing.T) {
	pdf := encryptedDoc(t, EncryptAES256, "test", "test", core.PermAll)
	if !bytes.Contains(pdf, []byte("/ID")) {
		t.Error("PDF missing /ID array in trailer")
	}
}

func TestEncryptionMultiplePages(t *testing.T) {
	doc := NewDocument(PageSizeA4)
	doc.SetEncryption(EncryptionConfig{
		Algorithm:    EncryptAES256,
		UserPassword: "multi",
		Permissions:  core.PermAll,
	})
	for i := 0; i < 3; i++ {
		p := layout.NewParagraph("Page content for encryption test", font.Helvetica, 12)
		doc.Add(p)
		doc.Add(layout.NewAreaBreak())
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	runQpdfCheckEncrypted(t, buf.Bytes(), "multi")
}

// runQpdfCheckEncrypted validates an encrypted PDF using qpdf.
func runQpdfCheckEncrypted(t *testing.T, pdfBytes []byte, password string) {
	t.Helper()
	qpdfPath, err := exec.LookPath("qpdf")
	if err != nil {
		t.Skip("qpdf not installed, skipping validation")
	}
	tmpFile := filepath.Join(t.TempDir(), "encrypted.pdf")
	if err := os.WriteFile(tmpFile, pdfBytes, 0644); err != nil {
		t.Fatalf("write temp PDF: %v", err)
	}
	args := []string{"--check"}
	if password != "" {
		args = append(args, "--password="+password)
	}
	args = append(args, tmpFile)
	cmd := exec.Command(qpdfPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("qpdf --check failed: %v\n%s", err, output)
	}
}

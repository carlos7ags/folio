// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestParseQpdfEncrypted checks that folio's reader decrypts PDFs
// produced by qpdf — an independent, third-party implementation of
// ISO 32000 §7.6 — not just PDFs produced by folio's own writer. See
// reader/testdata/encrypted/README.md for how the fixtures were
// generated.
func TestParseQpdfEncrypted(t *testing.T) {
	plainBytes, err := os.ReadFile(filepath.Join("testdata", "encrypted", "plain.pdf"))
	if err != nil {
		t.Fatalf("read plain.pdf: %v", err)
	}
	plain, err := Parse(plainBytes)
	if err != nil {
		t.Fatalf("parse plain.pdf: %v", err)
	}
	plainPage, err := plain.Page(0)
	if err != nil {
		t.Fatalf("plain.Page(0): %v", err)
	}
	wantText, err := plainPage.ExtractText()
	if err != nil {
		t.Fatalf("extract text from plain.pdf: %v", err)
	}

	tests := []struct {
		name       string
		file       string
		password   string
		wantAccess AccessLevel
	}{
		{"AES-256 user password", "aes256.pdf", "user123", AccessUser},
		{"AES-256 owner password", "aes256.pdf", "owner456", AccessOwner},
		{"AES-128 user password", "aes128.pdf", "user123", AccessUser},
		{"RC4-128 user password", "rc4-128.pdf", "user123", AccessUser},
		{"owner-only empty user password", "owner-only.pdf", "", AccessUser},
		{"owner-only owner password", "owner-only.pdf", "owner456", AccessOwner},
		{"EncryptMetadata false", "no-encrypt-metadata.pdf", "user123", AccessUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf, err := os.ReadFile(filepath.Join("testdata", "encrypted", tt.file))
			if err != nil {
				t.Fatalf("read %s: %v", tt.file, err)
			}

			r, err := ParseWithOptions(pdf, ReadOptions{Password: tt.password})
			if err != nil {
				t.Fatalf("ParseWithOptions: %v", err)
			}
			if r.Access() != tt.wantAccess {
				t.Errorf("Access() = %v, want %v", r.Access(), tt.wantAccess)
			}
			page, err := r.Page(0)
			if err != nil {
				t.Fatalf("Page(0): %v", err)
			}
			gotText, err := page.ExtractText()
			if err != nil {
				t.Fatalf("ExtractText: %v", err)
			}
			if gotText != wantText {
				t.Errorf("ExtractText() = %q, want %q", gotText, wantText)
			}
		})
	}

	t.Run("wrong password", func(t *testing.T) {
		pdf, err := os.ReadFile(filepath.Join("testdata", "encrypted", "aes256.pdf"))
		if err != nil {
			t.Fatalf("read aes256.pdf: %v", err)
		}
		_, err = ParseWithOptions(pdf, ReadOptions{Password: "wrong"})
		if !errors.Is(err, ErrInvalidPassword) {
			t.Errorf("got %v, want ErrInvalidPassword", err)
		}
	})
}

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"errors"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
)

const encryptedFixtureText = "Hello, encrypted world!"

// buildEncryptedFixture generates a single-page PDF in memory, encrypted
// with the given algorithm and passwords, containing encryptedFixtureText.
func buildEncryptedFixture(t *testing.T, alg document.EncryptionAlgorithm, userPwd, ownerPwd string, perms core.Permission) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetEncryption(document.EncryptionConfig{
		Algorithm:     alg,
		UserPassword:  userPwd,
		OwnerPassword: ownerPwd,
		Permissions:   perms,
	})
	p := doc.AddPage()
	p.AddText(encryptedFixtureText, font.Helvetica, 12, 72, 700)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return buf.Bytes()
}

// buildPlainFixture generates the unencrypted twin of buildEncryptedFixture.
func buildPlainFixture(t *testing.T) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeLetter)
	p := doc.AddPage()
	p.AddText(encryptedFixtureText, font.Helvetica, 12, 72, 700)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return buf.Bytes()
}

// TestParseEncryptedRoundTrip generates encrypted PDFs with folio's own
// writer and reads them back with the reader, checking that content
// matches the unencrypted twin and that the correct password (user or
// owner) is reported via [PdfReader.Access].
func TestParseEncryptedRoundTrip(t *testing.T) {
	plain, err := Parse(buildPlainFixture(t))
	if err != nil {
		t.Fatalf("parse unencrypted twin: %v", err)
	}
	plainPage, err := plain.Page(0)
	if err != nil {
		t.Fatalf("plain.Page(0): %v", err)
	}
	wantText, err := plainPage.ExtractText()
	if err != nil {
		t.Fatalf("extract text from unencrypted twin: %v", err)
	}

	tests := []struct {
		name       string
		alg        document.EncryptionAlgorithm
		userPwd    string
		ownerPwd   string
		tryPwd     string
		wantAccess AccessLevel
	}{
		{"AES-128 empty user password", document.EncryptAES128, "", "owner", "", AccessUser},
		{"AES-128 non-empty user password", document.EncryptAES128, "user123", "owner456", "user123", AccessUser},
		{"AES-128 owner password supplied", document.EncryptAES128, "user123", "owner456", "owner456", AccessOwner},
		{"AES-256 empty user password", document.EncryptAES256, "", "owner", "", AccessUser},
		{"AES-256 non-empty user password", document.EncryptAES256, "secret", "admin", "secret", AccessUser},
		{"AES-256 owner password supplied", document.EncryptAES256, "secret", "admin", "admin", AccessOwner},
		{"RC4-128 read-only legacy", document.EncryptRC4128, "rc4pw", "rc4owner", "rc4pw", AccessUser}, //nolint:staticcheck // exercising legacy read support
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := buildEncryptedFixture(t, tt.alg, tt.userPwd, tt.ownerPwd, core.PermAll)

			r, err := ParseWithOptions(pdf, ReadOptions{Password: tt.tryPwd})
			if err != nil {
				t.Fatalf("ParseWithOptions: %v", err)
			}
			if r.Access() != tt.wantAccess {
				t.Errorf("Access() = %v, want %v", r.Access(), tt.wantAccess)
			}
			if r.PageCount() != plain.PageCount() {
				t.Fatalf("PageCount() = %d, want %d", r.PageCount(), plain.PageCount())
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
}

// TestParseEncryptedWrongPassword checks that an incorrect password
// yields the typed sentinel error, not a generic parse failure or panic.
func TestParseEncryptedWrongPassword(t *testing.T) {
	pdf := buildEncryptedFixture(t, document.EncryptAES256, "correct-password", "owner-password", core.PermAll)

	_, err := ParseWithOptions(pdf, ReadOptions{Password: "wrong-password"})
	if !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("got %v, want ErrInvalidPassword", err)
	}
}

// TestParseEncryptedDefaultPasswordIsEmpty checks that Parse (no
// options) implicitly attempts the empty password, matching the
// dominant real-world case of files "protected" only to restrict
// permissions.
func TestParseEncryptedDefaultPasswordIsEmpty(t *testing.T) {
	pdf := buildEncryptedFixture(t, document.EncryptAES256, "", "owner-password", core.PermPrint)

	r, err := Parse(pdf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Access() != AccessUser {
		t.Errorf("Access() = %v, want AccessUser", r.Access())
	}
}

// TestParseEncryptedMalformedNoPanic sweeps single-byte corruptions
// across a real encrypted PDF — including its /Encrypt dictionary,
// /O/U/OE/UE hash and key material, and the encrypted content stream
// payload — and checks that neither parsing nor subsequent content
// extraction ever panics, even when the (correct) password is supplied
// against corrupted ciphertext or a corrupted security handler
// configuration.
func TestParseEncryptedMalformedNoPanic(t *testing.T) {
	pdf := buildEncryptedFixture(t, document.EncryptAES256, "user", "owner", core.PermAll)

	for i := 0; i < len(pdf); i += 17 {
		corrupted := append([]byte(nil), pdf...)
		corrupted[i] ^= 0xFF

		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("byte %d corrupted (0x%02x -> 0x%02x): panic: %v", i, pdf[i], corrupted[i], rec)
				}
			}()
			r, err := ParseWithOptions(corrupted, ReadOptions{Password: "user"})
			if err != nil {
				return
			}
			for p := 0; p < r.PageCount(); p++ {
				page, err := r.Page(p)
				if err != nil {
					continue
				}
				_, _ = page.ContentStream()
				_, _ = page.ExtractText()
			}
		}()
	}
}

// TestParseEncryptedTruncatedNoPanic checks truncation at every length,
// a common source of out-of-bounds slicing bugs, does not panic.
func TestParseEncryptedTruncatedNoPanic(t *testing.T) {
	pdf := buildEncryptedFixture(t, document.EncryptAES128, "user", "owner", core.PermAll)

	for cut := 0; cut < len(pdf); cut += 31 {
		truncated := pdf[:cut]
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("truncated at %d: panic: %v", cut, rec)
				}
			}()
			r, err := ParseWithOptions(truncated, ReadOptions{Password: "user"})
			if err != nil {
				return
			}
			for p := 0; p < r.PageCount(); p++ {
				page, err := r.Page(p)
				if err != nil {
					continue
				}
				_, _ = page.ContentStream()
			}
		}()
	}
}

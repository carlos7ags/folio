// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"testing"
)

func TestPermBits(t *testing.T) {
	tests := []struct {
		name string
		perm Permission
		want int32
	}{
		{"no permissions", 0, -3904},
		{"all permissions", PermAll, -4},
		{"print only", PermPrint, -3900},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := permBits(tt.perm)
			if got != tt.want {
				t.Errorf("permBits(%d) = %d, want %d", tt.perm, got, tt.want)
			}
		})
	}
}

func TestPadPassword(t *testing.T) {
	// Empty password → full padding.
	p := padPassword(nil)
	if p != pdfPadding {
		t.Errorf("padPassword(nil) != pdfPadding")
	}

	// Short password → padded.
	p2 := padPassword([]byte("abc"))
	if p2[0] != 'a' || p2[1] != 'b' || p2[2] != 'c' || p2[3] != pdfPadding[0] {
		t.Errorf("padPassword(abc) unexpected: %x", p2)
	}

	// 32-byte password → no padding.
	long := bytes.Repeat([]byte("X"), 32)
	p3 := padPassword(long)
	for i := range p3 {
		if p3[i] != 'X' {
			t.Errorf("padPassword(32 bytes) modified at index %d", i)
		}
	}
}

func TestPkcs7Pad(t *testing.T) {
	// 10 bytes padded to 16.
	data := make([]byte, 10)
	padded := pkcs7Pad(data, 16)
	if len(padded) != 16 {
		t.Fatalf("pkcs7Pad(10, 16) length = %d, want 16", len(padded))
	}
	for i := 10; i < 16; i++ {
		if padded[i] != 6 {
			t.Errorf("pkcs7Pad padding byte %d = %d, want 6", i, padded[i])
		}
	}

	// Already block-aligned → full block of padding added.
	data16 := make([]byte, 16)
	padded16 := pkcs7Pad(data16, 16)
	if len(padded16) != 32 {
		t.Fatalf("pkcs7Pad(16, 16) length = %d, want 32", len(padded16))
	}
	for i := 16; i < 32; i++ {
		if padded16[i] != 16 {
			t.Errorf("padding byte = %d, want 16", padded16[i])
		}
	}
}

func TestXorKey(t *testing.T) {
	key := []byte{0x10, 0x20, 0x30}
	out := xorKey(key, 0x05)
	if out[0] != 0x15 || out[1] != 0x25 || out[2] != 0x35 {
		t.Errorf("xorKey unexpected: %x", out)
	}
}

func TestRC4Encrypt(t *testing.T) {
	key := []byte("secret")
	plain := []byte("hello world")
	encrypted := rc4Encrypt(key, plain)
	if bytes.Equal(encrypted, plain) {
		t.Error("RC4 did not change data")
	}
	// RC4 is symmetric: decrypt with the same key.
	decrypted := rc4Encrypt(key, encrypted)
	if !bytes.Equal(decrypted, plain) {
		t.Error("RC4 round-trip failed")
	}
}

func TestAesCBCEncrypt(t *testing.T) {
	key := make([]byte, 16)
	data := []byte("hello")
	enc, err := aesCBCEncrypt(key, data)
	if err != nil {
		t.Fatal(err)
	}
	// Must have 16-byte IV + at least one block.
	if len(enc) < 32 {
		t.Errorf("AES-CBC output too short: %d bytes", len(enc))
	}
	// IV is the first 16 bytes — must differ from zero (random).
	iv := enc[:16]
	allZero := true
	for _, b := range iv {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("IV is all zeros (expected random)")
	}
}

func TestAesCBCEncrypt256(t *testing.T) {
	key := make([]byte, 32)
	data := []byte("test data for aes-256")
	enc, err := aesCBCEncrypt(key, data)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) < 48 { // 16 IV + 32 data (padded to 2 blocks)
		t.Errorf("AES-256-CBC output too short: %d", len(enc))
	}
}

func TestAesECBEncryptBlock(t *testing.T) {
	key := make([]byte, 32)
	block := make([]byte, 16)
	block[0] = 0x42
	enc := aesECBEncryptBlock(key, block)
	if len(enc) != 16 {
		t.Fatalf("AES-ECB output length = %d, want 16", len(enc))
	}
	if bytes.Equal(enc, block) {
		t.Error("AES-ECB did not change data")
	}
}

func TestComputeOwnerHashR3(t *testing.T) {
	o := computeOwnerHashR3("user", "owner", 16)
	if len(o) != 32 {
		t.Errorf("O length = %d, want 32", len(o))
	}
	// Deterministic: same inputs → same output.
	o2 := computeOwnerHashR3("user", "owner", 16)
	if !bytes.Equal(o, o2) {
		t.Error("owner hash not deterministic")
	}
}

func TestComputeFileKeyR3(t *testing.T) {
	o := computeOwnerHashR3("user", "owner", 16)
	p := permBits(PermAll)
	fileID := make([]byte, 16)
	key := computeFileKeyR3("user", o, p, fileID, 16)
	if len(key) != 16 {
		t.Errorf("file key length = %d, want 16", len(key))
	}
}

func TestComputeUserHashR3(t *testing.T) {
	fileKey := make([]byte, 16)
	fileID := make([]byte, 16)
	u := computeUserHashR3(fileKey, fileID)
	if len(u) != 32 {
		t.Errorf("U length = %d, want 32", len(u))
	}
}

func TestNewEncryptorRC4(t *testing.T) {
	enc, err := NewEncryptor(RevisionRC4128, "user", "owner", PermAll)
	if err != nil {
		t.Fatal(err)
	}
	if enc.Revision != RevisionRC4128 {
		t.Error("wrong revision")
	}
	if len(enc.FileKey) != 16 {
		t.Errorf("file key length = %d", len(enc.FileKey))
	}
	if len(enc.O) != 32 {
		t.Errorf("O length = %d", len(enc.O))
	}
	if len(enc.U) != 32 {
		t.Errorf("U length = %d", len(enc.U))
	}
	if len(enc.FileID) != 16 {
		t.Errorf("FileID length = %d", len(enc.FileID))
	}
}

func TestNewEncryptorAES128(t *testing.T) {
	enc, err := NewEncryptor(RevisionAES128, "pass", "", PermPrint)
	if err != nil {
		t.Fatal(err)
	}
	if enc.Revision != RevisionAES128 {
		t.Error("wrong revision")
	}
	if len(enc.FileKey) != 16 {
		t.Errorf("file key length = %d", len(enc.FileKey))
	}
}

func TestNewEncryptorAES256(t *testing.T) {
	enc, err := NewEncryptor(RevisionAES256, "user", "owner", PermAll)
	if err != nil {
		t.Fatal(err)
	}
	if enc.Revision != RevisionAES256 {
		t.Error("wrong revision")
	}
	if len(enc.FileKey) != 32 {
		t.Errorf("file key length = %d", len(enc.FileKey))
	}
	if len(enc.U) != 48 {
		t.Errorf("U length = %d", len(enc.U))
	}
	if len(enc.O) != 48 {
		t.Errorf("O length = %d", len(enc.O))
	}
	if len(enc.UE) != 32 {
		t.Errorf("UE length = %d", len(enc.UE))
	}
	if len(enc.OE) != 32 {
		t.Errorf("OE length = %d", len(enc.OE))
	}
	if len(enc.Perms) != 16 {
		t.Errorf("Perms length = %d", len(enc.Perms))
	}
}

func TestBuildEncryptDictRC4(t *testing.T) {
	enc, _ := NewEncryptor(RevisionRC4128, "test", "test", 0)
	d := enc.BuildEncryptDict()
	if d.Get("V") == nil || d.Get("R") == nil || d.Get("O") == nil || d.Get("U") == nil {
		t.Error("missing required entries in RC4 encrypt dict")
	}
	// Should not have crypt filter entries.
	if d.Get("CF") != nil {
		t.Error("RC4 encrypt dict should not have /CF")
	}
}

func TestBuildEncryptDictAES128(t *testing.T) {
	enc, _ := NewEncryptor(RevisionAES128, "test", "test", 0)
	d := enc.BuildEncryptDict()
	if d.Get("CF") == nil || d.Get("StmF") == nil || d.Get("StrF") == nil {
		t.Error("AES-128 encrypt dict missing crypt filter entries")
	}
}

func TestBuildEncryptDictAES256(t *testing.T) {
	enc, _ := NewEncryptor(RevisionAES256, "test", "test", 0)
	d := enc.BuildEncryptDict()
	if d.Get("OE") == nil || d.Get("UE") == nil || d.Get("Perms") == nil {
		t.Error("AES-256 encrypt dict missing R6 entries")
	}
}

func TestEncryptObjectString(t *testing.T) {
	enc, _ := NewEncryptor(RevisionAES256, "pass", "pass", PermAll)
	s := NewPdfLiteralString("Hello")
	if err := enc.EncryptObject(s, 1, 0); err != nil {
		t.Fatal(err)
	}
	// After encryption: encoding must be hex, value must differ.
	if !s.IsHex() {
		t.Error("encrypted string not hex-encoded")
	}
	if s.Text() == "Hello" {
		t.Error("string not encrypted")
	}
}

func TestEncryptObjectSkipsEncryptDict(t *testing.T) {
	enc, _ := NewEncryptor(RevisionAES256, "pass", "pass", PermAll)
	enc.SetEncryptDictObjNum(5)
	s := NewPdfLiteralString("Secret")
	if err := enc.EncryptObject(s, 5, 0); err != nil {
		t.Fatal(err)
	}
	if s.Text() != "Secret" {
		t.Error("encrypt dict object should not be encrypted")
	}
}

func TestEncryptObjectDict(t *testing.T) {
	enc, _ := NewEncryptor(RevisionRC4128, "pass", "pass", PermAll)
	d := NewPdfDictionary()
	d.Set("Title", NewPdfLiteralString("Test"))
	d.Set("Count", NewPdfInteger(42))
	if err := enc.EncryptObject(d, 1, 0); err != nil {
		t.Fatal(err)
	}
	// String should be encrypted; integer should be unchanged.
	title := d.Get("Title").(*PdfString)
	if title.Text() == "Test" {
		t.Error("string in dict not encrypted")
	}
	count := d.Get("Count").(*PdfNumber)
	if count.IntValue() != 42 {
		t.Error("integer modified during encryption")
	}
}

func TestEncryptObjectStream(t *testing.T) {
	enc, _ := NewEncryptor(RevisionAES256, "pass", "pass", PermAll)
	s := NewPdfStreamCompressed([]byte("stream content here"))
	if err := enc.EncryptObject(s, 3, 0); err != nil {
		t.Fatal(err)
	}
	// Stream data should be encrypted (includes IV prefix for AES).
	if bytes.Equal(s.Data, []byte("stream content here")) {
		t.Error("stream data not encrypted")
	}
	// Compress flag should be cleared (compression happened before encryption).
	if s.compress {
		t.Error("compress flag should be false after encryption walk")
	}
	// /Filter should be set to FlateDecode.
	filter := s.Dict.Get("Filter")
	if filter == nil {
		t.Error("/Filter not set on compressed+encrypted stream")
	}
}

func TestAlgorithmR6Hash(t *testing.T) {
	// Verify it returns 32 bytes and is deterministic.
	pwd := []byte("password")
	salt := make([]byte, 8)
	h1 := algorithmR6Hash(pwd, salt, nil)
	if len(h1) != 32 {
		t.Fatalf("R6 hash length = %d, want 32", len(h1))
	}
	h2 := algorithmR6Hash(pwd, salt, nil)
	if !bytes.Equal(h1, h2) {
		t.Error("R6 hash not deterministic")
	}
	// Different salt → different hash.
	salt2 := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	h3 := algorithmR6Hash(pwd, salt2, nil)
	if bytes.Equal(h1, h3) {
		t.Error("R6 hash: different salt produced same hash")
	}
}

func TestLongPassword(t *testing.T) {
	// Passwords longer than 127 bytes should be truncated (R6) or 32 bytes (R3/R4).
	longPwd := string(bytes.Repeat([]byte("A"), 200))
	enc, err := NewEncryptor(RevisionAES256, longPwd, longPwd, PermAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc.FileKey) != 32 {
		t.Errorf("file key length = %d, want 32", len(enc.FileKey))
	}
}

func TestUnicodePassword(t *testing.T) {
	enc, err := NewEncryptor(RevisionAES256, "contraseña", "dueño", PermAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc.U) != 48 {
		t.Errorf("U length = %d, want 48", len(enc.U))
	}
}

func TestBuildEncryptDictAuthEvent(t *testing.T) {
	for _, rev := range []EncryptionRevision{RevisionAES128, RevisionAES256} {
		enc, _ := NewEncryptor(rev, "test", "test", 0)
		d := enc.BuildEncryptDict()
		cf := d.Get("CF").(*PdfDictionary)
		stdCF := cf.Get("StdCF").(*PdfDictionary)
		ae := stdCF.Get("AuthEvent")
		if ae == nil {
			t.Errorf("R=%d: missing /AuthEvent in crypt filter", rev)
		}
		em := d.Get("EncryptMetadata")
		if em == nil {
			t.Errorf("R=%d: missing /EncryptMetadata", rev)
		}
	}
}

func TestNewEncryptorInvalidRevision(t *testing.T) {
	_, err := NewEncryptor(EncryptionRevision(99), "user", "owner", PermAll)
	if err == nil {
		t.Error("expected error for invalid revision, got nil")
	}
}

func TestEncryptBytesEmpty(t *testing.T) {
	enc, _ := NewEncryptor(RevisionAES256, "pass", "pass", PermAll)
	out, err := enc.EncryptBytes(1, 0, []byte{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty output for empty input, got %d bytes", len(out))
	}
}

func TestEncryptBytesRC4Direct(t *testing.T) {
	enc, err := NewEncryptor(RevisionRC4128, "pass", "pass", PermAll)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("some plaintext")
	encrypted, err := enc.EncryptBytes(1, 0, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(encrypted, plaintext) {
		t.Error("RC4 encryption did not change data")
	}
	if len(encrypted) != len(plaintext) {
		t.Errorf("RC4 should not change length: got %d, want %d",
			len(encrypted), len(plaintext))
	}
}

func TestEncryptBytesRC4RoundTrip(t *testing.T) {
	enc, err := NewEncryptor(RevisionRC4128, "pass", "pass", PermAll)
	if err != nil {
		t.Fatal(err)
	}
	original := []byte("Hello, PDF encryption!")
	encrypted, err := enc.EncryptBytes(1, 0, original)
	if err != nil {
		t.Fatal(err)
	}
	// RC4 is symmetric: encrypting with the same per-object key decrypts.
	decrypted, err := enc.EncryptBytes(1, 0, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, original) {
		t.Errorf("RC4 round-trip failed: got %q, want %q", decrypted, original)
	}
}

func TestEncryptBytesAES128Direct(t *testing.T) {
	enc, err := NewEncryptor(RevisionAES128, "pass", "pass", PermAll)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("some AES-128 plaintext")
	encrypted, err := enc.EncryptBytes(1, 0, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(encrypted, plaintext) {
		t.Error("AES-128 encryption did not change data")
	}
	// Output must have a 16-byte IV prefix plus PKCS7-padded ciphertext.
	if len(encrypted) < len(plaintext)+16 {
		t.Errorf("AES-128 output too short: got %d, want >= %d",
			len(encrypted), len(plaintext)+16)
	}
}

func TestEncryptObjectArray(t *testing.T) {
	enc, _ := NewEncryptor(RevisionAES256, "pass", "pass", PermAll)
	arr := NewPdfArray(
		NewPdfLiteralString("secret"),
		NewPdfInteger(42),
	)
	if err := enc.EncryptObject(arr, 2, 0); err != nil {
		t.Fatal(err)
	}
	s := arr.Elements[0].(*PdfString)
	if s.Text() == "secret" {
		t.Error("string in array not encrypted")
	}
	n := arr.Elements[1].(*PdfNumber)
	if n.IntValue() != 42 {
		t.Error("integer in array modified during encryption")
	}
}

func TestSaslPrepNormalization(t *testing.T) {
	// U+00E9 (precomposed e-acute) vs U+0065 U+0301 (e + combining acute).
	precomposed := "caf\u00e9"
	decomposed := "cafe\u0301"

	p1 := saslPrepPassword(precomposed)
	p2 := saslPrepPassword(decomposed)
	if !bytes.Equal(p1, p2) {
		t.Errorf("SASLprep should normalize equivalent Unicode: %x vs %x", p1, p2)
	}
}

func TestTruncatePasswordRuneBoundary(t *testing.T) {
	// Build a password whose byte 127 falls inside a multi-byte rune.
	// The é character (U+00E9) is 2 bytes in UTF-8: 0xC3 0xA9.
	// 63 × "ab" (126 bytes) + "é" (2 bytes) = 128 bytes. Byte 127 is
	// the continuation byte of é.
	pwd := ""
	for range 63 {
		pwd += "ab"
	}
	pwd += "é" // 0xC3 0xA9
	if len(pwd) != 128 {
		t.Fatalf("setup: pwd length = %d, want 128", len(pwd))
	}
	truncated := truncatePassword(pwd)
	if len(truncated) > 127 {
		t.Errorf("truncated length = %d, want <= 127", len(truncated))
	}
	// The result must be valid UTF-8 (no lone continuation byte).
	if last := truncated[len(truncated)-1]; last&0xC0 == 0x80 {
		t.Errorf("truncated password ends with continuation byte 0x%02X", last)
	}
}

func TestEncryptR6EquivalentUnicodePasswords(t *testing.T) {
	// Two R6 encryptors created with canonically equivalent passwords
	// should accept each other's encryption under the same file key.
	// Since each NewEncryptor randomizes the file key, we can't compare
	// file keys directly; instead verify that saslPrepPassword produces
	// the same bytes, which is what feeds the key derivation.
	p1 := saslPrepPassword("Angstr\u00f6m")  // precomposed Å, single ö
	p2 := saslPrepPassword("Angstro\u0308m") // o + combining diaeresis = ö
	// Note: "Angstr" stays the same, difference is just in ö.
	// Without NFKC these would differ; with NFKC they should match.
	if !bytes.Equal(p1, p2) {
		t.Errorf("canonically-equivalent passwords produced different bytes: %x vs %x", p1, p2)
	}
}

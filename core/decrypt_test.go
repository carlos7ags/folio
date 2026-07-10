// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"errors"
	"testing"
)

// newDecryptorFromEncryptor builds a Decryptor that shares an
// Encryptor's derived key material, for exercising DecryptBytes without
// going through password authentication.
func newDecryptorFromEncryptor(e *Encryptor) *Decryptor {
	return &Decryptor{
		Revision:        e.Revision,
		FileKey:         e.FileKey,
		EncryptMetadata: true,
		keyLen:          e.keyLen,
	}
}

func TestDecryptBytesRoundTrip(t *testing.T) {
	revisions := []struct {
		name string
		rev  EncryptionRevision
	}{
		{"RC4-128", RevisionRC4128},
		{"AES-128", RevisionAES128},
		{"AES-256", RevisionAES256},
	}

	for _, tc := range revisions {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := NewEncryptor(tc.rev, "user", "owner", PermAll)
			if err != nil {
				t.Fatalf("NewEncryptor: %v", err)
			}
			dec := newDecryptorFromEncryptor(enc)

			plain := []byte("The quick brown fox jumps over the lazy dog.")
			ciphertext, err := enc.EncryptBytes(7, 0, plain)
			if err != nil {
				t.Fatalf("EncryptBytes: %v", err)
			}
			if bytes.Equal(ciphertext, plain) {
				t.Fatal("EncryptBytes did not change the data")
			}

			got, err := dec.DecryptBytes(7, 0, ciphertext)
			if err != nil {
				t.Fatalf("DecryptBytes: %v", err)
			}
			if !bytes.Equal(got, plain) {
				t.Errorf("round-trip mismatch: got %q, want %q", got, plain)
			}
		})
	}
}

func TestDecryptBytesWrongObjectNumberFails(t *testing.T) {
	// A different object number derives a different per-object key, so
	// decryption must not silently produce the original plaintext.
	enc, err := NewEncryptor(RevisionAES128, "user", "owner", PermAll)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	dec := newDecryptorFromEncryptor(enc)

	plain := []byte("secret payload, sixteen+ bytes long")
	ciphertext, err := enc.EncryptBytes(1, 0, plain)
	if err != nil {
		t.Fatalf("EncryptBytes: %v", err)
	}

	got, err := dec.DecryptBytes(2, 0, ciphertext)
	if err == nil && bytes.Equal(got, plain) {
		t.Error("decrypting under the wrong object number should not reproduce the plaintext")
	}
}

func TestNewDecryptorPasswordMatrix(t *testing.T) {
	revisions := []EncryptionRevision{RevisionRC4128, RevisionAES128, RevisionAES256}

	for _, rev := range revisions {
		t.Run(revisionName(rev), func(t *testing.T) {
			enc, err := NewEncryptor(rev, "userpass", "ownerpass", PermPrint|PermExtract)
			if err != nil {
				t.Fatalf("NewEncryptor: %v", err)
			}
			dict := enc.BuildEncryptDict()

			t.Run("user password", func(t *testing.T) {
				dec, err := NewDecryptor(dict, enc.FileID, "userpass")
				if err != nil {
					t.Fatalf("NewDecryptor: %v", err)
				}
				if dec.Access != AccessUser {
					t.Errorf("Access = %v, want AccessUser", dec.Access)
				}
				if !bytes.Equal(dec.FileKey, enc.FileKey) {
					t.Error("recovered file key does not match the encryptor's key")
				}
			})

			t.Run("owner password", func(t *testing.T) {
				dec, err := NewDecryptor(dict, enc.FileID, "ownerpass")
				if err != nil {
					t.Fatalf("NewDecryptor: %v", err)
				}
				if dec.Access != AccessOwner {
					t.Errorf("Access = %v, want AccessOwner", dec.Access)
				}
				if !bytes.Equal(dec.FileKey, enc.FileKey) {
					t.Error("recovered file key does not match the encryptor's key")
				}
			})

			t.Run("wrong password", func(t *testing.T) {
				_, err := NewDecryptor(dict, enc.FileID, "not-the-password")
				if !errors.Is(err, ErrInvalidPassword) {
					t.Errorf("got %v, want ErrInvalidPassword", err)
				}
			})
		})
	}
}

func TestNewDecryptorEmptyUserPassword(t *testing.T) {
	// The dominant real-world case: an empty user password with any
	// viewer opening the file silently. Empty string must be attempted,
	// not treated as "no password".
	for _, rev := range []EncryptionRevision{RevisionRC4128, RevisionAES128, RevisionAES256} {
		t.Run(revisionName(rev), func(t *testing.T) {
			enc, err := NewEncryptor(rev, "", "ownerpass", PermPrint)
			if err != nil {
				t.Fatalf("NewEncryptor: %v", err)
			}
			dict := enc.BuildEncryptDict()

			dec, err := NewDecryptor(dict, enc.FileID, "")
			if err != nil {
				t.Fatalf("NewDecryptor with empty password: %v", err)
			}
			if dec.Access != AccessUser {
				t.Errorf("Access = %v, want AccessUser", dec.Access)
			}
		})
	}
}

func TestNewDecryptorUnsupportedRevision(t *testing.T) {
	dict := NewPdfDictionary()
	dict.Set("Filter", NewPdfName("Standard"))
	dict.Set("V", NewPdfInteger(99))
	dict.Set("R", NewPdfInteger(99))
	dict.Set("O", NewPdfHexString(string(make([]byte, 32))))
	dict.Set("U", NewPdfHexString(string(make([]byte, 32))))
	dict.Set("P", NewPdfInteger(-4))

	_, err := NewDecryptor(dict, nil, "")
	if !errors.Is(err, ErrUnsupportedEncryption) {
		t.Errorf("got %v, want ErrUnsupportedEncryption", err)
	}
}

func TestNewDecryptorNonStandardFilter(t *testing.T) {
	dict := NewPdfDictionary()
	dict.Set("Filter", NewPdfName("Adobe.PubSec"))
	dict.Set("R", NewPdfInteger(3))

	_, err := NewDecryptor(dict, nil, "")
	if !errors.Is(err, ErrUnsupportedEncryption) {
		t.Errorf("got %v, want ErrUnsupportedEncryption", err)
	}
}

func TestNewDecryptorRejectsHostileDict(t *testing.T) {
	// Malformed /Encrypt dictionaries a hostile or corrupt PDF might
	// contain: wrong types, too-short strings, missing required
	// entries. NewDecryptor must return an error, never panic or read
	// out of bounds.
	tests := []struct {
		name string
		dict *PdfDictionary
	}{
		{"missing everything", NewPdfDictionary()},
		{"O is an integer, not a string", func() *PdfDictionary {
			d := NewPdfDictionary()
			d.Set("Filter", NewPdfName("Standard"))
			d.Set("R", NewPdfInteger(3))
			d.Set("O", NewPdfInteger(123))
			d.Set("U", NewPdfHexString(string(make([]byte, 32))))
			return d
		}()},
		{"O and U too short for R3", func() *PdfDictionary {
			d := NewPdfDictionary()
			d.Set("Filter", NewPdfName("Standard"))
			d.Set("R", NewPdfInteger(3))
			d.Set("O", NewPdfHexString("ab"))
			d.Set("U", NewPdfHexString("cd"))
			return d
		}()},
		{"R6 with truncated O/U/OE/UE", func() *PdfDictionary {
			d := NewPdfDictionary()
			d.Set("Filter", NewPdfName("Standard"))
			d.Set("V", NewPdfInteger(5))
			d.Set("R", NewPdfInteger(6))
			d.Set("StmF", NewPdfName("StdCF"))
			d.Set("StrF", NewPdfName("StdCF"))
			cf := NewPdfDictionary()
			stdCF := NewPdfDictionary()
			stdCF.Set("CFM", NewPdfName("AESV3"))
			cf.Set("StdCF", stdCF)
			d.Set("CF", cf)
			d.Set("O", NewPdfHexString(string(make([]byte, 5))))
			d.Set("U", NewPdfHexString(string(make([]byte, 5))))
			d.Set("OE", NewPdfHexString(string(make([]byte, 5))))
			d.Set("UE", NewPdfHexString(string(make([]byte, 5))))
			return d
		}()},
		{"R4 without a crypt filter dictionary", func() *PdfDictionary {
			d := NewPdfDictionary()
			d.Set("Filter", NewPdfName("Standard"))
			d.Set("V", NewPdfInteger(4))
			d.Set("R", NewPdfInteger(4))
			d.Set("O", NewPdfHexString(string(make([]byte, 32))))
			d.Set("U", NewPdfHexString(string(make([]byte, 32))))
			return d
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("NewDecryptor panicked: %v", r)
				}
			}()
			_, err := NewDecryptor(tt.dict, []byte("shortid"), "any-password")
			if err == nil {
				t.Fatal("expected an error for a malformed /Encrypt dictionary")
			}
		})
	}
}

func TestAesCBCDecryptRejectsHostileCiphertext(t *testing.T) {
	key := make([]byte, 16)

	corrupted, err := aesCBCEncrypt(key, []byte("exactly16 bytes!"))
	if err != nil {
		t.Fatalf("aesCBCEncrypt: %v", err)
	}
	// Flip the last byte: CBC decryption's avalanche effect makes the
	// last plaintext block effectively random, so the PKCS#7 padding
	// check almost certainly fails.
	corrupted[len(corrupted)-1] ^= 0xFF

	tests := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"shorter than one block", []byte{1, 2, 3}},
		{"IV only, no ciphertext block", make([]byte, 16)},
		{"misaligned ciphertext", append(make([]byte, 16), 1, 2, 3)},
		{"aligned but corrupted padding", corrupted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("aesCBCDecrypt panicked: %v", r)
				}
			}()
			if _, err := aesCBCDecrypt(key, tt.data); err == nil {
				t.Error("expected an error for hostile ciphertext")
			}
		})
	}
}

func revisionName(rev EncryptionRevision) string {
	switch rev {
	case RevisionRC4128:
		return "RC4-128"
	case RevisionAES128:
		return "AES-128"
	case RevisionAES256:
		return "AES-256"
	default:
		return "unknown"
	}
}

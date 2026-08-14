// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
)

// AccessLevel reports which password unlocked an encrypted document,
// per ISO 32000-1 §7.6.3-7.6.4.
type AccessLevel int

const (
	AccessNone  AccessLevel = iota // document is not encrypted
	AccessUser                     // opened with the user (open) password
	AccessOwner                    // opened with the owner (permissions) password
)

// ErrInvalidPassword is returned when a candidate password matches
// neither the user nor the owner password recorded in /Encrypt.
var ErrInvalidPassword = errors.New("core: invalid password")

// ErrUnsupportedEncryption is returned for security-handler
// configurations the Standard-handler decoder does not implement:
// non-Standard /Filter, unsupported /V or /R, or a crypt filter other
// than the standard AESV2/AESV3 configuration.
var ErrUnsupportedEncryption = errors.New("core: unsupported encryption")

// Decryptor decrypts strings and stream payloads for a document
// encrypted with the Standard security handler. It mirrors [Encryptor]:
// key derivation is identical for encryption and decryption, and RC4 is
// symmetric, so decryption reuses most of the same unexported helpers.
type Decryptor struct {
	Revision        EncryptionRevision
	FileKey         []byte // 16 bytes (RC4/AES-128) or 32 bytes (AES-256)
	Access          AccessLevel
	P               int32 // permission flags from /P — authenticated against /Perms for R=6, informational only (never enforced)
	EncryptMetadata bool  // /EncryptMetadata; false means /Metadata streams are stored in the clear

	keyLen int
}

// NewDecryptor reads a resolved /Encrypt dictionary and authenticates
// password, trying it first as the user password (Algorithm 6 / 2.A),
// then as the owner password (Algorithm 7 / 2.A). fileID is the first
// element of the trailer /ID array and may be nil.
//
// A wrong password returns [ErrInvalidPassword]. A configuration this
// decoder does not implement — a non-Standard or public-key handler, an
// unsupported /V or /R, or a crypt filter other than AESV2/AESV3 —
// returns [ErrUnsupportedEncryption] naming the value seen. Malformed
// /O, /U, /OE, or /UE entries (wrong type or too short) also return
// [ErrUnsupportedEncryption] rather than reading out of bounds.
func NewDecryptor(dict *PdfDictionary, fileID []byte, password string) (*Decryptor, error) {
	if dict == nil {
		return nil, fmt.Errorf("%w: missing /Encrypt dictionary", ErrUnsupportedEncryption)
	}
	if filter, ok := dict.Get("Filter").(*PdfName); !ok || filter.Value != "Standard" {
		return nil, fmt.Errorf("%w: security handler %q", ErrUnsupportedEncryption, nameOrEmpty(dict.Get("Filter")))
	}

	r := dictInt(dict, "R", 0)
	p := int32(dictInt(dict, "P", 0))

	switch r {
	case 3:
		return newDecryptorR3R4(dict, fileID, password, RevisionRC4128, r, p)
	case 4:
		if err := requireStdCF(dict, "AESV2"); err != nil {
			return nil, err
		}
		return newDecryptorR3R4(dict, fileID, password, RevisionAES128, r, p)
	case 6:
		if err := requireStdCF(dict, "AESV3"); err != nil {
			return nil, err
		}
		return newDecryptorR6(dict, fileID, password, p)
	default:
		return nil, fmt.Errorf("%w: revision R=%d", ErrUnsupportedEncryption, r)
	}
}

// newDecryptorR3R4 authenticates password against /O and /U for
// revisions 3 (RC4-128) and 4 (AES-128), which share Algorithms 2, 3,
// 6, and 7 from ISO 32000-1 §7.6.3.
func newDecryptorR3R4(dict *PdfDictionary, fileID []byte, password string, rev EncryptionRevision, r int, p int32) (*Decryptor, error) {
	o, u := dictBytes(dict, "O"), dictBytes(dict, "U")
	if len(o) < 32 || len(u) < 16 {
		return nil, fmt.Errorf("%w: malformed /O or /U for revision R=%d", ErrUnsupportedEncryption, r)
	}
	o = o[:32]
	keyLen := keyLengthBytes(dict, rev)
	encryptMetadata := dictBool(dict, "EncryptMetadata", true)

	// Try password as the user password (Algorithm 6): the candidate
	// only needs to reproduce the first 16 bytes of /U.
	fileKey := computeFileKeyR3(password, o, p, fileID, keyLen)
	if bytes.Equal(computeUserHashR3(fileKey, fileID)[:16], u[:16]) {
		return &Decryptor{Revision: rev, FileKey: fileKey, Access: AccessUser, P: p, EncryptMetadata: encryptMetadata, keyLen: keyLen}, nil
	}

	// Try password as the owner password (Algorithm 7): derive the RC4
	// key from the owner password alone, then undo the 20 RC4 rounds
	// Algorithm 3 applied when writing /O to recover the padded user
	// password. Algorithm 2 is idempotent on an already-padded 32-byte
	// input, so the recovered bytes can be fed back in directly.
	ownerKey := ownerRC4Key(password, keyLen)
	recovered := append([]byte(nil), o...)
	for i := 19; i >= 1; i-- {
		recovered = rc4Encrypt(xorKey(ownerKey, byte(i)), recovered)
	}
	recovered = rc4Encrypt(ownerKey, recovered)

	fileKey = computeFileKeyR3(string(recovered), o, p, fileID, keyLen)
	if bytes.Equal(computeUserHashR3(fileKey, fileID)[:16], u[:16]) {
		return &Decryptor{Revision: rev, FileKey: fileKey, Access: AccessOwner, P: p, EncryptMetadata: encryptMetadata, keyLen: keyLen}, nil
	}

	return nil, ErrInvalidPassword
}

// aesECBDecryptBlock decrypts a single 16-byte block with AES-ECB — the
// inverse of aesECBEncryptBlock (core/encrypt.go), used only for the R6
// /Perms entry.
func aesECBDecryptBlock(key, block16 []byte) []byte {
	b, _ := aes.NewCipher(key)
	out := make([]byte, 16)
	b.Decrypt(out, block16)
	return out
}

// validatePermsR6 implements ISO 32000-2 Algorithm 13: decrypt /Perms
// with the file key and confirm it authenticates the plaintext /P value.
func validatePermsR6(dict *PdfDictionary, fileKey []byte, p int32) error {
	perms := dictBytes(dict, "Perms")
	if len(perms) != 16 {
		return fmt.Errorf("%w: malformed /Perms for revision R=6", ErrUnsupportedEncryption)
	}
	plain := aesECBDecryptBlock(fileKey, perms)
	if plain[9] != 'a' || plain[10] != 'd' || plain[11] != 'b' {
		return errors.New("core: /Perms block does not authenticate (bad marker)")
	}
	if int32(binary.LittleEndian.Uint32(plain[0:4])) != p {
		return errors.New("core: /Perms permission bits do not match /P (file tampered?)")
	}
	return nil
}

// newDecryptorR6 authenticates password against /O and /U for revision
// 6 (AES-256, PDF 2.0), per ISO 32000-2 §7.6.4.3.
func newDecryptorR6(dict *PdfDictionary, fileID []byte, password string, p int32) (*Decryptor, error) {
	o, u := dictBytes(dict, "O"), dictBytes(dict, "U")
	oe, ue := dictBytes(dict, "OE"), dictBytes(dict, "UE")
	if len(o) < 48 || len(u) < 48 || len(oe) != 32 || len(ue) != 32 {
		return nil, fmt.Errorf("%w: malformed /O, /U, /OE, or /UE for revision R=6", ErrUnsupportedEncryption)
	}
	o, u = o[:48], u[:48]
	encryptMetadata := dictBool(dict, "EncryptMetadata", true)
	pwd := saslPrepPassword(password)

	uHash, uValSalt, uKeySalt := u[0:32], u[32:40], u[40:48]
	if bytes.Equal(algorithmR6Hash(pwd, uValSalt, nil), uHash) {
		fileKey, err := aesCBCDecryptNoPadding(algorithmR6Hash(pwd, uKeySalt, nil), ue)
		if err != nil {
			return nil, fmt.Errorf("core: decrypt: unwrap /UE: %w", err)
		}
		if err := validatePermsR6(dict, fileKey, p); err != nil {
			return nil, err
		}
		return &Decryptor{Revision: RevisionAES256, FileKey: fileKey, Access: AccessUser, P: p, EncryptMetadata: encryptMetadata, keyLen: 32}, nil
	}

	// Owner path: the hash and key-unwrap both fold in the full 48-byte
	// /U value (§7.6.4.3.4, Algorithm 2.A steps for the owner password).
	oHash, oValSalt, oKeySalt := o[0:32], o[32:40], o[40:48]
	if bytes.Equal(algorithmR6Hash(pwd, oValSalt, u), oHash) {
		fileKey, err := aesCBCDecryptNoPadding(algorithmR6Hash(pwd, oKeySalt, u), oe)
		if err != nil {
			return nil, fmt.Errorf("core: decrypt: unwrap /OE: %w", err)
		}
		if err := validatePermsR6(dict, fileKey, p); err != nil {
			return nil, err
		}
		return &Decryptor{Revision: RevisionAES256, FileKey: fileKey, Access: AccessOwner, P: p, EncryptMetadata: encryptMetadata, keyLen: 32}, nil
	}

	return nil, ErrInvalidPassword
}

// ownerRC4Key computes the RC4 key derived from an owner password alone
// (ISO 32000-1 §7.6.3.3, Algorithm 3 steps a-d): pad, MD5, then 50
// further rounds of MD5, truncated to keyLen bytes.
func ownerRC4Key(ownerPwd string, keyLen int) []byte {
	padded := padPassword([]byte(ownerPwd))
	h := md5.Sum(padded[:])
	for range 50 {
		h = md5.Sum(h[:])
	}
	return h[:keyLen]
}

// keyLengthBytes determines the file-key length in bytes for R3/R4,
// clamped to the range Algorithm 1 supports (5-16 bytes) so a malformed
// /Length cannot produce a zero- or over-length key.
func keyLengthBytes(dict *PdfDictionary, rev EncryptionRevision) int {
	if rev == RevisionAES128 {
		// V=4 crypt filters carry /Length in BYTES inside /CF/StdCF,
		// distinct from the top-level /Length (bits).
		if cf, ok := dict.Get("CF").(*PdfDictionary); ok {
			if stdCF, ok := cf.Get("StdCF").(*PdfDictionary); ok {
				if n := dictInt(stdCF, "Length", 0); n > 0 {
					return clampKeyLen(n)
				}
			}
		}
	}
	return clampKeyLen(dictInt(dict, "Length", 128) / 8)
}

// clampKeyLen bounds n to the key lengths Algorithm 1 supports.
func clampKeyLen(n int) int {
	if n < 5 {
		return 5
	}
	if n > 16 {
		return 16
	}
	return n
}

// requireStdCF validates that /StmF and /StrF both select /StdCF and
// that /CF/StdCF's /CFM matches want. Identity, per-filter overrides,
// and RC4-under-V4 crypt filters are out of scope for this decoder.
func requireStdCF(dict *PdfDictionary, want string) error {
	stmF, _ := dict.Get("StmF").(*PdfName)
	strF, _ := dict.Get("StrF").(*PdfName)
	if stmF == nil || stmF.Value != "StdCF" || strF == nil || strF.Value != "StdCF" {
		return fmt.Errorf("%w: crypt filter StmF=%q StrF=%q", ErrUnsupportedEncryption,
			nameOrEmpty(dict.Get("StmF")), nameOrEmpty(dict.Get("StrF")))
	}
	cfDict, _ := dict.Get("CF").(*PdfDictionary)
	var stdCF *PdfDictionary
	if cfDict != nil {
		stdCF, _ = cfDict.Get("StdCF").(*PdfDictionary)
	}
	if stdCF == nil {
		return fmt.Errorf("%w: missing /CF/StdCF dictionary", ErrUnsupportedEncryption)
	}
	cfm, _ := stdCF.Get("CFM").(*PdfName)
	if cfm == nil || cfm.Value != want {
		return fmt.Errorf("%w: crypt filter method %q", ErrUnsupportedEncryption, nameOrEmpty(stdCF.Get("CFM")))
	}
	return nil
}

// dictInt reads an integer dictionary entry, returning def if absent or
// not a number.
func dictInt(dict *PdfDictionary, key string, def int) int {
	if num, ok := dict.Get(key).(*PdfNumber); ok {
		return num.IntValue()
	}
	return def
}

// dictBool reads a boolean dictionary entry, returning def if absent or
// not a boolean.
func dictBool(dict *PdfDictionary, key string, def bool) bool {
	if b, ok := dict.Get(key).(*PdfBoolean); ok {
		return b.Bool()
	}
	return def
}

// dictBytes reads a string dictionary entry as raw bytes, returning nil
// if absent or not a string.
func dictBytes(dict *PdfDictionary, key string) []byte {
	if s, ok := dict.Get(key).(*PdfString); ok {
		return []byte(s.Text())
	}
	return nil
}

// nameOrEmpty returns a PdfName's value, or "" if obj is not a name.
func nameOrEmpty(obj PdfObject) string {
	if n, ok := obj.(*PdfName); ok {
		return n.Value
	}
	return ""
}

// objectKeyRC4 derives the per-object RC4 key (Algorithm 1, steps a-e).
// Mirrors [Encryptor.objectKeyRC4] — the same key decrypts what it
// encrypted.
func (d *Decryptor) objectKeyRC4(objNum, genNum int) []byte {
	h := md5.New()
	h.Write(d.FileKey)
	var buf [5]byte
	buf[0], buf[1], buf[2] = byte(objNum), byte(objNum>>8), byte(objNum>>16)
	buf[3], buf[4] = byte(genNum), byte(genNum>>8)
	h.Write(buf[:])
	sum := h.Sum(nil)
	return sum[:min(d.keyLen+5, 16)]
}

// objectKeyAES derives the per-object AES key with the "sAlT" suffix.
// Mirrors [Encryptor.objectKeyAES].
func (d *Decryptor) objectKeyAES(objNum, genNum int) []byte {
	h := md5.New()
	h.Write(d.FileKey)
	var buf [5]byte
	buf[0], buf[1], buf[2] = byte(objNum), byte(objNum>>8), byte(objNum>>16)
	buf[3], buf[4] = byte(genNum), byte(genNum>>8)
	h.Write(buf[:])
	h.Write([]byte("sAlT"))
	return h.Sum(nil)[:16]
}

// DecryptBytes decrypts a string or stream payload belonging to the
// given indirect object (Algorithm 1). Objects nested inside an object
// stream must NOT be passed through this a second time — the
// containing /ObjStm was already decrypted as a whole.
func (d *Decryptor) DecryptBytes(objNum, genNum int, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	switch d.Revision {
	case RevisionRC4128:
		return rc4Encrypt(d.objectKeyRC4(objNum, genNum), data), nil
	case RevisionAES128:
		return aesCBCDecrypt(d.objectKeyAES(objNum, genNum), data)
	case RevisionAES256:
		return aesCBCDecrypt(d.FileKey, data)
	default:
		return nil, fmt.Errorf("%w: revision %d", ErrUnsupportedEncryption, d.Revision)
	}
}

// DecryptObject walks obj in place, decrypting every string it
// contains. Streams are decrypted separately via [Decryptor.DecryptBytes]
// on their raw payload before decompression, so this walk descends into
// dictionaries, arrays, and strings only — mirroring [Encryptor.walkEncrypt]
// in reverse. Indirect references are left untouched; the object they
// point to is decrypted under its own object number when resolved.
func (d *Decryptor) DecryptObject(obj PdfObject, objNum, genNum int) error {
	switch o := obj.(type) {
	case *PdfString:
		dec, err := d.DecryptBytes(objNum, genNum, []byte(o.value))
		if err != nil {
			return fmt.Errorf("core: decrypt string (obj %d): %w", objNum, err)
		}
		o.value = string(dec)
	case *PdfDictionary:
		for _, entry := range o.Entries {
			if err := d.DecryptObject(entry.Value, objNum, genNum); err != nil {
				return err
			}
		}
	case *PdfArray:
		for _, elem := range o.Elements {
			if err := d.DecryptObject(elem, objNum, genNum); err != nil {
				return err
			}
		}
	}
	return nil
}

// aesCBCDecrypt decrypts data produced by [aesCBCEncrypt]: a 16-byte IV
// prefix followed by AES-CBC ciphertext PKCS#7-padded to the block
// size. Returns an error — never panics — on a too-short, misaligned,
// or invalidly padded payload, since a hostile or corrupt PDF can put
// arbitrary bytes here even after the password has been verified.
func aesCBCDecrypt(key, data []byte) ([]byte, error) {
	if len(data) < 2*aes.BlockSize {
		return nil, fmt.Errorf("core: decrypt: ciphertext too short (%d bytes)", len(data))
	}
	iv, ciphertext := data[:aes.BlockSize], data[aes.BlockSize:]
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("core: decrypt: ciphertext length %d is not block-aligned", len(ciphertext))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("core: decrypt: %w", err)
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ciphertext)
	return pkcs7Unpad(plain)
}

// pkcs7Unpad strips and validates PKCS#7 padding, the inverse of
// [pkcs7Pad]. Returns an error on malformed padding instead of trusting
// attacker-controlled bytes.
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("core: decrypt: empty plaintext")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > aes.BlockSize || padLen > len(data) {
		return nil, fmt.Errorf("core: decrypt: invalid PKCS#7 padding")
	}
	for _, b := range data[len(data)-padLen:] {
		if int(b) != padLen {
			return nil, fmt.Errorf("core: decrypt: invalid PKCS#7 padding")
		}
	}
	return data[:len(data)-padLen], nil
}

// aesCBCDecryptNoPadding decrypts a block-aligned payload with a zero
// IV and no padding — the inverse of [aesECBLikeEncrypt] — used to
// unwrap the R6 file key from /UE or /OE.
func aesCBCDecryptNoPadding(key, data []byte) ([]byte, error) {
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("core: decrypt: payload length %d is not block-aligned", len(data))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("core: decrypt: %w", err)
	}
	plain := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(plain, data)
	return plain, nil
}

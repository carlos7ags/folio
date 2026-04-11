// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Permission flags for PDF document encryption (ISO 32000 Table 22).
// Combine with | to grant multiple permissions.
type Permission uint32

const (
	PermPrint         Permission = 1 << 2  // bit 3: print
	PermModify        Permission = 1 << 3  // bit 4: modify contents
	PermExtract       Permission = 1 << 4  // bit 5: copy/extract text and graphics
	PermAnnotate      Permission = 1 << 5  // bit 6: add/modify annotations, fill forms
	PermFillForms     Permission = 1 << 8  // bit 9: fill existing form fields
	PermExtractAccess Permission = 1 << 9  // bit 10: extract for accessibility
	PermAssemble      Permission = 1 << 10 // bit 11: assemble (insert, rotate, delete pages)
	PermPrintHigh     Permission = 1 << 11 // bit 12: high-quality print

	// PermAll grants all permissions.
	PermAll = PermPrint | PermModify | PermExtract | PermAnnotate |
		PermFillForms | PermExtractAccess | PermAssemble | PermPrintHigh
)

// permBits converts Permission to the signed 32-bit value used in the /P entry.
// Bits 7-8 and 13-32 must be 1; bits 1-2 must be 0.
func permBits(p Permission) int32 {
	return int32(uint32(p) | 0xFFFFF0C0)
}

// EncryptionRevision identifies the encryption algorithm.
type EncryptionRevision int

const (
	// RevisionRC4128 selects the RC4-128 Standard Security Handler (V=2, R=3).
	//
	// Deprecated: RC4 is cryptographically broken and kept only for backward
	// compatibility with legacy readers. Use [RevisionAES256] for new documents.
	RevisionRC4128 EncryptionRevision = 3
	RevisionAES128 EncryptionRevision = 4 // AES-128-CBC (V=4, R=4)
	RevisionAES256 EncryptionRevision = 6 // AES-256-CBC (V=5, R=6)
)

// pdfPadding is the 32-byte padding string from ISO 32000 §7.6.3.3, Algorithm 2.
var pdfPadding = [32]byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41,
	0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80,
	0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

// Encryptor handles PDF object encryption for a single document.
type Encryptor struct {
	Revision EncryptionRevision
	FileKey  []byte // 16 bytes (RC4/AES-128) or 32 bytes (AES-256)
	O, U     []byte // owner/user hash (32 bytes for R3/R4, 48 bytes for R6)
	OE, UE   []byte // owner/user key encryption (32 bytes, R6 only)
	Perms    []byte // encrypted permissions (16 bytes, R6 only)
	P        int32  // permission flags
	FileID   []byte // 16-byte file identifier

	keyLen            int // key length in bytes (16 or 32)
	encryptDictObjNum int // object number of /Encrypt dict (skip during walk)
}

// NewEncryptor creates an Encryptor for the given revision and passwords.
// If ownerPassword is empty, it defaults to userPassword.
func NewEncryptor(rev EncryptionRevision, userPassword, ownerPassword string, perms Permission) (*Encryptor, error) {
	if ownerPassword == "" {
		ownerPassword = userPassword
	}

	fileID, err := randomBytes(16)
	if err != nil {
		return nil, fmt.Errorf("encrypt: generate file ID: %w", err)
	}

	p := permBits(perms)

	switch rev {
	case RevisionRC4128:
		return newEncryptorR3(userPassword, ownerPassword, p, fileID)
	case RevisionAES128:
		return newEncryptorR4(userPassword, ownerPassword, p, fileID)
	case RevisionAES256:
		return newEncryptorR6(userPassword, ownerPassword, p, fileID)
	default:
		return nil, fmt.Errorf("encrypt: unsupported revision %d", rev)
	}
}

// SetEncryptDictObjNum records the object number of the /Encrypt dictionary
// so it can be skipped during the encryption walk.
func (e *Encryptor) SetEncryptDictObjNum(n int) {
	e.encryptDictObjNum = n
}

// EncryptBytes encrypts data for the given indirect object.
func (e *Encryptor) EncryptBytes(objNum, genNum int, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	switch e.Revision {
	case RevisionRC4128:
		return rc4Encrypt(e.objectKeyRC4(objNum, genNum), data), nil
	case RevisionAES128:
		return aesCBCEncrypt(e.objectKeyAES(objNum, genNum), data)
	case RevisionAES256:
		return aesCBCEncrypt(e.FileKey, data)
	default:
		return data, nil
	}
}

// EncryptObject walks a PdfObject tree and encrypts all strings and
// stream data in place. The /Encrypt dictionary object is skipped.
func (e *Encryptor) EncryptObject(obj PdfObject, objNum, genNum int) error {
	if objNum == e.encryptDictObjNum {
		return nil
	}
	return e.walkEncrypt(obj, objNum, genNum)
}

// walkEncrypt recursively visits a PdfObject tree, encrypting strings and
// stream data in place.
func (e *Encryptor) walkEncrypt(obj PdfObject, objNum, genNum int) error {
	switch o := obj.(type) {
	case *PdfString:
		enc, err := e.EncryptBytes(objNum, genNum, []byte(o.value))
		if err != nil {
			return fmt.Errorf("encrypt string (obj %d): %w", objNum, err)
		}
		o.value = string(enc)
		o.encoding = StringHexadecimal // binary data must be hex-encoded
	case *PdfDictionary:
		for _, entry := range o.Entries {
			if err := e.walkEncrypt(entry.Value, objNum, genNum); err != nil {
				return err
			}
		}
	case *PdfArray:
		for _, elem := range o.Elements {
			if err := e.walkEncrypt(elem, objNum, genNum); err != nil {
				return err
			}
		}
	case *PdfStream:
		// Encrypt strings in the stream dictionary.
		for _, entry := range o.Dict.Entries {
			if err := e.walkEncrypt(entry.Value, objNum, genNum); err != nil {
				return err
			}
		}
		// Compress data first (if applicable), then encrypt.
		data := o.Data
		if o.compress {
			compressed, err := deflate(data)
			if err != nil {
				return fmt.Errorf("compress stream (obj %d): %w", objNum, err)
			}
			data = compressed
			o.Dict.Set("Filter", NewPdfName("FlateDecode"))
			o.compress = false
		}
		enc, err := e.EncryptBytes(objNum, genNum, data)
		if err != nil {
			return fmt.Errorf("encrypt stream (obj %d): %w", objNum, err)
		}
		o.Data = enc
	}
	return nil
}

// BuildEncryptDict returns the /Encrypt dictionary for the trailer.
func (e *Encryptor) BuildEncryptDict() *PdfDictionary {
	d := NewPdfDictionary()
	d.Set("Filter", NewPdfName("Standard"))
	d.Set("P", NewPdfInteger(int(e.P)))

	switch e.Revision {
	case RevisionRC4128:
		d.Set("V", NewPdfInteger(2))
		d.Set("R", NewPdfInteger(3))
		d.Set("Length", NewPdfInteger(128))
		d.Set("O", NewPdfHexString(string(e.O)))
		d.Set("U", NewPdfHexString(string(e.U)))

	case RevisionAES128:
		d.Set("V", NewPdfInteger(4))
		d.Set("R", NewPdfInteger(4))
		d.Set("Length", NewPdfInteger(128))
		d.Set("O", NewPdfHexString(string(e.O)))
		d.Set("U", NewPdfHexString(string(e.U)))
		// Crypt filter dictionary.
		stdCF := NewPdfDictionary()
		stdCF.Set("Type", NewPdfName("CryptFilter"))
		stdCF.Set("CFM", NewPdfName("AESV2"))
		stdCF.Set("AuthEvent", NewPdfName("DocOpen"))
		stdCF.Set("Length", NewPdfInteger(16))
		cf := NewPdfDictionary()
		cf.Set("StdCF", stdCF)
		d.Set("CF", cf)
		d.Set("StmF", NewPdfName("StdCF"))
		d.Set("StrF", NewPdfName("StdCF"))
		d.Set("EncryptMetadata", NewPdfBoolean(true))

	case RevisionAES256:
		d.Set("V", NewPdfInteger(5))
		d.Set("R", NewPdfInteger(6))
		d.Set("Length", NewPdfInteger(256))
		d.Set("O", NewPdfHexString(string(e.O)))
		d.Set("U", NewPdfHexString(string(e.U)))
		d.Set("OE", NewPdfHexString(string(e.OE)))
		d.Set("UE", NewPdfHexString(string(e.UE)))
		d.Set("Perms", NewPdfHexString(string(e.Perms)))
		// Crypt filter dictionary.
		stdCF := NewPdfDictionary()
		stdCF.Set("Type", NewPdfName("CryptFilter"))
		stdCF.Set("CFM", NewPdfName("AESV3"))
		stdCF.Set("AuthEvent", NewPdfName("DocOpen"))
		stdCF.Set("Length", NewPdfInteger(32))
		cf := NewPdfDictionary()
		cf.Set("StdCF", stdCF)
		d.Set("CF", cf)
		d.Set("StmF", NewPdfName("StdCF"))
		d.Set("StrF", NewPdfName("StdCF"))
		d.Set("EncryptMetadata", NewPdfBoolean(true))
	}
	return d
}

// --- Revision 3 (RC4-128) ---

// newEncryptorR3 creates an Encryptor using RC4-128 (Revision 3).
func newEncryptorR3(userPwd, ownerPwd string, p int32, fileID []byte) (*Encryptor, error) {
	const keyLen = 16
	o := computeOwnerHashR3(userPwd, ownerPwd, keyLen)
	fileKey := computeFileKeyR3(userPwd, o, p, fileID, keyLen)
	u := computeUserHashR3(fileKey, fileID)
	return &Encryptor{
		Revision: RevisionRC4128,
		FileKey:  fileKey, O: o, U: u,
		P: p, FileID: fileID, keyLen: keyLen,
	}, nil
}

// computeOwnerHashR3 computes the owner password hash (O value) using Algorithm 3
// from ISO 32000 §7.6.3.3.
func computeOwnerHashR3(userPwd, ownerPwd string, keyLen int) []byte {
	// Step a-d: hash the owner password.
	padded := padPassword([]byte(ownerPwd))
	h := md5.Sum(padded[:])
	for range 50 {
		h = md5.Sum(h[:])
	}
	key := h[:keyLen]

	// Step e-f: RC4-encrypt the padded user password.
	paddedUser := padPassword([]byte(userPwd))
	result := rc4Encrypt(key, paddedUser[:])

	// Step g: 19 additional RC4 rounds with XOR'd keys.
	for i := 1; i <= 19; i++ {
		result = rc4Encrypt(xorKey(key, byte(i)), result)
	}
	return result // 32 bytes
}

// computeFileKeyR3 computes the file encryption key using Algorithm 2
// from ISO 32000 §7.6.3.3.
func computeFileKeyR3(userPwd string, o []byte, p int32, fileID []byte, keyLen int) []byte {
	padded := padPassword([]byte(userPwd))
	h := md5.New()
	h.Write(padded[:])
	h.Write(o)
	var pbuf [4]byte
	binary.LittleEndian.PutUint32(pbuf[:], uint32(p))
	h.Write(pbuf[:])
	h.Write(fileID)
	sum := h.Sum(nil)
	for range 50 {
		tmp := md5.Sum(sum[:keyLen])
		sum = tmp[:]
	}
	return sum[:keyLen]
}

// computeUserHashR3 computes the user password hash (U value) for R=3 using
// Algorithm 5 from ISO 32000 §7.6.3.4.
func computeUserHashR3(fileKey, fileID []byte) []byte {
	h := md5.New()
	h.Write(pdfPadding[:])
	h.Write(fileID)
	sum := h.Sum(nil)

	result := rc4Encrypt(fileKey, sum)
	for i := 1; i <= 19; i++ {
		result = rc4Encrypt(xorKey(fileKey, byte(i)), result)
	}
	// Pad to 32 bytes.
	u := make([]byte, 32)
	copy(u, result)
	return u
}

// objectKeyRC4 derives the per-object RC4 key (Algorithm 1, step a-e).
func (e *Encryptor) objectKeyRC4(objNum, genNum int) []byte {
	h := md5.New()
	h.Write(e.FileKey)
	var buf [5]byte
	buf[0] = byte(objNum)
	buf[1] = byte(objNum >> 8)
	buf[2] = byte(objNum >> 16)
	buf[3] = byte(genNum)
	buf[4] = byte(genNum >> 8)
	h.Write(buf[:])
	sum := h.Sum(nil)
	n := min(e.keyLen+5, 16)
	return sum[:n]
}

// --- Revision 4 (AES-128) ---

// newEncryptorR4 creates an Encryptor using AES-128-CBC (Revision 4).
func newEncryptorR4(userPwd, ownerPwd string, p int32, fileID []byte) (*Encryptor, error) {
	const keyLen = 16
	o := computeOwnerHashR3(userPwd, ownerPwd, keyLen) // same algorithm
	fileKey := computeFileKeyR3(userPwd, o, p, fileID, keyLen)
	u := computeUserHashR3(fileKey, fileID)
	return &Encryptor{
		Revision: RevisionAES128,
		FileKey:  fileKey, O: o, U: u,
		P: p, FileID: fileID, keyLen: keyLen,
	}, nil
}

// objectKeyAES derives the per-object AES key with "sAlT" suffix.
func (e *Encryptor) objectKeyAES(objNum, genNum int) []byte {
	h := md5.New()
	h.Write(e.FileKey)
	var buf [5]byte
	buf[0] = byte(objNum)
	buf[1] = byte(objNum >> 8)
	buf[2] = byte(objNum >> 16)
	buf[3] = byte(genNum)
	buf[4] = byte(genNum >> 8)
	h.Write(buf[:])
	h.Write([]byte("sAlT"))
	sum := h.Sum(nil)
	return sum[:16]
}

// --- Revision 6 (AES-256) ---

// newEncryptorR6 creates an Encryptor using AES-256-CBC (Revision 6, PDF 2.0).
func newEncryptorR6(userPwd, ownerPwd string, p int32, fileID []byte) (*Encryptor, error) {
	// SASLprep (NFKC normalize + truncate to 127 bytes at rune boundary).
	uPwd := saslPrepPassword(userPwd)
	oPwd := saslPrepPassword(ownerPwd)

	// Random 32-byte file encryption key.
	fileKey, err := randomBytes(32)
	if err != nil {
		return nil, err
	}

	// User: validation salt (8) + key salt (8).
	uValSalt, err2 := randomBytes(8)
	if err2 != nil {
		return nil, err2
	}
	uKeySalt, err2 := randomBytes(8)
	if err2 != nil {
		return nil, err2
	}

	// U = hash(pwd, valSalt, "") || valSalt || keySalt
	uHash := algorithmR6Hash(uPwd, uValSalt, nil)
	u := make([]byte, 48)
	copy(u[0:32], uHash)
	copy(u[32:40], uValSalt)
	copy(u[40:48], uKeySalt)

	// UE = AES-256-CBC(key=hash(pwd, keySalt, ""), iv=zeros, data=fileKey)
	ueKey := algorithmR6Hash(uPwd, uKeySalt, nil)
	ue, err2 := aesECBLikeEncrypt(ueKey, fileKey)
	if err2 != nil {
		return nil, err2
	}

	// Owner: validation salt (8) + key salt (8).
	oValSalt, err2 := randomBytes(8)
	if err2 != nil {
		return nil, err2
	}
	oKeySalt, err2 := randomBytes(8)
	if err2 != nil {
		return nil, err2
	}

	// O = hash(pwd, valSalt, U) || valSalt || keySalt
	oHash := algorithmR6Hash(oPwd, oValSalt, u)
	o := make([]byte, 48)
	copy(o[0:32], oHash)
	copy(o[32:40], oValSalt)
	copy(o[40:48], oKeySalt)

	// OE = AES-256-CBC(key=hash(pwd, keySalt, U), iv=zeros, data=fileKey)
	oeKey := algorithmR6Hash(oPwd, oKeySalt, u)
	oe, err2 := aesECBLikeEncrypt(oeKey, fileKey)
	if err2 != nil {
		return nil, err2
	}

	// Perms: AES-256-ECB encrypt 16-byte permissions block.
	perms, err := buildPermsBlock(p)
	if err != nil {
		return nil, err
	}
	encPerms := aesECBEncryptBlock(fileKey, perms)

	return &Encryptor{
		Revision: RevisionAES256,
		FileKey:  fileKey, O: o, U: u, OE: oe, UE: ue,
		Perms: encPerms, P: p, FileID: fileID, keyLen: 32,
	}, nil
}

// algorithmR6Hash implements Algorithm 2.B from ISO 32000-2.
func algorithmR6Hash(password, salt, userKey []byte) []byte {
	// Initial hash: SHA-256(password || salt || userKey).
	h := sha256.New()
	h.Write(password)
	h.Write(salt)
	h.Write(userKey)
	k := h.Sum(nil) // 32 bytes

	// Round numbering is 1-based per the spec and qpdf reference implementation.
	for round := 1; ; round++ {
		// K1 = (password || K || userKey) repeated 64 times.
		single := make([]byte, 0, len(password)+len(k)+len(userKey))
		single = append(single, password...)
		single = append(single, k...)
		single = append(single, userKey...)
		k1 := make([]byte, 0, len(single)*64)
		for range 64 {
			k1 = append(k1, single...)
		}

		// E = AES-128-CBC(key=K[0:16], iv=K[16:32], data=K1).
		// K[0:16] always comes from a hash output, so NewCipher cannot fail
		// for a standards-compliant AES implementation; panic instead of
		// silently discarding the error.
		block, err := aes.NewCipher(k[0:16])
		if err != nil {
			panic("core: aes.NewCipher on hash-derived key failed: " + err.Error())
		}
		cbc := cipher.NewCBCEncrypter(block, k[16:32])
		e := make([]byte, len(k1))
		cbc.CryptBlocks(e, k1)

		// Determine hash function from first 16 bytes mod 3.
		// Since 256 ≡ 1 (mod 3), the big-endian integer mod 3 equals the byte sum mod 3.
		var byteSum int
		for _, b := range e[:16] {
			byteSum += int(b)
		}
		mod3 := byteSum % 3

		switch mod3 {
		case 0:
			sum := sha256.Sum256(e)
			k = sum[:]
		case 1:
			sum := sha512.Sum384(e)
			k = sum[:]
		case 2:
			sum := sha512.Sum512(e)
			k = sum[:]
		}

		if round >= 64 && int(e[len(e)-1]) <= round-32 {
			break
		}
	}
	return k[:32]
}

// buildPermsBlock creates the 16-byte permissions plaintext for R=6.
func buildPermsBlock(p int32) ([]byte, error) {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(p))
	// Bytes 4-7: 0xFFFFFFFF.
	buf[4], buf[5], buf[6], buf[7] = 0xFF, 0xFF, 0xFF, 0xFF
	// Byte 8: 'T' (encrypt metadata = true).
	buf[8] = 'T'
	// Bytes 9-11: 'adb' (per spec).
	buf[9], buf[10], buf[11] = 'a', 'd', 'b'
	// Bytes 12-15: random.
	if _, err := rand.Read(buf[12:16]); err != nil {
		return nil, fmt.Errorf("encrypt: random bytes for perms block: %w", err)
	}
	return buf, nil
}

// --- Cryptographic helpers ---

// padPassword pads or truncates pwd to exactly 32 bytes using the standard
// PDF padding string.
func padPassword(pwd []byte) [32]byte {
	var result [32]byte
	n := copy(result[:], pwd)
	if n < 32 {
		copy(result[n:], pdfPadding[:32-n])
	}
	return result
}

// truncatePassword truncates a UTF-8 password to at most 127 bytes, as
// required by ISO 32000-2 §7.6.4.3.3. Truncation respects rune boundaries:
// if byte 127 falls inside a multi-byte sequence, it is cut at the preceding
// rune start so the result is always valid UTF-8.
func truncatePassword(pwd string) []byte {
	b := []byte(pwd)
	if len(b) <= 127 {
		return b
	}
	// Walk backward from byte 127 until we find a valid rune start.
	i := 127
	for i > 0 && !utf8.RuneStart(b[i]) {
		i--
	}
	return b[:i]
}

// saslPrepPassword applies a simplified SASLprep (RFC 4013) profile to a
// PDF password, as required by ISO 32000-2 §7.6.4.3.3 for revision 6.
// The full SASLprep specification includes character mapping, prohibited
// character checks, and bidirectional string validation. This implementation
// performs NFKC normalization — the most impactful step, which ensures that
// canonically equivalent Unicode strings (e.g., precomposed "é" vs. "e" +
// combining acute) produce the same encryption key — then truncates to
// 127 bytes at a rune boundary.
func saslPrepPassword(pwd string) []byte {
	return truncatePassword(norm.NFKC.String(pwd))
}

// rc4Encrypt encrypts (or decrypts) data using RC4 with the given key.
func rc4Encrypt(key, data []byte) []byte {
	c, _ := rc4.NewCipher(key)
	dst := make([]byte, len(data))
	c.XORKeyStream(dst, data)
	return dst
}

// xorKey returns a copy of key with every byte XORed with x.
func xorKey(key []byte, x byte) []byte {
	out := make([]byte, len(key))
	for i, b := range key {
		out[i] = b ^ x
	}
	return out
}

// aesCBCEncrypt encrypts data with AES-CBC, prepending a random IV.
// Data is PKCS#7 padded to block size.
func aesCBCEncrypt(key, data []byte) ([]byte, error) {
	padded := pkcs7Pad(data, aes.BlockSize)
	iv, err := randomBytes(aes.BlockSize)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, padded)
	return append(iv, encrypted...), nil
}

// aesECBLikeEncrypt encrypts data with AES-CBC using a zero IV and no PKCS7 padding.
// Used for UE/OE computation where data is already block-aligned (32 bytes).
func aesECBLikeEncrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := make([]byte, aes.BlockSize)
	encrypted := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, data)
	return encrypted, nil
}

// aesECBEncryptBlock encrypts a single 16-byte block with AES-ECB.
func aesECBEncryptBlock(key, block16 []byte) []byte {
	b, _ := aes.NewCipher(key)
	dst := make([]byte, 16)
	b.Encrypt(dst, block16)
	return dst
}

// pkcs7Pad appends PKCS#7 padding to data so its length is a multiple of blockSize.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return padded
}

// randomBytes returns n cryptographically random bytes.
func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

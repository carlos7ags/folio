// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import "github.com/carlos7ags/folio/core"

// EncryptionAlgorithm selects the encryption scheme.
type EncryptionAlgorithm int

const (
	// EncryptRC4128 uses RC4 with a 128-bit key (PDF 1.4+, widely compatible).
	EncryptRC4128 EncryptionAlgorithm = iota
	// EncryptAES128 uses AES-128-CBC (PDF 1.6+, recommended minimum).
	EncryptAES128
	// EncryptAES256 uses AES-256-CBC (PDF 2.0, strongest).
	EncryptAES256
)

// EncryptionConfig holds the settings for document encryption.
type EncryptionConfig struct {
	Algorithm     EncryptionAlgorithm
	UserPassword  string          // password to open the document (may be empty)
	OwnerPassword string          // password for full access (defaults to UserPassword)
	Permissions   core.Permission // granted permissions when opened with user password
}

// SetEncryption enables encryption on the document. The configuration
// specifies the algorithm, passwords, and permissions.
//
//	doc.SetEncryption(document.EncryptionConfig{
//	    Algorithm:     document.EncryptAES256,
//	    UserPassword:  "secret",
//	    OwnerPassword: "admin",
//	    Permissions:   core.PermPrint | core.PermExtract,
//	})
func (d *Document) SetEncryption(cfg EncryptionConfig) {
	d.encryption = &cfg
}

// revisionFromAlgorithm maps the public enum to the core revision.
func revisionFromAlgorithm(alg EncryptionAlgorithm) core.EncryptionRevision {
	switch alg {
	case EncryptAES128:
		return core.RevisionAES128
	case EncryptAES256:
		return core.RevisionAES256
	case EncryptRC4128:
		// Intentional: the public EncryptRC4128 option exists for PDF 1.4
		// compatibility. The deprecation on core.RevisionRC4128 targets new
		// callers, not this explicit mapping.
		return core.RevisionRC4128 //nolint:staticcheck // SA1019
	default:
		// Unknown enum value — fall back to the recommended minimum
		// instead of silently selecting broken RC4.
		return core.RevisionAES128
	}
}

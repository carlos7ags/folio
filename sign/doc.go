// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package sign implements PAdES digital signatures for PDF documents
// per ISO 32000 §12.8 and ETSI TS 102 778.
//
// Four conformance levels are supported:
//
//   - B-B  — basic signature (CMS/PKCS#7 detached)
//   - B-T  — adds an RFC 3161 timestamp
//   - B-LT — embeds revocation data (OCSP, CRL) in a Document Security Store (§12.8.6.3)
//   - B-LTA — adds a document-level timestamp for long-term archival
//
// Signing uses PDF incremental updates to preserve existing content
// and prior signatures. CMS structures are built from scratch using
// encoding/asn1 — no external cryptography dependencies.
//
// Use [SignPDF] with an [Options] containing a [Signer] (local key
// via [NewLocalSigner] or external HSM via [NewExternalSigner]),
// optional [TSAClient] and [OCSPClient] for higher conformance levels.
package sign

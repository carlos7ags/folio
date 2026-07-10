# Design: reading encrypted PDFs

This document describes the read-side counterpart to folio's existing
Standard security handler write support (`document.SetEncryption`,
`core.Encryptor`). It covers the API shape, password semantics, which
security-handler configurations are supported, the decrypt-on-resolve
design in the reader, how a decrypted document interacts with
downstream operations (merge, redact, re-encryption), and the
security posture callers should assume.

## 1. API shape

`reader.ReadOptions` gains a `Password string` field:

```go
type ReadOptions struct {
    Strictness   Strictness
    MaxCache     int
    MemoryLimits MemoryLimits
    Password     string
}
```

`reader.Parse` (no options) implicitly attempts the empty password.
This was chosen over a separate `reader.ParseEncrypted` entry point
because the single most common real-world case — a file whose
"protection" only restricts permissions, opened with an empty user
password by every viewer without prompting — needs zero caller
friction. A document that isn't encrypted at all is unaffected: the
password is only consulted when a trailer `/Encrypt` entry is present.
Callers that need to prompt for a password can catch
`reader.ErrInvalidPassword` and retry with `ParseWithOptions`.

An empty string is a genuine password attempt, not "no password
supplied" — there is no separate "did the caller supply a password"
flag. This matches the spec: the empty string is a valid password, and
the Standard handler always attempts whatever password it is given.

## 2. Password semantics

Per ISO 32000-1 §7.6.3 / ISO 32000-2 §7.6.4, `core.NewDecryptor` tries
the candidate password first as the **user (open) password**
(Algorithm 6 / 2.A), then as the **owner (permissions) password**
(Algorithm 7 / 2.A — recover the padded user password from `/O`, or for
R6, re-derive against `/O`'s validation salt with `/U` folded in). The
result records which one matched:

```go
type AccessLevel int

const (
    AccessNone  AccessLevel = iota // not encrypted
    AccessUser                     // matched the user password
    AccessOwner                    // matched the owner password
)
```

`reader.PdfReader.Access()` exposes this (aliased from
`core.AccessLevel`). A password that matches neither returns
`reader.ErrInvalidPassword` — a typed sentinel distinguishable from a
corrupt-file parse error, so callers can prompt and retry rather than
treating the file as unreadable. Password verification always happens
**before** any object content is decrypted: `core.NewDecryptor` fully
authenticates against `/O` and `/U` (and unwraps `/OE`/`/UE` for R6)
before returning a `*Decryptor`, and the reader only wires a decryptor
into the resolver after that call succeeds. A wrong-but-plausible key
is never used to decrypt content — the U/O hash comparison gates
everything.

## 3. Revision matrix

| Revision | `/V` | `/R` | Algorithm | Status |
|---|---|---|---|---|
| RC4-128 | 2 | 3 | RC4, symmetric | **Supported (read-only legacy)**. Spec-required for older files; folio never offers RC4 as a write-side choice for new documents beyond the existing deprecated option. |
| AES-128 | 4 | 4 | AES-128-CBC, `/CF /StdCF /CFM /AESV2` | **Supported**. Requires `/StmF` and `/StrF` to both select `/StdCF`; any other crypt-filter configuration is rejected (see below). |
| AES-256 | 5 | 6 | AES-256-CBC, `/CF /StdCF /CFM /AESV3` | **Supported**. Same `/StdCF`-only requirement. |
| RC4-40 | 1 | 2 | RC4 with a default 40-bit key | Open question (§6). Rare in practice; folio's write side never produced it. |
| AES-256 (deprecated) | 5 | 5 | Pre-ISO-32000-2 draft R5 hashing | Open question (§6). Superseded by R6 before publication; essentially unseen in the wild. |
| Public-key (certificate) handler | any | any | RSA/PKI-based | **Permanent clean error.** `/Filter` other than `/Standard` returns `core.ErrUnsupportedEncryption` naming the filter. Out of scope — see §6. |
| Non-standard crypt filters (`Identity`, per-filter overrides, `/CFM V2` under `/V 4`) | 4 | 4 | — | Clean error naming the crypt filter. Partial encryption (e.g. strings encrypted, streams not) is not modeled. |

Any `/R` outside {3, 4, 6}, or a `/V`/`/R` combination whose crypt
filter isn't exactly `/StdCF` with the expected `/CFM`, returns
`core.ErrUnsupportedEncryption` naming the value seen — never a panic,
never a silent pass-through of ciphertext as if it were plaintext.

## 4. Decrypt-on-resolve design

The hook lives in `reader/resolver.go`. `resolver` gained a
`decryptor *core.Decryptor` field (nil for unencrypted documents) and a
`SetDecryptor` setter. `reader.ParseWithOptions` resolves the trailer's
`/Encrypt` entry through the resolver **before** attaching a decryptor
(so the dictionary's own strings — `/O`, `/U`, etc. — come back
untouched, since they are never encrypted per §7.6), builds a
`core.Decryptor` from it, and only then calls `SetDecryptor`. Every
object resolved afterward is decrypted transparently:

- **Non-stream objects** (`resolver.Resolve`): after parsing, if a
  decryptor is attached, `Decryptor.DecryptObject` walks the object,
  decrypting every `core.PdfString` it contains, using the object's own
  number and generation (Algorithm 1). Dictionaries and arrays are
  walked; indirect references are left alone — the object they point to
  is decrypted under its own number when it is later resolved.
- **Stream objects** (`resolver.resolveStream`): the dictionary's
  strings are decrypted the same way. The raw payload is decrypted via
  `Decryptor.DecryptBytes` using the object's own number, **before**
  decompression — on-disk order is compress-then-encrypt, so reading
  back is decrypt-then-decompress.
- **Object-stream members** (`resolver.resolveCompressed`): never
  decrypted individually. Per §7.5.7, the entire containing `/ObjStm`
  is encrypted as a unit; by the time `resolveCompressed` parses an
  object out of it, `stream.Data` was already decrypted when the
  `/ObjStm` itself went through `resolveStream`. `resolveCompressed`
  does not call into the decryptor at all, which is what makes this
  correct — there's no flag to remember to check.
- **Cross-reference streams**: never decrypted, because they never flow
  through the resolver — `parseXrefTable` parses them directly from raw
  file bytes before a resolver (or a decryptor) exists at all.
- **`/Metadata` streams under `/EncryptMetadata false`**: `resolveStream`
  checks `Decryptor.EncryptMetadata` (defaults to `true` if absent) and
  a stream's `/Type`; if metadata encryption is disabled and the stream
  is `/Type /Metadata`, its payload is left as-is rather than run
  through `DecryptBytes`.
- **The `/Encrypt` dictionary's own strings**: never encrypted, and
  never decrypted. Because it's resolved and cached before
  `SetDecryptor` runs, any later reference to that object number returns
  the same cached, un-decrypted dictionary — correct by construction,
  not by a special case in `Resolve`.

Cache interaction: plaintext is what gets cached. Every decrypt call
happens on the freshly parsed object before `cacheObject` stores it, so
a cache hit always returns already-decrypted content — an object is
decrypted exactly once, regardless of how many times it's resolved.

## 5. Downstream refusals

A document read with a correct password is fully decrypted in memory —
`PdfReader` and everything built on top of it (`reader.Merge`,
redaction, page import, text/content extraction) sees plaintext
`PdfDictionary`/`PdfString`/`PdfStream` objects exactly as if the
source had never been encrypted. The writer-side refusals in
`document/writer_xref_stream.go` (object streams, orphan sweep,
recompression, dedup, content cleanup combined with encryption) apply
to documents this *process* is actively encrypting on write — they key
off `Writer.encryptor`, which is unrelated to how the input was read.
A decrypted-then-modified-then-saved document does not trip those
refusals, and by default is written back out **unencrypted**: the
reader does not carry the original `/Encrypt` configuration forward,
and nothing re-encrypts on save automatically. If the caller wants the
output protected, they call `document.SetEncryption` explicitly on the
write side, same as for any other document. Silently re-encrypting
would require deciding on behalf of the caller which password to use
for output, and would make "read, tweak metadata, save" accidentally
produce a locked file — the spike's answer is that explicit is safer
than convenient here (see open question in §6).

## 6. Security posture

folio's role is limited to key derivation and content decryption: given
a correct password, the caller gets full object access — the same
trust model every general-purpose PDF library uses (there is no
enforcement layer between "decrypted the file" and "can read every
object"). `/P` permission bits are surfaced via `Decryptor.P` /
the `/Encrypt` dictionary reachable from `PdfReader.Trailer()`, not
enforced. A library has no reliable way to prevent a caller who already
has the plaintext from ignoring permission bits — enforcement belongs
in a viewer UI, not here. This mirrors the write side, where
`core.Permission` flags are written into `/P` and `/Perms` (R6) but are
advisory, not access-controlled.

Decryption paths are defensive against hostile input by construction:
`core.NewDecryptor` validates every dictionary entry's type and length
before slicing it (malformed `/O`, `/U`, `/OE`, `/UE` return
`ErrUnsupportedEncryption` rather than reading out of bounds);
`aesCBCDecrypt` and `aesCBCDecryptNoPadding` check block alignment and
minimum length before calling into `crypto/cipher` (which panics on
misaligned input) and validate PKCS#7 padding by hand rather than
trusting it. None of this depends on the password already having been
verified — a corrupt or adversarial ciphertext block can still appear
after a correct password unlocks the file (a partially-corrupted real
file, or a deliberately crafted one), so the per-object decrypt path
has to be independently safe.

## Open questions

1. **RC4-40 (`/V 1`, `/R 2`) and the deprecated R5 AES-256 draft
   revision**: support for completeness, or leave as a permanent clean
   error? Both are rare; R2 slightly more plausible for very old files.
2. **`/EncryptMetadata false` and per-crypt-filter `/CF` configurations
   beyond the standard AESV2/AESV3 single-filter case**: the spike
   handles the `/EncryptMetadata false` + `/Metadata` interaction, but
   documents with multiple named crypt filters (some streams under one
   filter, some under another, or `Identity` for a subset) are rejected
   outright. Worth modeling, or an edge case to keep erroring on?
3. **Should `PdfReader` expose `/P` permission bits and the matched
   access level as a more prominent, viewer-friendly API** (e.g. named
   permission-check methods) than reading the `/Encrypt` dictionary by
   hand through `Trailer()`?
4. **Re-encryption on write**: should `reader.Merge` or redaction of a
   decrypted file offer a "preserve original encryption" option, or is
   explicit caller re-encryption (§5) sufficient indefinitely? This
   matters more once folio grows an in-place edit/save path rather than
   read-then-rebuild.
5. **Public-key (certificate) security handler**: permanently out of
   scope, or a future direction? It requires X.509/PKCS#7 handling well
   beyond `/Filter /Standard` and would be a much larger effort than
   this spike.
6. **Optimize-pipeline interaction**: any future `reader`-based optimize
   direction gains encrypted-input support for free once this lands
   (documents are plaintext in memory post-read), but its *output* is
   unencrypted per §5 — whoever builds that pipeline needs to either
   surface that clearly or wire in explicit re-encryption, not assume
   the encrypted-ness of the input carries through.
7. **Owner-password recovery correctness for exotic `/O` lengths**: the
   R3/R4 owner path assumes `/O` is exactly 32 bytes (truncating longer
   values). Real-world files that pad `/O` differently (some malformed
   writers emit more than 32 bytes) are currently handled by truncation
   rather than rejection — worth revisiting if it causes false
   `ErrInvalidPassword` results on owner-password recovery for real
   files.

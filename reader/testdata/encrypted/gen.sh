#!/bin/sh
# Regenerates the qpdf-encrypted fixtures from plain.pdf.
# Requires qpdf >= 11. Run from this directory.
set -eu

# AES-256, R6 (ISO 32000-2). User password "user123", owner "owner456".
qpdf --encrypt user123 owner456 256 -- plain.pdf aes256.pdf

# AES-128 (R4, /AESV2).
qpdf --encrypt user123 owner456 128 --use-aes=y -- plain.pdf aes128.pdf

# RC4-128 (R3, legacy).
qpdf --allow-weak-crypto --encrypt user123 owner456 128 --use-aes=n -- plain.pdf rc4-128.pdf

# Owner-password-only: empty user password, opens without a password at user level.
qpdf --encrypt "" owner456 256 -- plain.pdf owner-only.pdf

# AES-256 with /EncryptMetadata false (cleartext metadata).
qpdf --encrypt user123 owner456 256 --cleartext-metadata -- plain.pdf no-encrypt-metadata.pdf

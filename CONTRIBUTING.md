# Contributing to Folio

Thank you for your interest in contributing to Folio! This document covers the workflow and guidelines for contributing.

## Development Workflow

All development happens on **`main`**. There is no long-lived development branch.

1. Fork the repository
2. Create a feature branch from `main` (e.g., `feature/add-xyz`, `fix/issue-123`)
3. Make your changes
4. Open a pull request against `main`

Releases are cut by tagging `main` (e.g., `v0.5.0`).

## Prerequisites

- Go 1.25+
- `qpdf` (used in tests): `brew install qpdf` (macOS) or `apt-get install qpdf` (Linux)

## Building and Testing

```bash
# Build all packages
make build

# Run tests with race detection
make test

# Check formatting, vet, and test
make check

# Format code
make fmt
```

## Before Submitting a PR

Make sure your changes pass all CI checks locally:

```bash
make check
```

This runs formatting checks, `go vet`, and the full test suite.

### Code Style

- Run `gofmt -s -w .` before committing. CI will reject unformatted code.
- Follow standard Go conventions and idioms.
- Keep changes focused — one logical change per PR.
- Add tests for new functionality.

### Clean Room Policy

Folio is licensed under Apache 2.0 and developed independently. Do **not** reference, port, or adapt code from other PDF libraries. All contributions must be original work.

## Reporting Issues

Open an issue on GitHub. Include:

- What you were trying to do
- What happened instead
- Minimal reproduction steps (a code snippet or PDF file if applicable)
- Go version and OS

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

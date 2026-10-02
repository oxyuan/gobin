# gobin

A collection of standalone Go CLI utilities, each living in its own `package main` directory.

## Tech stack

- Language: Go 1.27 (`go.mod`, module `gobin`); toolchain used locally: go1.27.1
- Build system: Go toolchain + `Makefile` (no other task runner)
- Dependencies: `github.com/dlclark/regexp2` v1.12.0, `golang.org/x/net` v0.59.0
- No framework; utilities use only the standard library (`flag`, `sync`, `os/exec`, `net/http`)

## Package manager & commands

Go modules. Canonical commands live in the `Makefile`:

- Install deps: `go mod download` (keep it tidy with `go mod tidy`)
- Build all tools: `make build` (or `make`) → `dist/arthas`, `dist/fs`, `dist/gitu`
- Build one tool: `go build -o dist/fs ./fs`
- Run: `go run ./fs`, `go run ./gitu`, `go run ./arthas`
- Test: `make test` (`go test ./...`) — no test files exist yet
- Vet: `make vet` (`go vet ./...`)
- Lint: not configured (no golangci-lint/staticcheck config; `gofmt` is enforced instead)
- Format: `make fmt` (write) / `make fmt-check` (CI gate)
- Clean: `make clean`

## Repository structure

- `arthas/` — fetches a hardcoded list of Arthas doc links, converts each page to markdown, writes `arthas_docs_md/*.md` in the CWD. Takes no flags.
- `fs/` — searches file contents. Flags: `-f` file-name pattern (default `prod.yml$`), `-s` plain string, `-ss` regex (mutually exclusive with `-s`), `-e` excluded dirs, `-m` module override, `-P` parallelism, `-c` case sensitivity, `-l` context lines; optional positional search path.
- `gitu/` — walks the CWD and updates a branch in every git repo found. Flags: `-b` branch (default `master`), `-p` parallelism.
- `Makefile` — build/test/vet/fmt entry points referenced by CI.
- `dist/` — build output (git-ignored).

## CI

GitHub Actions: `.github/workflows/go.yml`.

- Triggers: push to `main`, PR to `main`, release created
- `verify` job (ubuntu-latest, Go `1.27.x`): `make fmt-check`, `make vet`, `make test`
- `build` job (after `verify`): cross-compiles all three tools for `linux|windows|darwin` × `amd64|arm64` with `-trimpath -ldflags="-s -w"`; artifact name `gobin_<os>_<arch>`, binaries `gobin_<tool>_<os>_<arch>[.exe]`
- `release` job (release events only): merges artifacts and attaches them to the GitHub Release (`softprops/action-gh-release@v2`)

## Conventions

- One utility per top-level directory, each an independent `package main` with a single `main.go`.
- CLI config is grouped into a `Config` struct; flags parsed with the standard `flag` package.
- Concurrency via `sync.WaitGroup` + a buffered semaphore channel sized by the `-p`/`-P` flag.
- Code must be `gofmt`-clean (CI fails otherwise); comments are mixed Chinese/English.

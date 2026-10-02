# gobin

A collection of standalone Go CLI utilities, one `main` package per directory.

## Build

```sh
make build          # -> dist/arthas, dist/fs, dist/gitu
make test vet       # go test ./... / go vet ./...
```

Requires Go 1.27+.

## Tools

### 1. arthas

Fetch every page linked from a hardcoded Arthas command list and convert it to
markdown under `arthas_docs_md/`. No flags.

```sh
go run ./arthas
```

### 2. fs

Search the content of files with specified file types.

```sh
go run ./fs -f '\.go$' -s 'TODO' -e vendor -l 2 ./path
```

Flags: `-f` file pattern, `-s` string pattern / `-ss` regex pattern,
`-e` excluded dirs, `-m` module override, `-P` parallelism, `-c` case sensitive,
`-l` context lines.

### 3. gitu

Parallelly update all specified branches of the projects in the current directory.

```sh
go run ./gitu -b master -p 8
```

# gobin — build/test helpers for the standalone CLI utilities.

GO    ?= go
TOOLS ?= arthas fs gitu
DIST  ?= dist

.PHONY: all build test vet fmt fmt-check clean

all: build

## build: compile every CLI into ./dist
build:
	@mkdir -p $(DIST)
	@for t in $(TOOLS); do \
		echo "building $(DIST)/$$t"; \
		$(GO) build -trimpath -o $(DIST)/$$t ./$$t || exit 1; \
	done

## test: run the test suite (no tests yet => "no test files")
test:
	$(GO) test ./...

## vet: run go vet
vet:
	$(GO) vet ./...

## fmt: format all Go sources in place
fmt:
	gofmt -w .

## fmt-check: fail if any file is not gofmt-clean
fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "not gofmt-clean:"; echo "$$out"; exit 1; fi

## clean: remove build output
clean:
	rm -rf $(DIST)

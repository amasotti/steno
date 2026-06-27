set shell := ["bash", "-uc"]

default:
    @just --list

fmt:
    @files="$(fd -e go)"; if [ -n "$files" ]; then gofmt -w $files; fi

test:
    GOCACHE=/Users/toni/halb-personal/steno/.cache/go-build GOMODCACHE=/Users/toni/halb-personal/steno/.cache/go-mod go test ./...

build:
    mkdir -p bin
    GOCACHE=/Users/toni/halb-personal/steno/.cache/go-build GOMODCACHE=/Users/toni/halb-personal/steno/.cache/go-mod go build -o bin/steno ./cmd/steno

lint:
    golangci-lint run

vuln:
    GOCACHE=/Users/toni/halb-personal/steno/.cache/go-build GOMODCACHE=/Users/toni/halb-personal/steno/.cache/go-mod govulncheck ./...

ci-local: fmt lint test build

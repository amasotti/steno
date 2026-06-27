set shell := ["bash", "-uc"]

default:
    @just --list

fmt:
    @files="$(fd -e go)"; if [ -n "$files" ]; then gofmt -w $files; fi

test:
    GOCACHE=/Users/toni/halb-personal/steno/.cache/go-build go test ./...

build:
    mkdir -p bin
    GOCACHE=/Users/toni/halb-personal/steno/.cache/go-build go build -o bin/steno ./cmd/steno

lint:
    staticcheck ./...

ci-local: fmt test build

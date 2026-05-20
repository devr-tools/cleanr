GO ?= go
VERSION ?= dev
GOCACHE ?= $(CURDIR)/.gocache

export GOCACHE

.PHONY: fmt fmt-check lint test gofiles check build release deploy clean

fmt:
	$(GO) run ./cmd/cleanr-dev fmt

fmt-check:
	$(GO) run ./cmd/cleanr-dev fmt-check

lint:
	$(GO) run ./cmd/cleanr-dev lint

test:
	$(GO) run ./cmd/cleanr-dev test

gofiles:
	$(GO) run ./cmd/cleanr-dev gofiles

check:
	$(GO) run ./cmd/cleanr-dev check

build:
	$(GO) run ./cmd/cleanr-dev build -output dist/cleanr

release:
	$(GO) run ./cmd/cleanr-dev release -version $(VERSION) -output dist/releases

deploy: release

clean:
	rm -rf dist .gocache

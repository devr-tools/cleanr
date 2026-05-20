GO ?= go
VERSION ?= dev
GOCACHE ?= $(CURDIR)/.gocache
REPORT_FORMAT ?= text
REPORT_PRESET ?= fail
REPORT_INPUT ?=

.DEFAULT_GOAL := menu

export GOCACHE

.PHONY: menu help fmt fmt-check lint test gofiles check build release deploy report clean

menu:
	@printf "\ncleanr make menu\n\n"
	@printf "  make fmt         Format Go files\n"
	@printf "  make fmt-check   Verify Go files are formatted\n"
	@printf "  make lint        Run go vet\n"
	@printf "  make test        Run the Go test suite\n"
	@printf "  make gofiles     Validate and list Go file layout\n"
	@printf "  make check       Run gofiles, fmt-check, lint, and test\n"
	@printf "  make build       Build the cleanr CLI to dist/cleanr\n"
	@printf "  make release     Package release artifacts to dist/releases (use VERSION=...)\n"
	@printf "  make deploy      Alias for make release\n"
	@printf "  make report      Preview the terminal report UI\n"
	@printf "  make clean       Remove dist/ and .gocache/\n"
	@printf "\nVariables:\n"
	@printf "  VERSION=%s\n" "$(VERSION)"
	@printf "  GO=%s\n" "$(GO)"
	@printf "  REPORT_FORMAT=%s\n" "$(REPORT_FORMAT)"
	@printf "  REPORT_PRESET=%s\n" "$(REPORT_PRESET)"
	@printf "  REPORT_INPUT=%s\n" "$(REPORT_INPUT)"
	@printf "  GOCACHE=%s\n\n" "$(GOCACHE)"

help: menu

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

report:
	$(GO) run ./cmd/cleanr-dev report -format $(REPORT_FORMAT) -preset $(REPORT_PRESET) $(if $(REPORT_INPUT),-input $(REPORT_INPUT),)

clean:
	rm -rf dist .gocache

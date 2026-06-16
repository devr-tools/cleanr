GO ?= go
ifeq ($(strip $(GOROOT)),)
else ifeq ($(wildcard $(GOROOT)),)
GO := env -u GOROOT $(GO)
endif
VERSION ?= dev
REPOSITORY ?= devr-tools/cleanr
SOURCE_SHA256 ?=
HOMEBREW_LICENSE ?=
GOCACHE ?= $(CURDIR)/.gocache
REPORT_FORMAT ?= text
REPORT_PRESET ?= fail
REPORT_INPUT ?=
CI_BASE_REF ?=
UI_GOROOT ?= /opt/homebrew/opt/go/libexec
UI_GOPATH ?= $(HOME)/go

.DEFAULT_GOAL := menu

export GOCACHE

DASHBOARD_OUTPUT ?= dist/dashboard/report.html

.PHONY: menu help fmt fmt-check lint test test-dashboard dashboard test-review-ui preview-review-ui gofiles check ci commit build release homebrew-formula deploy report clean

menu:
	@printf "\ncleanr make menu\n\n"
	@printf "  make fmt         Format Go files\n"
	@printf "  make fmt-check   Verify Go files are formatted\n"
	@printf "  make lint        Run go vet\n"
	@printf "  make test        Run the Go test suite\n"
	@printf "  make test-dashboard  Run focused HTML dashboard tests\n"
	@printf "  make dashboard   Render a sample HTML dashboard to dist/dashboard/\n"
	@printf "  make test-review-ui  Run focused interactive dataset review tests\n"
	@printf "  make preview-review-ui  Launch the live interactive dataset review preview\n"
	@printf "  make gofiles     Validate and list Go file layout\n"
	@printf "  make check       Run gofiles, fmt-check, lint, and test\n"
	@printf "  make ci          Run the local CI parity gate used before commit\n"
	@printf "  make commit      Stage, commit, and push the current branch with a conventional commit message\n"
	@printf "  make build       Build the cleanr CLI to dist/cleanr\n"
	@printf "  make release     Package release artifacts to dist/releases (use VERSION=...)\n"
	@printf "  make homebrew-formula  Generate a Homebrew formula for a release (use VERSION=...)\n"
	@printf "  make deploy      Alias for make release\n"
	@printf "  make report      Preview the terminal report UI\n"
	@printf "  make clean       Remove dist/ and .gocache/\n"
	@printf "\nVariables:\n"
	@printf "  VERSION=%s\n" "$(VERSION)"
	@printf "  GO=%s\n" "$(GO)"
	@printf "  SOURCE_SHA256=%s\n" "$(SOURCE_SHA256)"
	@printf "  HOMEBREW_LICENSE=%s\n" "$(HOMEBREW_LICENSE)"
	@printf "  REPORT_FORMAT=%s\n" "$(REPORT_FORMAT)"
	@printf "  REPORT_PRESET=%s\n" "$(REPORT_PRESET)"
	@printf "  REPORT_INPUT=%s\n" "$(REPORT_INPUT)"
	@printf "  CI_BASE_REF=%s\n" "$(CI_BASE_REF)"
	@printf "  GOCACHE=%s\n" "$(GOCACHE)"
	@printf "  UI_GOROOT=%s\n" "$(UI_GOROOT)"
	@printf "  UI_GOPATH=%s\n\n" "$(UI_GOPATH)"

help: menu

fmt:
	$(GO) run ./cmd/cleanr-dev fmt

fmt-check:
	$(GO) run ./cmd/cleanr-dev fmt-check

lint:
	$(GO) run ./cmd/cleanr-dev lint

test:
	$(GO) run ./cmd/cleanr-dev test

test-dashboard:
	env -u GOROOT GOCACHE=$(GOCACHE) go test ./tests/report ./tests/cli -run 'Test(WriteReportSupportsAllFormats|ReportPackageSupportsPlainAndColorText|CLIRunWritesHTMLReport|TrendsCommandWritesHTMLSummary)$$'

dashboard:
	mkdir -p $(dir $(DASHBOARD_OUTPUT))
	$(GO) run ./cmd/cleanr-dev report -format html -preset $(REPORT_PRESET) -output $(DASHBOARD_OUTPUT)

test-review-ui:
	@GOROOT=$(UI_GOROOT) GOPATH=$(UI_GOPATH) GOCACHE=$(GOCACHE) go run ./cmd/cleanr-dev test-review-ui

preview-review-ui:
	@GOROOT=$(UI_GOROOT) GOPATH=$(UI_GOPATH) GOCACHE=$(GOCACHE) go run ./cmd/cleanr-dev preview-review-ui

gofiles:
	$(GO) run ./cmd/cleanr-dev gofiles

check:
	$(GO) run ./cmd/cleanr-dev check

ci:
	$(GO) run ./cmd/cleanr-dev ci $(if $(CI_BASE_REF),-base-ref $(CI_BASE_REF),)

commit:
	@./scripts/commit.sh

build:
	$(GO) run ./cmd/cleanr-dev build -output dist/cleanr

release:
	$(GO) run ./cmd/cleanr-dev release -version $(VERSION) -output dist/releases

homebrew-formula:
	$(GO) run ./cmd/cleanr-dev homebrew-formula -version $(VERSION) -repository $(REPOSITORY) -source-sha256 $(SOURCE_SHA256) $(if $(HOMEBREW_LICENSE),-license $(HOMEBREW_LICENSE),) -output dist/releases/$(VERSION)/cleanr.rb

deploy: release

report:
	$(GO) run ./cmd/cleanr-dev report -format $(REPORT_FORMAT) -preset $(REPORT_PRESET) $(if $(REPORT_INPUT),-input $(REPORT_INPUT),)

clean:
	rm -rf dist .gocache

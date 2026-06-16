# Changelog

## Unreleased

### Features

* add native `grpc` target coverage alongside the existing HTTP, GraphQL, CLI, MCP, OpenAI, Anthropic, and `openai_compatible` adapters
* expand `openai_compatible` support with configurable provider labels, auth headers, auth schemes, and compatibility metadata
* add native OpenAPI-driven scenario generation, contract diffing, and HTTP request overrides for REST contract testing
* add transcript-aware request construction, stronger tool-call assertions, mocked tool transcript coverage, and richer streaming/load assertions
* add advanced LLM judge features including calibration against labeled data, ensemble and cascade evaluation, confidence-oriented reporting, and multi-model comparison support
* add adversarial scenario generation mode for red-team dataset authoring
* add LangSmith and OpenLLMetry import paths for replay/trend ingestion
* add `cleanr explain` for replay-backed failure summaries and machine-readable fix suggestions
* add natural-language scenario authoring via `cleanr generate "test that ..."` and expand MCP server lifecycle tools for config validation, dataset review, trend analysis, report rendering, and failure explanation
* add `agent` and `html` report formats plus static HTML trend/dashboard export
* add `cleanr watch` for rerunning suites on file changes during local iteration
* add GitLab-native dotenv and annotations outputs for run and dataset review flows
* add a plugin registry plus WASM plugin execution through the runtime

### Improvements

* expand trend analysis to capture load metrics, replay artifacts, and scenario transcript diffs across retained runs
* improve HTML dashboard readability with a cleaner layout, structured detail rendering, rectangular status pills, and ASCII cleanr branding
* improve MCP runtime reporting so agents can work through run, validate, review, trend, and explain workflows from one server surface
* improve local CI parity by keeping Semgrep runnable through either a direct `semgrep` binary or `python -m semgrep` fallback when available

## [0.8.0](https://github.com/devr-tools/cleanr/compare/v0.7.0...v0.8.0) (2026-06-16)


### Features

* new features ([3f98d83](https://github.com/devr-tools/cleanr/commit/3f98d83e836e9314104e6d7597cbf682f94b9c2d))
* new features ([6f16756](https://github.com/devr-tools/cleanr/commit/6f16756c4b6dd6389975a192ac9bc9cae27512cb))

## [0.7.0](https://github.com/devr-tools/cleanr/compare/v0.6.0...v0.7.0) (2026-06-15)


### Features

* add openai-compatible target and trend reporting ([c060fdb](https://github.com/devr-tools/cleanr/commit/c060fdb5337f000dab5bec23ef8b3efd1fcf54ca))
* feat: add openai-compatible target and trend reporting ([69f6d44](https://github.com/devr-tools/cleanr/commit/69f6d44d06bd1926c40b53dc1722ceaa32539eb7))

## [0.6.0](https://github.com/devr-tools/cleanr/compare/v0.5.0...v0.6.0) (2026-06-10)


### Features

* added llm judge ([91d16e8](https://github.com/devr-tools/cleanr/commit/91d16e8b8cfdc267e3ac418d577eb92dd46bf6d6))

## [0.5.0](https://github.com/devr-tools/cleanr/compare/v0.4.0...v0.5.0) (2026-06-04)


### Features

* added github output and doctor ([2766e14](https://github.com/devr-tools/cleanr/commit/2766e14738225f785f0c49ed208e33d245a082da))

## [0.4.0](https://github.com/devr-tools/cleanr/compare/v0.3.0...v0.4.0) (2026-05-28)


### Features

* braintrust integration ([d3076cb](https://github.com/devr-tools/cleanr/commit/d3076cbce487039c899677b412e810e2f0225bd3))

## [0.3.0](https://github.com/devr-tools/cleanr/compare/v0.2.0...v0.3.0) (2026-05-27)


### Features

* **feat: repo folder for cleanr docs:** repo folder for cleanr docs ([cdb351b](https://github.com/devr-tools/cleanr/commit/cdb351bed9c855483e7a407c92923e5b856c047e))
* repo folder for cleanr docs ([e4528c3](https://github.com/devr-tools/cleanr/commit/e4528c36fded95c70cbb73957ca4c686a3512fb4))

## [0.2.0](https://github.com/devr-tools/cleanr/compare/v0.1.0...v0.2.0) (2026-05-27)


### Features

* feat:  ([815dc77](https://github.com/devr-tools/cleanr/commit/815dc77fcd8f82b17bafa82de8ce2dba5c5640b9))
* 1 ([556b679](https://github.com/devr-tools/cleanr/commit/556b679231f6d2649af37223338c756ebd1fdb71))
* inital pre release ([bd69a48](https://github.com/devr-tools/cleanr/commit/bd69a4866019e949416898c76a0daa5aa3ac4826))
* push v0 ([08ff556](https://github.com/devr-tools/cleanr/commit/08ff5560afee9ad6a9ac75d8f2029aec0e2c854c))
* v0 ([73ff044](https://github.com/devr-tools/cleanr/commit/73ff044e8430e0b5746c3293fd92caa1e4286256))

## Changelog

## Changelog

All notable changes to this project will be documented in this file.

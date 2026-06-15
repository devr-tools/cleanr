# Expansion Roadmap

## Phase 1: Coverage Foundation

- Ship `openai_compatible` as a first-class target for OpenAI-style endpoints with configurable provider label, auth header, auth scheme, and quirks profile.
- Preserve the existing native `openai` and `anthropic` adapters as strict provider implementations.
- Expose structured tool-call arguments in normalized responses so assertions can validate agent behavior, not just plain text.

## Phase 2: Conversation and Agent Loops

- Add transcript-aware scenarios with first-class turns instead of only `system` plus `input`.
- Support mocked tool results as transcript fixtures so agent loops can be tested hermetically.
- Add first-class tool-call expectations for name, order, and schema-valid arguments.

## Phase 3: Streaming and Multimodal

- Add streaming-aware target execution and assertions for time-to-first-token, chunk cadence, truncated streams, and malformed SSE recovery.
- Extend scenarios to cover image, audio, and PDF inputs.
- Add judge hooks for multimodal outputs where response text alone is insufficient.

## Phase 4: Provider and Platform Breadth

- Add native adapters where OpenAI-compatibility is not enough: Azure OpenAI, Bedrock, Vertex, and Gemini.
- Add vendor profiles on top of `openai_compatible` for local-model and proxy ecosystems such as Ollama, llama.cpp, vLLM, and Mistral-compatible endpoints.

## Phase 5: MCP as a Target

- Add `target.type: mcp` so cleanr can call MCP tools directly instead of only serving them.
- Reuse the existing suites and assertions against MCP tool behavior, latency, schema validity, and error handling.
- Add injection-resistance scenarios around MCP tool invocation and result handling.

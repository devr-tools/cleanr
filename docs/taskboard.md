# Taskboard

## In Flight

- [x] Add `openai_compatible` target plumbing across config, adapter dispatch, metadata, and MCP target catalog.
- [x] Expose structured tool-call arguments in normalized responses and assertion paths.
- [ ] Add docs and examples for `openai_compatible`.

## Next

- [ ] Add transcript-aware scenarios with ordered turns and legacy single-turn compatibility.
- [ ] Add first-class tool-call expectations for name, order, and argument validation.
- [ ] Add mocked tool-result fixtures for hermetic agent-loop tests.
- [ ] Add `target.type: mcp` for direct MCP tool invocation.

## Later

- [ ] Add streaming capture and assertions for TTFT, chunk cadence, and malformed SSE recovery.
- [ ] Add multimodal scenario inputs and output judges.
- [ ] Add native Azure OpenAI, Bedrock, Vertex, and Gemini adapters.
- [ ] Add provider aliases and quirks profiles for Ollama, llama.cpp, vLLM, and similar OpenAI-compatible targets.

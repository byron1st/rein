# Rein

Rein is a minimal Go CLI agent core that runs a synchronous OpenAI-compatible chat completions loop and dispatches local tools through a small registry.

## Key Requirements

- Minimal Agent Loop: Keep state as append-only messages and let model tool calls drive control flow.
- Local CLI First: Build a single binary with no server, daemon, or background process.
- Tool Failures Stay in Loop: Return recoverable tool errors as tool-message content instead of aborting the session.
- Bounded Tool Output: Cap tool output around 50KB or 2000 lines and offload oversized content to a temp file.
- Minimal Dependencies: Prefer the standard library over external ones.

## Core Commands

- `make build`: Build `bin/rein`.
- `make test`: Run the full Go test suite with race detection and coverage.
- `make test-single PKG=./pkg/agent TEST=TestName`: Run one test by name.
- `make lint`: Check formatting and run `go vet`.
- `make lint-fix`: Format Go files.
- `make run`: Run the CLI entrypoint.
- `make tidy`: Clean up Go module files.
- `make clean`: Remove generated build and coverage artifacts.
- `make help`: List available Makefile targets.

## Architecture Overview

- `cmd/rein/`: CLI entrypoint, environment loading, and session start.
- `pkg/agent/`: Agentic loop, append-only message session, turn control, and max-turn handling.
- `pkg/llm/`: OpenAI-compatible `/v1/chat/completions` client using `net/http` and JSON types.
- `pkg/tool/`: Tool interface, registry, dispatch, and built-in filesystem, shell, and search tools.
- `docs/`: SPEC, Go conventions, and long-form project references.

## Code Conventions

- Keep core provider integration on `net/http` and `encoding/json` unless the SPEC changes.
- Wrap errors with operation context using `fmt.Errorf("read_file %s: %w", path, err)`.
- Keep tool schemas owned by each tool through its `Schema()` method.
- Execute multiple tool calls sequentially in response order.
- See [docs/go-conventions.md](docs/go-conventions.md) for the full Go rules.

## Testing

Use the standard Go `testing` package with table-driven tests and `t.Run` subtests. Keep tests next to the code they verify, use `httptest` or hand-written fakes at boundaries, and run `make test` before committing behavior changes.

## Boundaries

- NEVER add LangChain or other provider frameworks for A.1/A.2; use the OpenAI-compatible HTTP API directly.
- NEVER move reusable packages under `internal/`; keep the SPEC-directed importable packages under `pkg/`.
- NEVER terminate the loop for recoverable tool failures; return the error string in the tool message.
- NEVER make tool output unbounded; cap it and offload large output to a temp file.
- NEVER add prompt assembly, context compaction, GUI, or policy hooks in this scope; leave them for later SPECs.

## References

- [docs/SPEC.md](docs/SPEC.md)
- [docs/go-conventions.md](docs/go-conventions.md)

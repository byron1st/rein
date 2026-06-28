# Go Conventions

Project-local Go conventions for this repository. The agent file (`CLAUDE.md` / `AGENTS.md`) links here so an AI coding agent loads only the prescriptive rules and not the rationale.

## Project Layout

- `cmd/{binary}/` — `main` packages, one subdirectory per binary.
- `internal/` — application code that must not be imported from outside this module.
- `pkg/` — only when something is genuinely intended for external import. Default to `internal/`.
- `docs/` — SPEC.md, conventions, and any long-form references the agent file links to.
- Tests live next to the code they verify (`foo.go` ↔ `foo_test.go`).

## Errors

- Declare every error at the top of the file where it is used as an exported sentinel: `var ErrFailedToUpsertSchools = errors.New("failed to upsert schools")`.
- Name a sentinel after the failure, not the location: the message must read as "an error occurred" — use `failed to <verb> <object>` or a phrase naming the concrete condition (`ErrBadRequestToLLMStatus`, `ErrLLMContextCanceled`). A bare operation name used only to mark where an error passed through (`ErrUpsertSchools`, `ErrLLMTransport`) is prohibited.
- Each `Err...` sentinel must be used in **exactly one** call site. Sharing one sentinel across multiple call sites is prohibited — declare a new variable for each use.
- Wrap with a sentinel **only at the boundary with a standard-library or third-party function**, and only once: `errors.Join(ErrFailedToDecodeLLMResponse, err)`. That join is the single point where the failure gains its sentinel.
- When you originate the error yourself (a condition you detect, rather than an error value handed back to you), build it from the sentinel and add dynamic detail with `fmt.Errorf`: `errors.Join(ErrBadRequestToLLMStatus, fmt.Errorf("status %d", code))`.
- When you propagate an error returned by **another function in this codebase**, return it unchanged — it already carries its sentinel, so re-wrapping only nests redundant context. This pass-through is what keeps error wrapping bounded.
- Join a new sentinel onto an already-wrapped error only when adding a genuinely new fact, never to re-mark a passthrough — e.g. a retry loop that gives up: `errors.Join(ErrLLMRetriesExhausted, lastErr)`.

## Testing

- Use the standard `testing` package and table-driven tests for branching logic.
- Use `t.Run` for subtests so failures point at the specific case.
- Prefer `testify/require` for assertions when external dependencies are available; otherwise stick to standard `t.Errorf` / `t.Fatalf`.
- Mock at interface boundaries. Generate mocks with `mockery` or hand-roll small fakes — choose one and stay consistent within the project.
- Each test should fail with a message that explains *what* was wrong, not just that something was wrong: `"expected user_id %d, got %d"`.
- Write tests only in the external `{PACKAGE_NAME}_test` package, so tests cover exported functions/methods only.
  - Do not place test files in the same package as the implementation. For example, `internal/server/server_test.go` must declare `package server_test`, not `package server`.

## Linting and Formatting

- `gofmt` is non-negotiable. Files that do not pass `gofmt` are broken.
- Run `golangci-lint run ./...` before every commit. The CI gate uses the same command.
- Imports follow `goimports` ordering: standard library, third-party, then internal — separated by blank lines.

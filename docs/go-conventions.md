# Go Conventions

Project-local Go conventions for this repository. The agent file (`CLAUDE.md` / `AGENTS.md`) links here so an AI coding agent loads only the prescriptive rules and not the rationale.

## Project Layout

- `cmd/{binary}/` — `main` packages, one subdirectory per binary.
- `internal/` — application code that must not be imported from outside this module.
- `pkg/` — only when something is genuinely intended for external import. Default to `internal/`.
- `docs/` — SPEC.md, conventions, and any long-form references the agent file links to.
- Tests live next to the code they verify (`foo.go` ↔ `foo_test.go`).

## Errors

- Declare every error at the top of the file where it is used as an exported sentinel: `var ErrUpsertSchools = errors.New("upsert schools")`.
- Each `Err...` sentinel must be used in **exactly one** call site. Sharing one sentinel across multiple call sites is prohibited — declare a new variable for each use.
- Prefer to Wrap errors by joining the sentinel with the underlying error: `errors.Join(ErrUpsertSchools, err)`.

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

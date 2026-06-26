---
Application: rein
JiraTicket: NO-JIRA
PlanType: multi-steps-sub
Timestamp: 20260626142129
Title: agent-core
Step: 4
---

# Step 4: exec+search 도구 (`bash` / `grep` / `glob`)

Part of main plan: [20260626142129_NO-JIRA_PLAN_agent-core.md](./20260626142129_NO-JIRA_PLAN_agent-core.md)

## Goal

서브프로세스를 spawn하는 도구 3종을 `Tool` 인터페이스로 구현한다. `bash`는 셸 실행, `grep`/`glob`은 각각 `rg`/`fd`를 절대경로로 resolve해 래핑한다. 출력은 `capOutput`을 적용하고 실패는 에러 문자열로 반환한다.

## Implements

- FR-2 (partial): `bash`, `grep`(rg), `glob`(fd). cwd 고정·timeout·절대경로 resolve·미설치 안내 규칙 포함.

## Depends On

Step 2 (`Tool` 인터페이스, `capOutput`).

## Tasks

- [ ] `pkg/tool/exec.go` — `bash`: args `{command string, timeout *int(seconds)}`. `exec.CommandContext(ctx, "bash", "-c", command)` 사용, `cmd.Dir = projectRoot`(생성자 `NewBashTool(projectRoot string)`로 주입), 기본 timeout **300s**(`context.WithTimeout`). stdout+stderr 결합 + exit code를 합쳐 문자열로 반환하고 `capOutput` 적용. timeout 초과 시 그 사실을 결과 문자열에 명시. (위험 커맨드 차단 등 정책 강제는 B.6로 위임 — 이번 범위 밖.)
- [ ] `pkg/tool/search.go` — `grep`: args `{pattern string, path *string, glob *string}`. 바이너리명(`"rg"`)을 필드로 보관하고 `exec.LookPath`로 절대경로 resolve. 미설치 시 명확한 설치 안내 sentinel 에러. `rg <pattern> [path] [--glob <glob>]` 호출, no-match(exit 1)는 에러가 아닌 빈/안내 결과로 처리, 출력 `capOutput`. 테스트 주입을 위해 생성자에서 바이너리명을 받을 수 있게 한다(기본 `"rg"`).
- [ ] `pkg/tool/search.go` — `glob`: args `{pattern string, path *string}`. 바이너리명(`"fd"`) 절대경로 resolve, 미설치 안내 sentinel 에러, `fd <pattern> [path]` 호출, 출력 `capOutput`. 생성자에서 바이너리명 주입 가능(기본 `"fd"`).
- [ ] 각 도구 생성자 제공(`NewBashTool(projectRoot)`, `NewGrepTool(...)`, `NewGlobTool(...)`) — Step 6에서 registry 등록.
- [ ] 에러 컨벤션 — sentinel + `errors.Join`, 각 call site 별도 sentinel(`ErrBashRun`, `ErrRgNotFound`, `ErrRgRun`, `ErrFdNotFound`, `ErrFdRun` 등).

## Affected Files

| Action | Path | Description |
|--------|------|-------------|
| Create | `pkg/tool/exec.go` | `bash` 도구(cwd 고정·timeout) |
| Create | `pkg/tool/search.go` | `grep`(rg)·`glob`(fd) 도구 |
| Create | `pkg/tool/exec_test.go` | bash 테스트(`package tool_test`) |
| Create | `pkg/tool/search_test.go` | grep/glob 테스트(`package tool_test`) |

## Tests

- bash: `echo` 명령 stdout/exit 코드 검증, `pwd`로 cwd=projectRoot 검증, 짧은 timeout으로 타임아웃 동작 검증, 대용량 출력 cap 적용.
- grep: `t.TempDir`에 파일 생성 후 패턴 매칭 결과 검증, no-match 시 에러 아님. 미설치 분기는 존재하지 않는 바이너리명을 주입해 설치 안내 에러 검증. 실제 `rg` 없으면 happy path는 `exec.LookPath`로 `t.Skip`.
- glob: `t.TempDir`에 파일 생성 후 패턴으로 경로 반환 검증. 미설치 분기는 가짜 바이너리명 주입으로 검증. `fd` 없으면 happy path `t.Skip`.

## Build Verification

```bash
go build ./... && go test ./... && go vet ./...
gofmt -l .
# 또는: make build && make test && make lint
```

## Completion Checklist

- [ ] All tasks completed
- [ ] All tests written and passing
- [ ] Build verification passes
- [ ] No regressions from previous steps
- [ ] 절대경로 resolve·미설치 안내·cap·에러 컨벤션 준수

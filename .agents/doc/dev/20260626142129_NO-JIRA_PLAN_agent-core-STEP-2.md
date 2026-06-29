---
Application: rein
JiraTicket: NO-JIRA
PlanType: multi-steps-sub
Timestamp: 20260626142129
Title: agent-core
Step: 2
---

# Step 2: `pkg/tool` 엔진 + 출력 cap 헬퍼

Report: [20260626142129_NO-JIRA_IMPL_agent-core-STEP-2.md](./20260626142129_NO-JIRA_IMPL_agent-core-STEP-2.md)

Part of main plan: [20260626142129_NO-JIRA_PLAN_agent-core.md](./20260626142129_NO-JIRA_PLAN_agent-core.md)

## Goal

도구를 `name + JSON schema + handler`로 등록·dispatch하는 엔진과, 모든 도구가 공유할 출력 cap/temp 오프로드 헬퍼를 만든다. 이후 모든 도구(Step 3·4)와 루프(Step 5)의 기반이 된다.

## Implements

- FR-2 (partial): `Tool` 인터페이스, `Registry`(등록/dispatch), 출력 캡 + temp-file 오프로드. 개별 도구는 Step 3·4.

## Depends On

None.

## Tasks

- [x] `pkg/tool/tool.go` — `Tool` 인터페이스 정의: `Name() string` / `Schema() json.RawMessage` / `Execute(ctx context.Context, args json.RawMessage) (string, error)`. `Schema()`는 OpenAI function 객체 inner(`{"name","description","parameters"}`)를 반환한다는 주석 명시.
- [x] `pkg/tool/registry.go` — `Registry`(내부 `map[string]Tool`): `NewRegistry()`, `Register(t Tool)`, `Get(name string) (Tool, bool)`, `List() []Tool`(정의 순서/정렬), `Dispatch(ctx context.Context, name string, args json.RawMessage) (string, error)`. 미등록 name → sentinel 에러(`ErrUnknownTool`)로 반환(호출부가 tool 메시지로 흡수). dispatch는 해당 Tool의 `Execute` 결과를 그대로 반환.
- [x] `pkg/tool/output.go` — `capOutput(s string) (string, error)` 헬퍼: 50KB(`const maxBytes = 50 * 1024`) 또는 2000라인(`const maxLines = 2000`) 초과 시 절단하고, 전체 원본을 `os.MkdirTemp("", "rein-*")` 하위 파일에 기록한 뒤 절단본 말미에 안내 주석(예: `\n[output truncated: N bytes/M lines. full output: <path>]`) 추가. 미초과 시 원본 그대로 반환. 오프로드 실패는 sentinel + `errors.Join`로 래핑.
- [x] 에러 컨벤션 — sentinel + `errors.Join`, 각 sentinel 1회 사용(`ErrUnknownTool`, `ErrOffloadCreate`, `ErrOffloadWrite` 등 분리).

## Affected Files

| Action | Path | Description |
|--------|------|-------------|
| Create | `pkg/tool/tool.go` | `Tool` 인터페이스 |
| Create | `pkg/tool/registry.go` | `Registry` + dispatch |
| Create | `pkg/tool/output.go` | 출력 cap + temp-file 오프로드 헬퍼 |
| Create | `pkg/tool/registry_test.go` | registry 테스트(`package tool_test`) |
| Create | `pkg/tool/output_test.go` | cap/오프로드 테스트(`package tool_test`) |

## Tests

- registry: 등록 후 `Get`/`List`/`Dispatch` happy path(fake Tool로). 미등록 name → `ErrUnknownTool` 검증. dispatch가 Tool 에러를 그대로 전파하는지 검증.
- cap: 상한 미만 입력 → 그대로 반환(오프로드 파일 없음). 바이트 초과 입력 → 절단 + 안내 주석 + temp 파일 존재 + 파일 내용이 원본 전체와 일치. 라인 초과 입력 → 동일 검증. 안내 주석에서 경로 파싱해 파일 읽어 비교.
- 경계값: 정확히 maxBytes/maxLines일 때 절단 여부 일관성 확인.

## Build Verification

```bash
go build ./... && go test ./... && go vet ./...
gofmt -l .
# 또는: make build && make test && make lint
```

## Completion Checklist

- [x] All tasks completed
- [x] All tests written and passing
- [x] Build verification passes
- [x] No regressions from previous steps
- [x] 에러 컨벤션 준수

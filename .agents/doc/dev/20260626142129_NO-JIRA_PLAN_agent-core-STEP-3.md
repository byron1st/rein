---
Application: rein
JiraTicket: NO-JIRA
PlanType: multi-steps-sub
Timestamp: 20260626142129
Title: agent-core
Step: 3
---

# Step 3: fs 도구 (`read_file` / `write_file` / `edit_file`)

Part of main plan: [20260626142129_NO-JIRA_PLAN_agent-core.md](./20260626142129_NO-JIRA_PLAN_agent-core.md)

## Goal

파일 시스템 도구 3종을 `Tool` 인터페이스로 구현한다. 각 도구는 자체 OpenAI function schema를 제공하고, 출력은 Step 2의 `capOutput`을 적용하며, 실패는 에러 문자열로 반환해 루프가 자기수정하게 한다.

## Implements

- FR-2 (partial): `read_file`, `write_file`, `edit_file`. `edit_file`의 unique-match 비즈니스 규칙 포함.

## Depends On

Step 2 (`Tool` 인터페이스, `capOutput`).

## Tasks

- [ ] `pkg/tool/fs.go` — `read_file`: args `{path string, offset *int, limit *int}`. 파일 읽기, offset/limit 지정 시 **라인 단위** 슬라이스(offset=시작 라인, limit=라인 수). 결과에 `capOutput` 적용. 파일 없음/권한 거부 → sentinel 에러. `Schema()`는 path 필수, offset/limit 선택으로 기술.
- [ ] `pkg/tool/fs.go` — `write_file`: args `{path string, content string}`. `os.WriteFile`로 생성/덮어쓰기. 성공 시 요약 문자열(예: `wrote N bytes to <path>`). 쓰기 실패 → sentinel 에러.
- [ ] `pkg/tool/fs.go` — `edit_file`: args `{path string, old_str string, new_str string}`. 파일 읽고 `old_str` 출현 횟수 카운트: **정확히 1회**일 때만 치환 후 기록. 0회 → sentinel 에러(미치환). 2회 이상 → `"N matches found; provide more surrounding context"` 의미의 sentinel 에러(미치환). 읽기/쓰기 실패도 각각 별도 sentinel.
- [ ] 각 도구를 생성하는 생성자(`NewReadFileTool()` 등) 제공 — Step 6에서 registry에 등록.
- [ ] 에러 컨벤션 — sentinel + `errors.Join`, 각 call site마다 별도 sentinel(`ErrReadFileOpen`, `ErrWriteFile`, `ErrEditFileRead`, `ErrEditFileNoMatch`, `ErrEditFileMultiMatch`, `ErrEditFileWrite` 등).

## Affected Files

| Action | Path | Description |
|--------|------|-------------|
| Create | `pkg/tool/fs.go` | `read_file`/`write_file`/`edit_file` 도구 |
| Create | `pkg/tool/fs_test.go` | fs 도구 테스트(`package tool_test`, `t.TempDir`) |

## Tests

- `t.TempDir` 기반.
- read_file: 기본 읽기, offset/limit 슬라이스, 없는 파일 → 에러 문자열, 대용량 파일에서 cap 적용(절단 + temp 경로 안내).
- write_file: 신규 생성, 기존 덮어쓰기, 내용 검증.
- edit_file: 유일 매칭 치환 성공(파일 내용 검증), 0매칭 에러(파일 불변), 다중 매칭 → `"N matches found..."` 에러(파일 불변).
- 인자 JSON 파싱 실패(잘못된 args) → `Execute`가 에러 반환(루프가 흡수할 형태).

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
- [ ] 출력 cap·에러 컨벤션 준수

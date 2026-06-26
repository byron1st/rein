---
Application: rein
JiraTicket: NO-JIRA
PlanType: multi-steps-sub
Timestamp: 20260626142129
Title: agent-core
Step: 6
---

# Step 6: `cmd/rein` 와이어링 (단발 실행)

Part of main plan: [20260626142129_NO-JIRA_PLAN_agent-core.md](./20260626142129_NO-JIRA_PLAN_agent-core.md)

## Goal

지금까지의 패키지를 엮어 동작하는 단일 바이너리 CLI를 완성한다. env를 로딩하고 registry에 6개 도구를 등록한 뒤, 프롬프트 1개를 받아 루프를 끝까지 돌리고 최종 assistant content를 출력하는 **단발 실행** 진입점이다.

## Implements

- FR-1·FR-2 통합: 전체 와이어링 + 단발 실행 진입점. (개별 FR 로직은 Step 1–5에서 구현됨.)

## Depends On

Step 1, Step 2, Step 3, Step 4, Step 5 (전체).

## Tasks

- [ ] `cmd/rein/main.go` — env 로딩: `OPENAI_BASE_URL`, `OPENAI_API_KEY`, `OPENAI_MODEL`을 `os.Getenv`로 읽는다. 필수값(base_url, model) 누락 시 명확한 에러 출력 후 비정상 종료.
- [ ] 프롬프트 입력 — `os.Args[1:]`가 있으면 join해서 사용, 없으면 stdin 전체를 읽어 사용(단발 실행).
- [ ] projectRoot 결정 — `os.Getwd()`. `bash` 도구에 주입.
- [ ] registry 부팅 — `tool.NewRegistry()`에 6개 도구 등록(`read_file`/`write_file`/`edit_file`/`bash`(projectRoot)/`grep`/`glob`).
- [ ] client·agent 구성 — `llm.New(baseURL, apiKey)` + `agent.New(client, registry, model)`.
- [ ] 시드 메시지 — 최소 `system`(A.3 범위 밖이라 한 줄 수준 상수) + 초기 `user`(프롬프트). `agent.Run`에 전달.
- [ ] 출력 — 최종 메시지들 중 마지막 assistant content를 stdout으로 출력. `Run`이 error 반환 시 stderr 로그 + `os.Exit(1)`.
- [ ] slog 핸들러 — stderr로 구조적 로그 출력하도록 `slog.SetDefault` 또는 logger 주입.
- [ ] 테스트 가능성 — env 파싱·프롬프트 결정(args vs stdin)·최종 content 추출을 얇은 함수로 분리해 단위 테스트. `main` 자체는 얇게 유지.
- [ ] 에러 컨벤션 — sentinel + `errors.Join`(예: `ErrMissingBaseURL`, `ErrMissingModel`, `ErrReadStdin` 등 각 1회).

## Affected Files

| Action | Path | Description |
|--------|------|-------------|
| Modify | `cmd/rein/main.go` | 빈 `main()`를 단발 실행 진입점으로 구현 |
| Create | `cmd/rein/main_test.go` | env 파싱·프롬프트 결정·content 추출 헬퍼 테스트(`package main`로 비공개 헬퍼 검증하거나 헬퍼를 별도 파일/함수로 노출) |

## Tests

- env 파싱 헬퍼: 필수값 누락/정상 케이스.
- 프롬프트 결정 헬퍼: args 우선, args 없을 때 stdin 사용.
- 최종 content 추출 헬퍼: 메시지 슬라이스에서 마지막 assistant content 반환.
- (선택) 통합 스모크: fake/로컬 Ollama 대상 수동 실행으로 end-to-end 확인.

## Build Verification

```bash
go build ./... && go test ./... && go vet ./...
gofmt -l .
make build      # bin/rein 산출 확인

# 수동 실행 예(로컬 Ollama 등 OpenAI 호환 엔드포인트 필요):
export OPENAI_BASE_URL=http://localhost:11434
export OPENAI_MODEL=<model>
# export OPENAI_API_KEY=...   # 필요 시
./bin/rein "이 저장소 구조를 요약해줘"
```

## Completion Checklist

- [ ] All tasks completed
- [ ] All tests written and passing
- [ ] Build verification passes (`make build`로 `bin/rein` 생성)
- [ ] No regressions from previous steps
- [ ] 단발 실행·env 로딩·6도구 등록·stderr slog·에러 컨벤션 준수

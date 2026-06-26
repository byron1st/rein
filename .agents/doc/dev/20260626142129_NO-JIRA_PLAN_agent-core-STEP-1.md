---
Application: rein
JiraTicket: NO-JIRA
PlanType: multi-steps-sub
Timestamp: 20260626142129
Title: agent-core
Step: 1
---

# Step 1: `pkg/llm` OpenAI 호환 클라이언트

Part of main plan: [20260626142129_NO-JIRA_PLAN_agent-core.md](./20260626142129_NO-JIRA_PLAN_agent-core.md)

Report: [20260626142129_NO-JIRA_IMPL_agent-core-STEP-1.md](./20260626142129_NO-JIRA_IMPL_agent-core-STEP-1.md)

## Goal

OpenAI Chat Completions 호환 엔드포인트와 통신하는 클라이언트를 `net/http`로 구현한다. 루프가 의존할 `Client` 인터페이스와 req/resp·tool_calls 타입을 정의하고, 네트워크/일시 오류에 대한 지수 백오프 5회 재시도를 포함한다. 인터페이스로 추상화해 `httptest`/fake로 결정적 테스트가 가능하게 한다.

## Implements

- FR-1 (partial): LLM 왕복(POST /chat/completions, non-streaming)과 네트워크 오류 재시도 정책. 루프 제어 흐름 자체는 Step 5.

## Depends On

None.

## Tasks

- [x] `pkg/llm/types.go` — OpenAI 호환 JSON 타입 정의: `Message{Role, Content, ToolCalls, ToolCallID, Name}`(assistant tool_call 메시지의 `content:null` 표현을 위해 `Content`는 `omitempty` 또는 `*string`로; tool_calls 송신 시 `omitempty`), `ToolCall{ID, Type, Function}`, `FunctionCall{Name, Arguments string}`(arguments는 JSON 인코딩된 문자열), `ToolDefinition{Type, Function json.RawMessage}`, `ChatCompletionRequest{Model, Messages, Tools []ToolDefinition (omitempty)}`, `ChatCompletionResponse{Choices}`, `Choice{Message, FinishReason}`.
- [x] `pkg/llm/client.go` — `Client` 인터페이스: `CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)`.
- [x] `pkg/llm/client.go` — `httpClient` 구현: `baseURL`, `apiKey`, `*http.Client` 보관. `New(baseURL, apiKey string, opts ...) Client`. 요청 시 `POST {baseURL}/v1/chat/completions`, `Authorization: Bearer {apiKey}`(apiKey 비어있으면 헤더 생략 — 로컬 Ollama 대응), `Content-Type: application/json`. body는 `encoding/json`으로 마샬.
- [x] 재시도 로직 — 네트워크/전송 오류 및 5xx 응답에 대해 지수 백오프로 **최대 5회** 재시도(base 지연 → 2배씩 증가), 소진 시 sentinel 에러 반환. 4xx는 즉시 에러 반환(재시도 안 함). `ctx` 취소를 존중.
- [x] 에러 컨벤션 적용 — `go-conventions.md` 기준 sentinel + `errors.Join`. 각 call site마다 별도 sentinel(예: `ErrLLMRequestBuild`, `ErrLLMTransport`, `ErrLLMDecode`, `ErrLLMStatus`, `ErrLLMRetriesExhausted` 등 — 각 1회만 사용).
- [x] 문서 정합 — `CLAUDE.md:36`, `AGENTS.md:36`, `docs/SPEC.md:59`의 `fmt.Errorf("read_file %s: %w", ...)` 예시를 `go-conventions.md` 기준(sentinel + `errors.Join`)으로 수정. 기존 섹션 구조/항목은 유지하고 해당 문장 내용만 정정.

## Affected Files

| Action | Path | Description |
|--------|------|-------------|
| Create | `pkg/llm/types.go` | OpenAI 호환 req/resp·tool_calls 타입 |
| Create | `pkg/llm/client.go` | `Client` 인터페이스 + `net/http` 구현 + 재시도 |
| Create | `pkg/llm/client_test.go` | `httptest` 기반 클라이언트 테스트(`package llm_test`) |
| Modify | `docs/SPEC.md` | 에러 래핑 예시를 `errors.Join` 기준으로 정정(line 59) |
| Modify | `CLAUDE.md` | 에러 래핑 예시를 `errors.Join` 기준으로 정정(line 36) |
| Modify | `AGENTS.md` | 에러 래핑 예시를 `errors.Join` 기준으로 정정(line 36) |

## Tests

- `package llm_test`, `httptest.NewServer`로 가짜 엔드포인트 구성.
- 정상 응답 파싱: content만 있는 응답 → `Choices[0].Message.Content` 검증.
- tool_calls 파싱: `tool_calls` 배열이 있는 응답 → `ToolCalls`/`FunctionCall.Arguments` 정확히 디코드.
- 요청 바디 검증: 서버 핸들러에서 수신한 body가 `model`/`messages`/`tools`(있을 때)를 올바르게 포함.
- 재시도: 첫 N회 500 → 이후 200 시나리오에서 최종 성공, 호출 횟수 검증.
- 재시도 소진: 계속 500/전송 오류 → 5회 후 에러 반환(에러가 sentinel 포함).
- 4xx: 400 → 재시도 없이 즉시 에러.
- 인증 헤더: apiKey 설정/미설정 각각에서 `Authorization` 헤더 유무 검증.

## Build Verification

```bash
go build ./... && go test ./... && go vet ./...
gofmt -l .   # 출력 없어야 함
# 또는: make build && make test && make lint
```

## Completion Checklist

- [x] All tasks completed
- [x] All tests written and passing
- [x] Build verification passes
- [x] No regressions from previous steps (해당 없음 — 첫 스텝)
- [x] 에러 컨벤션(sentinel + errors.Join) 적용 및 문서 정합 완료

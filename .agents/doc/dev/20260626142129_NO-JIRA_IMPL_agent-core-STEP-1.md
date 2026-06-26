---
Application: rein
JiraTicket: NO-JIRA
ReportType: multi-steps-step
Timestamp: 20260626142129
Title: agent-core
Step: 1
ReviewBase: 작업 트리 변경(main, HEAD 8fcaf71). 신규 파일이 untracked이므로 전체 확인은 `git add -A && git diff --cached HEAD`
---

# Step 1: `pkg/llm` OpenAI 호환 클라이언트

Plan: [20260626142129_NO-JIRA_PLAN_agent-core-STEP-1.md](./20260626142129_NO-JIRA_PLAN_agent-core-STEP-1.md)

## Summary

OpenAI Chat Completions 호환 엔드포인트와 통신하는 `pkg/llm` 클라이언트를 `net/http`로 구현했다. 루프가 의존할 `Client` 인터페이스와 req/resp·tool_calls 타입을 정의하고, 전송 오류·5xx에 대해 지수 백오프 5회 재시도(총 6회 시도)·4xx 즉시 실패·ctx 취소 존중을 포함한다. 사용자 지시에 따라 HTTP 왕복을 `HTTPDoer` 인터페이스로 분리하고 mockery(testify 템플릿)로 생성한 mock을 주입해 모든 분기를 결정적으로 검증한다.

## Review Map

- **See the change**: `git add -A && git diff --cached HEAD` (HEAD 8fcaf71). 아래 모든 `file:line` 앵커는 이 스냅샷 기준.
- **Reading order** — foundation-first:

| # | Group | Risk | Lens | Intent (one line) |
|---|-------|------|------|-------------------|
| 1 | 와이어 타입 (`types.go`) | low | skim | OpenAI 호환 JSON req/resp·tool_calls 표현 |
| 2 | 계약 + 주입 seam (`client.go` 상단) | medium | top-down | `Client`/`HTTPDoer` 인터페이스, sentinel, 옵션, `New` |
| 3 | 재시도 루프 (`client.go` 메서드) | high | line-by-line | `CreateChatCompletion`/`do`/`backoff` 제어 흐름 |
| 4 | 테스트 + mock (`*_test.go`, `.mockery.yml`) | low | test-as-spec | 분기별 결정적 검증, mock 생성 설정 |
| 5 | 문서 정정 (`CLAUDE.md`/`AGENTS.md`/`SPEC.md`) | low | skim | 에러 래핑 예시를 `errors.Join`로 정합 |

Risk: low / medium / high. Lens: line-by-line / top-down / bottom-up+flow / test-as-spec / skim.

## Red Flags

- **RF1** `pkg/llm/client.go:36-39` — 신규 공개 표면: 사용자 요청으로 `HTTPDoer` 인터페이스와 `WithHTTPDoer` 옵션을 export했다. 계획은 `httptest` 기반이었으나 mock 주입을 위해 transport seam을 공개 API로 노출한 점이 계획 대비 추가 표면이다(의도된 deviation, 아래 Deviations 참조).
- **RF2** `go.mod:5` — 신규 의존성 `github.com/stretchr/testify v1.11.1` 추가. 단, mock 파일이 `_test.go`(`package llm_test`)라 **테스트 전용 의존성**이며 프로덕션 빌드/바이너리에는 포함되지 않는다(`go build ./...` 통과).
- **RF3** `pkg/llm/mock_httpdoer_test.go` — mockery 자동 생성 파일(`DO NOT EDIT`). 손으로 수정하지 말고 `.mockery.yml` 변경 후 `mockery`로 재생성해야 한다.

## Open Questions

- **OQ1** `pkg/llm/client.go:121-126` — 상태코드 분기를 `>=500` 재시도 / `>=400` 즉시실패 / 그 외 디코드로 두었다. 3xx(리다이렉트)는 실제로는 `*http.Client`가 처리하지만, mock 경로에서 3xx가 오면 디코드 단계로 떨어진다. 챗 API에서 3xx는 기대되지 않아 별도 처리하지 않았는데, 명시적 거부가 더 안전한지 검토 필요.
- **OQ2** `pkg/llm/types.go:9-11` — `Content`를 `*string`(+`omitempty`)로 두어 assistant tool_call 메시지의 `content:null`을 nil로 구분한다. Step 5 루프에서 메시지를 구성/재전송할 때 이 포인터 의미가 그대로 쓰일지(빈 문자열 vs nil 구분 필요 여부) 후속 스텝에서 확정 필요.

## Change Walkthrough (foundation-first)

### 1. 와이어 타입 (`types.go`) [low / skim]
OpenAI 호환 JSON 모양을 그대로 표현하는 순수 데이터 타입. 로직 없음.
- `pkg/llm/types.go:8` `Message` — `Content *string \`json:",omitempty"\``로 `content:null`(nil)과 빈 문자열을 구분, nil이면 송신 시 생략. `ToolCalls`/`ToolCallID`/`Name`도 `omitempty`.
- `pkg/llm/types.go:17` `ToolCall`, `:25` `FunctionCall` — `FunctionCall.Arguments`는 JSON 인코딩된 **문자열**(OpenAI tool-calling 와이어 포맷 그대로).
- `pkg/llm/types.go:32` `ToolDefinition` — `Function json.RawMessage`로 각 tool이 소유한 스키마를 그대로 전달.
- `pkg/llm/types.go:38` `ChatCompletionRequest` — `Tools`는 `omitempty`(미등록 시 키 자체 생략), `:45` `ChatCompletionResponse`/`:50` `Choice`.

### 2. 계약 + 주입 seam (`client.go` 상단) [medium / top-down]
루프가 의존할 인터페이스와, 테스트가 transport를 갈아끼우는 지점.
- `pkg/llm/client.go:16` `maxRetries=5` / `:19` `defaultBaseDelay=500ms` — 재시도 횟수·기본 백오프 베이스.
- `pkg/llm/client.go:21-30` sentinel 8종 — `go-conventions` 규칙대로 **call site마다 1개**씩 선언(`ErrLLMEncodeRequest`/`BuildRequest`/`Transport`/`ServerStatus`/`Status`/`Decode`/`Canceled`/`RetriesExhausted`).
- `pkg/llm/client.go:36` `HTTPDoer` — `Do(*http.Request)(*http.Response,error)`. `*http.Client`가 구조적으로 만족. mock 주입 seam(사용자 지시 핵심).
- `pkg/llm/client.go:42` `Client` — 루프가 의존할 유일 메서드 `CreateChatCompletion`.
- `pkg/llm/client.go:46` `httpClient` 구조체, `:54` `Option`, `:58` `WithHTTPDoer`(mock 주입), `:63` `WithRetryBaseDelay`(테스트에서 백오프 단축).
- `pkg/llm/client.go:69` `New` — 기본 `doer=&http.Client{}`, `baseDelay=defaultBaseDelay`로 초기화 후 옵션 적용. `apiKey` 비면 Authorization 생략은 `do`에서 처리.

### 3. 재시도 루프 (`client.go` 메서드) [high / line-by-line]
이 스텝의 핵심 제어 흐름. 결정적 동작이 테스트로 고정되어 있으니 테스트와 대조해 읽을 것.
- `pkg/llm/client.go:82` `CreateChatCompletion` — 1회 마샬(`ErrLLMEncodeRequest`) 후 `attempt 0..maxRetries`(총 6회) 루프. `attempt>0`이면 먼저 `backoff`. `do`가 `retry=true`면 계속, `false`면 즉시 반환, 소진 시 `errors.Join(ErrLLMRetriesExhausted, lastErr)`.
- `pkg/llm/client.go:109` `do` — 매 시도마다 `bytes.NewReader(body)`로 **새 요청** 생성(body 재사용 가능). `baseURL` 우측 `/` 트림 후 `/v1/chat/completions` 부착, `Content-Type` 항상·`Authorization`은 apiKey 있을 때만. 전송오류→`(true, ErrLLMTransport)`, `>=500`→`(true, ErrLLMServerStatus)`, `>=400`→`(false, ErrLLMStatus)`, 그 외 `json.NewDecoder(...).Decode`(`ErrLLMDecode`). 반환 bool이 "재시도 가능" 신호.
- `pkg/llm/client.go:142` `backoff` — `delay = baseDelay << (attempt-1)`(2배씩 증가), `select`로 `time.After` vs `ctx.Done()` 경쟁 → 취소 시 `ErrLLMCanceled`로 즉시 반환(ctx 존중).

### 4. 테스트 + mock (`*_test.go`, `.mockery.yml`) [low / test-as-spec]
`package llm_test`(외부 패키지)에서 공개 API만 검증. mock은 `_test.go`라 테스트 시에만 컴파일.
- `pkg/llm/client_test.go` — 아래 Testing 절 참조. `newTestClient`가 `WithRetryBaseDelay(time.Microsecond)`로 재시도 테스트를 빠르게 유지.
- `pkg/llm/mock_httpdoer_test.go` — mockery 생성 `MockHTTPDoer`(testify). 수정 금지.
- `.mockery.yml` — `template: testify`, 대상 `HTTPDoer` 하나. `dir={{.InterfaceDir}}`, `filename=mock_httpdoer_test.go`, `pkgname=llm_test`로 외부 테스트 패키지에 인-플레이스 생성.

### 5. 문서 정정 [low / skim]
계획의 "문서 정합" 태스크. 섹션 구조 유지, 해당 문장만 정정.
- `CLAUDE.md:36`, `AGENTS.md:36` — 에러 래핑 예시를 `errors.Join(ErrReadFile, err)`(call-site sentinel) 기준으로 교체.
- `docs/SPEC.md:59` — 동일 취지로 한국어 문장 정정.

## Key Decisions

- **transport seam 위치**: `http.RoundTripper`(Transport) 대신 `Do(req)` 수준에서 추상화. 재시도/상태코드/전송오류를 `*http.Client`의 리다이렉트 처리 없이 mock이 직접 통제할 수 있어 결정적 테스트에 유리.
- **재시도 횟수 의미**: "5회 재시도" = 최초 1회 + 재시도 5회 = **총 6회 시도**로 확정(`totalAttempts=6`으로 테스트에 고정).
- **mock을 `_test.go`로 생성**: testify가 프로덕션 의존성으로 새지 않도록 `package llm_test` 인-플레이스 파일로 출력. `go build ./...`에 testify 미포함.

## Deviations from Plan

- 계획 `## Tests`는 `httptest.NewServer` 기반이었으나, 사용자 지시로 HTTP 왕복을 `HTTPDoer` 인터페이스로 추상화하고 **mockery + testify** mock으로 대체했다. 그 결과 `httptest` 미사용, 대신 공개 API에 `HTTPDoer`/`WithHTTPDoer`가 추가되었다(→ **RF1**). 검증 케이스 범위(정상/tool_calls/요청바디/재시도/소진/4xx/인증헤더)는 계획과 동일하게 모두 충족하고, 전송오류 소진·디코드 실패·ctx 취소 케이스를 추가했다.
- 계획 `New(baseURL, apiKey string, opts ...)`에 테스트용 `WithRetryBaseDelay` 옵션을 추가했다(재시도 테스트의 백오프 단축용). 기본 동작(500ms 베이스)은 불변.

## Testing

- `pkg/llm/client_test.go:TestCreateChatCompletion_ParsesContent` — content만 있는 200 응답 → `Choices[0].Message.Content`/`FinishReason` 디코드 고정.
- `..._ParsesToolCalls` — `content:null`+`tool_calls` 응답 → `Content`는 nil, `ToolCall.ID/Type/Function.Name/Arguments` 정확 디코드(arguments는 JSON 문자열).
- `..._SendsRequest` — 요청 `POST`·URL·`Content-Type`·body의 `model`/`messages`/`tools` 포함을 mock에서 직접 검증.
- `..._OmitsToolsWhenEmpty` — tools 미등록 시 body에 `tools` 키 부재(`omitempty`) 고정.
- `..._RetriesThenSucceeds` — 500×2 후 200 → 성공 + `Do` 정확히 3회.
- `..._RetriesExhaustedOnServerError` — 지속 500 → 6회 후 `ErrLLMRetriesExhausted`∧`ErrLLMServerStatus`.
- `..._RetriesExhaustedOnTransportError` — 지속 전송오류 → 6회 후 `ErrLLMRetriesExhausted`∧`ErrLLMTransport`.
- `..._ClientErrorNoRetry` — 400 → 재시도 없이 `ErrLLMStatus`, `Do` 1회, `RetriesExhausted` 아님.
- `..._DecodeError` — 200+깨진 JSON → `ErrLLMDecode`, `Do` 1회.
- `..._AuthorizationHeader` (table) — apiKey 유/무 → `Authorization: Bearer ...` 유무.
- `..._HonorsContextCancellationDuringBackoff` — 취소된 ctx + 긴 백오프 → `ErrLLMCanceled`로 즉시 반환.

## Manual Verification

- [ ] `make build && make test && make lint` 재실행하여 그린 확인(현재 세션에서는 통과: race+cover 95.6%).
- [ ] (선택) 실제 Ollama 등 로컬 OpenAI 호환 엔드포인트에 `New(baseURL, "")`로 1회 왕복해 `apiKey` 미설정 시 Authorization 생략이 실제로 동작하는지 스모크 확인. 단위 테스트로 헤더 유무는 이미 검증됨.
- [ ] `.mockery.yml`로 `mockery` 재실행 시 `pkg/llm/mock_httpdoer_test.go`가 동일하게 재생성되는지 확인(생성물 드리프트 없음).

## Coverage

- `go test -race -cover ./pkg/llm/` → **95.6%** of statements.

## Fix

### Fix 1 — 2026-06-26 16:25 — HTTPDoer를 pkg/httputil.PostJSON으로 분리
- **Root cause**: 버그가 아니라 사용자 요청 리팩터로, 재사용 가능한 HTTP 전송 추상화를 `pkg/llm`에서 신규 `pkg/httputil` 패키지로 분리하고 thin `Do(req)`를 요청 빌드·헤더 설정·Do 호출을 감싸는 `PostJSON(ctx, url, body, headers)`로 재설계했다(향후 다른 기능도 재사용).
- **Change**: `pkg/httputil.Client.PostJSON`이 기존 `client.go` L111–L120(요청 생성+헤더+Do)을 캡슐화하고 헤더 값은 `map[string]string`으로 호출자가 주입하도록 했으며, `pkg/llm`은 `HTTPDoer`를 제거하고 `httputil.Client`를 주입받아 기존 재시도 의미(빌드 실패=비재시도 `ErrLLMBuildRequest`, 전송 실패=재시도 `ErrLLMTransport`)를 `errors.Is(err, httputil.ErrBuildRequest)` 분기로 보존한다 — 동작 변경 없이 경계만 옮긴 최소 변경.
- **Files changed**:
  - `pkg/httputil/client.go` (생성)
  - `pkg/httputil/client_test.go` (생성)
  - `pkg/llm/client.go`
  - `pkg/llm/client_test.go`
  - `pkg/llm/mock_client_test.go` (생성) / `pkg/llm/mock_httpdoer_test.go` (삭제)
  - `.mockery.yml`
- **Verification**: `mockery` ✅, `make build` ✅, `make test` ✅ (`pkg/httputil`·`pkg/llm` 모두 ok), `make lint` ✅ (`gofmt -l .` 빈 출력 + `go vet ./...` 클린).
- **Commit**: not committed (per policy).
- **Notes**: PostJson 대신 PostJSON으로 명명(코드베이스 이니셜리즘 컨벤션: `HTTPDoer`/`URL`/`ToolCallID`와 정합). `//go:generate mockery`와 mock 생성을 `httputil.Client` 대상으로 옮기되 출력은 여전히 `pkg/llm`의 `package llm_test`(`mock_client_test.go`)로 두어 testify가 프로덕션 빌드에 새지 않게 유지. 옵션 `WithHTTPDoer` → `WithHTTPClient`로 개명(테스트 전용이라 안전).

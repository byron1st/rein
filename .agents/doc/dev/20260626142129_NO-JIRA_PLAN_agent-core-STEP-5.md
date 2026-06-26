---
Application: rein
JiraTicket: NO-JIRA
PlanType: multi-steps-sub
Timestamp: 20260626142129
Title: agent-core
Step: 5
---

# Step 5: `pkg/agent` 동기 루프

Part of main plan: [20260626142129_NO-JIRA_PLAN_agent-core.md](./20260626142129_NO-JIRA_PLAN_agent-core.md)

## Goal

LLM과 도구 실행을 엮는 코어 루프를 구현한다. *"The loop is the agent"* — 제어 흐름은 모델의 tool call에 있고, 상태는 `[]llm.Message` append가 전부. `llm.Client`와 `tool.Registry`를 주입받아 fake로 결정적 테스트가 가능하게 한다.

## Implements

- FR-1: Agentic Loop의 제어 흐름(turn 반복, tool_calls 순차 실행, 종료 조건, max_turns, 에러 흡수, slog 관측).

## Depends On

Step 1 (`llm.Client`, 타입), Step 2 (`tool.Registry`).

## Tasks

- [ ] `pkg/agent/agent.go` — `Agent` 구조체: `client llm.Client`, `registry *tool.Registry`, `model string`, `maxTurns int`(기본 50), `logger *slog.Logger`. `New(client, registry, model string, opts ...Option) *Agent`(maxTurns/logger 옵션). 도구 정의는 `registry.List()`의 각 `Schema()`를 `llm.ToolDefinition{Type:"function", Function: schema}`로 래핑해 요청에 포함.
- [ ] `pkg/agent/agent.go` — `Run(ctx context.Context, messages []llm.Message) ([]llm.Message, error)` 루프:
  1. `for turn := 1; turn <= maxTurns; turn++`: turn 로그.
  2. `client.CreateChatCompletion(ctx, req{model, messages, tools})` 호출. 에러(재시도 소진 등 치명) → `messages, err` 반환(루프 중단).
  3. `msg := resp.Choices[0].Message`를 append.
  4. `len(msg.ToolCalls)==0`이면 종료 → `messages, nil`(최종 assistant content는 호출부가 사용).
  5. 각 `tool_call`을 **배열 순서대로** 순차 실행: `registry.Dispatch(ctx, tc.Function.Name, json.RawMessage(tc.Function.Arguments))`. 에러(미등록·인자파싱·도구실패)는 `result = err.Error()`로 흡수. 결과를 `{role:"tool", tool_call_id: tc.ID, content: result}`로 append. 도구명/소요/결과 길이 로그.
  6. 루프 복귀(2).
  7. `maxTurns` 초과 시: 안내 메시지(예: `{role:"assistant", content:"max turns (50) reached..."}` 또는 system 안내)를 append 후 `messages, nil` 반환.
- [ ] 에러 컨벤션 — sentinel + `errors.Join`. 루프 중단 사유(예: `ErrLLMCall`)는 1회 사용 sentinel로 래핑하되, **도구 실패는 절대 error로 루프를 중단하지 않고** tool 메시지 문자열로만 흡수.
- [ ] slog — `turn` 번호, `tool` 호출명/소요(ms)/결과 길이를 stderr 구조적 로그로 기록(logger 미주입 시 `slog.Default()`).

## Affected Files

| Action | Path | Description |
|--------|------|-------------|
| Create | `pkg/agent/agent.go` | `Agent` + `Run` 루프 |
| Create | `pkg/agent/agent_test.go` | fake client/registry 기반 루프 테스트(`package agent_test`) |

## Tests

- fake `llm.Client`(스크립트된 응답 시퀀스 반환)와 fake `Tool`(registry 등록)로 구성.
- tool_calls 없는 응답 → 즉시 종료, 최종 content/메시지 시퀀스 검증.
- 단일 tool_call → dispatch 후 tool 메시지 append, 다음 응답에서 종료. 메시지 순서(assistant→tool→assistant) 검증.
- 다중 tool_calls → 배열 순서대로 순차 실행 검증(호출 순서 기록).
- 미등록 tool → tool 메시지에 에러 문자열, 루프 유지.
- 도구 Execute 에러 → tool 메시지에 에러 문자열, 루프 유지.
- 인자 JSON 파싱 실패 → tool 메시지에 에러, 루프 유지.
- max_turns 초과(항상 tool_calls 반환하는 fake) → 안내 메시지 append 후 종료.
- client 치명 오류 → `Run`이 error 반환.

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
- [ ] 도구 실패가 루프를 중단하지 않음·max_turns·slog·에러 컨벤션 준수

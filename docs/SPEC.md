# Rein — Agent Core (A.1 Agentic Loop / A.2 Tool Engine)

개인 개발 harness "Rein"의 미니멀 AI 에이전트 코어. 이번 스펙은 가장 먼저 구현할 **A.1 Agentic Loop**와 **A.2 Tool 실행 엔진 + 최소 도구 세트**만 다룬다. A.3(시스템 프롬프트 조립)·A.4(컨텍스트 관리)·B(harness 지원)는 후속 스펙으로 분리한다.

## Tech Stack

- **Language**: Go — 항상 최신 stable (현재 1.26.x)
- **Runtime form**: 단일 바이너리 CLI
- **LLM 연동**: `net/http` 직접 구현, **OpenAI Chat Completions 호환** 엔드포인트 (`/v1/chat/completions`), function tool-calling 포맷
- **LLM provider**: `base_url` 교체로 Ollama(로컬) / OpenAI / flagship 커버
- **Testing**: Go 표준 `testing`, table-driven
- **Logging**: `log/slog`
- **CI**: 미정 (Open Questions)

## Architecture

### Context

```
   Developer (CLI)
        │  prompt
        ▼
  ┌───────────────┐   OpenAI-compatible    ┌────────────────────┐
  │  Rein Agent   │── /chat/completions ──▶│ LLM Provider       │
  │  Core (Go)    │◀──  (net/http, JSON) ──│ Ollama / OpenAI /… │
  └──────┬────────┘                        └────────────────────┘
         │ tool dispatch
         ▼
  ┌──────────────────────────────────────────────┐
  │ Tools: read_file/write_file/edit_file,       │
  │        bash, grep(rg), glob(fd)              │
  └──────┬───────────────────────────────────────┘
         ▼
   Filesystem · subprocess (rg / fd / bash)
```

### Runtime

단일 Go 프로세스. 서버/포트 없음. **동기 request-response 루프** — 매 turn마다 LLM에 1회 POST(non-streaming)하고, 완성된 `tool_calls` 배열을 받아 순회 실행한다. `bash`·`rg`·`fd`는 `os/exec`로 서브프로세스 spawn. 상태는 인메모리 `[]Message` append-only.

### Code / Module

패키지를 `pkg/` 하위에 두어 추후 라이브러리 형태로 외부에서 import 가능하게 한다(`internal/`은 외부 import를 차단하므로 사용하지 않음).

```
cmd/rein/           CLI 진입점 (env 로딩, 세션 시작)
pkg/agent/          agentic loop, 세션([]Message), turn 제어        → FR-1
pkg/llm/            OpenAI 호환 클라이언트(net/http), req/resp 타입,
                    tool_calls ↔ 내부 타입 매핑
pkg/tool/           Tool 인터페이스, Registry, dispatch             → FR-2
  ├ fs.go           read_file / write_file / edit_file
  ├ exec.go         bash
  └ search.go       grep(rg) / glob(fd)
```

## Conventions

- **도구 실패는 루프를 죽이지 않는다**: 도구 에러는 Go error로 전파해 루프를 중단하는 대신, `role:"tool"` 메시지 content에 에러 문자열로 담아 모델에 반환(자기수정 유도). 네트워크·시스템 치명 오류만 루프 중단.
- **에러 래핑**: call site마다 sentinel을 선언하고 `errors.Join(ErrReadFile, err)`로 underlying 에러와 묶어 컨텍스트 포함.
- **도구 출력 캡**: 모든 도구 출력은 **~50KB / 2000라인** 상한. 초과 시 중간 절단하고 전체 내용은 temp-file로 오프로드한 뒤 그 경로를 출력에 안내.
- **Tool schema**: 각 Tool이 자신의 OpenAI function schema(JSON)를 자체 제공(`Schema()`).
- **Config (env)**: `OPENAI_BASE_URL`, `OPENAI_API_KEY`, `OPENAI_MODEL`.

## Functional Requirements

### FR-1: Agentic Loop (A.1)

LLM과 도구 실행을 엮는 코어 루프. *"The loop is the agent"* — 제어 흐름은 모델의 tool call에 있고, 상태는 `[]Message`에 append하는 것이 전부다.

- **Input**: 시드 `system` 메시지(이 스펙 범위에선 최소) + 초기 `user` 메시지
- **Output**: `tool_calls`가 더 이상 없는 최종 `assistant` 메시지
- **동작**:
  1. `messages`에 system + user 시드
  2. `POST /chat/completions` (등록된 `tools` 포함, non-streaming)
  3. 응답에 `tool_calls`가 있으면 → 각 tool_call을 **순차 실행** → 각 결과를 `role:"tool"`(해당 `tool_call_id`) 메시지로 append → 2로 복귀
  4. `tool_calls`가 없으면 종료, `assistant` content 반환
- **Business rules**:
  - `max_turns`(기본 **50**) 초과 시 루프 중단 + 안내 메시지 append
  - 한 응답에 여러 `tool_calls`가 오면 배열 순서대로 **순차 실행**
  - `messages`는 append-only, 절단/압축 없음(A.4 범위)
- **Edge cases**:
  - `tool_call.arguments` JSON 파싱 실패 → 해당 tool 메시지에 에러 반환(루프 유지)
  - 미등록 tool name → 에러 반환(루프 유지)
  - LLM 호출 네트워크 오류 → **지수 백오프 5회 재시도 후 중단**

메시지 흐름 예시:

```json
[
  {"role":"system","content":"..."},
  {"role":"user","content":"foo.go 의 버그를 고쳐줘"},
  {"role":"assistant","content":null,
   "tool_calls":[{"id":"call_1","type":"function",
     "function":{"name":"read_file","arguments":"{\"path\":\"foo.go\"}"}}]},
  {"role":"tool","tool_call_id":"call_1","content":"<파일 내용>"},
  {"role":"assistant","content":"수정했습니다. ..."}
]
```

### FR-2: Tool 실행 엔진 + 최소 도구 세트 (A.2)

도구를 `name + JSON schema + handler`로 등록하고 dispatch하는 엔진과, 6개 기본 도구.

- **Tool 인터페이스** (⚠️ ASSUMED — 구현하며 확정, 현재는 예상 시그니처):
  ```go
  type Tool interface {
      Name() string
      Schema() json.RawMessage              // OpenAI function schema
      Execute(ctx context.Context, args json.RawMessage) (string, error)
  }
  ```
- **Registry**: `map[string]Tool` + dispatch(name, args). 부팅 시 6개 등록.
- **도구 목록**:

  | 도구 | 입력 | 동작 |
  | --- | --- | --- |
  | `read_file` | `path` (`offset`,`limit` 선택) | 파일 내용 반환(출력 캡 적용) |
  | `write_file` | `path`, `content` | 생성/덮어쓰기 |
  | `edit_file` | `path`, `old_str`, `new_str` | **unique-match 필수** 치환 |
  | `bash` | `command` (`timeout` 선택) | 셸 실행, stdout+stderr+exit 반환 |
  | `grep` | `pattern` (`path`,`glob` 선택) | `rg` 래핑 |
  | `glob` | `pattern` (`path` 선택) | `fd` 래핑 |

- **Business rules**:
  - **`edit_file`**: `old_str`가 파일에서 **정확히 1회** 매칭될 때만 치환. 0개 또는 2개 이상이면 에러(치환 안 함).
  - **`bash`**: `cwd`=프로젝트 루트 고정, 기본 `timeout` **300s**, 출력 캡 적용. 정책 강제(위험 커맨드 차단 등)는 B.6 hook으로 위임(이 스펙 범위 밖).
  - **`grep`/`glob`**: 각각 `rg`/`fd`를 절대경로로 resolve해 호출(향후 GUI 환경의 PATH 비상속 대비, CLI에서도 일관).
  - 모든 도구 출력은 캡 + temp-file 오프로드.
- **Edge cases**:
  - `edit_file` 다중 매칭 → `"N matches found; provide more surrounding context"` 에러
  - 파일 없음/권한 거부 → 에러 문자열 반환
  - `rg`/`fd` 미설치 → 명확한 설치 안내 에러

## Quality Attributes

- **Performance**: 단일 turn 레이턴시는 LLM 왕복이 지배적. 도구 실행은 로컬, `bash` 기본 timeout 300s.
- **Resource cap**: 도구 출력 50KB/2000라인 상한으로 컨텍스트 폭증 방지.
- **Observability**: `slog` 구조적 로그(turn 번호, tool 호출명/소요/결과 길이)를 stderr로 출력.
- **Testability**: LLM 클라이언트를 인터페이스로 추상화 → `httptest`/mock으로 결정적 루프 테스트. 도구는 `t.TempDir()` 기반 테스트.
- **Security**: 신뢰된 로컬 개인 harness 전제. `bash` 임의 실행 허용(정책은 B.6 이후).

## Constraints

- 하드웨어: M1 MacBook Air 16GB — 로컬 Ollama 모델 크기/열 제약.
- 플랫폼: macOS 우선(향후 GUI도 macOS 타깃).
- 의존 최소화: 코어는 `net/http` + `encoding/json`. `rg`/`fd`는 시스템 바이너리로 필수 전제.

## Dependencies

- **Go stdlib**: `net/http`, `encoding/json`, `os/exec`, `context`, `log/slog`.
- **시스템 바이너리**: ripgrep(`rg`), fd.
- **LLM 엔드포인트**: OpenAI 호환(Ollama 로컬 / OpenAI / 기타).
- **langchaingo**: 이번 A.1/A.2 범위에서는 **미사용**(provider를 net/http로 직접 구현). 추후 멀티 프로바이더 추상화가 필요해지면 `llms` 패키지 재검토.

## Open Questions

- `Tool` 인터페이스 최종 시그니처 (구현하며 확정)
- temp-file 오프로드 위치 및 정리(cleanup) 정책
- CI 시스템

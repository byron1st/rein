---
Application: rein
JiraTicket: NO-JIRA
ReportType: multi-steps-step
Timestamp: 20260626142129
Title: agent-core
Step: 2
ReviewBase: git diff main...feature/step-2-tool
---

# Step 2: `pkg/tool` 엔진 + 출력 cap 헬퍼

Plan: [20260626142129_NO-JIRA_PLAN_agent-core-STEP-2.md](./20260626142129_NO-JIRA_PLAN_agent-core-STEP-2.md)

## Summary

FR-2의 엔진 부분만 구현했다 — 도구를 `name + JSON schema + handler`로 등록·dispatch하는 `Registry`와, 모든 도구가 공유할 출력 cap(50KB/2000라인) + temp-file 오프로드 헬퍼 `capOutput`. 개별 도구(read_file/bash/grep 등)는 Step 3·4에서 이 인터페이스와 Registry 위에 올라간다. 에러는 go-conventions.md를 그대로 따라 실패-명칭 sentinel + 단일 call-site `errors.Join`로 묶고, 타 패키지 함수 에러는 re-wrap 없이 pass-through했다. SPEC/플랜이 "중간 절단"이라고 적어둔 반면 마커가 말미에 붙고 전체 원본이 오프로드된다는 점에서 가장 단순하고 일관된 해석인 head 절단을 적용했으며, 이는 OQ1로 리뷰 게이트에서 확인을 요청한다.

## Review Map

- **See the change**: `git diff main...feature/step-2-tool`. 본 보고서의 모든 `file:line` 앵커는 해당 스냅샷 기준이다.
- **Reading order** — foundation-first; 정의에서 사용 순서로 읽으면 심볼을 정의 전에 만나지 않는다:

| # | Group | Risk | Lens | Intent (one line) |
|---|-------|------|------|-------------------|
| 1 | `Tool` 인터페이스 | low | skim | 도구 계약 — Name/Schema/Execute 3개 메서드로 모든 도구를 동형으로 취급 |
| 2 | `Registry` 엔진 | medium | line-by-line | 등록 순서 보존 + 이름 조회 + dispatch 에러 처리(unknown은 sentinel, tool 에러는 pass-through) |
| 3 | `capOutput` 출력 cap | high | line-by-line | 출력 크기 상한 + 오프로드 + 절단 마커 — SPEC의 "출력 캡" 비즈니스 룰을 코드로 옮긴 유일한 지점 |
| 4 | 테스트 | low | test-as-spec | registry/cap의 동작을 실행 가능한 명세로 고정 |

## Red Flags

- **RF1** `pkg/tool/export_test.go:10` — 플랜의 "Files to create (exactly these)" 표에 없는 6번째 파일이 추가됐다. `capOutput`/`maxBytes`/`maxLines`가 서브플랜대로 unexported인데 go-conventions.md가 외부 `tool_test` 패키지를 강제하는 충돌을 메우기 위한 표준 Go 브릿지(`export_test.go`)이며, 테스트 로직은 없고 별칭만 노출한다. 리뷰어가 이 파일 존재 자체를 승인해야 한다(자세한 사유는 Deviations).

## Open Questions

- **OQ1** `pkg/tool/output.go:55` (`truncateHead`) — SPEC/메인 플랜은 "중간 절단"이라 적었지만, 마커가 절단본 말미에 붙고 전체 원본이 temp 파일로 오프로드된다는 점에서 head 절단(앞부분 보존 + 말미 마커)을 적용했다. 서브플랜의 모든 테스트 단정(절단 + 마커 + temp 파일 내용 == 원본 전체 + 경로 파싱 가능)을 만족하고 마커 위치와 가장 일관되지만, "중간 절단"의 의도(head/tail 양쪽 보존 등)가 있었는지 리뷰 게이트에서 확정이 필요하다.

## Change Walkthrough (foundation-first)

### 1. `Tool` 인터페이스 [low / skim]
도구 계약을 단일 인터페이스로 고정해 Registry와 향후 도구들이 동형으로 결합되도록 한다.
- `pkg/tool/tool.go:10` `Tool` — 3-메서드 계약. `Schema()`는 OpenAI function 객체의 inner JSON(`{"name","description","parameters"}`)을 반환하고 바깥 `{"type":"function","function":...}` 래핑은 `pkg/agent`(Step 5) 책임임을 주석으로 명시.
- `pkg/tool/tool.go:16` `Schema` — 위 계약의 핵심 결정을 주석에 기록하여 Step 5 와이어링 시 혼선 방지.

### 2. `Registry` 엔진 [medium / line-by-line]
등록 순서 보존과 에러 컨벤션(pass-through vs sentinel 래핑)의 경계를 정확히 나눈 지점.
- `pkg/tool/registry.go:12` `ErrUnknownTool` — 구체 조건명 sentinel. Dispatch 1곳에서만 사용.
- `pkg/tool/registry.go:16` `Registry` — `tools map[string]Tool`(조회) + `order []Tool`(결정적 나열). map 순회가 비결정적이므로 order 슬라이스를 병행 유지.
- `pkg/tool/registry.go:22` `NewRegistry` — map을 즉시 초기화(이후 Register의 nil-map 쓰기 방지).
- `pkg/tool/registry.go:29` `Register` — 새 이름은 order에 append, 기존 이름은 map만 덮어쓰고 order 위치는 유지(중복 금지). 에러 반환 없음(플랜 시그니처).
- `pkg/tool/registry.go:38` `Get` — 단순 map 조회.
- `pkg/tool/registry.go:45` `List` — order의 복사본 반환(외부 변형으로부터 내부 보호); 빈 경우 non-nil 빈 슬라이스.
- `pkg/tool/registry.go:55` `Dispatch` — 미등록 시 `errors.Join(ErrUnknownTool, fmt.Errorf("tool %q not registered", name))`(동적 detail 포함). 등록 시 `t.Execute(ctx, args)`를 그대로 반환 — re-wrap 금지(go-conventions.md pass-through 규칙).

### 3. `capOutput` 출력 cap [high / line-by-line]
SPEC "도구 출력 캡" 비즈니스 룰의 유일한 구현 지점. 경계값(초과 vs 미초과)이 여기서 결정된다.
- `pkg/tool/output.go:11` `maxBytes` / `pkg/tool/output.go:13` `maxLines` — 50KB / 2000라인 상한. unexported(같은 패키지 도구들만 호출하는 공유 헬퍼).
- `pkg/tool/output.go:16` `ErrFailedToCreateOffloadDir` / `pkg/tool/output.go:17` `ErrFailedToWriteOffload` — 실패-명칭 sentinel. 각각 `os.MkdirTemp`, `os.WriteFile` 1곳에서만 사용. 플랜 예시명(`ErrOffloadCreate` 등) 대신 go-conventions.md가 강제하는 `failed to <verb> <object>` 형태를 채택(`pkg/llm` sentinel 스타일과 일치).
- `pkg/tool/output.go:25` `capOutput` — 상한 이하면 원본 그대로(마커/오프로드 없음). 초과면 원본 전체를 temp 파일에 기록 후 head 절단본 + 말미 마커(원본 바이트/라인 수 + 절대경로) 반환. temp 디렉터리 자동 정리 없음(OS 주기 정리에 위임, SPEC 정책).
- `pkg/tool/output.go:46` `lineCount` — 빈 문자열 0, 그 외 `\n` 개수 + 1(끝 newline 없는 마지막 줄도 카운트). 초과 판정과 마커 표시에 같은 함수를 써서 경계 일관성 유지.
- `pkg/tool/output.go:55` `truncateHead` — 먼저 라인 바운드(앞 maxLines줄 보존), 그다음 바이트 바운드(앞 maxBytes바이트 컷). head 절단 전략 — OQ1 참조.

### 4. 테스트 [low / test-as-spec]
- `pkg/tool/export_test.go:10` — `CapOutput`/`MaxBytes`/`MaxLines` 별칭 노출(외부 테스트 패키지에서 unexported 심볼 접근용). 테스트 함수 없음. RF1 참조.

## Key Decisions

- **sentinel 명명**: 플랜이 예시로 든 `ErrOffloadCreate`/`ErrOffloadWrite` 대신 `ErrFailedToCreateOffloadDir`/`ErrFailedToWriteOffload`를 썼다. go-conventions.md가 "bare operation/location 이름 금지, `failed to <verb> <object>` 또는 구체 조건명"을 강제하며, 기존 `pkg/llm` sentinel(`ErrFailedToDecodeLLMResponse` 등)과 동일 스타일로 맞췄다.
- **head 절단**: SPEC의 "중간 절단" 표현 대신 head 절단 적용. 마커가 말미에 붙고 전체 원본이 오프로드된다는 구조에서 가장 단순·일관된 해석이며 서브플랜의 모든 테스트 단정을 만족한다(Simplicity First). OQ1에서 리뷰 확정 예정.
- **pass-through vs re-wrap**: `Dispatch`는 tool의 `Execute` 에러를 re-wrap 없이 그대로 반환한다. go-conventions.md "타 코드베이스 함수 에러는 이미 sentinel을 가지므로 re-wrap 금지"에 근거. `TestDispatch_PropagatesToolError`가 이 결정을 고정.

## Deviations from Plan

- **6번째 파일 `pkg/tool/export_test.go` 추가**: 서브플랜의 "Affected Files" 표는 5개 파일만 나열하지만, `capOutput`/`maxBytes`/`maxLines`를 unexported로 유지(서브플랜 명시)하면서 go-conventions.md가 강제하는 외부 `tool_test` 패키지에서 테스트하려면 브릿지가 필수다. 대안(1) 심볼을 export → 서브플랜 시그니처 위반, (2) 내부 패키지 테스트 → go-conventions.md 위반. `export_test.go`는 표준 Go 관용구로 테스트 로직이 없고 별칭만 노출하며 test 빌드에만 포함된다. RF1로 표시.
- **sentinel 이름 변경**: `ErrOffloadCreate`/`ErrOffloadWrite` → `ErrFailedToCreateOffloadDir`/`ErrFailedToWriteOffload`. 플랜이 "등"(etc.)로 열어둔 부분이며 go-conventions.md 준수를 위한 필수 변경.

## Testing

- `pkg/tool/registry_test.go:28` `TestRegister_Get_List_Dispatch` — 등록→Get/List/Dispatch happy path. **고정하는 동작**: List가 등록 순서를 보존, Register overwrite가 order 위치를 유지하며 중복하지 않음, Dispatch가 args를 verbatim 전달.
- `pkg/tool/registry_test.go:79` `TestDispatch_UnknownTool_ReturnsErrUnknownTool` — **고정**: 미등록 name이 `ErrUnknownTool`에 `errors.Is` true, 에러 메시지가 도구 이름을 포함.
- `pkg/tool/registry_test.go:86` `TestDispatch_PropagatesToolError` — **고정**: tool 에러를 re-wrap 없이 그대로 전파(`errors.Is(err, errBoom)` true, `errors.Is(err, ErrUnknownTool)` false).
- `pkg/tool/registry_test.go:96` `TestList_EmptyRegistry_ReturnsNonNilEmptySlice` — **고정**: 빈 Registry의 List가 non-nil 빈 슬라이스.
- `pkg/tool/output_test.go:26` `TestCapOutput_UnderLimit_ReturnedUnchanged` — **고정**: 상한 미만 입력은 원본 그대로, 마커 없음.
- `pkg/tool/output_test.go:35` `TestCapOutput_OverBytes_TruncatesAndOffloads` — **고정**: 바이트 초과 시 head가 앞 maxBytes바이트로 시작, 마커 존재, 파싱한 경로의 파일 내용이 원본 전체와 일치.
- `pkg/tool/output_test.go:50` `TestCapOutput_OverLines_TruncatesAndOffloads` — **고정**: 라인 초과 시 head가 앞 maxLines줄, 마커 존재, 오프로드 파일 == 원본 전체.
- `pkg/tool/output_test.go:70` `TestCapOutput_BoundaryExactlyMaxBytes_NotTruncated` — **고정**: 정확히 maxBytes는 미초과(절단 없음).
- `pkg/tool/output_test.go:79` `TestCapOutput_BoundaryExactlyMaxLines_NotTruncated` — **고정**: 정확히 maxLines는 미초과(절단 없음).

오프로드 실패 경로(`ErrFailedToCreateOffloadDir`/`ErrFailedToWriteOffload`)는 `os.MkdirTemp`/`os.WriteFile` 실패를 결정적으로 주입할 수 없어 테스트하지 않았다. 프로덕션 코드는 sentinel + `errors.Join` 래핑을 그대로 둠(weaken하지 않음). Notes 참조.

## Manual Verification

None. 순수 라이브러리 패키지로 UI·외부 side effect·네트워크·파일시스템 영구 변경이 없고, 오프로드는 `os.MkdirTemp` 임시 디렉터리에만 쓴다. 검증은 `go test -race ./pkg/tool/...`와 `golangci-lint run ./...`로 충분하다(모두 통과).

## Notes

- `golangci-lint` v2.12.2 설치됨 — CI 게이트 명령 `golangci-lint run ./...` 실행, 0 issues.
- 오프로드 실패 경로 테스트 생략: `os.MkdirTemp`/`os.WriteFile` 실패 주입이 결정적으로 어려워 테스트하지 않음. 프로덕션 sentinel + 래핑 코드는 유지.
- `go mod tidy` 실행 중 `gopkg.in/check.v1` 모듈 캐시 다운로드가 발생했으나 go.mod/go.sum은 변경되지 않았다(기존 간접 의존성, `git diff main..HEAD -- go.mod go.sum`이 공백).
- `pkg/tool`은 `pkg/llm` 또는 어떤 내부 패키지에도 의존하지 않는다(독립 패키지, 메인 플랜 의존 방향 준수).

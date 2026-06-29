---
Application: rein
JiraTicket: NO-JIRA
ReportType: multi-steps-step
Timestamp: 20260626142129
Title: agent-core
Step: 3
ReviewBase: git diff main...feature/step-3
---

# Step 3: fs 도구 (`read_file` / `write_file` / `edit_file`)

Plan: [20260626142129_NO-JIRA_PLAN_agent-core-STEP-3.md](./20260626142129_NO-JIRA_PLAN_agent-core-STEP-3.md)

## Summary

FR-2의 파일시스템 도구 3종(`read_file`/`write_file`/`edit_file`)을 `Tool` 인터페이스 구현체로 추가했다. 각 도구는 자체 OpenAI function schema를 제공하고, `read_file`은 Step 2의 `capOutput`으로 출력을 캡핑하며, `edit_file`은 `old_str`가 정확히 1회 매칭할 때만 치환하는 unique-match 비즈니스 규칙을 적용한다. 모든 실패는 exported sentinel + `errors.Join`으로 래핑해 루프가 tool 메시지로 흡수할 수 있도록 한다.

## Review Map

- **See the change**: `git diff main...feature/step-3`. Every `file:line` anchor in this report is valid against that snapshot.
- **Reading order** - foundation-first; read top to bottom and you never meet a symbol before its definition:

| # | Group | Risk | Lens | Intent (one line) |
|---|-------|------|------|-------------------|
| 1 | sentinels & types | low | skim | 도구별 에러 sentinel과 args 구조체 — 실패 지점을 이름짓는 계약 |
| 2 | read_file 핵심 로직 | high | line-by-line | 파일 읽기 + 1-indexed 라인 슬라이스 + capOutput 적용 |
| 3 | write_file 핵심 로직 | medium | top-down | os.WriteFile 래핑 + 바이트 요약 반환 |
| 4 | edit_file 핵심 로직 | high | line-by-line | unique-match 카운트/치환/거부 비즈니스 규칙 |
| 5 | 생성자 & schema | low | skim | New*Tool 생성자와 정적 JSON schema |
| 6 | tests | low | test-as-spec | t.TempDir 기반 외부 패키지 테스트 |

Risk: low / medium / high. Lens: line-by-line / top-down / bottom-up+flow / test-as-spec / skim.

## Red Flags

None

## Open Questions

None

## Change Walkthrough (foundation-first)

### 1. sentinels & types [low / skim]
도구별 실패 지점을 이름짓는 exported sentinel과 args 파싱 구조체. 각 sentinel은 정확히 한 call site에서만 사용된다(go-conventions.md "sentinel은 정확히 1 call site" 준수).
- `pkg/tool/fs.go:13` `Err*` sentinels — `ErrReadFileOpen`, `ErrWriteFile`, `ErrEditFileRead/NoMatch/MultiMatch/Write` 및 각 도구의 arg-parse/missing-path sentinel 2종씩(총 6종: `ErrReadFileArgsParse`/`ErrReadFileMissingPath`, `ErrWriteFileArgsParse`/`ErrWriteFileMissingPath`, `ErrEditFileArgsParse`/`ErrEditFileMissingPath`). go-conventions.md의 "sentinel은 실패를 서술" 규칙에 맞춰 메시지를 "failed to ..." 형태로 작성하며, json.Unmarshal 실패 call site와 path 누락 검증 call site 각각에 별도 sentinel을 선언.
- `pkg/tool/fs.go:52` `readFileArgs` — `Offset`/`Limit`를 `*int`로 받아 absent=nil → 기본값 분기를 명확히 함.
- `pkg/tool/fs.go:124` `writeFileArgs`, `pkg/tool/fs.go:169` `editFileArgs` — 각각 `json` 태그 기반 파싱.

### 2. read_file 핵심 로직 [high / line-by-line]
파일 읽기 → 옵션 라인 슬라이스 → capOutput. 라인 슬라이스의 1-indexed 경계 처리가 핵심 위험.
- `pkg/tool/fs.go:58` `readFileTool.Execute` — `os.ReadFile` 실패를 `ErrReadFileOpen`으로 래핑. offset/limit 중 하나라도 non-nil이면 `sliceLines`로 분기.
- `pkg/tool/fs.go:82` `sliceLines` — `strings.Split(content, "\n")` 후 1-indexed offset → 0-indexed 변환. offset이 총 라인 수 초과면 `""` 반환(에러 아님). nil/음수 offset은 기본값 1, nil/0/음수 limit은 "남은 전부". `min(startIdx+limit, len(lines))`로 끝 인덱스 클램프.

### 3. write_file 핵심 로직 [medium / top-down]
단순 `os.WriteFile` 래핑. 빈 content도 유효한 빈 파일 쓰기로 허용(path만 필수 검증).
- `pkg/tool/fs.go:129` `writeFileTool.Execute` — `0o644` 권한으로 생성/덮어쓰기. 성공 시 `fmt.Sprintf("wrote %d bytes to %s", len(content), path)` 요약 반환. 출력이 작으므로 capOutput 미적용(요약은 항상 bound 내).

### 4. edit_file 핵심 로직 [high / line-by-line]
unique-match 비즈니스 규칙. 0/1/2+ 분기가 파일 불변성을 보장해야 한다.
- `pkg/tool/fs.go:175` `editFileTool.Execute` — `os.ReadFile` 실패 → `ErrEditFileRead`.
- `pkg/tool/fs.go:190` `strings.Count(content, a.OldStr)`로 매칭 수 계산 후 switch: 0 → `ErrEditFileNoMatch`(파일 미변경), 1 → `strings.Replace(..., 1)` 치환 후 `os.WriteFile` 실패 시 `ErrEditFileWrite`, 2+ → `ErrEditFileMultiMatch`에 매칭 수를 `fmt.Errorf`로 포함(파일 미변경). 0/2+ 분기는 디스크에 쓰지 않으므로 파일 불변이 보장된다.

### 5. 생성자 & schema [low / skim]
Step 6 registry 등록을 위한 생성자와 정적 schema.
- `pkg/tool/fs.go:32` `NewReadFileTool`, `pkg/tool/fs.go:105` `NewWriteFileTool`, `pkg/tool/fs.go:149` `NewEditFileTool` — 각각 `*readFileTool`/`*writeFileTool`/`*editFileTool` 반환.
- `Schema()` 메서드 — `json.RawMessage` 원시 리터럴로 inner function 객체(`{"name","description","parameters"}`) 반환. `pkg/agent`가 `{"type":"function","function":<schema>}`로 래핑.

### 6. tests [low / test-as-spec]
`package tool_test`, `t.TempDir`, `testify/require` 기반. Step 2 테스트 스타일 일치.
- `pkg/tool/fs_test.go:11` `jsonArgs` 헬퍼 — 테스트 fixture에서 marshal 에러를 즉시 실패시켜 call site를 한 줄로 유지.
- `pkg/tool/fs_test.go:17`–`TestReadFile_*` — 기본 읽기, offset/limit 슬라이스(+ 경계 케이스 테이블), missing file → `errors.Is(ErrReadFileOpen)`, 대용량 파일 → `[output truncated` 마커 + offload 파일 내용 검증, malformed args → `errors.Is(ErrReadFileArgsParse)`, missing path → `errors.Is(ErrReadFileMissingPath)`, schema/name 검증.
- `TestWriteFile_*` — 신규 생성(바이트 수+경로 요약 검증), 덮어쓰기, 쓰기 실패 → `ErrWriteFile`, 빈 content → 빈 파일, malformed args → `errors.Is(ErrWriteFileArgsParse)`, missing path → `errors.Is(ErrWriteFileMissingPath)`, schema/name 검증.
- `TestEditFile_*` — unique-match 치환(디스크 내용 검증), 0매칭 → `ErrEditFileNoMatch` + 파일 불변, 다중 매칭 → `ErrEditFileMultiMatch` + "3 matches found" 메시지 + 파일 불변, missing file → `ErrEditFileRead`, malformed args → `errors.Is(ErrEditFileArgsParse)`, missing path → `errors.Is(ErrEditFileMissingPath)`, schema/name 검증.

## Key Decisions

- **arg sentinel per-call-site 분할(해결됨)**: 원래 `ErrReadFileArgs`/`ErrWriteFileArgs`/`ErrEditFileArgs` 각각을 json.Unmarshal 실패 call site와 path 누락 검증 call site 두 곳에서 재사용했으나, go-conventions.md의 "sentinel은 정확히 1 call site" 규칙 위반이었다. 리뷰 게이트에서 해당 위반을 해결하기 위해 각 도구의 arg sentinel을 parse 전용(`Err*ArgsParse`)과 missing-path 전용(`Err*MissingPath`)으로 분할했다. 이제 모든 sentinel이 정확히 한 call site에서만 사용된다.
- **offset 1-indexed 규약**: sub-plan이 "1-indexed line number"를 요구. `sliceLines`는 1-indexed 입력을 0-indexed 슬라이스 인덱스로 변환. nil/0/음수는 기본값(1 / 남은 전부)으로 정규화.
- **write_file 빈 content 허용**: content 필드를 비어있어도 유효한 빈 파일 쓰기로 처리. path만 필수 검증. 이는 "Both required"가 JSON schema의 필드 존재 의미(Schema에 `required`로 명시)이지, Execute 단의 비어있지 않음 강제가 아님으로 해석했다.
- **read_file 요약 미캡핑 vs write/edit 요약**: write_file/edit_file의 요약 문자열은 항상 bound 내이므로 capOutput을 거치지 않고 직반환. read_file은 본문이 임의 길이이므로 capOutput 적용. Step 4 도구들도 동일 패턴을 따를 수 있다.

## Testing

- `pkg/tool/fs_test.go:TestReadFile_BasicRead` — 전체 파일 반환을 고정.
- `pkg/tool/fs_test.go:TestReadFile_OffsetLimit_LineSlicing` — offset=2/limit=2가 2~3번째 라인을 반환함을 고정.
- `pkg/tool/fs_test.go:TestReadFile_LineSlicing_EdgeCases` — offset-only/limit-only/초과/0/음수 경계를 테이블로 고정.
- `pkg/tool/fs_test.go:TestReadFile_MissingFile_ReturnsErrReadFileOpen` — `errors.Is(ErrReadFileOpen)`으로 sentinel 래핑을 고정.
- `pkg/tool/fs_test.go:TestReadFile_LargeFile_TruncatesAndOffloads` — capOutput 오프로드 경로와 offload 파일 무결성을 고정.
- `pkg/tool/fs_test.go:TestReadFile_MalformedArgs_ReturnsErrReadFileArgsParse`, `TestReadFile_MissingPath_ReturnsErrReadFileMissingPath` — `errors.Is`로 parse/missing-path sentinel을 고정.
- `pkg/tool/fs_test.go:TestWriteFile_CreateNewFile` — 요약 문자열 포맷("wrote N bytes to <path>")과 디스크 내용을 고정.
- `pkg/tool/fs_test.go:TestWriteFile_OverwriteExistingFile` — 덮어쓰기 동작을 고정.
- `pkg/tool/fs_test.go:TestWriteFile_EmptyContent_CreatesEmptyFile` — 빈 content 유효성을 고정.
- `pkg/tool/fs_test.go:TestWriteFile_MalformedArgs_ReturnsErrWriteFileArgsParse`, `TestWriteFile_MissingRequiredFields_ReturnsErrWriteFileMissingPath` — `errors.Is`로 parse/missing-path sentinel을 고정.
- `pkg/tool/fs_test.go:TestEditFile_UniqueMatch_ReplacesContent` — 정확히 1회 매칭 시 치환 결과를 고정.
- `pkg/tool/fs_test.go:TestEditFile_NoMatch_ReturnsErrEditFileNoMatch_FileUnchanged` — 0매칭 시 에러 + 파일 불변을 고정.
- `pkg/tool/fs_test.go:TestEditFile_MultiMatch_ReturnsErrEditFileMultiMatch_FileUnchanged` — 다중 매칭 시 매칭 수 메시지 + 파일 불변을 고정.
- `pkg/tool/fs_test.go:TestEditFile_MalformedArgs_ReturnsErrEditFileArgsParse`, `TestEditFile_MissingPath_ReturnsErrEditFileMissingPath` — `errors.Is`로 parse/missing-path sentinel을 고정.
- 각 도구의 `Schema_*`/`Name` 테스트 — schema JSON 구조와 도구명을 고정.

## Manual Verification

- [ ] `make test` 실행 시 `pkg/tool` 패키지가 PASS하는지 확인(자동 검증 통과됨, 병합 전 재확인 권장).
- [ ] `TestReadFile_LargeFile_TruncatesAndOffloads`가 생성하는 임시 오프로드 파일 경로가 OS temp 디렉터리(`/var/folders/.../rein-*/output.txt` 또는 `$TMPDIR/rein-*/output.txt`) 하위인지 육안으로 1회 확인 — 자동 정리 없음은 의도된 동작(Step 2 정책).
- [ ] `edit_file` 다중 매칭 에러 메시지가 `"N matches found; provide more surrounding context"` 형태인지 실제 출력 1회 확인.

## Deviations from Plan

- **write_file content 검증 완화**: sub-plan은 "Both required"로 기술했으나, Execute 단에서 content 빈 문자열을 유효한 빈 파일 쓰기로 허용했다. JSON schema의 `required`는 LLM 측 필드 존재 보장이고, Execute는 path만 강제 검증한다. 이유: 빈 파일 생성은 합법적 연산이며 `*string` 분기는 과도한 추상화. 해당 테스트(`TestWriteFile_EmptyContent_CreatesEmptyFile`)로 동작을 고정.

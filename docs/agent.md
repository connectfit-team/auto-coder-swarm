# 우리 에이전트 — 어떻게 만들어졌고 어디를 고치면 되나

도구를 쥔 에이전트를 자체 LLM 으로 돌린다. codex·Claude Code 와 **같은 모양**이고,
다른 점은 창이 작다는 것뿐이다. 그 하나가 설계의 거의 전부를 정한다.

## 먼저 알아야 할 것 — 잰 값 (추측 아님)

| | 값 | 어디서 왔나 |
|---|---|---|
| 모델 | **Qwen2.5-Coder-32B-Instruct-AWQ** | `curl localhost:8000/v1/models` (이름만 `gemma4:31b`) |
| 창 | **16,384 토큰** | vllm.service 의 `--max-model-len` |
| 네이티브 `tool_calls` | **안 된다 — 늘 빈 배열** | 같은 요청 3회, 3/3 |
| REA MCP 도구 | 170개 | `agent -list-tools` |
| REA 도구 스키마 전체 | **약 931,000 토큰 = 창의 57배** | 스키마 글자수 × 토큰 어림 |
| 가장 큰 도구 하나 | `compare_web_captures` 75,501자 | 같은 측정 |

**창이 16k 라는 것이 이 설계의 전부다.** 상용 에이전트는 200k 를 전제한다.
그래서 기능을 그대로 베끼면 안 되고, 같은 **결정**을 우리 크기로 다시 내려야 한다.

### 네이티브 도구 호출이 왜 안 되나

vLLM 은 `--tool-call-parser hermes` 로 떠 있어 모델이 `<tool_call>` 을 내기를
기다린다. 그런데 Qwen 템플릿에서 `<tools>` 는 **도구 목록을 감싸는 입력 태그**고,
이 모델이 **둘을 헷갈려 입력 태그를 제 출력에 흉내 낸다.** 템플릿은 정상이다
(`tool_call` 이 14번 들어 있다). 모델이 흘리는 것이다.

→ 고치는 길은 **파서를 넓히는 것**(`internal/agent/toolcalls.go`). vLLM 을 다시
띄울 필요가 없고, CIE 도 같은 까닭으로 자체 파서를 쓴다.

## 돌리는 법

    LLM_DIRECT_URL=http://localhost:8000 \
      ./bin/agent -config ~/.acs/mcp.json -goal "..."

    ./bin/agent -config ~/.acs/mcp.json -list-tools   # 붙은 도구만 본다

`-config` 는 **codex·Claude Code 가 쓰는 그 형식**이다. 설정을 그대로 옮겨 올 수 있다.

    {"mcpServers": {"rea": {"command": "...", "args": ["-y","rea-agents@latest","--mcp"]}}}

| 깃발 | 뜻 |
|---|---|
| `-defer-tools` (기본 켬) | 스키마를 미리 안 알리고 `find_tools` 로만 찾게 한다 |
| `-turns` | 턴 예산 |
| `-list-tools` | 붙은 도구만 보여 주고 끝낸다 (LLM 안 씀) |
| `-verify` | 서버가 내놓는 것을 다 받는지 전수로 본다 (LLM 안 씀) |

## REA 가 내놓는 것을 다 받나 (2026-10-13 재 봤다)

    capabilities: {"tools":{"listChanged":true},"completions":{},"prompts":{"listChanged":true}}

| REA 가 내놓는 것 | 우리 |
|---|---|
| `tools` 170개 | **다 받는다.** 안 알리는 36개도 `find_tools` 로 찾고 부를 수 있다 |
| `instructions` 304자 | **받는다**(#220) — 지시문에 싣는다 |
| `prompts` 6개 (작업 절차) | **받는다**(#220) — 이름만 알리고 `get_workflow` 로 본문을 받는다 |
| `resources` | REA 가 **안 내놓는다**(Method not found) |
| `tools.listChanged` 알림 | **안 받는다** — 한 판 도는 동안 도구가 바뀌지 않는다 |

**말로 답하지 말고 돌려서 답한다:**

    ./bin/agent -config ~/.acs/mcp.json -verify

LLM 을 안 쓰므로 측정이 도는 중에도 돌릴 수 있다. 못 받는 것이 있으면 1 로 끝난다.

    ✓ 이름으로 부를 수 있다: 170/170
    ✓ 안 알리는 36개도 find_tools 로 찾힌다 (알리는 것 134개)
    ✓ 스키마가 있다: 170/170
    ✓ 작업 절차를 받아 온다: 6/6
    ✓ 쓰는 법을 받았다

`prompts` 는 도구 하나가 아니라 **일 전체의 차례**다:
`investigate_feature` · `compare_application_versions` · `verify_reconstruction` ·
`trace_crash` · `audit_residual_unknowns` · `prepare_bounded_process_capture`.

## 어디에 무엇이 있나

| 파일 | 하는 일 |
|---|---|
| `internal/mcpclient/client.go` | MCP stdio 서버 하나와 말한다. initialize·tools/list·tools/call 셋뿐 |
| `internal/mcpclient/config.go` | `mcpServers` 설정을 읽고 서버를 띄운다. **하나가 안 떠도 나머지로 간다** |
| `internal/mcpclient/pick.go` | 창에 맞게 도구를 고르고(`Pick`), 혼자 창을 먹는 것을 숨기고(`Visible`), 모델이 찾게 한다(`FindTools`) |
| `internal/agent/toolcalls.go` | 모델이 낸 도구 호출을 긁어낸다. 태그 네 가지 + 펜스 + 맨 JSON + 사내 형식 |
| `internal/agentloop/loop.go` | 루프 본체 |
| `cmd/agent/main.go` | 한 판 돌리는 명령 |

## 루프가 한 턴에 하는 일

1. 궤적을 창 안에 눌러 넣는다 (`trim`, **짝을 지켜** 자른다)
2. 남은 턴이 2 이하면 모델에게 알린다
3. 모델에게 묻는다
4. 도구 호출을 긁어낸다 (`ParseToolCalls`)
5. **없으면 거기서 끝이다** — 더 찾으라고 밀지 않는다
6. 있으면 부른다. 되풀이 호출·같은 결과는 건너뛰고 그렇게 말한다
7. 결과와 **오류를 같은 대화로** 되돌린다

## codex 와 대조 — 가져온 것과 안 가져온 것

원본: `~/oss/codex/codex-rs/core/src` (office).

| codex | 무엇 | 우리 |
|---|---|---|
| `mcp_tool_exposure.rs` `ToolExposure::Direct/Deferred/Hidden` | 도구 노출 세 단계 | **가져왔다** — `DeferAll` + `Visible` |
| `if search_tool_enabled { Deferred }` | 검색 도구가 있으면 **전부** 미룬다 | **가져왔다** — `-defer-tools` 기본 켬 |
| `MAX_..._SPEC_BYTES 8000` / `TOTAL 64000` | 도구 하나·전부 상한 | **가져왔다** — 창 비례로 2,500/5,000 |
| `thread_rollout_truncation.rs` | 턴 경계로 자른다 | **가져왔다** — 짝을 지켜 자른다 |
| `rollout_budget.rs` | 남은 예산을 모델에게 알린다 | **가져왔다** |
| `responses_retry.rs` (5초→60초) | 연결 재시도 | **안 가져왔다** |
| `compact*.rs` (7개 파일) | 요약으로 압축 | **안 가져왔다** — 지금은 앞에서 자르기만 |
| `turn_diff_tracker.rs` | 턴 누적 diff | ACS 에 이미 있다 (`turn_decls.go`) |
| `codex_delegate.rs` | 하위 에이전트 | ACS 에 이미 있다 |
| `guardian_review.rs` | 심사 | ACS 의 관문이 그 자리다 (`gates.go`) |
| `hook_runtime.rs` | 훅 | **안 가져왔다** |

aider 에서: 오류를 같은 대화로 되돌리는 것(`base_coder.py:924 run_one`,
`max_reflections = 3`). **가져왔다.**

## 말 — 번역하지 않는다

원본을 대조했다(2026-10-13):

| | 언어 처리 |
|---|---|
| codex | **없다.** `translate` 는 전부 「도구 호출→연산 변환」 같은 딴 뜻이다 |
| hermes | `translate_cwd_for_wsl_backend` — WSL 경로 변환이지 언어가 아니다 |
| aider | **번역하지 않는다.** 알아내서 프롬프트에 한 줄 적는다 |

aider 는 `--chat-language` → `locale` → `LANG`/`LANGUAGE`/`LC_ALL`/`LC_MESSAGES`
차례로 보고 `Reply in Korean.` 을 넣는다(`base_coder.py:1094`). 못 알아내면
`"the same language they are using"`. **사용자 글은 그대로 보낸다.**

우리도 번역하지 않는다. 다만 **사람에게 하는 말과 기계에게 하는 말을 가른다** —
도구 이름·설명이 영어면 찾을 낱말도 영어라야 한다. 안 가르면 둘 다 반만 된다
(실측: `find_tools` 질의가 한국어로 나가 하나도 안 맞았다, #210).

`SWARM_REPLY_LANGUAGE` 로 손으로 정할 수 있다.

## 하지 마라 — 이미 재 봤다

- **도구를 안 불렀을 때 「더 찾아라」 고 밀지 마라.** CIE 가 40문항으로 쟀다 —
  정답률 52%→48%, 235초→420초, 도구 호출 1.9회→1.5회. 산문 턴만 하나 더 늘었다.
- **목표만 보고 도구를 골라 주지 마라.** 목표는 한국어고 카탈로그는 영어라
  맞는 낱말이 0개가 되고, 그러면 「앞에서부터 N개」로 조용히 퇴화한다. 실측에서
  엉뚱한 6개가 나갔다. codex 도 검색 도구가 있으면 **안 고른다.**
- **스키마를 `<tools>` 로 감싸 알리지 마라.** 이 모델은 가까이 본 태그를 흉내 낸다.

## REA 는 이 기계에서 다 돈다 (2026-10-13)

    npx rea-agents@latest doctor
    healthy: true   environment_healthy: true

Ghidra 12.1.4 를 깔았다 — `~/ghidra_12.1.4_PUBLIC`. ghidra·platform·architecture·
installation·version·**headless**·**native_decompiler**·java 전부 healthy.
Hopper·IDA 는 상용이라 안 깔았고, 그래도 전체는 healthy 다.

우리 에이전트가 쓰게 `~/.acs/mcp.json` 의 `env` 에 `GHIDRA_INSTALL_DIR` 을 넣었다.
Ghidra 를 옮기거나 올리면 **거기도 같이 고쳐야 한다.**

### 끝까지 돌아간 증거

목표: 「바이너리 /bin/true 를 열어서 그 안에 있는 함수 목록을 확인해 줘」

    [MCP] rea: 도구 170개
    [TOOLS] 혼자 창을 먹는 도구 36개는 숨긴다
    [TOOLS] 도구 170개(보이는 것 134개) 가운데 1개를 알린다
    [TOOL] find_tools → … / [TOOL] open_binary → …

답에 `__fpending`·`dcgettext`·`mbrtoc32`·`__stack_chk_fail` 이 나왔다 —
coreutils 의 **실제 임포트**다. 지어낸 것이 아니다.

### 함정 — `JAVA_HOME` 이 없으면 Ghidra 가 조용히 안 돈다

`rea doctor` 는 **healthy 라고 한다.** doctor 는 스스로 Java 를 찾아내기 때문이다.
그런데 MCP 서버로 띄우면 그 환경이 안 넘어가서 도구가 이렇게 답한다:

    provider_unavailable — The selected analysis provider is unavailable.

`~/.acs/mcp.json` 의 `env` 에 **둘 다** 넣어야 한다:

    "GHIDRA_INSTALL_DIR": "/home/cnf/ghidra_12.1.4_PUBLIC",
    "JAVA_HOME": "/usr/lib/jvm/java-21-openjdk-amd64"

넣기 전 7턴·오류 3개 → 넣은 뒤 **4턴·오류 0개**.

### 함정 — 도구 이름

이름은 CLI 와 다르다. `capabilities` 는 **CLI 명령이고 MCP 도구가 아니다** —
모델이 목표 문구를 보고 그 이름을 지어내 불렀다. 지금은 비슷한 이름을 함께
일러 준다(#215).

## 아직 거친 곳 (다음에 할 일)

1. ~~도구를 찾고 나서 안 부른다~~ — **고쳤다(#214).** 까닭은 `find_tools` 가
   이름과 설명만 줘서 모델이 **인자를 몰랐던** 것이다. codex 는 검색 결과에
   입력 속성 이름을 싣는다(`mcp.rs build_mcp_search_text`). 그대로 가져온 뒤
   모델이 맞는 도구를 찾아 불렀다.
2. **턴을 찾기에 많이 쓴다.** 갈래를 보여 주고(#216) 나아졌지만 아직 더듬는다.
3. **압축이 없다** — 길어지면 앞을 버릴 뿐 요약하지 않는다. codex 의 `compact*`,
   hermes 의 `trajectory_compressor` 가 본보기.
4. **연결 재시도가 없다.**

## 고칠 때 지킬 것

- **판수를 모아 재고 고친다.** 4판 표본으로 고치다 관문 오탐을 넷 만들었다.
  12판 이상, **두 번 이상 나오는 까닭만** 고친다.
- **돌려 보고 고친다.** #210·#211·#212 는 전부 실제로 돌려 보고서야 보였다.
  책상에서는 안 보였다.
- **안 쟀으면 안 쟀다고 적는다.**

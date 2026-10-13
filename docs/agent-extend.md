# 고치는 법 — 짧은 안내

`docs/agent.md` 가 「왜 이렇게 생겼나」 라면, 이것은 「무엇을 어디에 더하나」 다.

## MCP 서버를 하나 더 붙인다

`~/.acs/mcp.json` 에 한 칸 더한다. 코드는 안 고친다.

    {"mcpServers": {
      "rea": {"command": "...", "args": ["-y","rea-agents@latest","--mcp"]},
      "ckh": {"command": "/path/to/ckh-mcp", "args": []}
    }}

- 이름이 겹치는 도구는 `서버이름__도구이름` 으로 갈린다(`AllTools`).
- 하나가 안 떠도 나머지로 간다. 못 뜬 것은 로그에 남는다.
- `"disabled": true` 로 꺼 둘 수 있다.

## 내장 도구를 더한다 (MCP 를 안 거치는 것)

`find_tools` 가 본보기다(`internal/agentloop/loop.go`).

1. `mcpclient.Tool` 값을 하나 만든다 — 이름·설명·JSON 스키마.
2. `Loop.dispatch` 에 그 이름을 가로채는 갈래를 더한다.
3. `Run` 에서 `picked` 앞에 넣는다.

**손의 관문을 도구로 다는 자리가 여기다.** `t.gateChecks(diff)` 를 부르는
`check` 도구를 만들면, 모델이 한 턴 안에서 위반을 보고 고친다 — 지금처럼 시도
하나를 통째로 버리지 않는다. 관문은 이미 `internal/orchestrator/gates.go` 한 곳에
모여 있다(#205). **판정을 두 곳에서 하지 마라** — 도구가 통과라 한 것을 관문이
막으면 모델은 고칠 길이 없다.

## 다른 모델로 바꾼다

`internal/llm/FromEnv` 한 곳에서만 정한다. 바꾸기 전에 **다시 재라**:

    curl -s localhost:8000/v1/models                    # 창과 실제 모델
    # tools 를 넣어 한 번 불러 보고 tool_calls 가 차는지 본다

창이 달라지면 `MaxToolChars`·`MaxTotalChars`·`ResultTokens` 를 **비례로** 옮긴다.
codex 값(8,000/64,000)은 200k 창 기준이다.

## 도구 결과가 너무 길 때

`Loop.ResultTokens` (기본 1200). 창이 16k 라 결과 하나가 2,000자를 넘으면
금세 궤적을 밀어낸다. 잘린 것은 「…(잘렸다)」 로 알린다 — **조용히 자르지 마라.**

## 시험하는 법

- 루프는 가짜 LLM 으로 시험한다(`internal/agentloop/loop_test.go` 의 `fakeLLM`).
  미리 적어 둔 답을 차례로 내놓고, 받은 대화를 적어 둔다 — **다음 턴이 도구
  결과를 실제로 보고 있는지**까지 본다.
- MCP 클라이언트는 가짜 서버로 시험한다(파이프, 자식 프로세스 없이).
  알림 끼어들기·오류 응답을 같이 본다.
- 파서 시험은 **실제로 관측된 글자**를 쓴다. 지어낸 입력으로 시험하면 모델이
  진짜로 내는 꼴을 못 잡는다.

## 실제로 한 판 돌려 본다 (가장 중요)

    LLM_DIRECT_URL=http://localhost:8000 ./bin/agent -config ~/.acs/mcp.json \
      -turns 6 -goal "..."

#210·#211·#212 는 **전부 돌려 보고서야 보였다.** 단위시험은 다 통과하고 있었다.

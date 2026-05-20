# 🛡️ 에이전트 작업 범위 제한 (Mandate)

1. **배타적 작업 범위**: 본 에이전트는 오직 `/home/cnf/projects/auto-coder-swarm` 디렉토리 내의 파일만 수정(`replace`, `write_file`)할 수 있습니다.
2. **타 프로젝트 수정 금지**: `code-insight-engine` 등 외부 프로젝트의 코드를 직접 수정하는 행위는 엄격히 금지됩니다.
3. **연동 필요 시 절차**: 외부 프로젝트의 기능 추가나 API 변경이 필요한 경우, 직접 수정하지 말고 반드시 **사용자에게 "인계 문서(Handoff) 작성" 및 "타 에이전트 요청 제안"**을 Inquiry로 먼저 보고해야 합니다.
4. **절대 원칙**: "연동 아키텍처 구축"이라는 명분으로도 이 규칙을 위반할 수 없습니다.

# Project Instructions: Auto-Coder Swarm

## Remote Execution Policy
- **Host**: `192.168.120.54` (cnf)
- **Policy**: All shell commands, build procedures, and workspace manipulations must be performed on this remote host via SSH.
- **Exception**: Document updates (e.g., this `GEMINI.md`) should be maintained in synchronization with the local workspace.

## Engineering Standards
- **Branch Protection**: NEVER commit directly to `main` or `master`. All changes must occur in a unique feature branch and be submitted via Pull Request.
- **Language**: Go 1.23+
- **Architecture**: Layered Architecture (cmd/app, internal/agent, internal/workspace, etc.)
- **Conventions**:
  - Use explicit Error Handling (no ignoring errors).
  - Use Manual Tool Tagging (`<|tool_call|>`) for LLM interactions to maintain consistency with the Engine.
  - Asynchronous task processing for concurrent user requests.
- **Validation**: Always run `go build` before restarting services.

## AI Engine & Tools
- **Engine**: Ollama (Gemma 4 31B) via `code-insight-engine` Oracle.
- **Tools**: Every code modification MUST be reviewed by a separate 'Reviewer' agent.

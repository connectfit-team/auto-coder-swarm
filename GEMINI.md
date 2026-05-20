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

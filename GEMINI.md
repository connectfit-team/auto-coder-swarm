# Auto-Coder Swarm: Guidelines & Context

## Project Structure
- `cmd/swarm/main.go`: Entry point, orchestrates workers and API/Dashboard.
- `internal/agent/`: Multi-agent definitions (Planner, Coder, Critic, Reviewer, RiskAssessor).
- `internal/orchestrator/`: State machine and logic for task execution.
- `internal/storage/`: GORM/SQLite storage layer for tasks and logs.
- `internal/stream/`: SSE manager for real-time CoT broadcasting.
- `internal/voter/`: Multi-model consensus engine.
- `internal/workspace/`: Git Worktree based isolated sandboxing.

## Operational Standards
- Always use `X-API-Key` for API calls when `SWARM_API_KEY` is set.
- Agent thoughts are logged to `agent_thoughts.log` and broadcasted via SSE.
- Workspaces are created under `/tmp` and automatically cleaned up.
- Master repositories are located at `/home/cnf/projects/code-insight-engine/repos`.

## Verification Checklist
- [ ] Compiles: `go build ./...`
- [ ] Lint: No critical issues.
- [ ] Tests: `go test ./...`

---
*Last Updated: 2026-05-20*

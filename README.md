# Auto-Coder Swarm

A Go-based Multi-Agent system for autonomous code modification. It communicates with the read-only `code-insight-engine` to gain context, modifies code in strictly isolated ephemeral workspaces, reviews changes, and generates Pull Requests.

## Architecture
- **cmd/swarm**: Entry point for the service (event listener for change requests).
- **internal/workspace**: Manages isolated ephemeral directories for concurrent user requests.
- **internal/insightclient**: Client to communicate with the Oracle (`code-insight-engine`).
- **internal/agent**: LLM-driven expert agents (Planner, Coder, Reviewer, RiskAssessor).
- **internal/orchestrator**: Pipeline manager routing the task through the agents.
- **internal/gitmgr**: Handles Git operations (clone, commit, push, PR creation).

## Core Principles
1. **Zero Interference**: Every request runs in an isolated `/tmp/workspace_<uuid>`.
2. **Oracle Dependency**: Uses `code-insight-engine` for discovery, never searches blindly.
3. **Safety First**: Reviewer and Risk Assessor agents must approve before PR generation.

# 🤖 AI Handoff Document: Auto-Coder Swarm (Production Ready)

**Target Audience:** Future AI Agents & Maintainers
**Date:** 2026-05-20
**Status:** Phase 6 (Stateless Connectivity) - Mature Node

---

## 1. Environment & Architecture
- **Host:** 192.168.120.54 (cnf)
- **Tech Stack:** Go 1.25+, SQLite (GORM), GitHub CLI (gh), Ollama (Gemma 4 31B)
- **Service:** Listening on `:8006` (HTTP Stateless API)
- **Path:** `/home/cnf/projects/auto-coder-swarm`

## 2. Capabilities (Steps 1-19)
- **Instant Sandboxing**: Hardlink-based (`cp -al`) repo replication from master cache in `< 1s`.
- **Parallel Multi-Agent Audit**: Reviewer and RiskAssessor run concurrently using Go routines.
- **Oracle Integration**: Deep technical context acquisition from `code-insight-engine`.
- **Conflict Prevention**: SQLite-backed repo-level locking for safe concurrency.
- **Dynamic Verification**: In-sandbox `go build` check to ensure code compilation.
- **Real PR Automation**: Automated branch creation, commit, push, and `gh pr create`.
- **Stateless API**: Supports structured JSON (`StatelessRequest`) to skip Oracle or force repo/constraints.

## 3. Operations & Debugging
- **Main Entry:** `cmd/swarm/main.go` (Worker pool & Job queue).
- **Primary Logs:** `service.log` (System state, networking, queue status).
- **Agent Thoughts:** `agent_thoughts.log` (Full prompts and raw LLM reasoning).
- **Recovery:** Service automatically resets `RUNNING` jobs to `PENDING` and clears locks on startup.

## 4. Immediate Next Tasks
- **Step 21: Auto-Test Generation**: Enhance Coder agent to generate unit tests alongside fixes.
- **Step 20: External Bridge (gRPC)**: Upgrade the simple HTTP listener to a formal gRPC/REST interface.
- **Cleanup Policy**: Ensure `/tmp/swarm_ws_*` are monitored if the service crashes during cleanup.

## 5. Security Policy
- **Branch Protection**: NEVER commit directly to `main` or `master`. Feature branches and PRs only.
- **Zero Token Leak**: Uses local `gh auth` keychain; no secrets stored in code.

---
*Verified by Gemini CLI Engineer. Ready for Hand-off.*

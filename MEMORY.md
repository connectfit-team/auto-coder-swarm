# Project Memory - Code-Insight Engine

## Architectural Decisions

### Robust Infrastructure Trigger Logic (2026-05-19)
- **Problem**: Strict keyword matching for "kubernetes" or "k8s" caused missed triggers due to typos (e.g., "kuberntes").
- **Solution**: Use partial string matching for core terms. 
    - `kuber` catches `kubernetes`, `kuberntes`, `kubernets`.
    - `k8` catches `k8s`, `k8`.
- **Implementation**: Updated `infraAllowed` logic in `internal/business/service.go` to include these partial matches along with `ingress` and `deployment`.
- **System Prompt**: Reinforced this logic by explicitly telling the LLM it has permission to search `kubernetes-manifests` when these topics are mentioned, even with typos.

### Management Dashboard Implementation (2026-05-20)
- **Decision**: Implement a Go-based web dashboard (SSR) on port 8005 for project oversight.
- **Functionality**:
    - **Git Timeline**: Tracks engine development history.
    - **Live Editing**: Direct updates to `PROJECTS.md` and `PROGRESS.md` via web forms.
    - **Process Monitoring**: Real-time view of the `AnalysisTask` queue status.
- **Architecture**: Unified process model where the Slack Bot and Web Server share the same `AnalysisService` instance.

## Auto-Coder Swarm (Hands)

### Enterprise API Security (2026-05-20)
- **Decision**: Protect the Swarm API with X-API-Key headers.
- **Implementation**: Added `checkAuth` middleware to `internal/api/handler.go`.
- **Configuration**: Uses `SWARM_API_KEY` environment variable. If empty, allows all requests (Dev mode).

### Live Chain-of-Thought (CoT) Streaming
- **Technology**: Server-Sent Events (SSE).
- **Endpoint**: `GET /task/stream?id=X`.
- **Integration**: `internal/agent` package broadcasts reasoning chunks to the `internal/stream` manager.

### Unified Interaction Dispatcher (2026-05-21)
- **Decision**: Centralize all interaction channels (Slack, Web, API) through a single `InteractionDispatcher.Dispatch` method.
- **Problem**: Inconsistent history management and task tracking between Slack and Web interfaces.
- **Solution**: 
    - Created `internal/interaction.Dispatcher` interface to break cyclic dependencies.
    - Standardized feedback via `FeedbackHandler` callbacks.
    - Automated task (WorkID) creation for all real-time requests.
- **Benefit**: 100% logic consistency across all input sources and centralized observability on the dashboard.

### Deep Observability with Prometheus (2026-05-21)
- **Decision**: Implement a real-time monitoring layer using Prometheus to track task latency, agent success rates, and token usage without affecting performance.
- **Implementation**:
    - Created `internal/observability` for metric definitions.
    - Exposed `/metrics` endpoint on port :8006.
    - Tracked: `swarm_task_duration_seconds`, `swarm_agent_operations_total`, `swarm_llm_token_usage_total`, and `swarm_active_workers`.
- **Benefit**: Provides data-driven insights into system bottlenecks and LLM costs. Zero performance impact due to asynchronous, lightweight collection.



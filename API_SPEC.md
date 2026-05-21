# 🔌 API Specification: Auto-Coder Swarm (Headless)

이 문서는 외부 대시보드 및 연동 시스템을 위한 API 명세를 제공합니다. 모든 엔드포인트는 CORS를 지원하며, `X-API-Key` 인증이 필요합니다.

## 1. Authentication
모든 쓰기(POST) 요청 및 설정 조회(GET) 요청에는 헤더에 `X-API-Key`를 포함해야 합니다.

## 2. Task Management

### 2.1 List Tasks
전체 작업 목록을 조회합니다.
- **URL**: `GET /api/v1/tasks`
- **Response**: `JSON Array of SwarmTask`

### 2.2 Get Task Detail
특정 작업의 상세 정보 및 실행 로그를 조회합니다.
- **URL**: `GET /api/v1/tasks/detail?id={task_id}`
- **Response**: `{ "task": {...}, "logs": [...] }`

### 2.3 Submit Task
새로운 자동 수정 작업을 제출합니다.
- **URL**: `POST /api/v1/tasks`
- **Body**: `orchestrator.StatelessRequest`
- **Response**: `{ "task_id": 1, "status": "PENDING" }`

### 2.4 Stop Task
실행 중인 작업을 즉시 중단합니다. 작업 상태는 `CANCELLED`로 변경됩니다.
- **URL**: `POST /api/v1/tasks/stop?id={task_id}`
- **Response**: `Plain Text Confirmation`

## 3. Real-time Streaming

### 3.1 CoT Stream (SSE)
에이전트의 사고 과정을 실시간으로 스트리밍합니다.
- **URL**: `GET /task/stream?id={task_id}`
- **Protocol**: `Server-Sent Events (SSE)`
- **Data Format**: `data: { "agent": "Planner", "message": "..." }`

## 4. System Settings

### 4.1 Get Settings
현재 모델 설정 및 Ollama 가용 모델 목록을 조회합니다.
- **URL**: `GET /api/v1/settings`
- **Response**: `{ "available_models": [...], "primary_model": "...", "voter_models": [...] }`

### 4.2 Update Settings
모델 구성을 동적으로 변경합니다.
- **URL**: `POST /api/v1/settings`
- **Body**: `{ "primary_model": "...", "voter_models": [...], "swarm_api_key": "..." }`

---
*Last Updated: 2026-05-21*

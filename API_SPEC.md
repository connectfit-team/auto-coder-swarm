# 🔌 API Specification: Auto-Coder Swarm (Normalized)

모든 엔드포인트는 CORS를 지원하며, `X-API-Key` 인증이 필수입니다. 모든 작업 식별자는 `W-XXXXXX` 형식을 따릅니다.

## 1. Authentication
- Header: `X-API-Key: {your_secret}`

## 2. Task Management

### 2.1 List Tasks
- **URL**: `GET /api/v1/tasks`
- **Response**: `JSON Array of SwarmTask` (ID: string)

### 2.2 Get Task Detail
- **URL**: `GET /api/v1/tasks/detail?id={work_id}`
- **Example**: `/api/v1/tasks/detail?id=W-54281`

### 2.3 Submit Task (Programmatic)
- **URL**: `POST /api/v1/tasks`
- **Body**: `orchestrator.StatelessRequest`
- **Response**: `{ task_id: W-54281, status: PENDING }`

### 2.4 Submit Chat (Conversational)
- **URL**: `POST /api/v1/chat`
- **Body**: `{"message": "user request text"}`
- **Response**: `{ task_id: W-54281, status: PENDING, message: "Task created from chat" }`

### 2.5 Stop Task & Cascade CIE (Deep Stop)
- **URL**: `POST /api/v1/tasks/stop?id={work_id}`

## 3. Real-time Streaming
- **URL**: `GET /task/stream?id={work_id}`
- **Protocol**: `SSE`
- **Data**: `data: { agent: Planner, message: ... }`

---
*Last Updated: 2026-05-21 (Chat API Integration & Handler Modularization)*

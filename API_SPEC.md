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

## 4. Event Architecture (NATS JetStream)
Zero-Latency 연동을 위한 이벤트 통신 명세입니다.

### 4.1 NATS Configuration
- **Server**: `nats://localhost:4222`
- **Stream**: `SWARM_EVENTS`
- **Storage**: `FileStorage` (Durable)

### 4.2 Published Subjects
- `swarm.analysis.done.<work_id>`: CIE가 분석 완료 시 발행. Swarm은 이를 수신하여 Polling 없이 즉시 결과를 조회(GET /api/tasks/result)합니다.
  - Payload Schema (JSON):
    ```json
    {
      "work_id": "W-XXXXXX",
      "status": "completed",
      "completed_at": "2026-05-22T15:00:00Z"
    }
    ```

---
*Last Updated: 2026-05-22 (Hybrid API + NATS JetStream Integration)*

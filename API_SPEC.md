# 🔌 API Specification: Auto-Coder Swarm (Normalized)

모든 작업 식별자는 `W-XXXXXX` 형식을 따릅니다.

## 1. 인증

`/api/v1/*` 는 전부 `X-API-Key` 를 봅니다. 길 목록 하나로 등록하고 그 목록을
감싸므로 새 길을 더해도 빠지지 않습니다(`internal/api/handler.go` 의 `routes`).

- 헤더: `X-API-Key: {열쇠}`
- **열쇠가 정해져 있지 않으면 전부 통과입니다.** 그 상태로 도는 것은 시작
  기록에 경고로 남습니다. `SWARM_REQUIRE_API_KEY` 를 주면 열쇠 없이는 뜨지
  않습니다. 열쇠를 정하는 법은 DEPLOYMENT.md 를 봅니다
- 브라우저의 예비 요청(`OPTIONS /api/v1/...`)은 열쇠를 묻지 않습니다 —
  예비 요청에는 열쇠가 없고, 막으면 본 요청이 오지 않습니다
- CORS 는 `Access-Control-Allow-Origin: *` 입니다. 열쇠를 헤더로만 받으므로
  다른 사이트의 스크립트는 열쇠 없이 부를 수 없습니다

### 대시보드(`/`, `/settings`, `/task/...`)

브라우저는 헤더를 못 붙입니다. 같은 열쇠를 `/unlock` 에서 한 번 넣으면 그
브라우저는 계속 쓸 수 있습니다(쿠키에는 열쇠가 아니라 지문이 담깁니다).
열쇠를 바꾸면 예전 쿠키는 그 자리에서 못 씁니다.

## 2. Task Management

### 2.1 List Tasks
- **URL**: `GET /api/v1/tasks`
- **Response**: `JSON Array of ACSTask` (ID: string)

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
*갱신: 2026-09-07 (인증을 코드에 맞춤)*

## 4. Event Architecture (NATS JetStream)
Zero-Latency 연동을 위한 이벤트 통신 명세입니다.

### 4.1 NATS Configuration
- **Server**: `nats://localhost:4222`
- **Stream**: `SWARM_EVENTS`
- **Storage**: `FileStorage` (Durable)

### 4.2 Published Subjects
- `swarm.analysis.done.<work_id>`: CIE가 분석 완료 시 발행. ACS은 이를 수신하여 Polling 없이 즉시 결과를 조회(GET /api/tasks/result)합니다.
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

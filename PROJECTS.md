# 📘 Projects: Auto-Coder Swarm (Master Spec)

## 1. 프로젝트 목적 (Purpose)
code-insight-engine의 분석 결과를 바탕으로 실제 코드를 수정하고, 다중 에이전트 협업을 통해 검증하며, 최종적으로 Pull Request를 자동 생성하는 자율형 코드 수정 플랫폼입니다.

## 2. 프로젝트 로드맵 (Roadmap)

### Phase 1~3: 기반 아키텍처 및 자율 스웜 (Completed)
- UUID 격리 환경, Git 연동, 멀티 에이전트(Planner, Coder, Reviewer) 루프 구축.

### Phase 4~6: 운영 신뢰성 및 확장성 (Completed)
- SQLite 작업 큐, 레포지토리 락(Lock), 사고 과정 중계, 무상태(Stateless) API 구축.

### Phase 7: 자가 치유 및 협업 (Completed)
- 빌드 에러 자가 치유, 대시보드 기반 Human-in-the-loop 승인 체계 구축.

### Phase 8: 전략적 지능 및 관찰성 (Active Verification)
- [x] Step 25: Performance Regression Test: 벤치마크 기반 성능 저하 감지.
- [x] Step 26: Multi-Model Swarm Voting: Gemma/Llama/Qwen 다수결 투표 시스템.
- [x] Step 28~29: Advanced Monitoring & Diff: 실시간 로그 및 시각적 코드 검토 기능.
- [x] Step 30: Multi-Worker Parallelism: 3x 동시 워커 및 원자적 작업 수주.
- [x] Step 31: Agentic Self-Healing Pro: 에러 로그 분석 기반 지능형 수리.
- [x] Step 32: Instant Sandboxing: Git Worktree를 이용한 밀리초 단위 작업 공간 생성.
- [x] Step 33: Agentic Dialogue & Negotiation: 비판 에이전트와의 토론을 통한 계획 정밀화.
- [x] Step 34: CoT Streaming: 에이전트 사고 과정의 실시간 SSE 브로드캐스팅.

### Phase 9: 엔터프라이즈 엔지니어링 (Next)
- [x] Step 35: Enterprise API Security: X-API-Key 기반 인증 및 권한 관리.
- [ ] Step 36: Persistence & Recovery: 단계별 작업 복구 및 세션 유지.
- [ ] Step 37: Prometheus Metrics: 시스템 성능 지표 시각화.
- [ ] Step 38: Swarm Activity Reporting: 일일 브리핑 및 자동 요약 리포트.

## 3. 설계 철학 (Design Philosophy)
- Isolation-First: Git Worktree를 통한 깨끗하고 빠른 격리 환경 유지.
- High-Precision: 다중 모델 합의, 에이전트 간 토론, 자가 치유를 통한 무결점 지향.
- Observability: SSE 실시간 스트리밍을 통한 투명한 AI 사고 과정 노출.

## 4. 시스템 구조 (Architecture)
- /cmd/swarm: 진입점 및 워커 제어.
- /internal/voter: 집단 지성 투표 엔진.
- /internal/agent: Planner(Refiner), Coder(Debugger), Critic, Reviewer, RiskAssessor.
- /internal/stream: CoT 실시간 SSE 중계 서버.
- /internal/storage: SQLite 기반 데이터 영속성 및 타임라인 로그.
- /internal/web: 한글 관리 대시보드 (Port 8006).

---
*본 문서는 시스템의 최종 명세서이며, 모든 개발 단계의 이력을 포함합니다.*
*최종 갱신: 2026-05-20*

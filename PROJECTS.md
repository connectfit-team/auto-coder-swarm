# 📘 Projects: Auto-Coder Swarm (Master Spec)

## 1. 프로젝트 목적 (Purpose)
code-insight-engine의 분석 결과를 바탕으로 실제 코드를 수정하고, 다중 에이전트 협업을 통해 검증하며, 최종적으로 Pull Request를 자동 생성하는 자율형 코드 수정 플랫폼입니다.

## 2. 프로젝트 로드맵 (Roadmap)

### Phase 1~7: 기반 구축 및 자율성 확보 (Completed)
- UUID 격리, Git 연동, SQLite 작업 큐, 자가 치유(Self-Healing), Human-in-the-loop 승인 체계.

### Phase 8: 전략적 지능 및 관찰성 (Completed)
- [x] Step 25~26: 성능 벤치마크 및 다중 모델(Gemma/Llama/Qwen) 투표 시스템.
- [x] Step 30~32: 3x 동시 워커, 지능형 수리, Git Worktree 샌드박싱.
- [x] Step 33~34: 에이전트 간 토론(Dialogue) 및 실시간 사고 과정(CoT) SSE 스트리밍.

### Phase 9: 엔터프라이즈 엔지니어링 (Active)
- [x] **Step 35: Enterprise API Security**: X-API-Key 기반 인증 구현.
- [x] **Step 36: Persistent CoT & Log Policy**: 사고 과정 DB 영구 저장 및 100MB 단위 로그 로테이션(최대 10개 유지).
- [ ] **Step 37: Persistence & Recovery**: 작업 상태 상세 추적 및 충돌 후 자동 복구 강화.
- [ ] **Step 38: Prometheus Metrics**: 시스템 성능 지표(레이턴시, 워커 부하) 시각화.
- [ ] **Step 39: Daily Activity Reporting**: 작업 내역 자동 요약 및 보고.

## 3. 설계 철학 (Design Philosophy)
- Isolation-First: Git Worktree를 통한 깨끗하고 빠른 격리 환경 유지.
- High-Precision: 다중 모델 합의 및 에이전트 간 토론을 통한 무결점 지향.
- Transparency: DB 영구 저장 및 SSE 스트리밍을 통한 투명한 사고 과정 노출.
- Resource-Friendly: 100MB 단위 로그 로테이션 및 자동 정리를 통한 디스크 관리.

## 4. 시스템 구조 (Architecture)
- /cmd/swarm: 진입점 및 워커 제어.
- /internal/agent: 에이전트 논리 및 LLM 래퍼 (로그 로테이션 포함).
- /internal/stream: 실시간 SSE 중계 (히스토리 로드 지원).
- /internal/storage: SQLite 기반 영속성 (ThoughtLog 추가).

---
*최종 갱신: 2026-05-20 (Full Audit 완료)*

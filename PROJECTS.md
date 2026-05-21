# 📘 Projects: Auto-Coder Swarm (Master Spec)

## 1. 프로젝트 목적 (Purpose)
`code-insight-engine` (CIE, Eyes)의 분석 결과와 Swarm(Hands)의 지능형 실행력을 결합하여, 코드 수정-검증-PR 생성을 자동화하는 엔터프라이즈급 자율형 멀티 에이전트 시스템입니다.

## 2. 관리 핵심 문서 (Core Documents)
1. **PROJECTS.md**: 마스터 사양 및 중장기 로드맵.
2. **PROGRESS.md**: 현재 진행 단계, 성과 추적 및 운영(Systemd) 지침.
3. **API_SPEC.md**: 외부 연동을 위한 Headless REST API 명세 (W-XXXXXX ID 체계 준수).
4. **GEMINI.md**: 4단계 워크플로우 및 시스템 무결성(Functional Integrity) 4대 원칙.

## 3. 핵심 아키텍처 (Key Architecture)
- **4-Stage Intelligent Pipeline**: `INSPECTION`(규모 파악) -> `STRATEGY`(전략 수립) -> `ANALYSIS`(CIE 이해) -> `IMPLEMENTATION`(Swarm 구현) 체계.
- **Multi-Agent Governance**: `Planner`(설계), `Coder`(수행), `Critic`(리스크), `Reviewer`(기능 승인)의 상호 견제 체계.
- **Deep Observability (Prometheus)**: 에이전트별 소요 시간, 토큰 소모량, 성공률을 실시간 수집하여 `/metrics` 엔드포인트로 노출하는 모니터링 계층.
- **Advanced Self-Healing v2**: 빌드/테스트 실패 시 `Healer` 에이전트가 오류 로그를 분석하여 지능적 수정(의존성 설치, 코드 보정)을 수행하는 자가 치유 루프.

- **Context-Aware Lifecycle**: `exec.CommandContext`를 통한 즉각적인 작업 중단 및 안전한 자원 회수.

## 4. 로드맵 (Roadmap)
### Phase 9: 엔터프라이즈 엔지니어링 (Stable)
- [x] **Step 38: Headless API & SPEC**: REST API 기반 아키텍처 및 W-XXXXXX ID 체계 완료.
- [x] **Step 41: Adaptive Pipeline**: 4단계 지능형 워크플로우(Inspection-Strategy-Analysis-Implementation) 구축 완료.
- [x] **Step 44: Task Hardening**: 즉각적인 작업 중단 및 CIE 비동기 API 완전 동기화 완료.
- [x] **Step 45: Functional Integrity Mandate**: GEMINI.md에 무결성 보장 원칙 및 역할 분리(Eyes/Hands) 각인 완료.
- [x] **Step 46: Multi-Agent Consensus**: 4단계 거버넌스 및 리스크 검토 프로세스 고도화 완료.
- [x] **Step 47: Security Guardrails**: 코드 내 기밀 정보(Secrets) 및 취약점 자동 감지 엔진 구축 완료.
- [x] **Step 48: Advanced Self-Healing v2**: LLM 기반 오류 진단 및 자동 코드 보정 엔진 구축 완료.
- [x] **Step 42: Prometheus Metrics**: 작업 지연 시간, 에이전트 성공률 및 토큰 사용량 시각화 완료.
- [x] **Step 43: Swarm Activity Reporting**: 일일 작업 요약 및 브리핑 자동 생성 엔진 구축 완료.
- [x] **Step 49: Deep Observability & Raw Traceability**: RAW LLM Prompt 및 Response 로깅 체계 추가 완료.
- [x] **Step 50: Codebase Modularization**: Orchestrator 패키지 내 장문 파일 기능별 분리 완료 (`flow_analysis.go`, `flow_planning.go`, 등).
- [x] **Step 51: API/Web Modularization & Conversational UI**: `handler.go` 및 `dashboard.go` 분할 완료 및 대시보드 내 실시간 채팅 지시 기능(Chat API) 연동 완료.

---
*최종 감사: 2026-05-21 (대화형 인터페이스 구축 및 3차 코드베이스 모듈화 완료)*

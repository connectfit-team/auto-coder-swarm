# 📘 Projects: Auto-Coder Swarm (Master Spec)

## 1. 프로젝트 목적 (Purpose)
`code-insight-engine` (CIE)의 분석 결과를 바탕으로 실제 코드를 수정하고, 다중 에이전트 협업을 통해 검증하며, 최종적으로 Pull Request를 자동 생성하는 자율형 코드 수정 플랫폼입니다.

## 2. 관리 핵심 문서 (Core Documents)
1. **PROJECTS.md**: 마스터 사양 및 로드맵.
2. **PROGRESS.md**: 현재 진행 중인 단계 및 성과 추적.
3. **PROCESSES.md**: 시스템 운영 및 리소스 관리 프로세스.
4. **API_SPEC.md**: 외부 연동을 위한 Headless REST API 명세.
5. **GEMINI.md**: 시스템 무결성 보장(Functional Integrity) 원칙 및 통합 지침.

## 3. 핵심 아키텍처 (Key Architecture)
- **CIE-Async Integration**: CIE의 비동기 분석 API를 준수하며, 결과 폴링 및 고유 세션 ID 격리를 통해 무결성을 확보합니다.
- **Deep Inspection Timeline**: `ORACLE`(CIE 질의), `DETECTION_RAW`(LLM 응답 원본) 등 모든 의사결정 단계를 가감 없이 노출합니다.
- **Context-Aware Lifecycle**: `exec.CommandContext`를 전면 도입하여 하위 프로세스까지 즉시 제어 및 중단이 가능합니다.
- **LLM-Autonomous Classification**: `tree` 기반 구조 분석을 통해 프로젝트 정체를 파악하고 적응형 빌드 트랙을 자동 선택합니다.

## 4. 로드맵 (Roadmap)
### Phase 9: 엔터프라이즈 엔지니어링 (Stable)
- [x] **Step 38: Headless API & SPEC**: REST API 기반 아키텍처 완성.
- [x] **Step 41: LLM-Autonomous Classification**: 브레인 기반 프로젝트 판별 및 적응형 빌드 구축 완료.
- [x] **Step 44: Task Termination & CIE Sync**: 즉각적인 작업 중단 및 CIE 비동기 API 완전 동기화 완료.
- [x] **Step 45: Functional Integrity Mandate**: GEMINI.md에 무결성 보장 4대 원칙 각인 완료.
- [ ] **Step 42: Prometheus Metrics**: 작업 지연 시간 및 워커 부하 지표 시각화.
- [ ] **Step 43: Swarm Activity Reporting**: 일일 작업 요약 및 브리핑 자동 생성.

---
*최종 감사: 2026-05-21 (시스템 무결성 및 비동기 동기화 완료)*

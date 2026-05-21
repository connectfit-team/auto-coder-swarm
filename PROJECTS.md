# 📘 Projects: Auto-Coder Swarm (Master Spec)

## 1. 프로젝트 목적 (Purpose)
`code-insight-engine`의 분석 결과를 바탕으로 실제 코드를 수정하고, 다중 에이전트 협업을 통해 검증하며, 최종적으로 Pull Request를 자동 생성하는 자율형 코드 수정 플랫폼입니다.

## 2. 관리 핵심 문서 (Core Documents)
1. **PROJECTS.md**: 마스터 사양 및 로드맵.
2. **PROGRESS.md**: 현재 진행 중인 단계 및 성과 추적.
3. **PROCESSES.md**: 시스템 운영, 복구 및 자원 관리 프로세스.
4. **API_SPEC.md**: 외부 연동을 위한 Headless REST API 명세.
5. **DEPLOYMENT.md**: 설치 및 배포 가이드.

## 3. 핵심 아키텍처 (Key Architecture)
- **Headless API-First**: 모든 기능이 REST API로 노출.
- **Deep Inspection Timeline**: 인터랙티브 타임라인을 통해 에이전트의 사고 과정, 프롬프트, 상세 요약을 투명하게 공개.
- **Extensible UI Helper**: Go 템플릿 엔진에 사용자 정의 함수(FuncMap)를 도입하여 대시보드 확장성 확보.
- **LLM-Autonomous Classification**: 프로젝트의 정체를 스스로 파악하고 최적의 검증 도구를 선택하는 지능형 엔진.
- **Context-Aware Control**: 모든 외부 명령어 실행 시 Context를 적용하여 즉각적인 작업 중단 및 자원 회수 보장.

## 4. 로드맵 (Roadmap)
### Phase 9: 엔터프라이즈 엔지니어링 (Active)
- [x] **Step 38: Headless API & SPEC**: API 기반 아키텍처 전환 완료.
- [x] **Step 39: Deep Inspection UI**: 타임라인 아코디언 및 프롬프트 감사 기능 완료.
- [x] **Step 40: Extensible Template Engine**: 템플릿 엔진 버그 수정 및 확장 구조 구축 완료.
- [x] **Step 41: LLM-Autonomous Classification**: 브레인 기반 프로젝트 판별 및 적응형 빌드 구축 완료.
- [x] **Step 44: Task Termination Control**: API 및 UI를 통한 즉각적 작업 중단 기능 구현 완료.
- [ ] **Step 42: Prometheus Metrics**: 작업 지연 시간 및 워커 부하 지표 시각화.
- [ ] **Step 43: Swarm Activity Reporting**: 일일 작업 요약 및 브리핑 자동 생성.

---
*최종 감사: 2026-05-21 (작업 제어 및 로그 가시성 강화 완료)*

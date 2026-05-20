# 📘 Projects: Auto-Coder Swarm (Master Spec)

## 1. 프로젝트 목적 (Purpose)
`code-insight-engine`의 분석 결과를 바탕으로 실제 코드를 수정하고, 다중 에이전트 협업을 통해 검증하며, 최종적으로 Pull Request를 자동 생성하는 자율형 코드 수정 플랫폼입니다.

## 2. 프로젝트 로드맵 (Roadmap)

### Phase 1: 기반 아키텍처 및 격리 환경 (Completed)
- Workspace Isolation: UUID 기반 임시 작업 디렉토리 생성 및 관리.
- Git Integration: 원격 레포지토리 Clone, Branch 생성, Push 자동화.
- Engine Oracle Client: `code-insight-engine` API 연동 모듈.

### Phase 2: 멀티 에이전트 스웜 (Swarm) 구현
- Planner Agent: 요청 분석 및 수정 계획 수립.
- Coder Agent: 실제 코드 파일 수정 로직.
- Reviewer Agent: 컨벤션 및 로직 오류 검증.
- Risk Assessor: 영향도 평가 및 변경 위험도 분석.

### Phase 3: 자동 PR 및 피드백 루프
- PR Generator: GitHub/GitLab API 연동 (PR 생성 및 설명 작성).
- Feedback Loop: 리뷰어 거절 시 재수정 루프 구현.
- Concurrency Management: 다중 유저 요청 병렬 처리 최적화.

## 3. 설계 철학 (Design Philosophy)
- Isolation-First: 메인 소스 코드 보호를 위해 항상 격리된 임시 폴더에서 작업.
- Multi-Step Verification: 작성-리뷰-위험평가의 3단계 검증 필수.
- Oracle-Dependent: 분석은 스스로 하지 않고 오직 `code-insight-engine`의 결과를 신뢰.

## 4. 시스템 구조 (Architecture)
- /cmd/swarm: 메인 진입점.
- /internal/workspace: 격리 환경 관리.
- /internal/agent: 특화 에이전트 로직.
- /internal/orchestrator: 에이전트 간 워크플로우 제어.
- /internal/gitmgr: Git 및 PR 작업 담당.
- /internal/insightclient: 분석 엔진 통신용 클라이언트.

### Phase 4: 운영 최적화 및 실전 통합 (Planned)
- Real-World API Integration: GitHub/GitLab API 연동을 통한 실제 PR 자동화.
- Advanced Risk Assessment: 변경 사항의 파급 효과를 분석하는 위험 평가 에이전트 도입.
- Live Observability: 멀티 에이전트의 사고 과정을 실시간 모니터링 및 로깅.

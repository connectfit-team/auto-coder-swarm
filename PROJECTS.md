# 📘 Projects: Auto-Coder Swarm (Master Spec)

## 1. 프로젝트 목적 (Purpose)
`code-insight-engine`의 분석 결과를 바탕으로 실제 코드를 수정하고, 다중 에이전트 협업을 통해 검증하며, 최종적으로 Pull Request를 자동 생성하는 자율형 코드 수정 플랫폼입니다.

## 2. 프로젝트 로드맵 (Roadmap)

### Phase 1: 기반 아키텍처 및 격리 환경 (Completed)
- Workspace Isolation: UUID 기반 임시 작업 디렉토리 생성 및 관리.
- Git Integration: 원격 레포지토리 Clone, Branch 생성, Push 자동화.
- Engine Oracle Client: `code-insight-engine` API 연동 모듈.

### Phase 2: 멀티 에이전트 스웜 (Swarm) 구현 (Completed)
- Planner Agent: 요청 분석 및 수정 계획 수립.
- Coder Agent: 실제 코드 파일 수정 로직.
- Reviewer Agent: 컨벤션 및 로직 오류 검증.

### Phase 3: 자율 사이클 및 동시성 관리 (Completed)
- Feedback Loop: 리뷰어 거절 시 재수정 루프 구현 (최대 3회).
- Concurrency Management: Worker Pool을 통한 병렬 요청 처리.
- Simulation: PR 생성 시뮬레이션 및 슬랙 보고.

### Phase 4: 운영 최적화 및 신뢰성 강화 (In Progress)
- Advanced Verification: 격리 환경 내 실제 빌드/테스트 수행 (Step 13).
- System Reliability: SQLite 기반 작업 큐 영속성 도입 (Step 14).
- Conflict Prevention: 레포지토리 단위 작업 잠금(Lock) 시스템 (Step 15).
- Real-World API: GitHub/GitLab API 연동 (Step 10).
- Live Observability: 멀티 에이전트 사고 과정 실시간 중계 (Completed).

## 3. 설계 철학 (Design Philosophy)
- Isolation-First: 메인 소스 코드 보호를 위해 항상 격리된 임시 폴더에서 작업.
- No-Direct-Commit: 어떤 상황에서도 `main` 또는 `master` 브랜치에 직접 커밋하지 않으며, 오직 피처 브랜치와 PR을 통해서만 수정 사항을 제출함.
- Multi-Step Verification: 작성-리뷰-위험평가-빌드테스트의 다단계 검증 필수.
- Oracle-Dependent: 분석은 스스로 하지 않고 오직 `code-insight-engine`의 결과를 신뢰.

## 4. 시스템 구조 (Architecture)
- /cmd/swarm: 메인 진입점 및 워커 풀 관리.
- /internal/workspace: 격리 환경 관리.
- /internal/agent: 특화 에이전트 로직 (Planner, Coder, Reviewer, RiskAssessor).
- /internal/orchestrator: 에이전트 간 워크플로우 제어 및 상태 보고.
- /internal/gitmgr: Git 및 PR 작업 담당.
- /internal/insightclient: 분석 엔진 통신용 클라이언트.

### Phase 5: 성능 및 확장성 고도화 (Planned)
- Instant Sandboxing: 하드링크(`cp -al`) 또는 Git Worktree를 이용한 초고속 격리 환경 생성.
- Parallel Multi-Agent Audit: Reviewer와 RiskAssessor의 병렬 실행을 통한 검증 시간 단축.
- Context Optimization: 에이전트 간 토큰 소모 최소화를 위한 지능형 컨텍스트 압축.

### Phase 6: 무상태성 및 에이전트 그룹 확장 (Stateless & Inter-Group)
- Stateless Request API: 요청 간 의존성을 배제하고 컨텍스트를 데이터베이스화하여 확장성 확보.
- External Agent Bridge: 다른 에이전트 그룹(Multi-Agent Swarms)과 통신하기 위한 표준 인터페이스(gRPC/REST) 구축.
- Multi-Oracle Router: 여러 분석 엔진으로부터 교차 검증된 정보를 수집하는 라우팅 로직.

### Phase 7: 자가 치유 및 고급 자율성 (Planned)
- Self-Healing Build: 빌드 에러 로그를 분석하여 종속성 설치(`go mod tidy` 등)를 스스로 수행.
- Human-in-the-Loop Approval: 위험도가 높은 수정에 대해 슬랙 인터랙티브 버튼으로 사람의 최종 승인 획득.
- Multi-Model Swarm Voting: 여러 LLM 모델(Gemma, Claude 등)의 교차 검증을 통한 리뷰 정확도 향상.

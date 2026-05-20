# 📋 Operational Processes: Auto-Coder Swarm

## 1. Task Lifecycle (작업 생명주기)
1.  **Submission**: POST /api/v1/tasks를 통해 작업 요청 수신. (X-API-Key 인증 필수)
2.  **Oracle Consultation**: code-insight-engine(Port 8005)에 코드 맥락 및 파일 위치 분석 요청.
3.  **Planning & Dialogue**:
    *   Planner가 초기 계획 수립.
    *   Voter: 다중 모델(Gemma/Llama/Qwen) 투표를 통한 계획 선정.
    *   Critic: 선정된 계획에 대한 비판 및 보완 토론 진행.
4.  **Instant Sandboxing**: Git Worktree를 사용하여 초고속 격리 작업 공간 생성.
5.  **Coding & Test**: Coder가 파일 수정 및 단위 테스트 생성.
6.  **Verification (4-Step)**:
    *   Build: 컴파일 및 정적 분석 (Go/Flutter).
    *   Self-Healing: 실패 시 에러 로그 분석 후 자가 치유 시도.
    *   Review: Reviewer 에이전트의 코드 품질 검사.
    *   Risk: RiskAssessor의 보안/성능 리스크 평가.
7.  **Human Approval**: 대시보드(Port 8006)에서 Diff 검토 및 최종 승인.
8.  **Finalize**: GitHub PR 생성 및 Oracle 지식 동기화.

## 2. Recovery Process (복구 절차)
- **Service Crash**: ResetRunningToPending()을 통해 실행 중이던 작업을 PENDING으로 자동 복구.
- **Lock Deadlock**: 서비스 재시작 시 모든 RepoLock을 초기화하여 교착 상태 해제.

## 3. Deployment (배포 프로세스)
- **Build**: go build -o bin/swarm cmd/swarm/main.go
- **Restart**: 기존 프로세스 종료 후 SWARM_API_KEY 환경변수와 함께 백그라운드 실행.
- **Log**: service.log (시스템 로그), agent_thoughts.log (추론 로그) 모니터링.

---
*Last Updated: 2026-05-20*

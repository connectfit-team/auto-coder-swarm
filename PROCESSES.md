# 📋 Operational Processes: Auto-Coder Swarm

## 1. Task Lifecycle (작업 생명주기)
1.  **Submission**: POST /api/v1/tasks (X-API-Key 필수).
2.  **Analysis**: Oracle(Port 8005) 연동 코드 분석.
3.  **Refined Planning**: 
    - Multi-Model Voting (Winner 선정).
    - Proposer-Critic Dialogue (계획 보강).
4.  **Sandbox**: Git Worktree 기반 격리 공간 생성.
5.  **Coding**: Coder 에이전트 수정 및 테스트 생성.
6.  **Verify**: Build -> Self-Healing -> Review -> Risk 평가.
7.  **Human Loop**: 대시보드(Port 8006) 시각적 승인.
8.  **Finalize**: PR 생성 및 지식 동기화.

## 2. Resource Management (자원 관리)
- **Logs**: `agent_thoughts.log`가 100MB 도달 시 로테이트. 최근 10개 파일만 유지하고 나머지는 자동 삭제.
- **Database**: 모든 에이전트 추론 과정은 `ThoughtLog` 테이블에 저장되어 무한 히스토리 조회 지원.
- **Workspace**: 작업 완료 후 `git worktree remove` 및 임시 디렉토리 완전 삭제.

## 3. Maintenance (유지보수)
- **Restart**: 바이너리 빌드 후 기존 프로세스 종료 및 재시작.
- **Recovery**: 서비스 시작 시 `ResetRunningToPending()` 호출로 비정상 종료 작업 자동 복구.

---
*Last Updated: 2026-05-20*

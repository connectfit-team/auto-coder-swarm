# 📋 Operational Processes: Auto-Coder Swarm

## 1. Deployment & Portability
- **Environment Driven**: 모든 핵심 설정(DB 경로, Oracle URL, 저장소 경로 등)은 환경변수를 통해 주입됩니다.
- **Clone & Run**: 레포지토리를 클론한 후 적절한 환경변수만 설정하면 즉시 동일한 환경을 구축할 수 있습니다.
- **Git Hygiene**: 바이너리(`bin/`) 및 민감한 데이터(`swarm.db`)는 절대 커밋되지 않도록 `.gitignore`로 관리됩니다.

## 2. Environment Variables (Required)
- `SWARM_API_KEY`: API 호출 보안 키.
- `ORACLE_URL`: Code-Insight Engine 주소 (Default: http://localhost:8005).
- `MASTER_REPOS_PATH`: 분석 및 수정 대상 원본 레포지토리들이 모여있는 경로.
- `WORKSPACE_BASE_PATH`: 에이전트가 작업할 임시 샌드박스 경로 (Default: /tmp).

## 3. Maintenance
- **Rebuild**: `go build -o bin/swarm cmd/swarm/main.go`
- **Graceful Restart**: 기존 프로세스 종료 후 새로운 환경변수와 함께 백그라운드 실행.
- **Recovery**: 시작 시 미결된 작업을 자동으로 `PENDING`으로 돌려 복구합니다.

---
*Last Updated: 2026-05-20*

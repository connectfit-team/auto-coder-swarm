# 📋 Operational Processes: Auto-Coder Swarm

## 1. Documentation & Governance
모든 기술적 변화는 다음 프로세스를 따릅니다:
- **API 수정**: `API_SPEC.md`를 선제적으로 갱신하여 클라이언트 호환성 보장.
- **아키텍처 변화**: `PROJECTS.md`의 설계 원칙 섹션 업데이트.
- **배포 설정 변경**: `DEPLOYMENT.md` 및 `.env.example` 동기화.

## 2. Resource Management
- **LLM Context Switching**: 로딩 지연 방지를 위해 항상 `keep_alive: -1`을 사용하며 VRAM 총량을 상시 모니터링합니다.
- **Log Management**: `agent_thoughts.log`는 100MB 도달 시 로테이트하며 최근 10개만 유지합니다.

## 3. Maintenance & Recovery
- **Binary Build**: `go build -o bin/swarm cmd/swarm/main.go` (미사용 변수 없는 클린 빌드 필수)
- **Self-Healing**: 서비스 시작 시 중단된 작업을 자동 탐지하여 `PENDING`으로 복구합니다.
- **State Recovery**: `ContextState` DB 필드를 통해 실패한 작업의 기술적 맥락을 복구합니다.

---
*Last Updated: 2026-05-21*

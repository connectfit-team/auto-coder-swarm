# 📋 Operational Processes: Auto-Coder Swarm

## 1. Documentation & Governance
모든 기술적 변화는 다음 프로세스를 따릅니다:
- **API 수정**: `API_SPEC.md`를 선제적으로 갱신하여 클라이언트 호환성 보장.
- **아키텍처 변화**: `PROJECTS.md`의 설계 원칙 섹션 업데이트.
- **배포 설정 변경**: `DEPLOYMENT.md` 및 `.env.example` 동기화.

## 2. Intelligence & Verification (New)
- **Autonomous Detection**: 프로젝트 시작 시 `tree` 또는 `ls -R` 결과를 LLM에 전달하여 프로젝트 타입을 판별합니다.
- **Adaptive Build**: LLM이 처방한 `build_command`와 `bench_command`를 사용하여 환경에 특화된 검증을 수행합니다.
- **Risk Review**: LLM이 산출한 `risk_assessment`를 타임라인에 기록하여 환경적 위험 요소를 사전에 공유합니다.

## 3. Resource Management
- **LLM Context Switching**: 로딩 지연 방지를 위해 항상 `keep_alive: -1`을 사용하며 VRAM 총량을 상시 모니터링합니다.
- **Log Management**: `agent_thoughts.log`는 100MB 도달 시 로테이트하며 최근 10개만 유지합니다.

## 4. Maintenance & Recovery
- **Binary Build**: `go build -o bin/swarm cmd/swarm/main.go` (미사용 변수 없는 클린 빌드 필수)
- **Self-Healing**: 서비스 시작 시 중단된 작업을 자동 탐지하여 `PENDING`으로 복구합니다.
- **State Recovery**: `ContextState` DB 필드를 통해 실패한 작업의 기술적 맥락을 복구합니다.

---
*Last Updated: 2026-05-21 (Intelligent Detection Process Added)*

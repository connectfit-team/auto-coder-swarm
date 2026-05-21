# 📋 Operational Processes: Auto-Coder Swarm

## 1. Documentation Standard (문서 관리 표준)
모든 개발 및 운영 단계에서 다음 문서들을 항상 최신으로 유지합니다:
- **API 수정 시**: `API_SPEC.md`를 즉시 갱신하여 외부 연동 호환성을 보장합니다.
- **신규 기능 추가 시**: `PROJECTS.md`와 `PROGRESS.md`에 마일스톤을 기록합니다.
- **운영 환경 변경 시**: `PROCESSES.md`와 `DEPLOYMENT.md`를 업데이트합니다.

## 2. Resource Management (자원 관리)
- **VRAM Optimization**: Ollama 모델은 `keep_alive: -1`로 상주시켜 로딩 지연을 방지합니다. 불필요한 vLLM 프로세스는 철저히 배제합니다.
- **Log Rotation**: `agent_thoughts.log`는 100MB 도달 시 자동 로테이트하며 최근 10개만 유지합니다.

## 3. Recovery Process (복구 절차)
- **Stuck Task Reset**: 시스템 재시작 시 `RUNNING` 상태의 작업을 `PENDING`으로 자동 복구하여 영속성을 보장합니다.
- **State Audit**: `ContextState` 필드를 통해 중단된 작업의 기술적 맥락을 복구합니다.

---
*Last Updated: 2026-05-21*

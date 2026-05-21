# Auto-Coder Swarm: Guidelines & Context

## 🔌 Integrated Architecture: CIE & Swarm
Auto-Coder Swarm은 `code-insight-engine` (CIE, Eyes)와 상호작용하며 동작합니다. 모든 통합 로직은 두 프로젝트의 `API_SPEC.md`를 기반으로 합니다.

### 1. Code-Insight Engine (Eyes) Dependency
에이전트는 코드 분석 및 파일 위치 파악을 위해 CIE API를 상시 모니터링하고 호출해야 합니다.
- **Spec Source**: `/home/cnf/projects/code-insight-engine/API_SPEC.md`
- **Primary Endpoints**:
    - `POST /analyze`: 코드 맥락 분석 및 기술 가이드 획득.
    - `POST /api/v1/navigate`: 관련 레포지토리 식별.
    - `POST /api/v1/report_result`: 수정 완료 후 CIE 지식 베이스 업데이트 보고.

### 2. Auto-Coder Swarm (Hands) Exposure
CIE나 외부 시스템이 Swarm에 작업을 지시할 때 사용하는 표준 명세입니다.
- **Spec Source**: `./API_SPEC.md`
- **Primary Endpoints**:
    - `POST /api/v1/tasks`: 자동 수정 작업 제출.
    - `GET /task/stream?id={id}`: 에이전트 사고 과정 실시간 구독.
    - **Note**: 기능 추가 시 반드시 `API_SPEC.md`를 갱신하여 CIE가 신규 기능을 인지하도록 해야 합니다.

## 📋 Operational Standards
- **Documentation-First**: API 명세 변경 시 코드를 수정하기 전에 `API_SPEC.md`를 먼저 업데이트합니다.
- **Security**: 모든 통신에는 `X-API-Key` 헤더가 필수입니다.
- **Isolation**: Workspaces는 `/tmp/swarm_ws_*`에서 수행되며 자동 정리됩니다.
- **Master Repos**: `/home/cnf/projects/code-insight-engine/repos` 경로를 참조합니다.

## ✅ Verification Checklist
- [ ] Compiles: `go build ./cmd/swarm`
- [ ] API Sync: CIE와 Swarm의 SPEC 버전 일치 여부 확인.
- [ ] Tests: `go test ./...`

---
*Last Updated: 2026-05-21 (CIE-Swarm Sync Protocol Established)*

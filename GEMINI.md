# Auto-Coder Swarm: Guidelines & Context

## 🛡️ 시스템 무결성 보장(Functional Integrity) 원칙
Auto-Coder Swarm 시스템의 신뢰성과 안정성을 위해 다음 원칙을 준수하며, 위반 시 작업 종료를 금지합니다.

1. **Pre-Task 목적 명확화**: 모든 작업 시작 전, 목적을 재확인하고 시스템의 본분(자율 코드 수정 및 검증)을 저해하지 않는지 판단합니다.
2. **Zero-Mock 원칙**: 프로덕션 코드에 'Mock content', 'TODO', 'return nil'과 같은 빈 껍데기 코드를 주입하는 행위를 전면 금지합니다. 모든 코드는 실제 비즈니스 로직을 포함해야 합니다.
3. **Mandatory 실증 검증**: 작업 후에는 반드시 컴파일, 빌드, 혹은 API 테스트를 통해 시스템이 실제 코드를 읽고 동작하는지 실증적으로 증명해야만 작업을 종료할 수 있습니다.
4. **완벽한 추적성**: 모든 변경 사항은 SESSION_LOG.md와 git log에 기록하여 회귀(Regression) 발생 시 즉시 복구가 가능하게 합니다.
5. **실시간 원격 동기화(Continuous Sync)**: 모든 코드 수정, 문서 갱신, 혹은 설정 변경 발생 시 즉시 `git commit` 및 `git push`를 수행하여 로컬과 원격 서버(`192.168.120.54`) 간의 상태를 완벽히 일치시킵니다.
6. **원격 전용 실행 원칙(Remote-Only Execution)**: 모든 빌드, 테스트, 데이터베이스(SQLite) 저장, 그리고 실제 에이전트 작업은 100% 원격 서버(`192.168.120.54`)에서만 수행됩니다.
7. **모델 무결성(Model Integrity)**: 시스템의 '두뇌' 역할을 하는 Primary Model은 반드시 **사고 및 추론이 가능한 모델(gemma4 등)**로 유지되어야 합니다. 임베딩 전용 모델(bge-m3 등)로의 오설정을 엄격히 금지하며, 세션 시작 시 항상 이를 확인해야 합니다.

## 🔌 Integrated Architecture: CIE & ACS
ACS은 code-insight-engine (CIE, Eyes)과 상호작용하며 동작합니다.

### 1. 4-Stage Workflow (Eyes & Hands Integration)
효율적인 작업 수행을 위해 모든 작업은 다음 4단계를 거칠니다.
1. **INSPECTION (Lightweight)**: CIE에게 작업 범위 내 파일 목록과 라인 수(LOC)만 요청하여 규모 파악.
2. **STRATEGY (Architect)**: 검사 결과 기반 작업 가능 여부 판단 및 '논리 분석용 질의' 생성.
3. **ANALYSIS (Eyes - CIE)**: CIE의 코드 기능 논리 이해 및 기술적 요약 제공.
4. **IMPLEMENTATION (Hands - ACS)**: ACS의 Planner/Coder가 실질적 코드 수정 및 검증 수행.

## 📋 Operational Standards
- **Documentation-First**: API 명세 변경 시 API_SPEC.md를 먼저 업데이트합니다.
- **Security**: 모든 통신에는 X-API-Key 헤더가 필수입니다.
- **Isolation**: Workspaces는 /tmp/swarm_ws_*에서 수행되며 자동 정리됩니다.

---
*최종 갱신: 2026-05-22 (기본 모델 gemma4:31b 업그레이드 및 무결성 강화)*

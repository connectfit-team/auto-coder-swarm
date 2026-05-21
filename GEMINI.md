# Auto-Coder Swarm: Guidelines & Context

## 🛡️ 시스템 무결성 보장(Functional Integrity) 4대 원칙
Auto-Coder Swarm 시스템의 신뢰성과 안정성을 위해 다음 원칙을 준수하며, 위반 시 작업 종료를 금지합니다.

1. **Pre-Task 목적 명확화**: 모든 작업 시작 전, 목적을 재확인하고 시스템의 본분(자율 코드 수정 및 검증)을 저해하지 않는지 판단합니다.
2. **Zero-Mock 원칙**: 프로덕션 코드에 'Mock content', 'TODO', 'return nil'과 같은 빈 껍데기 코드를 주입하는 행위를 전면 금지합니다. 모든 코드는 실제 비즈니스 로직을 포함해야 합니다.
3. **Mandatory 실증 검증**: 작업 후에는 반드시 컴파일, 빌드, 혹은 API 테스트를 통해 시스템이 실제 코드를 읽고 동작하는지 실증적으로 증명해야만 작업을 종료할 수 있습니다.
4. **완벽한 추적성**: 모든 변경 사항은 SESSION_LOG.md와 git log에 기록하여 회귀(Regression) 발생 시 즉시 복구가 가능하게 합니다.

## 🔌 Integrated Architecture: CIE & Swarm
Swarm은 code-insight-engine (CIE, Eyes)과 상호작용하며 동작합니다.

### 1. 4-Stage Workflow (Eyes & Hands Integration)
효율적인 작업 수행을 위해 모든 작업은 다음 4단계를 거칩니다.

1. **INSPECTION (Lightweight)**: CIE에게 작업 범위 내 파일 목록과 라인 수(LOC)만 라이트하게 요청하여 규모를 파악합니다.
2. **STRATEGY (Architect)**: 검사 결과를 바탕으로 작업 가능 여부를 판단하고, CIE에게 보낼 '논리 분석용 질의(Precision Query)'를 생성합니다.
3. **ANALYSIS (Eyes - CIE)**: CIE는 코드의 기능을 논리적으로 이해하고 기술적 요약을 제공합니다. (구현 지시가 아닌 '이해'에 집중)
4. **IMPLEMENTATION (Hands - Swarm)**: Swarm의 Planner와 Coder가 CIE의 이해를 바탕으로 실제 코드를 수정하고 검증합니다.

### 2. Code-Insight Engine (Eyes) Dependency
- **Async Spec**: CIE의 비동기 API(POST /analyze -> GET /api/tasks/result)를 준수해야 합니다.
- **Session Isolation**: 각 작업은 고유한 session_id를 사용하여 컨텍스트 오염을 방지합니다.

### 2. Auto-Coder Swarm (Hands) Exposure
- **Context-Aware Control**: 모든 외부 명령어 실행 시 CommandContext를 적용하여 사용자의 중단 명령에 즉각 반응해야 합니다.
- **Observability**: 타임라인에 ORACLE(CIE 질의), DETECTION_RAW(LLM 응답 원본) 단계를 노출하여 투명성을 확보합니다.

## 📋 Operational Standards
- **Documentation-First**: API 명세 변경 시 API_SPEC.md를 먼저 업데이트합니다.
- **Security**: 모든 통신에는 X-API-Key 헤더가 필수입니다.
- **Isolation**: Workspaces는 /tmp/swarm_ws_*에서 수행되며 자동 정리됩니다.

---
*최종 갱신: 2026-05-21 (시스템 무결성 보장 원칙 공식 각인)*

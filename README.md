# 🐝 Auto-Coder Swarm (Hands)

`Auto-Coder Swarm`은 `code-insight-engine` (CIE, Eyes)의 분석 결과와 LLM의 지능형 실행력을 결합하여, **코드 분석-수정-검증-PR 생성** 전 과정을 자동화하는 엔터프라이즈급 자율형 멀티 에이전트 시스템입니다.

---

## 🛠️ 핵심 아키텍처 (4-Stage Pipeline)

ACS은 CIE와 상호작용하며 다음 4단계 워크플로우를 통해 작업을 완수합니다.

1.  **INSPECTION (Intelligence-First)**: 대상 범위의 파일 목록과 규모를 지능적으로 파악합니다.
2.  **STRATEGY (Architect)**: 분석 결과를 바탕으로 최적의 작업 전략과 정밀 질의문을 생성합니다.
3.  **ANALYSIS (Eyes - CIE)**: CIE를 통해 코드의 논리적 구조를 완벽히 이해합니다.
4.  **IMPLEMENTATION (Hands - ACS)**: Planner와 Coder 에이전트가 실제 코드를 수정하고 실증 검증을 수행합니다.

---

## 🛡️ 시스템 무결성 5대 원칙 (Mandates)

1.  **Pre-Task 목적 명확화**: 모든 작업 시작 전 목적을 재확인합니다.
2.  **Zero-Mock 원칙**: 프로덕션 코드에 빈 껍데기 코드 주입을 전면 금지합니다.
3.  **Mandatory 실증 검증**: 컴파일 및 빌드 테스트를 거쳐야만 작업을 종료합니다.
4.  **완벽한 추적성**: 모든 변경 사항은 `SESSION_LOG.md`와 Git Log에 기록됩니다.
5.  **실시간 원격 동기화 (Continuous Sync)**: 모든 수정 사항은 즉시 커밋 및 푸시됩니다.

---

## 📂 프로젝트 가이드 문서 (Master Docs)

상세한 운영 및 개발 지침은 아래 문서를 참조하십시오.

*   [**PROJECTS.md**](./PROJECTS.md): 마스터 사양 및 중장기 로드맵.
*   [**PROGRESS.md**](./PROGRESS.md): 현재 진행 단계 및 운영(Systemd) 지침.
*   [**GEMINI.md**](./GEMINI.md): 에이전트 행동 강령 및 무결성 보장 원칙.
*   [**API_SPEC.md**](./API_SPEC.md): 외부 연동을 위한 REST API 명세.
*   [**SESSION_LOG.md**](./SESSION_LOG.md): 최신 작업 이력 및 의사결정 기록.

---

## ⚙️ 시작하기 (Quick Start)

### 1. 환경 설정
`.env` 파일을 생성하고 필요한 API Key 및 경로를 설정합니다.
```bash
cp .env.example .env
```

### 2. 빌드 및 실행
```bash
go build -o bin/swarm cmd/swarm/main.go
./bin/swarm
```

### 3. API 테스트
```bash
curl -X POST http://localhost:8006/api/v1/tasks \
     -H "X-API-Key: secret" \
     -d '{"user_request": "repo_name 경로의 코드를 분석해줘"}'
```

---
*Last Updated: 2026-05-21 (High-Precision Introspection & Modularization Applied)*

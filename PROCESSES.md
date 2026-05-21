# 📋 Operational Processes: Auto-Coder Swarm

## 1. UI & Visualization Standard
- **Interactive Inspection**: 타임라인 클릭 시 나타나는 \`Summary\`와 \`Prompt\`는 에이전트의 의사결정을 검증하는 핵심 수단입니다.
- **Template Helpers**: \`internal/web/dashboard.go\`의 \`helpers()\`를 통해 대시보드 기능을 확장합니다.

## 2. Resource & Performance
- **VRAM**: Ollama \`keep_alive: -1\`을 유지하여 로딩 지연을 최소화합니다.
- **Logs**: 100MB 로테이션 및 DB 영구 저장을 통해 무한 히스토리 조회를 보장합니다.

## 3. Maintenance
- **API Spec**: \`API_SPEC.md\`와 코드를 항상 동기화하여 Headless 아키텍처의 무결성을 유지합니다.
- **Recovery**: 비정상 종료 시 \`ContextState\`를 참조하여 지능형 복구를 수행합니다.

---
*Last Updated: 2026-05-21*

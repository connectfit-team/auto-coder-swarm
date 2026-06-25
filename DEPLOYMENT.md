# 🚀 Deployment Guide: Auto-Coder Swarm

이 문서는 Auto-Coder Swarm을 새로운 서버에 설치하고 실행하기 위한 가이드를 제공합니다.

## 1. Prerequisites (사전 준비)
- **Go**: 1.25.0 이상
- **gh CLI**: GitHub PR 생성을 위해 필요
- **Ollama**: 에이전트 브레인(LLM) 구동을 위해 필요
- **Code-Insight Engine**: 코드 분석을 담당하는 오라클 서버

## 2. Setup Steps (설치 단계)

### 1) Repository Clone
```bash
git clone https://github.com/connectfit-team/auto-coder-swarm.git
cd auto-coder-swarm
```

### 2) Environment Configuration
`.env.example` 파일을 복사하여 `.env` 파일을 생성하고 본인의 환경에 맞게 수정합니다.
```bash
cp .env.example .env
# .env 파일 내의 MASTER_REPOS_PATH, ORACLE_URL 등을 수정하세요.
```

### 3) Export Variables
현재 세션에서 환경변수를 로드합니다 (또는 시스템 환경변수에 등록).
```bash
export $(grep -v '^#' .env | xargs)
```

### 4) Systemd Service Setup
`scripts/auto-coder-swarm.service` 파일을 `/etc/systemd/system/`으로 복사하여 데몬으로 등록합니다.
```bash
sudo cp scripts/auto-coder-swarm.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable auto-coder-swarm
```

### 5) Build & Run
이후부터는 `deploy.sh`를 활용하거나 직접 systemctl 명령으로 관리합니다.
```bash
# 바이너리 빌드 및 서비스 재시작
./deploy.sh

# 상태 확인
sudo systemctl status auto-coder-swarm
# 로그 모니터링
sudo journalctl -u auto-coder-swarm -f
```

## 3. Configuration Details (설정값 설명)
| Key | Description | Default |
|-----|-------------|---------|
| `SWARM_API_KEY` | API 및 대시보드 접근용 보안 키 | (Empty) |
| `MASTER_REPOS_PATH` | 분석 대상 레포지토리들이 위치한 절대 경로 | `/home/cnf/...` |
| `ORACLE_URL` | Code-Insight Engine 주소 | `http://localhost:8005` |
| `LISTEN_ADDR` | 대시보드 포트 설정 | `:8006` |

---
*Last Updated: 2026-05-21*

#!/bin/bash
set -e

APP_DIR="/home/cnf/projects/auto-coder-swarm"
BIN="$APP_DIR/bin/swarm"
SERVICE="auto-coder-swarm.service"
BRANCH=$(cd "$APP_DIR" && git branch --show-current)
export PATH=$PATH:/usr/local/go/bin

cd "$APP_DIR"

echo "===================================="
echo "  ACS Deploy Script (Systemd)"
echo "  $(date +%Y-%m-%d\ %H:%M:%S)"
echo "===================================="

# 1. Git Pull
echo ""
echo "[1/5] Git Pull (branch: $BRANCH)..."
git fetch origin
git reset --hard "origin/$BRANCH"
HASH=$(git rev-parse --short HEAD)

# 2. Go Deps + Build
echo ""
echo "[2/5] Building..."
go mod tidy 2>&1
# **꾸러미로 빌드한다. 파일 하나로 빌드하면 안 된다.**
#
# ./cmd/swarm/main.go 로 빌드하면 같은 꾸러미의 다른 파일이 빠진다.
# 실측으로 cmd/swarm/deps_ready.go 를 더한 뒤 `undefined: depsReady` 로
# 빌드가 깨졌고, set -e 로 여기서 멈춰 **배포가 안 됐는데 안 된 줄을
# 몰랐다**(PR #57 이 그렇게 배포되지 않은 채 남아 있었다).
go build -o "$BIN" ./cmd/swarm 2>&1
echo "  → Build OK ($(du -h $BIN | cut -f1))"

# 3. 문법 검사기
#
# 검사기가 없으면 그 언어의 검사가 사라지는데, 사라진 것이 통과처럼 보인다.
# 배포마다 깔고 살아 있는지 확인한다. 실패해도 배포는 멈추지 않는다 —
# PR 은 그것을 "문법 확인 못 함" 으로 알린다.
echo ""
echo "[3/5] Syntax checkers..."
bash scripts/install-checkers.sh || true

# 4. Systemd File Update (Optional)
sudo cp scripts/auto-coder-swarm.service /etc/systemd/system/ 2>/dev/null || true
sudo systemctl daemon-reload

# 5. Restart
echo ""
echo "[5/5] Restarting systemd service..."
sudo systemctl restart "$SERVICE"

echo ""
echo "===================================="
echo "  ✅ Deploy SUCCESS"
echo "  Commit: $HASH"
echo "===================================="
exit 0

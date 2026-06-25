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
echo "[1/4] Git Pull (branch: $BRANCH)..."
git fetch origin
git reset --hard "origin/$BRANCH"
HASH=$(git rev-parse --short HEAD)

# 2. Go Deps + Build
echo ""
echo "[2/4] Building..."
go mod tidy 2>&1
go build -o "$BIN" ./cmd/swarm/main.go 2>&1
echo "  → Build OK ($(du -h $BIN | cut -f1))"

# 3. Systemd File Update (Optional)
sudo cp scripts/auto-coder-swarm.service /etc/systemd/system/ 2>/dev/null || true
sudo systemctl daemon-reload

# 4. Restart
echo ""
echo "[4/4] Restarting systemd service..."
sudo systemctl restart "$SERVICE"

echo ""
echo "===================================="
echo "  ✅ Deploy SUCCESS"
echo "  Commit: $HASH"
echo "===================================="
exit 0

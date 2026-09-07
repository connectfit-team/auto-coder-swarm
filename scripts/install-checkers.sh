#!/bin/bash
# 문법 검사기를 이 저장소에서 ~/tools 로 깐다. 여러 번 돌려도 된다.
#
# 검사기가 없으면 검사가 사라지는데, 사라진 것이 통과처럼 보인다
# (파서가 없으면 판단하지 않는다). 그래서 깔고 나서 **일부러 틀린 파일이
# 걸리는지** 확인하는 것까지 이 스크립트의 몫이다.
#
# Dart SDK 는 여기서 받지 않는다 — 크고, 받는 곳을 손으로 확인해야 한다.
# 없으면 어디에 두어야 하는지만 알린다.
set -u

SRC="$(cd "$(dirname "$0")/../tools/tsparse" && pwd)"
DEST="$HOME/tools/tsparse"
FAIL=0

echo "[1/3] TypeScript·Svelte 파서를 $DEST 에 깐다"
if ! command -v node >/dev/null; then
  echo "  ✗ node 가 없다 — TS·Svelte 검사가 돌지 않는다"
  FAIL=1
else
  mkdir -p "$DEST"
  cp "$SRC/parse.js" "$SRC/package.json" "$DEST/"
  if [ ! -d "$DEST/node_modules/typescript" ] || [ ! -d "$DEST/node_modules/svelte" ]; then
    (cd "$DEST" && npm install --no-audit --no-fund) || {
      echo "  ✗ npm install 실패 — TS·Svelte 검사가 돌지 않는다"
      FAIL=1
    }
  fi
fi

echo "[2/3] 파서가 살아 있는지 본다"
if command -v node >/dev/null && [ -f "$DEST/parse.js" ]; then
  T=$(mktemp -d)
  printf 'let x: number = 1;\n' > "$T/good.ts"
  printf 'let x: number = ;\n' > "$T/bad.ts"
  node "$DEST/parse.js" "$T/good.ts" >/dev/null 2>&1
  [ $? -eq 0 ] || { echo "  ✗ 멀쩡한 파일을 막는다"; FAIL=1; }
  node "$DEST/parse.js" "$T/bad.ts" >/dev/null 2>&1
  [ $? -eq 1 ] || { echo "  ✗ 틀린 파일이 걸리지 않는다 — 검사가 죽어 있다"; FAIL=1; }
  node "$DEST/parse.js" "$T/없는파일.ts" >/dev/null 2>&1
  [ $? -eq 2 ] || { echo "  ✗ 없는 파일을 문법 문제로 읽는다"; FAIL=1; }
  rm -rf "$T"
  [ $FAIL -eq 0 ] && echo "  ✓ 맞는 것은 통과, 틀린 것은 걸린다"
fi

echo "[3/3] Dart"
DART="${DART_BIN:-$HOME/tools/dart-sdk/bin/dart}"
command -v dart >/dev/null && DART=$(command -v dart)
if [ -x "$DART" ]; then
  T=$(mktemp -d)
  printf 'void main() { print(1);\n' > "$T/bad.dart"
  if "$DART" format --output=none "$T/bad.dart" 2>&1 | grep -q "could not be parsed"; then
    echo "  ✓ 틀린 Dart 가 걸린다 ($DART)"
  else
    echo "  ✗ 틀린 Dart 가 걸리지 않는다 — Dart 검사가 죽어 있다"
    FAIL=1
  fi
  rm -rf "$T"
else
  echo "  ✗ Dart 가 없다 — .dart 는 검사되지 않는다"
  echo "    ~/tools/dart-sdk 에 SDK 를 풀거나 DART_BIN 을 준다"
  FAIL=1
fi

echo
if [ $FAIL -eq 0 ]; then
  echo "✅ 검사기가 모두 살아 있다"
else
  echo "⚠️  위의 ✗ 는 그 언어의 파일이 검사되지 않는다는 뜻이다."
  echo "   PR 은 그것을 '문법 확인 못 함' 으로 알린다 — 통과로 세지 않는다."
fi
exit 0

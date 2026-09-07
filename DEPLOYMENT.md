# 설치와 배포 — Auto-Coder Swarm

## 1. 필요한 것

| 것 | 왜 | 없으면 |
|---|---|---|
| Go 1.25+ | 빌드 | — |
| gh CLI (로그인된 상태) | PR 을 열고, 같은 PR 이 이미 있는지 본다 | PR 을 못 연다 |
| NATS (JetStream) | 작업 사이 신호(`SWARM_EVENTS` 스트림) | **서비스가 뜨지 않는다** — `main` 이 여기서 멈춘다 |
| Redis | 조회 캐시 | 뜨기는 한다 |
| RabbitMQ | `LLM_DIRECT_URL` 이 없을 때 모델을 큐로 부른다 | 모델을 못 부른다 |
| code-insight-engine (:8005) | 어디를 고칠지 찾는다 | 값 추가 흐름이 멈춘다 |
| corporate-knowledge-hub (:8007) | 도메인 지식 | 그 조회만 빈다 |
| Dart SDK · node + 파서 | Dart·TS·Svelte 문법 검사 | 그 언어는 "확인 못 함" 으로 적힌다 |

문법 검사기는 저장소가 들고 있다:

```bash
bash scripts/install-checkers.sh   # ~/tools/tsparse 에 깔고, 살아 있는지 확인한다
```

Dart SDK 는 여기서 받지 않는다. `~/tools/dart-sdk` 에 풀거나 `DART_BIN` 으로 알려 준다.

## 2. 설정값

`.env.example` 을 복사해 쓴다. 기본값은 코드에 있는 그대로다.

| 이름 | 몫 | 기본값 |
|---|---|---|
| `SWARM_API_KEY` | API·대시보드 열쇠 | (없음 — 비면 인증이 없다) |
| `LISTEN_ADDR` | API·대시보드 주소 | `:8006` |
| `ORACLE_URL` | code-insight-engine | `http://localhost:8005` |
| `CIE_API_KEY` | CIE 열쇠 | (없음) |
| `CKH_URL` | corporate-knowledge-hub | `http://localhost:8007` |
| `CKH_API_KEY` | CKH 열쇠 | (없음) |
| `NATS_URL` | 신호 버스 | `nats://localhost:4222` |
| `REDIS_ADDR` | 캐시 | `localhost:6379` |
| `AMQP_URL` | LLM 큐 | `amqp://guest:guest@192.168.120.54:5672/` |
| `LLM_DIRECT_URL` | 있으면 큐를 거치지 않고 이 주소에 직접 묻는다 | (없음) |
| `LLM_API_URL` · `LLM_API_KEY` | 대시보드가 모델 목록을 볼 주소 | (없음) |
| `LLM_JUDGE_TEMPERATURE` | 판정 온도 — 판정은 흔들리면 안 된다 | `0` |
| `LLM_MAX_TOKENS` | 한 번에 받을 최대 길이 | `4096` |
| `MASTER_REPOS_PATH` | 저장소 사본이 있는 곳 | `/home/cnf/cie-repos` |
| `WORKSPACE_BASE_PATH` | 작업용 워크트리를 만들 곳 | `/tmp` |
| `SWARM_DB_PATH` | SQLite 파일 | `./swarm.db` |
| `DATABASE_DSN` | 주면 SQLite 대신 이 DB 를 쓴다 | (없음) |
| `TEMPLATES_PATH` | 대시보드 템플릿 | `./web/templates` |
| `SLACK_WEBHOOK_URL` | 끝났을 때 알릴 곳 | (없음) |
| `DART_BIN` · `TS_PARSER` | 문법 검사기 경로를 직접 줄 때 | 자동으로 찾는다 |

쓰는 모델 이름은 env 가 아니라 DB 설정값 `primary_model` 이다(없으면 `gemma4:31b`).
대시보드에서 바꾼다.

**함정 — `CIE_API_KEY` 가 없으면 값 추가 흐름이 통째로 서지 않는다.** CIE 가 401 을 주고
작업은 거기서 멈춘다. 전에는 그 401 을 "값 추가 요청이 아니다" 로 삼켜서, 설정 문제가
판단 문제로 위장돼 엉뚱한 흐름이 조용히 돌았다.

## 3. systemd

```bash
sudo cp scripts/auto-coder-swarm.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now auto-coder-swarm
```

열쇠는 유닛 파일에 두지 않는다. 사무실 기계에서는 드롭인으로 준다:

```
/etc/systemd/system/auto-coder-swarm.service.d/
  10-wait-deps.conf   의존 서비스가 뜬 뒤에 시작한다
  cie-key.conf        CIE_API_KEY
  direct-llm.conf     LLM_DIRECT_URL
```

드롭인은 이 저장소에 없다(열쇠가 들어 있다). 새 기계에 올릴 때는 손으로 만든다.
`deploy.sh` 는 유닛 파일만 덮어쓰므로 드롭인은 그대로 남는다.

## 4. 배포

```bash
./deploy.sh
```

`git fetch` → **현재 브랜치를 `origin/<브랜치>` 로 강제 정렬** → 빌드 → 검사기 설치 →
유닛 갱신 → 재시작.

**함정: 배포는 원격 브랜치를 본다.** 밀지 않은 커밋은 배포되지 않고 `git reset --hard` 에
지워진다. 배포 전에 밀어야 한다.

## 5. 살아 있는지 보기

```bash
systemctl status auto-coder-swarm
tail -f service.log                  # 유닛이 표준출력을 여기에 붙인다
curl -s localhost:8006/metrics | head
```

대시보드는 `http://<호스트>:8006` 이다. 작업 하나의 속내는 대시보드의 작업 상세
(deep technical log)에 남는다 — 무엇을 왜 건너뛰었는지가 거기 적힌다.

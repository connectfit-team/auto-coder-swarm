# Auto-Coder Swarm (ACS)

요청 하나를 받아 **필요한 저장소를 모두 고치고 PR 을 여는** 서비스다.
어디를 고쳐야 하는지는 code-insight-engine(CIE)이 찾고, ACS 가 적용한다.

- 들어오는 곳: `POST /api/v1/tasks`, `POST /api/v1/chat` (`X-API-Key`) — 명세는 [API_SPEC.md](./API_SPEC.md)
- 도는 곳: systemd `auto-coder-swarm.service`, 포트 8006, 작업자 3
- 저장소 사본: `MASTER_REPOS_PATH`(기본 `/home/cnf/cie-repos`) → 작업마다 `/tmp/swarm_ws_*` 아래 워크트리

## 두 갈래

작업을 집으면 먼저 CIE 에 "이미 있는 값 옆에 값 하나를 더하는 요청인가" 를 묻는다
(`internal/orchestrator/variant_entry.go`). 그 답에 따라 길이 갈린다.

### 값 추가 흐름 — 여러 저장소 (`variant_*.go`)

1. CIE 가 씨앗값이 나오는 저장소를 모두 훑어 **저장소마다 넣을 자리**를 준다
2. 저장소마다 워크트리를 만들고 **아래에서 위로** 넣는다 — 위부터 넣으면 뒤 자리의 줄 번호가 밀린다
3. 자리마다 넣기 전후를 견줘 문법이 깨지면 **그 자리만** 되돌리고 까닭을 남긴다
4. 브랜치 `feat/add-<값>-<작업번호>` 로 PR 을 연다
5. proto 저장소는 PR 이 아니라 protogen 의 `make push-*apis` 로 배포한다 — 사람이 돌린다
6. 먼저 머지·배포돼야 하는 것이 있으면 뒤따르는 PR 은 **초안**으로 열고 본문 맨 앞에 무엇을 기다리는지 적는다

한 저장소가 실패해도 나머지는 계속한다. 요청의 전부는 "**어느 저장소를 고쳐야 하는지 다 보여 주는 것**" 이기 때문이다.

### 결함 흐름 — 한 저장소 (`flow_*.go`)

분석 → 계획 → 수정 → 검증 → 검토를 최대 세 번 돈다(`flow.go`).

- 검토자가 반대해도 고친 것을 버리지 않는다. 남아 있으면 반대 의견을 붙여 **승인 대기**로 사람에게 넘긴다
- 승인은 "본 것을 그대로 올린다" 는 뜻이라, 승인 뒤에는 다시 분석하지 않는다 — 사람이 본 diff 와 올라가는 diff 가 달라지면 검토의 뜻이 사라진다

## 검증 — 무엇으로 막는가

| 언어 | 도구 | 도구가 없으면 |
|---|---|---|
| Go | `gofmt -e` + `go build ./...` | Go 는 이 기계에 늘 있다 |
| Dart | `dart format --output=none` (`DART_BIN`) | "문법 확인 못 함" 으로 PR 에 적는다 |
| TS·JS·Svelte | `tools/tsparse/parse.js` (`TS_PARSER`) | 같다 |
| proto | 중괄호 짝 | `protoc` 은 두지 않는다 |
| 전부 | 넣기 전후의 괄호 균형 차이 | — |

원칙은 하나다. **검사가 돌지 않은 것을 통과로 세지 않는다.**
도구가 없거나 파서가 파일을 못 읽은 것은 통과가 아니라 "확인 못 함" 이고, 그대로 PR 에 적는다.
`scripts/install-checkers.sh` 가 검사기를 깔고 **일부러 틀린 파일이 걸리는지** 확인한다 — 검사기가 죽으면 조용히 통과로 보이기 때문이다.

우리가 고친 파일만 본다. 저장소 전체를 보면 원래 포맷이 안 맞던 남의 파일 때문에 멀쩡한 PR 이 막힌다.

## 규칙

- `master` 에 직접 밀지 않는다. 브랜치 → PR → 사람이 머지한다
- proto 는 `protoc` 을 직접 돌리거나 `*.pb.go` 를 손으로 커밋하지 않는다 — protogen 의 make 만 쓴다
- 빈 껍데기(`TODO`, `return nil`)를 넣지 않는다
- 사람에게 보이는 글에 거짓을 적지 않는다. 못 한 것은 못 했다고 적는다
- 파일 하나는 500줄 아래로, 한 가지 몫만 진다

## 시작하기

```bash
cp .env.example .env      # 설정값은 DEPLOYMENT.md 의 표를 본다
bash scripts/install-checkers.sh
go build -o bin/swarm ./cmd/swarm/main.go && ./bin/swarm
```

```bash
curl -X POST http://localhost:8006/api/v1/chat \
     -H "X-API-Key: $SWARM_API_KEY" \
     -d '{"message":"소셜로그인 타입에 instagram 을 더해줘"}'
```

## 문서

* [DEPLOYMENT.md](./DEPLOYMENT.md) — 설치, 설정값, 의존 서비스, 배포
* [API_SPEC.md](./API_SPEC.md) — REST·SSE·NATS 명세
* [CLAUDE.md](./CLAUDE.md) — 이 저장소에서 일하는 규칙
* [PROJECTS.md](./PROJECTS.md) · [PROGRESS.md](./PROGRESS.md) — 사양과 진행

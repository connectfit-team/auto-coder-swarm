# 이 저장소에서 일하는 규칙 — Auto-Coder Swarm

## 무엇을 지키나

1. **빈 껍데기를 넣지 않는다.** `Mock content`, `TODO`, 자리만 채운 `return nil` 을
   프로덕션 코드에 넣지 않는다. 못 하는 것은 넣지 말고 못 한다고 적는다.
2. **고쳤다고 말하기 전에 돌려 본다.** 빌드하고, 시험을 돌리고, 값 추가 흐름은
   `~/variant-battery.sh` 아홉 예제의 숫자가 바뀌지 않았는지 본다.
3. **검사가 돌지 않은 것을 통과로 세지 않는다.** 도구가 없거나 파서가 파일을 못
   읽은 것은 통과가 아니라 "확인 못 함" 이고, 그대로 사람에게 적는다. 이 원칙을
   어기는 실수가 이 저장소에서 가장 자주 났다 — 종료 코드를 뒤집어 읽고, 확장자
   없는 임시 파일을 검사하고, 없는 파일에 0 을 받았다.
4. **남기는 글에 거짓을 적지 않는다.** "이미 다 들어 있다" 와 "넣으려다 되돌렸다"
   는 다른 말이다. 되돌린 자리는 까닭과 함께 남긴다.
5. **`master` 에 직접 밀지 않는다.** 브랜치를 따고 PR 을 열고, 머지는 사람이 한다.
   `git commit`·`git push` 는 브랜치에만 한다.
6. **proto 는 protogen 의 make 로만 배포한다.** `protoc` 을 직접 돌리거나
   `*.pb.go` 를 손으로 커밋하지 않는다. ACS 도 proto 저장소에는 PR 을 열지 않고
   `make push-*apis` 를 남긴다.
7. **파일 하나는 500줄 아래로, 한 가지 몫만.** 사람이 읽는 글과 판단하는 코드를
   같은 파일에 섞지 않는다(`variant_report.go` 가 그 자리다).

## 주석

짧게, 목적만. 코드를 읽어서 알 수 있는 것은 쓰지 않는다.

- 쓸 것: 왜 이렇게 했는지(코드만 봐선 알 수 없는 판단·제약), 함정(이 값을 바꾸면
  무엇이 조용히 깨지나)
- 쓰지 말 것: 변경 이력("전에는 X였는데"), 코드를 그대로 옮긴 설명, 강조 수사

이력과 근거는 커밋 메시지와 PR 본문에 적는다.

## 구조

- `internal/orchestrator/variant_*.go` — 값 추가 흐름(여러 저장소)
- `internal/orchestrator/flow_*.go` — 결함 흐름(한 저장소)
- `internal/orchestrator/{dart,node,go_build}_check.go` — 문법·타입 검사
- `tools/tsparse/` — TS·Svelte 파서. `scripts/install-checkers.sh` 가 `~/tools` 에 깐다
- 자세한 흐름은 [README.md](./README.md), 설정은 [DEPLOYMENT.md](./DEPLOYMENT.md)

## 도구

- **PR 은 `gh` 로 연다.** 같은 PR 이 이미 열려 있는지도 `gh pr list` 로 본다
  (`internal/gitmgr`, `internal/orchestrator/variant_dup.go`).
- **git 원격은 SSH 로 쓴다.** 사무실 기계의 신원은 `cnf-swbae` /
  `cnf-office-server`, 열쇠는 `~/.ssh/id_ed25519_cnf_office` 다.
- 모든 API 통신에 `X-API-Key` 가 필요하다.
- 작업 사본은 `/tmp/swarm_ws_*` 에 만들고 끝나면 지운다. 저장소 안에 검사용
  파일을 쓰지 않는다 — `git add .` 가 도는 흐름이라 PR 에 딸려 들어간다.

## 실행하는 곳

빌드·시험·DB·실제 작업은 모두 사무실 리눅스(`cnf@office`)에서 돈다.
서비스는 systemd `auto-coder-swarm.service`(포트 8006)이고, 배포는 `./deploy.sh` 가
**원격 브랜치**를 기준으로 한다 — 밀지 않은 커밋은 배포되지 않고 지워진다.

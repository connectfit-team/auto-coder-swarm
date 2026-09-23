package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
)

// 계약을 고칠 곳은 펴낸 결과물이 아니라 원본 .proto 다. 그 원본은
// proto-<apis> 저장소에 있다.
//
// 한때 protogen 을 임자로 넘겼다. protogen 은 그 저장소들을 **서브모듈로**
// 품고 있을 뿐이라 워크트리에는 그 경로가 없다 — 넘겨받은 자식이 「그 경로는
// protogen 에 없다」 로 세 번 돌다 죽었고, 연쇄 17건이 전부 산출물 0이었다.
// 사본의 작업 트리에는 서브모듈이 풀려 있어 os.Stat 이 통과한 것이 화근이다.
//
//	웹의 생성물   src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts
//	원본          proto-ceowebapis 의 ceoweb/v1/connect.service.proto
//	펴내는 법     protogen 에서 make push-ceowebapis
//	              (goprotoc.sh 로 컴파일하고 그 서브모듈을 커밋·푸시한다)
//
// 모델에게 묻지 않는다. 경로를 바꿔 파일이 있는지 보면 된다.

// protoSource 는 생성물 경로에서 원본 proto 의 저장소·경로·펴내는 법을 준다.
// 원본을 못 찾으면 빈 값이다.
func (t *taskContext) protoSource(generated string) (repo, protoPath, howToPublish string) {
	apis, rest := apisAndRest(generated)
	if apis == "" || rest == "" {
		return "", "", ""
	}
	repo = "proto-" + apis
	protoPath = strings.TrimSuffix(rest, filepath.Ext(rest)) + ".proto"

	root := t.orchestrator.wsMgr.RepoPath(repo)
	if root == "" {
		return "", "", ""
	}
	if st, err := os.Stat(filepath.Join(root, protoPath)); err != nil || st.IsDir() {
		return "", "", ""
	}
	return repo, protoPath, "protogen 에서 make push-" + apis
}

// apisAndRest 는 생성물 경로를 「어느 apis」 와 「그 안의 경로」 로 가른다.
func apisAndRest(generated string) (apis, rest string) {
	g := filepath.ToSlash(generated)
	const marker = "/protos/"
	switch {
	case strings.Contains(g, marker):
		rest = g[strings.Index(g, marker)+len(marker):]
	case strings.HasPrefix(g, "protos/"):
		rest = strings.TrimPrefix(g, "protos/")
	default:
		return "", ""
	}
	i := strings.Index(rest, "/")
	if i <= 0 {
		return "", ""
	}
	apis = strings.TrimPrefix(rest[:i], "proto-")
	return apis, rest[i+1:]
}

// howToPublishRepo 는 계약 저장소 이름에서 펴내는 법을 만든다.
//
// 계약을 고친 일이 승인을 기다리며 멈추면 결과가 비어 있었다. 사람이 볼
// 화면에 **다음 걸음**이 없는 것이다 — 이 저장소는 밀어서 펴내는 것이 아니라
// protogen 에서 make 로 펴낸다는 사실이 코드 어디에도 안 적혀 있다.
func howToPublishRepo(repoName string) string {
	apis := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(repoName)), "proto-")
	if apis == "" || apis == strings.ToLower(strings.TrimSpace(repoName)) {
		return ""
	}
	return "이 계약은 밀어서 펴내지 않는다. **protogen 에서 `make push-" + apis + "`** 를 돌리면\n" +
		"컴파일하고 커밋·푸시까지 한다. 소비하는 저장소는 그 뒤에 go get 한다.\n" +
		"protoc 를 직접 돌리거나 생성물(*.pb.go·생성된 .ts)을 손으로 커밋하지 않는다."
}

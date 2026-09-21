package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
)

// 계약을 고칠 곳은 **펴낸 결과물이 아니라 원본**이다.
//
// 담을 자리가 없을 때 proto-ceowebapis 에 일을 만들었다. 그런데 그 저장소는
// 펴낸 결과물이다 — 내용은 protogen 의 goprotoc.sh 가 생성하고
// `make push-ceowebapis` 가 밀어 넣는다. 거기를 고치라는 것은 **생성물을
// 손으로 고치라는 뜻**이고, 다음 발행 때 조용히 덮어써진다.
//
// 원본은 protogen 에 있고 경로가 그대로 대응한다.
//
//	웹의 생성물   src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts
//	원본          protogen/ceowebapis/ceoweb/v1/connect.service.proto
//	펴내는 법     make push-ceowebapis
//
// 모델에게 묻지 않는다. 경로를 바꿔 파일이 있는지 보면 된다.

const protoSourceRepo = "protogen"

// protoSource 는 생성물 경로에서 **원본 proto** 의 저장소·경로·발행 목표를 준다.
// 원본을 못 찾으면 빈 값 — 그때는 펴낸 저장소를 쓴다.
func (t *taskContext) protoSource(generated string) (repo, protoPath, makeTarget string) {
	g := filepath.ToSlash(generated)
	i := strings.Index(g, "/protos/")
	if i < 0 {
		if strings.HasPrefix(g, "protos/") {
			i = -1 // 맨 앞인 경우
		} else {
			return "", "", ""
		}
	}
	rest := g[i+len("/protos/"):]
	if i == -1 {
		rest = strings.TrimPrefix(g, "protos/")
	}
	apis := rest
	if j := strings.Index(rest, "/"); j > 0 {
		apis = rest[:j]
	}
	if apis == "" {
		return "", "", ""
	}
	apis = strings.TrimPrefix(apis, "proto-")

	rel := strings.TrimSuffix(rest, filepath.Ext(rest)) + ".proto"
	// protogen 안에서는 apis 이름이 폴더 이름 그대로다.
	rel = strings.Replace(rel, apis+"/", apis+"/", 1)

	root := t.orchestrator.wsMgr.RepoPath(protoSourceRepo)
	if root == "" {
		return "", "", ""
	}
	if st, err := os.Stat(filepath.Join(root, rel)); err != nil || st.IsDir() {
		return "", "", ""
	}
	return protoSourceRepo, rel, "make push-" + apis
}

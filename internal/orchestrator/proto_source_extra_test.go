package orchestrator

import (
	"strings"
	"testing"
)

// 생성물 경로에서 apis 와 그 안의 경로를 가른다.
func TestApisAndRest(t *testing.T) {
	cases := []struct {
		in, apis, rest string
	}{
		{"src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts", "ceowebapis", "ceoweb/v1/connect.service.ts"},
		{"protos/userapis/user/v1/service.ts", "userapis", "user/v1/service.ts"},
		{"src/protos/proto-workstampapis/workstamp/v1/service.ts", "workstampapis", "workstamp/v1/service.ts"},
		{"src/lib/types/connectactions.ts", "", ""},
		{"src/lib/server/protos/onlyapis", "", ""},
	}
	for _, c := range cases {
		apis, rest := apisAndRest(c.in)
		if apis != c.apis || rest != c.rest {
			t.Errorf("%s → (%q, %q), 기대 (%q, %q)", c.in, apis, rest, c.apis, c.rest)
		}
	}
}

// 원본은 proto-<apis> 저장소에 있다. protogen 은 그것을 서브모듈로 품을 뿐이라
// protogen 워크트리에는 그 경로가 없다.
func TestProtoSourcePointsAtTheSubmoduleRepo(t *testing.T) {
	src := readSource(t, "proto_source.go")
	for _, forbidden := range []string{"protoSourceRepo", `"protogen"`} {
		if strings.Contains(src, forbidden) {
			t.Errorf("원본 저장소를 protogen 으로 넘기는 옛 모양이 남아 있다: %s", forbidden)
		}
	}
	for _, must := range []string{`"proto-" + apis`, "make push-"} {
		if !strings.Contains(src, must) {
			t.Errorf("proto_source.go 에 %q 가 없다", must)
		}
	}
}

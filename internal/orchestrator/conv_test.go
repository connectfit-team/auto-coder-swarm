package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 관행은 물어볼 것이 아니라 세면 되는 것이다.
func TestProtoConventionCountsWhatTheRepoDoes(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("ceoweb/v1/a.service.proto", `service Internal {
  rpc X(RequestX) returns (ResponseX) {}
}
message RequestX {}
message ResponseX {}
`)
	mk("ceoweb/v1/b.service.proto", `service Internal {
  rpc Y(RequestY) returns (ResponseY) {}
}
message RequestY {}
message ResponseY {}
`)
	// 다른 폴더는 세지 않는다
	mk("other/v1/c.proto", "service Elsewhere {}\nmessage FooRequest {}\n")

	got := protoConvention(root, "ceoweb/v1/a.service.proto", "ceoweb/v1/b.service.proto")
	for _, must := range []string{"Internal 하나뿐", "새 서비스를 만들지 말고", "접두형"} {
		if !strings.Contains(got, must) {
			t.Errorf("관행에 %q 가 없다:\n%s", must, got)
		}
	}
	if strings.Contains(got, "Elsewhere") {
		t.Errorf("다른 폴더의 관행까지 실었다:\n%s", got)
	}
	if got := protoConvention("", "a", "b"); got != "" {
		t.Errorf("경로가 없는데 관행을 지어냈다: %q", got)
	}
}

// 넘기는 쪽지에 관행이 실려야 한다.
func TestHandoffCarriesTheConvention(t *testing.T) {
	src := readSource(t, "state_chain.go")
	if !strings.Contains(src, "protoConvention(") {
		t.Error("넘길 때 그 계약의 관행을 알려 주지 않는다")
	}
}

package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Dart 는 파싱에 실패해도 종료 코드가 0 이다. 글귀로 판단해야 한다.
func TestDartParseFailureIsCaught(t *testing.T) {
	if dartBin() == "" {
		t.Skip("dart 없음")
	}
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	ok := write("ok.dart", "void main() {\n  print(1);\n}\n")
	odd := write("odd.dart", "void main(){print(1);}\n") // 포맷만 다르다
	bad := write("bad.dart", "void main() {\n  } else if (x) {\n}\n")

	if msg := dartParses(ok); msg != "" {
		t.Errorf("멀쩡한 파일을 못 읽는다고 했다: %s", msg)
	}
	if msg := dartParses(odd); msg != "" {
		t.Errorf("포맷 차이를 문법 오류로 봤다: %s", msg)
	}
	msg := dartParses(bad)
	if msg == "" {
		t.Fatal("깨진 파일을 통과시켰다")
	}
	if !strings.Contains(msg, "line ") {
		t.Errorf("어디가 틀렸는지 없다: %s", msg)
	}
}

// 없는 파일에도 dart format 은 종료 코드 0 을 준다.
// 그것을 통과로 세면, 경로가 틀린 순간 Dart 검사가 조용히 사라진다.
func TestDartMissingFileIsNotPass(t *testing.T) {
	if dartBin() == "" {
		t.Skip("dart 없음")
	}
	if msg := dartParses(filepath.Join(t.TempDir(), "없는파일.dart")); msg == "" {
		t.Fatal("없는 파일이 통과했다")
	}
}

// 검증한 언어는 "확인 못 함" 에 들어가지 않는다.
func TestDartNotListedAsUnverifiedWhenSdkExists(t *testing.T) {
	if dartBin() == "" {
		t.Skip("dart 없음")
	}
	for _, k := range unverifiedKinds([]string{"a/b.dart"}) {
		if k == ".dart" {
			t.Error("dart 가 있는데 확인 못 했다고 적었다")
		}
	}
}

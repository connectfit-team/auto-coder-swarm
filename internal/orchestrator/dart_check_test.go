package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

// Dart 는 파싱에 실패해도 종료 코드가 0 이다. 글귀로 판단해야 한다.
func TestDartParseFailureIsCaught(t *testing.T) {
	if dartBin() == "" {
		t.Skip("dart 없음")
	}
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("ok.dart", "void main() {\n  print(1);\n}\n")
	write("odd.dart", "void main(){print(1);}\n") // 포맷만 다르다
	write("bad.dart", "void main() {\n  } else if (x) {\n}\n")

	if !dartParses(dir, "ok.dart") {
		t.Error("멀쩡한 파일을 못 읽는다고 했다")
	}
	if !dartParses(dir, "odd.dart") {
		t.Error("포맷 차이를 문법 오류로 봤다")
	}
	if dartParses(dir, "bad.dart") {
		t.Error("깨진 파일을 통과시켰다")
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

package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

// 종류 판별을 LLM 에 맡겼더니 빈 빌드 명령이 돌아왔고, `bash -c ""` 는 exit 0
// 이라 아무것도 검증하지 않고 "성공" 으로 지나갔다. 파일로 알 수 있는 것을
// 모델에게 물을 이유가 없다.
func TestDetectProjectFallback(t *testing.T) {
	cases := []struct{ marker, kind, build string }{
		{"go.mod", "Go", "go build ./... && go test -run '^$' -count=1 -vet=off ./..."},
		{"pubspec.yaml", "Flutter", "flutter analyze"},
		{"package.json", "NodeJS", "npm run build --if-present"},
		{"Cargo.toml", "Rust", "cargo check"},
	}
	for _, c := range cases {
		t.Run(c.marker, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, c.marker), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			kind, build := detectProjectFallback(dir)
			if kind != c.kind || build != c.build {
				t.Errorf("got (%q, %q), 기대 (%q, %q)", kind, build, c.kind, c.build)
			}
		})
	}
}

// Flutter 저장소 안에 node 도구가 섞여 있는 일이 잦다. 앞의 것이 이긴다.
func TestDetectProjectFallbackPrefersFirstMatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("x"), 0o644)
	if kind, _ := detectProjectFallback(dir); kind != "Flutter" {
		t.Errorf("got %q, 기대 Flutter", kind)
	}
}

// 표식이 없으면 빈 값을 준다 — 부르는 쪽이 그걸 보고 멈춘다.
// 아무 명령이나 지어내면 다시 조용히 통과한다.
func TestDetectProjectFallbackEmptyWhenUnknown(t *testing.T) {
	if kind, build := detectProjectFallback(t.TempDir()); kind != "" || build != "" {
		t.Errorf("모르는 저장소에 값을 지어냈다: %q %q", kind, build)
	}
}

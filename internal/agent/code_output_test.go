package agent

import (
	"strings"
	"testing"
)

// 실제로 빌드를 죽인 모양이다:
//
//	internal/domain/tag.go:1:1: expected 'package', found ``
//
// 프롬프트에 "코드만 내라" 고 적어도 지켜지지 않는다.
func TestCleanCodeOutputStripsFence(t *testing.T) {
	cases := []struct{ name, raw string }{
		{"펜스에 언어 표시", "```go\npackage domain\n\nfunc A() {}\n```"},
		{"펜스만", "```\npackage domain\n\nfunc A() {}\n```"},
		{"앞에 설명", "수정한 코드입니다:\n\n```go\npackage domain\n\nfunc A() {}\n```"},
		{"뒤에도 설명", "```go\npackage domain\n\nfunc A() {}\n```\n\n이렇게 고쳤습니다."},
		{"닫는 펜스를 빠뜨림", "```go\npackage domain\n\nfunc A() {}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CleanCodeOutput(c.raw)
			if !strings.HasPrefix(got, "package domain") {
				t.Errorf("코드로 시작하지 않는다: %q", got)
			}
			if strings.Contains(got, "```") {
				t.Errorf("펜스가 남았다: %q", got)
			}
		})
	}
}

// 설명 안에 짧은 예시가 섞여 있으면 본문을 골라야 한다.
func TestCleanCodeOutputPicksLongestBlock(t *testing.T) {
	raw := "예를 들면 ```go\nvar x int\n``` 처럼 씁니다. 전체는 이렇습니다:\n\n```go\npackage domain\n\ntype TagType int\n\nfunc (t TagType) String() string { return \"\" }\n```"
	got := CleanCodeOutput(raw)
	if !strings.HasPrefix(got, "package domain") {
		t.Errorf("짧은 예시를 골랐다: %q", got)
	}
}

// 펜스가 없으면 앞의 산문만 걷어낸다.
func TestCleanCodeOutputStripsLeadingProse(t *testing.T) {
	got := CleanCodeOutput("아래와 같이 고쳤습니다.\n\npackage domain\n\nfunc A() {}")
	if !strings.HasPrefix(got, "package domain") {
		t.Errorf("산문이 남았다: %q", got)
	}
}

// 멀쩡한 코드는 건드리지 않는다. 이게 대부분의 경우다.
func TestCleanCodeOutputLeavesCleanCode(t *testing.T) {
	src := "package domain\n\nimport \"fmt\"\n\nfunc A() { fmt.Println(\"x\") }"
	if got := CleanCodeOutput(src); got != src {
		t.Errorf("멀쩡한 코드를 바꿨다:\n%q", got)
	}
}

// 코드 한가운데를 자르면 안 된다. 앞부분만 본다.
func TestCleanCodeOutputDoesNotCutMidFile(t *testing.T) {
	src := "// 이 파일은 태그를 다룬다\n// 두 번째 줄\npackage domain\n\nfunc A() {}"
	got := CleanCodeOutput(src)
	if !strings.HasPrefix(got, "// 이 파일은") {
		t.Errorf("맨 앞 주석을 잘랐다: %q", got)
	}
}

func TestCleanCodeOutputEmpty(t *testing.T) {
	if CleanCodeOutput("   ") != "" {
		t.Error("빈 입력은 빈 출력이어야 한다")
	}
}

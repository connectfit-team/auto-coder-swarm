package agentloop

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/mcpclient"
)

func toolsNamed(names ...string) []mcpclient.Tool {
	var out []mcpclient.Tool
	for _, n := range names {
		out = append(out, mcpclient.Tool{Name: n, Schema: json.RawMessage(`{}`)})
	}
	return out
}

// 앞에서부터 자르면 안 된다. REA 의 SKILL.md 는 앞 1,500바이트가 「서버 등록법」
// 이고 — 이미 붙어 있는 우리에겐 쓸모가 없다 — 정작 필요한 「무엇부터 열어라」
// 표는 한가운데서 잘렸다.
func TestPickSectionsKeepsWhatNamesOurTools(t *testing.T) {
	body := "# REA\n도구 지침이다.\n\n" +
		"## Connect only when needed\n" +
		strings.Repeat("서버를 등록하는 이야기다. 도구 이름은 안 나온다. ", 40) + "\n\n" +
		"## Route the target first\n" +
		"- native binary: open_binary\n- android: inspect_android_package\n\n" +
		"## Finish the task\n" +
		strings.Repeat("마무리 이야기다. ", 20)

	got := pickSections(body, toolsNamed("open_binary", "inspect_android_package"), 400)

	if !strings.Contains(got, "open_binary") || !strings.Contains(got, "inspect_android_package") {
		t.Errorf("도구를 부르는 절을 버렸다:\n%s", got)
	}
	if strings.Contains(got, "서버를 등록하는 이야기") {
		t.Errorf("도구를 하나도 안 부르는 절을 실었다:\n%s", got)
	}
	// 머리말은 무엇을 하는 지침인지 알려 주므로 남긴다.
	if !strings.Contains(got, "도구 지침이다") {
		t.Errorf("머리말을 버렸다:\n%s", got)
	}
}

// 절의 차례는 지킨다 — 뒤섞으면 읽기 어렵다.
func TestPickSectionsKeepsOriginalOrder(t *testing.T) {
	body := "# 머리\n\n## 가\nopen_binary 하나\n\n## 나\nopen_binary 와 list_names 둘\n"
	got := pickSections(body, toolsNamed("open_binary", "list_names"), 400)
	if strings.Index(got, "## 가") > strings.Index(got, "## 나") {
		t.Errorf("차례가 뒤집혔다:\n%s", got)
	}
}

// 도구를 모르면 전처럼 앞에서 줄인다.
func TestPickSectionsFallsBackWithoutTools(t *testing.T) {
	body := "# 머리\n\n## 가\n내용\n"
	if got := pickSections(body, nil, 400); !strings.Contains(got, "내용") {
		t.Errorf("도구가 없을 때 빈 글을 냈다: %q", got)
	}
}

// **가장 쓸모 있는 절은 커도 버리지 않는다.** 건너뛰었더니 REA 의 「무엇부터
// 열어라」 표가 통째로 빠지고 점수가 3/4 에서 0/4 로 떨어졌다.
func TestBiggestUsefulSectionIsClippedNotDropped(t *testing.T) {
	routing := "## Route the target first\n" +
		"- native binary: open_binary 를 먼저 부른다\n" +
		strings.Repeat("표가 길다. open_binary 를 쓴다. ", 200)
	body := "# 머리\n\n" + routing + "\n\n## 작은 절\nlist_names 하나\n"

	got := pickSections(body, toolsNamed("open_binary", "list_names"), 300)

	if !strings.Contains(got, "open_binary 를 먼저 부른다") {
		t.Errorf("가장 쓸모 있는 절을 통째로 버렸다:\n%s", got)
	}
	if !strings.Contains(got, "줄였다") {
		t.Error("줄였다고 안 알린다")
	}
}

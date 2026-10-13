package agentloop

import (
	"strings"
	"testing"
)

func TestReplyLanguageFromEnv(t *testing.T) {
	t.Setenv("SWARM_REPLY_LANGUAGE", "")
	t.Setenv("LANGUAGE", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")

	t.Setenv("LANG", "ko_KR.UTF-8")
	if got := replyLanguage(); got != "한국어" {
		t.Errorf("ko_KR.UTF-8 → %q", got)
	}
	t.Setenv("LANG", "en_US.UTF-8")
	if got := replyLanguage(); got != "English" {
		t.Errorf("en_US.UTF-8 → %q", got)
	}
	// 모르는 말은 억지로 고르지 않는다.
	t.Setenv("LANG", "xx_YY.UTF-8")
	if got := replyLanguage(); got != "" {
		t.Errorf("모르는 말인데 골랐다: %q", got)
	}
	// 손으로 적은 것이 먼저다.
	t.Setenv("SWARM_REPLY_LANGUAGE", "日本語")
	if got := replyLanguage(); got != "日本語" {
		t.Errorf("손으로 적은 것을 안 쓴다: %q", got)
	}
}

// 사람에게 하는 말과 기계에게 하는 말을 **가른다.**
// 안 가르면 둘 다 반만 된다 — 실측에서 find_tools 질의가 한국어로 나가 영어
// 카탈로그와 하나도 안 맞았다(#210).
func TestLanguageRuleSplitsHumanAndMachine(t *testing.T) {
	t.Setenv("SWARM_REPLY_LANGUAGE", "한국어")

	withTools := languageRule(true)
	if !strings.Contains(withTools, "한국어 로 쓴다") {
		t.Errorf("답할 말을 안 정했다:\n%s", withTools)
	}
	if !strings.Contains(withTools, "영어로 쓴다") {
		t.Errorf("도구에 넘길 말을 안 정했다:\n%s", withTools)
	}

	// 도구 이름이 한국어면 영어로 찾으라고 하면 안 된다.
	korean := languageRule(false)
	if strings.Contains(korean, "영어로 쓴다") {
		t.Errorf("도구가 영어가 아닌데 영어로 찾으라고 한다:\n%s", korean)
	}

	// 못 알아내면 묻는 사람 말을 따른다 — aider 와 같다.
	t.Setenv("SWARM_REPLY_LANGUAGE", "")
	t.Setenv("LANG", "")
	t.Setenv("LANGUAGE", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	if got := languageRule(true); !strings.Contains(got, "묻는 사람이 쓴 말") {
		t.Errorf("못 알아냈을 때 기본이 없다:\n%s", got)
	}
}

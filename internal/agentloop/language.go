package agentloop

import (
	"os"
	"strings"
)

// 사람에게 하는 답의 말. 못 알아내면 「묻는 사람이 쓰는 말」 이다.
//
// aider 가 같은 일을 한다(base_coder.py:1094 get_user_language) —
// --chat-language → locale → LANG/LANGUAGE/LC_ALL/LC_MESSAGES 차례로 보고,
// 프롬프트에 `Reply in Korean.` 한 줄을 넣는다. **번역하지 않는다.**
// codex 와 hermes 에는 언어 처리가 아예 없다(원본 대조 2026-10-13).
func replyLanguage() string {
	if v := strings.TrimSpace(os.Getenv("SWARM_REPLY_LANGUAGE")); v != "" {
		return v
	}
	for _, k := range []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		v := strings.TrimSpace(os.Getenv(k))
		if v == "" {
			continue
		}
		code := strings.ToLower(strings.SplitN(strings.SplitN(v, ".", 2)[0], "_", 2)[0])
		if name, ok := languageNames[code]; ok {
			return name
		}
	}
	return ""
}

var languageNames = map[string]string{
	"ko": "한국어", "en": "English", "ja": "日本語",
	"zh": "中文", "es": "Español", "fr": "Français", "de": "Deutsch",
}

// languageRule 은 **사람에게 하는 말과 기계에게 하는 말을 가른다.**
//
// 이 둘을 안 가르면 둘 다 반만 된다. 실측에서 find_tools 질의가 한국어로 나가
// 영어 카탈로그와 하나도 안 맞았고, 그 판은 도구를 못 찾았다(#210).
// 거꾸로 답까지 영어로 하게 하면 읽는 사람이 손해다.
func languageRule(toolsAreEnglish bool) string {
	lang := replyLanguage()
	if lang == "" {
		lang = "묻는 사람이 쓴 말"
	}
	rule := "[말]\n- 사람에게 하는 답은 " + lang + " 로 쓴다."
	if toolsAreEnglish {
		rule += "\n- **도구에 넘기는 값은 영어로 쓴다** — 도구 이름·찾을 낱말·" +
			"경로는 영어다. 한국어로 찾으면 하나도 안 맞는다."
	}
	return rule
}

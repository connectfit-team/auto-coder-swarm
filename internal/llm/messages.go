package llm

import (
	"strings"

	"google.golang.org/adk/model"
)

// BuildMessages 는 대화를 **역할이 붙은 메시지 배열**로 만든다.
//
// 전에는 BuildPrompt 로 전부 눌러 `[USER]:`·`[ASSISTANT]:` 를 글자로 적고
// **사용자 메시지 하나**로 보냈다. 이 모델(Qwen2.5-Coder)의 대화 틀은 역할을
// 토큰으로 두르게 되어 있어서, 글자로 적은 역할은 그만큼 약하다. 모델이 제
// 지난 턴을 제 것으로 못 알아보면 이어 가지 않고 끝내 버린다.
//
// codex·aider·hermes 는 모두 역할이 붙은 배열을 보낸다.
//
// **한 턴짜리는 전과 똑같다.** ACS 의 다른 에이전트들(계획·코딩·비평·검토)은
// 사용자 메시지 하나만 보내므로 결과가 바뀌지 않는다.
func BuildMessages(req *model.LLMRequest) []map[string]string {
	if req == nil {
		return []map[string]string{{"role": "user", "content": ""}}
	}
	if len(req.Contents) == 0 {
		return []map[string]string{{"role": "user", "content": BuildPrompt(req)}}
	}

	tools := toolBlock(req)
	var out []map[string]string
	for i, c := range req.Contents {
		var txt strings.Builder
		for _, p := range c.Parts {
			txt.WriteString(partText(p))
		}
		body := txt.String()
		if i == 0 && tools != "" {
			body += tools
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		out = append(out, map[string]string{"role": roleOf(c.Role), "content": body})
	}
	if len(out) == 0 {
		return []map[string]string{{"role": "user", "content": BuildPrompt(req)}}
	}
	return out
}

func roleOf(r string) string {
	switch r {
	case "model", "assistant":
		return "assistant"
	case "system":
		return "system"
	default:
		return "user"
	}
}

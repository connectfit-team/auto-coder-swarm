package llm

import (
	"os"
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
// **다만 그들은 도구 결과를 `tool` 역할로 보낸다.** 우리는 네이티브 tool_calls
// 를 못 써서(vLLM 의 hermes 파서가 이 모델의 <tools> 를 못 잡는다) 도구 결과를
// user 로 넣는데, 역할이 진짜로 붙으면 모델이 그것을 **「사람이 새로 말을
// 걸었다」** 로 읽고 대화하듯 끝내 버린다. 실측에서 12문항 9/12 → 7/12 로
// 떨어졌고 평균 68초 → 14초로 **훨씬 빨리 포기했다.**
//
// 그래서 이것은 **기본으로 켜지 않는다.** 네이티브 tool_calls 가 되면 그때
// tool 역할과 함께 켠다.
//
// **한 턴짜리는 전과 똑같다.** ACS 의 다른 에이전트들(계획·코딩·비평·검토)은
// 사용자 메시지 하나만 보내므로 결과가 바뀌지 않는다.
func BuildMessages(req *model.LLMRequest) []map[string]string {
	if !rolesEnabled() {
		return []map[string]string{{"role": "user", "content": BuildPrompt(req)}}
	}
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

// rolesEnabled 는 역할이 붙은 배열을 쓸지 정한다.
//
// 재 보고 나빠서 기본은 꺼 둔다(LLM_ROLE_MESSAGES=1 로 켠다). 네이티브
// tool_calls 가 되면 그때 tool 역할과 함께 기본으로 켠다.
func rolesEnabled() bool {
	return strings.TrimSpace(os.Getenv("LLM_ROLE_MESSAGES")) == "1"
}

package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// BuildPrompt 는 대화와 도구 설명을 한 덩어리 프롬프트로 만든다.
// 전송 수단(큐·직접 호출)이 달라도 프롬프트는 같아야 한다.
func BuildPrompt(req *model.LLMRequest) string {
	toolDesc := toolBlock(req)

	var prompt strings.Builder
	for i, c := range req.Contents {
		txt := ""
		for _, p := range c.Parts {
			txt += partText(p)
		}
		if i == 0 && toolDesc != "" {
			txt += toolDesc
		}
		role := c.Role
		if role == "model" {
			role = "assistant"
		}
		prompt.WriteString(fmt.Sprintf("[%s]:\n%s\n\n", strings.ToUpper(role), txt))
	}
	return prompt.String()
}

// toolBlock 과 partText 는 BuildPrompt 와 BuildMessages 가 **같이 쓴다.**
// 두 곳에서 따로 만들면 한쪽만 고쳐져 서로 다른 글이 모델에 간다.
func toolBlock(req *model.LLMRequest) string {
	if req == nil || req.Config == nil || req.Config.Tools == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n[MANDATORY TOOL CALL FORMAT]\nTo use a tool, you MUST output: <|tool_call|>call:tool_name{\"arg\":\"val\"}<|tool_call|>\n\n[Available Tools]\n")
	for _, t := range req.Config.Tools {
		for _, fd := range t.FunctionDeclarations {
			params, _ := json.Marshal(fd.ParametersJsonSchema)
			fmt.Fprintf(&b, "- %s: %s (Parameters: %s)\n", fd.Name, fd.Description, string(params))
		}
	}
	return b.String()
}

func partText(p *genai.Part) string {
	switch {
	case p.Text != "":
		return p.Text
	case p.FunctionCall != nil:
		args, _ := json.Marshal(p.FunctionCall.Args)
		return fmt.Sprintf("<|tool_call|>call:%s%s<|tool_call|>", p.FunctionCall.Name, string(args))
	case p.FunctionResponse != nil:
		resp, _ := json.Marshal(p.FunctionResponse.Response)
		return fmt.Sprintf("\n[Tool Result: %s] %s\n", p.FunctionResponse.Name, string(resp))
	}
	return ""
}

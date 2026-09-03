package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
)

// BuildPrompt 는 대화와 도구 설명을 한 덩어리 프롬프트로 만든다.
// 전송 수단(큐·직접 호출)이 달라도 프롬프트는 같아야 한다.
func BuildPrompt(req *model.LLMRequest) string {
	var toolDesc strings.Builder
	if req.Config != nil && req.Config.Tools != nil {
		toolDesc.WriteString("\n\n[MANDATORY TOOL CALL FORMAT]\nTo use a tool, you MUST output: <|tool_call|>call:tool_name{\"arg\":\"val\"}<|tool_call|>\n\n[Available Tools]\n")
		for _, t := range req.Config.Tools {
			for _, fd := range t.FunctionDeclarations {
				params, _ := json.Marshal(fd.ParametersJsonSchema)
				toolDesc.WriteString(fmt.Sprintf("- %s: %s (Parameters: %s)\n", fd.Name, fd.Description, string(params)))
			}
		}
	}

	var prompt strings.Builder
	for i, c := range req.Contents {
		txt := ""
		for _, p := range c.Parts {
			if p.Text != "" {
				txt += p.Text
			} else if p.FunctionCall != nil {
				args, _ := json.Marshal(p.FunctionCall.Args)
				txt += fmt.Sprintf("<|tool_call|>call:%s%s<|tool_call|>", p.FunctionCall.Name, string(args))
			} else if p.FunctionResponse != nil {
				resp, _ := json.Marshal(p.FunctionResponse.Response)
				txt += fmt.Sprintf("\n[Tool Result: %s] %s\n", p.FunctionResponse.Name, string(resp))
			}
		}

		if i == 0 && toolDesc.Len() > 0 {
			txt += toolDesc.String()
		}

		role := c.Role
		if role == "model" {
			role = "assistant"
		}
		prompt.WriteString(fmt.Sprintf("[%s]:\n%s\n\n", strings.ToUpper(role), txt))
	}
	return prompt.String()
}

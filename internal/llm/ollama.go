package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"regexp"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type ollamaModel struct {
	name    string
	baseURL string
}

func NewOllamaModel(name, baseURL string) model.LLM {
	return &ollamaModel{
		name:    name,
		baseURL: baseURL,
	}
}

func (m *ollamaModel) Name() string {
	return m.name
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
	KeepAlive string         `json:"keep_alive,omitempty"`
}

type ollamaResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
}

func (m *ollamaModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		var toolDesc strings.Builder
		if req.Config != nil && req.Config.Tools != nil {
			toolDesc.WriteString("\n\n[MANDATORY TOOL CALL FORMAT]\n")
			toolDesc.WriteString("To use a tool, you MUST output: <|tool_call|>call:tool_name{\"arg\":\"val\"}<|tool_call|>\n\n[Available Tools]\n")
			for _, t := range req.Config.Tools {
				for _, fd := range t.FunctionDeclarations {
					params, _ := json.Marshal(fd.ParametersJsonSchema)
					toolDesc.WriteString(fmt.Sprintf("- %s: %s (Parameters: %s)\n", fd.Name, fd.Description, string(params)))
				}
			}
		}

		messages := make([]ollamaMessage, 0, len(req.Contents))
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
			messages = append(messages, ollamaMessage{
				Role:    role,
				Content: txt,
			})
		}

		apiReq := ollamaRequest{
			Model:     m.name,
			Messages:  messages,
			Stream:    true,
			KeepAlive: "24h", 
		}

		body, _ := json.Marshal(apiReq)
		httpReq, _ := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/api/chat", bytes.NewReader(body))

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			yield(nil, fmt.Errorf("ollama http request failed: %w", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errBody bytes.Buffer
			errBody.ReadFrom(resp.Body)
			yield(nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, errBody.String()))
			return
		}

		decoder := json.NewDecoder(resp.Body)
		var fullContent strings.Builder

		for {
			var apiResp ollamaResponse
			if err := decoder.Decode(&apiResp); err != nil {
				break
			}
			
			if apiResp.Message.Content != "" {
				fullContent.WriteString(apiResp.Message.Content)
				res := &model.LLMResponse{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{{Text: apiResp.Message.Content}},
					},
				}
				if !yield(res, nil) {
					return
				}
			}
		}

		rawContent := fullContent.String()
		re := regexp.MustCompile("(?s)<\\|tool_call\\>call:([^\\{]+)(\\{.*?\\})<\\|tool_call\\>")
		matches := re.FindAllStringSubmatch(rawContent, -1)

		if len(matches) > 0 {
			res := &model.LLMResponse{
				Content: &genai.Content{Role: "model"},
			}
			for _, m := range matches {
				name := strings.TrimSpace(m[1])
				argsStr := m[2]
				var args map[string]any
				if err := json.Unmarshal([]byte(argsStr), &args); err == nil {
					res.Content.Parts = append(res.Content.Parts, &genai.Part{
						FunctionCall: &genai.FunctionCall{
							Name: name,
							Args: args,
						},
					})
				}
			}
			res.FinishReason = genai.FinishReason("FINISH_REASON_FUNCTION_CALL")
			yield(res, nil)
		}
	}
}

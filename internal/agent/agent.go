package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type Broadcaster interface {
	Broadcast(taskID uint, agentName, message string)
}

var GlobalStream Broadcaster

type Agent interface {
	Name() string
	Process(ctx context.Context, input string) (string, error)
}

type Plan struct {
	RepoName string `json:"repo_name"`
	Changes  []FileChange `json:"changes"`
}

type FileChange struct {
	FilePath     string `json:"file_path"`
	Description  string `json:"description"`
	Instructions string `json:"instructions"`
}

func CallLLM(ctx context.Context, m model.LLM, agentName, prompt string) (string, error) {
	taskID, _ := ctx.Value("task_id").(uint)

	logPath := "/home/cnf/projects/auto-coder-swarm/agent_thoughts.log"
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		defer f.Close()
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(f, "\n==================== [%s] AGENT: %s ====================\n", timestamp, agentName)
		fmt.Fprintf(f, "[PROMPT]\n%s\n", prompt)
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: prompt},
				},
			},
		},
	}

	it := m.GenerateContent(ctx, req, false)
	var respText string
	for resp, err := range it {
		if err != nil {
			return "", err
		}
		for _, p := range resp.Content.Parts {
			chunk := p.Text
			respText += chunk
			if GlobalStream != nil && taskID > 0 {
				GlobalStream.Broadcast(taskID, agentName, chunk)
			}
		}
	}

	if f != nil {
		fmt.Fprintf(f, "[RESPONSE]\n%s\n", respText)
		fmt.Fprintf(f, "==============================================================\n")
	}
	
	return respText, nil
}

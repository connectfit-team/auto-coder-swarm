package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
	"github.com/connectfit-team/auto-coder-swarm/internal/observability"
)

func CallLLM(ctx context.Context, m model.LLM, agentName, prompt string) (string, error) {
	taskID, _ := ctx.Value("task_id").(string)
	enrichedPrompt := prompt
	if taskID != "" && GlobalStorage != nil {
		if state := GlobalStorage.GetContextState(taskID); state != "" {
			enrichedPrompt = fmt.Sprintf("[PAST CONTEXT & DECISIONS]\n%s\n\n[CURRENT TASK]\n%s", state, prompt)
		}
	}

	logPath := "/home/cnf/projects/auto-coder-swarm/agent_thoughts.log"
	rotateLogs(logPath)

	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		defer f.Close()
		fmt.Fprintf(f, "\n==================== [%s] AGENT: %s ====================\n", time.Now().Format("15:04:05"), agentName)
		fmt.Fprintf(f, "[PROMPT]\n%s\n", enrichedPrompt)
	}

	if taskID != "" && GlobalStorage != nil {
		GlobalStorage.AddThought(taskID, agentName, "[LLM CALL START]")
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: enrichedPrompt}}}},
	}

	it := m.GenerateContent(ctx, req, false)
	var respText string
	if f != nil { fmt.Fprint(f, "[RESPONSE]\n") }

	for resp, err := range it {
		if err != nil {
			observability.IncrementAgentOp(agentName, "llm_error")
			return "", err
		}
		if resp == nil || resp.Content == nil { continue }
		for _, p := range resp.Content.Parts {
			chunk := p.Text
			respText += chunk
			observability.AddTokenUsage(agentName, m.Name(), len(chunk)/4+1)
			if taskID != "" {
				if GlobalStream != nil { GlobalStream.Broadcast(taskID, agentName, chunk) }
				if GlobalStorage != nil { GlobalStorage.AddThought(taskID, agentName, chunk) }
			}
			if f != nil { fmt.Fprint(f, chunk) }
		}
	}

	if respText == "" { return "", fmt.Errorf("empty response from LLM for agent %s", agentName) }
	if f != nil { fmt.Fprint(f, "\n==============================================================\n") }
	return respText, nil
}

func rotateLogs(logPath string) {
	maxSize := int64(100 * 1024 * 1024)
	info, err := os.Stat(logPath)
	if err != nil || info.Size() < maxSize { return }
	os.Rename(logPath, logPath+"."+time.Now().Format("20060102150405"))
	matches, _ := filepath.Glob(logPath+".*")
	if len(matches) > 10 {
		sort.Strings(matches)
		for i := 0; i < len(matches)-10; i++ { os.Remove(matches[i]) }
	}
}

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
)

type Broadcaster interface {
	Broadcast(taskID uint, agentName, message string)
}

type Persister interface {
	AddThought(taskID uint, agentName, message string) error
}

var GlobalStream Broadcaster
var GlobalStorage Persister

type Agent interface {
	Name() string
	Process(ctx context.Context, input string) (string, error)
}

type Plan struct {
	RepoName string       `json:"repo_name"`
	Changes  []FileChange `json:"changes"`
}

type FileChange struct {
	FilePath     string `json:"file_path"`
	Description  string `json:"description"`
	Instructions string `json:"instructions"`
}

func rotateLogs(logPath string) {
	maxSize := int64(100 * 1024 * 1024) // 100MB
	maxFiles := 10

	info, err := os.Stat(logPath)
	if err != nil || info.Size() < maxSize {
		return
	}

	// 1. Rotate current file
	newName := logPath + "." + time.Now().Format("20060102150405")
	os.Rename(logPath, newName)

	// 2. Cleanup old backups (keep only maxFiles)
	pattern := logPath + ".*"
	matches, _ := filepath.Glob(pattern)
	if len(matches) > maxFiles {
		sort.Strings(matches) // Oldest first (timestamp based naming)
		for i := 0; i < len(matches)-maxFiles; i++ {
			os.Remove(matches[i])
		}
	}
}

func CallLLM(ctx context.Context, m model.LLM, agentName, prompt string) (string, error) {
	taskID, _ := ctx.Value("task_id").(uint)

	logPath := "/home/cnf/projects/auto-coder-swarm/agent_thoughts.log"
	rotateLogs(logPath)

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	header := fmt.Sprintf("\n==================== [%s] AGENT: %s ====================\n", timestamp, agentName)
	promptLog := fmt.Sprintf("[PROMPT]\n%s\n", prompt)

	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		defer f.Close()
		fmt.Fprint(f, header)
		fmt.Fprint(f, promptLog)
	}

	// Persist header and prompt to DB so history shows it immediately
	if taskID > 0 && GlobalStorage != nil {
		GlobalStorage.AddThought(taskID, "SYSTEM", header)
		GlobalStorage.AddThought(taskID, agentName, promptLog)
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
	
	if f != nil {
		fmt.Fprint(f, "[RESPONSE]\n")
	}

	for resp, err := range it {
		if err != nil {
			return "", err
		}
		for _, p := range resp.Content.Parts {
			chunk := p.Text
			respText += chunk
			
			if taskID > 0 {
				if GlobalStream != nil {
					GlobalStream.Broadcast(taskID, agentName, chunk)
				}
				if GlobalStorage != nil {
					GlobalStorage.AddThought(taskID, agentName, chunk)
				}
			}
			
			if f != nil {
				fmt.Fprint(f, chunk)
			}
		}
	}

	if f != nil {
		fmt.Fprintf(f, "\n==============================================================\n")
	}

	return respText, nil
}

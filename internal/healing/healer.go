package healing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"google.golang.org/adk/model"
)

type HealingAction string

const (
	ActionModifyCode   HealingAction = "MODIFY_CODE"
	ActionRunCommand   HealingAction = "RUN_COMMAND"
	ActionQueryContext HealingAction = "QUERY_CONTEXT"
	ActionRetry        HealingAction = "RETRY"
	ActionAbort        HealingAction = "ABORT"
)

type HealingStep struct {
	Action      HealingAction `json:"action"`
	TargetFile  string        `json:"target_file,omitempty"`
	Instruction string        `json:"instruction,omitempty"`
	Command     string        `json:"command,omitempty"`
	Reason      string        `json:"reason"`
}

type HealingPlan struct {
	Diagnosis string        `json:"diagnosis"`
	Steps     []HealingStep `json:"steps"`
}

type HealerAgent struct {
	llm model.LLM
}

func NewHealerAgent(m model.LLM) *HealerAgent {
	return &HealerAgent{llm: m}
}

func (a *HealerAgent) Name() string { return "Healer" }

func (a *HealerAgent) BuildPrompt(errorLog, projectType string, fileContents map[string]string) string {
	var files strings.Builder
	for path, content := range fileContents {
		files.WriteString(fmt.Sprintf("\n--- FILE: %s ---\n%s\n", path, content))
	}

	return fmt.Sprintf(`You are the Swarm Healer. 
Your goal is to diagnose build/test failures and propose surgical healing steps.
Don't just guess; analyze the error log and the provided file contents carefully.

[Project Type]
%s

[Error Log]
%s

[Relevant File Contents]
%s

MANDATORY JSON FORMAT:
{
  "diagnosis": "Detailed explanation of why it failed",
  "steps": [
    {
      "action": "MODIFY_CODE/RUN_COMMAND/QUERY_CONTEXT/RETRY/ABORT",
      "target_file": "path/to/file (if MODIFY_CODE)",
      "instruction": "What to change (if MODIFY_CODE)",
      "command": "Shell command to run (if RUN_COMMAND, e.g., 'go mod tidy')",
      "reason": "Why this step is necessary"
    }
  ]
}
Output ONLY a valid JSON object.`, projectType, errorLog, files.String())
}

func (a *HealerAgent) ProposeHealing(ctx context.Context, errorLog, projectType string, fileContents map[string]string) (HealingPlan, error) {
	resp, err := agent.CallLLM(ctx, a.llm, a.Name(), a.BuildPrompt(errorLog, projectType, fileContents))
	if err != nil {
		return HealingPlan{}, err
	}

	var plan HealingPlan
	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return HealingPlan{}, fmt.Errorf("failed to parse healing plan: %w", err)
	}
	return plan, nil
}

func extractJSON(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		return raw[start : end+1]
	}
	return raw
}

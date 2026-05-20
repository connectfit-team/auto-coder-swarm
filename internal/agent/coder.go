package agent

import (
	"context"
	"fmt"
	"os"
	"google.golang.org/adk/model"
)

type CoderAgent struct {
	llm model.LLM
}

func NewCoderAgent(m model.LLM) *CoderAgent {
	return &CoderAgent{llm: m}
}

func (a *CoderAgent) Name() string {
	return "Coder"
}

func (a *CoderAgent) ModifyFile(ctx context.Context, filePath string, instructions string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	prompt := fmt.Sprintf("You are the Swarm Coder.\n"+
		"Modify the following code based on the technical instructions.\n\n"+
		"MANDATORY RULES:\n"+
		"1. Provide the FULL file content after modification.\n"+
		"2. Do not include any conversational text, ONLY the code.\n"+
		"3. Maintain existing coding style.\n\n"+
		"[Technical Instructions]\n%s\n\n"+
		"[Original Code: %s]\n%s", instructions, filePath, string(content))

	newContent, err := CallLLM(ctx, a.llm, a.Name(), prompt)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(filePath, []byte(newContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Modified %s", filePath), nil
}

func (a *CoderAgent) Process(ctx context.Context, input string) (string, error) {
	return "Use ModifyFile", nil
}

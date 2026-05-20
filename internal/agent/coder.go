package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (a *CoderAgent) BuildRepairPrompt(filePath, content, instructions, buildError string) string {
	return fmt.Sprintf("You are the Swarm Debugging Expert.\n"+
		"The previous attempt to modify the code caused a BUILD ERROR.\n"+
		"Your goal is to fix the code to resolve the error while still fulfilling the original instructions.\n\n"+
		"MANDATORY RULES:\n"+
		"1. Provide the FULL file content after repair.\n"+
		"2. Do not include any conversational text, ONLY the code.\n"+
		"3. Focus specifically on fixing the mentioned build error.\n\n"+
		"[Original Instructions]\n%s\n\n"+
		"[Build Error Output]\n%s\n\n"+
		"[Current Code State: %s]\n%s", instructions, buildError, filePath, content)
}

func (a *CoderAgent) RepairFile(ctx context.Context, filePath, instructions, buildError string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	prompt := a.BuildRepairPrompt(filePath, string(content), instructions, buildError)
	newContent, err := CallLLM(ctx, a.llm, "Debugger", prompt)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(filePath, []byte(newContent), 0644)
	return fmt.Sprintf("Repaired %s", filePath), err
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

func (a *CoderAgent) GenerateTestFile(ctx context.Context, sourcePath string) (string, error) {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", err
	}

	ext := filepath.Ext(sourcePath)
	testPath := strings.TrimSuffix(sourcePath, ext)
	switch ext {
	case ".go": testPath += "_test.go"
	case ".ts", ".tsx": testPath += ".spec" + ext
	case ".dart": testPath += "_test.dart"
	default: return "", fmt.Errorf("unsupported extension for test generation: %s", ext)
	}

	prompt := fmt.Sprintf("You are the Swarm Test Engineer.\n"+
		"Create a comprehensive unit test for the following source code.\n\n"+
		"MANDATORY RULES:\n"+
		"1. Provide the FULL test file content.\n"+
		"2. Do not include any conversational text, ONLY the code.\n"+
		"3. Ensure high coverage and test edge cases.\n\n"+
		"[Source Code: %s]\n%s", sourcePath, string(content))

	testCode, err := CallLLM(ctx, a.llm, "Tester", prompt)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(testPath, []byte(testCode), 0644)
	if err != nil {
		return "", err
	}

	return testPath, nil
}

func (a *CoderAgent) Process(ctx context.Context, input string) (string, error) {
	return "Use ModifyFile or GenerateTestFile", nil
}

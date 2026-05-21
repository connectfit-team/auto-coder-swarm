package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func (o *SwarmOrchestrator) detectProjectTypeLLM(ctx context.Context, taskID string, path string) ProjectMetadata {
	primary, _ := o.loadModels()

	var structure string
	cmd := exec.CommandContext(ctx, "tree", "-L", "3", "-F", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd = exec.CommandContext(ctx, "ls", "-R", path)
		out, _ = cmd.CombinedOutput()
	}
	structure = string(out)
	if len(structure) > 4000 {
		structure = structure[:4000] + "..."
	}

	prompt := fmt.Sprintf(`레포지토리의 파일 구조를 분석하여 프로젝트의 성격과 최적의 빌드/검증 명령어를 판단해줘.
[Directory Structure]
%s

MANDATORY JSON FORMAT:
{
  "type": "Go/Flutter/Python/NodeJS/etc",
  "build_command": "상세 빌드 명령어 (예: flutter analyze, go build ./...)",
  "bench_command": "성능 측정 명령어",
  "key_files": ["주요 파일"],
  "risk_assessment": "환경적 리스크"
}
출력은 오직 JSON만 허용한다.`, structure)

	resp, _ := agent.CallLLM(ctx, primary, "Classifier", prompt)
	o.logDeepTechnical(ctx, taskID, "DETECTION_RAW", "프로젝트 분류 LLM 원본 응답", prompt, resp)

	var meta ProjectMetadata
	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
		return ProjectMetadata{Type: "Unknown", BuildCommand: "ls"}
	}
	return meta
}

func extractJSON(raw string) string {
	// Strip markdown formatting if present
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
	}
	if strings.HasSuffix(raw, "```") {
		raw = strings.TrimSuffix(raw, "```")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		return raw[start : end+1]
	}
	return raw
}

package orchestrator

import (
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/llm"
	"google.golang.org/adk/model"
)

func (o *SwarmOrchestrator) loadModels() (model.LLM, []model.LLM) {
	baseURL := "amqp://guest:guest@192.168.120.54:5672/"
	primaryName := o.store.GetSetting("primary_model")
	if primaryName == "" {
		primaryName = "gemma4:latest"
	}
	primary := llm.FromEnv(primaryName, baseURL)

	voterNames := strings.Split(o.store.GetSetting("voter_models"), ",")
	var voterLLMs []model.LLM
	for _, name := range voterNames {
		if n := strings.TrimSpace(name); n != "" {
			voterLLMs = append(voterLLMs, llm.FromEnv(n, baseURL))
		}
	}
	if len(voterLLMs) == 0 {
		voterLLMs = append(voterLLMs, primary)
	}
	return primary, voterLLMs
}

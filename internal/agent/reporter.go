package agent

import (
	"context"
	"fmt"

	"google.golang.org/adk/model"
)

type ReporterAgent struct {
	llm model.LLM
}

func NewReporterAgent(m model.LLM) *ReporterAgent {
	return &ReporterAgent{llm: m}
}

func (a *ReporterAgent) Name() string {
	return "Reporter"
}

func (a *ReporterAgent) GenerateSummary(ctx context.Context, tasks string) (string, error) {
	prompt := fmt.Sprintf(`You are the Swarm Activity Reporter.
Analyze the following list of tasks performed today and generate a concise, high-signal daily summary.
Highlight successful PRs, critical failures, and overall system health.

[Tasks Today]
%s

MANDATORY FORMAT (Markdown):
# 📊 Swarm Daily Activity Report (%s)

## 🎯 Executive Summary
...

## ✅ Key Achievements
- [TaskID] Summary of success

## ❌ Critical Failures & Insights
- [TaskID] Reason for failure and diagnosis

## 📈 System Metrics (Estimated)
- Total Tasks: ...
- Success Rate: ...%%`, tasks, "2026-05-21")

	return CallLLM(ctx, a.llm, a.Name(), prompt)
}

package agent

import (
	"context"
	"fmt"
	"google.golang.org/adk/model"
)

type SummarizerAgent struct {
	llm model.LLM
}

func NewSummarizerAgent(m model.LLM) *SummarizerAgent {
	return &SummarizerAgent{llm: m}
}

func (a *SummarizerAgent) Name() string {
	return "Summarizer"
}

func (a *SummarizerAgent) Process(ctx context.Context, oracleAnalysis string) (string, error) {
	prompt := fmt.Sprintf("You are the Technical Context Summarizer.\n"+
		"Extract only the essential technical signals needed for code modification from the following analysis.\n"+
		"Include: Target file paths, specific function names, required logic changes, and potential dependencies.\n"+
		"Exclude: Conversational filler, general explanations, and redundant examples.\n\n"+
		"[Oracle Analysis]\n%s", oracleAnalysis)

	return CallLLM(ctx, a.llm, a.Name(), prompt)
}

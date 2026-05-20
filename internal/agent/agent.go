package agent

import (
	"context"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

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

func CallLLM(ctx context.Context, m model.LLM, prompt string) (string, error) {
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
			respText += p.Text
		}
	}
	return respText, nil
}

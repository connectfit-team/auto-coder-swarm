package agent

import "context"

type Broadcaster interface {
	Broadcast(taskID string, agentName, message string)
}

type Persister interface {
	AddThought(taskID string, agentName, message string) error
	GetContextState(taskID string) string
}

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

var GlobalStream Broadcaster
var GlobalStorage Persister

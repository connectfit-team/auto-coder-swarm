package reporting

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"google.golang.org/adk/model"
)

type Service struct {
	store    *storage.Storage
	reporter *agent.ReporterAgent
}

func NewService(s *storage.Storage, m model.LLM) *Service {
	return &Service{
		store:    s,
		reporter: agent.NewReporterAgent(m),
	}
}

func (s *Service) GenerateDailyReport(ctx context.Context) (string, error) {
	tasks, err := s.store.GetTodayTasks()
	if err != nil {
		return "", err
	}

	if len(tasks) == 0 {
		return "오늘 수행된 작업이 없습니다.", nil
	}

	var sb strings.Builder
	for _, t := range tasks {
		sb.WriteString(fmt.Sprintf("- ID: %s, Status: %s, Repo: %s, Request: %s\n", t.ID, t.Status, t.RepoName, t.UserRequest))
		if t.Status == storage.StatusFailed {
			sb.WriteString(fmt.Sprintf("  Error: %s\n", t.ErrorLog))
		}
	}

	return s.reporter.GenerateSummary(ctx, sb.String())
}

func (s *Service) RunAutoReport(ctx context.Context) {
	log.Println("📊 Starting Swarm Activity Reporting Service")
}

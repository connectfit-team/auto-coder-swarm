package orchestrator

import (
	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

type ProjectMetadata struct {
	Type           string   `json:"type"`
	BuildCommand   string   `json:"build_command"`
	BenchCommand   string   `json:"bench_command"`
	KeyFiles       []string `json:"key_files"`
	RiskAssessment string   `json:"risk_assessment"`
}

type RunResult struct {
	RepoName string
	// 사람이 다음에 무엇을 할지 적은 글. 열린 PR 주소가 대부분이지만
	// 저장소가 여럿이면 여러 줄이고, proto 는 주소가 아니라 make 안내다.
	Result          string
	WaitingApproval bool
	ChainTasks      []StatelessRequest
}

type StatelessRequest struct {
	UserRequest     string   `json:"user_request"`
	AnalysisContext string   `json:"analysis_context,omitempty"`
	TargetRepo      string   `json:"target_repo,omitempty"`
	TargetFiles     []string `json:"target_files,omitempty"`
	Constraints     []string `json:"constraints,omitempty"`
	Depth           int      `json:"depth"`
	ParentRepos     []string `json:"parent_repos,omitempty"` // To prevent cycles in chain reactions
	// 이 요청을 낳은 작업. 화면이 형제 작업과 그 PR 들을 함께 보여 주는 데 쓴다.
	ParentTaskID string `json:"parent_task_id,omitempty"`
}

type TaskStrategy struct {
	TotalFiles     int      `json:"total_files"`
	TotalLines     int      `json:"total_lines"`
	ComplexityRisk string   `json:"complexity_risk"`
	ActionablePath []string `json:"actionable_path"`
	AnalysisQuery  string   `json:"analysis_query"` // Refined query for CIE
	IsFeasible     bool     `json:"is_feasible"`
}

type Plan struct {
	RepoName string             `json:"repo_name"`
	Changes  []agent.FileChange `json:"changes"`
}

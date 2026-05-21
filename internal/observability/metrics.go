package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// TaskDuration measures the time taken for each orchestration step.
	TaskDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "swarm_task_duration_seconds",
		Help:    "Duration of swarm orchestration steps.",
		Buckets: prometheus.DefBuckets,
	}, []string{"step", "repo"})

	// AgentSuccessRate tracks successful vs failed agent operations.
	AgentSuccessRate = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "swarm_agent_operations_total",
		Help: "Total number of agent operations by status.",
	}, []string{"agent", "status"})

	// TokenUsage tracks total tokens consumed by LLM calls (Estimated).
	TokenUsage = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "swarm_llm_token_usage_total",
		Help: "Estimated token usage by agent and model.",
	}, []string{"agent", "model"})

	// ActiveWorkers tracks current concurrent tasks.
	ActiveWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "swarm_active_workers",
		Help: "Number of currently active task workers.",
	})
)

// RecordStepDuration records the time spent in a specific pipeline step.
func RecordStepDuration(step, repo string, seconds float64) {
	TaskDuration.WithLabelValues(step, repo).Observe(seconds)
}

// IncrementAgentOp increments success/failure counters.
func IncrementAgentOp(agent, status string) {
	AgentSuccessRate.WithLabelValues(agent, status).Inc()
}

// AddTokenUsage adds to the token counter (naive estimation or from response).
func AddTokenUsage(agent, model string, tokens int) {
	TokenUsage.WithLabelValues(agent, model).Add(float64(tokens))
}

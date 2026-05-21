package observability

import (
	"testing"
)

func TestMetricsRecording(t *testing.T) {
	RecordStepDuration("test-step", "test-repo", 1.23)
	IncrementAgentOp("test-agent", "success")
	AddTokenUsage("test-agent", "test-model", 100)
}

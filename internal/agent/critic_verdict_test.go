package agent

import "testing"

const gateDiff = `diff --git a/internal/domain/tag.go b/internal/domain/tag.go
--- a/internal/domain/tag.go
+++ b/internal/domain/tag.go
@@
+func (t TagType) String() string { return "Unknown" }
`

// 실제로 작업을 통째로 버리게 만든 두 응답이다.
const vagueRisk = `RISK_DETECTED: The new String() method can lead to security vulnerabilities
if not handled carefully. Returning "Unknown" could potentially leak sensitive
information depending on how this method is used.`

const offTopic = "FEEDBACK: // Missing final value\nslog.Logger(\"Info\", \"k\", \"v\", \"extra\")\nLet's investigate why the logging call is failing."

func TestGateVerdict(t *testing.T) {
	t.Run("파일·줄을 못 대면 막지 않는다", func(t *testing.T) {
		if v := ParseCriticVerdict(vagueRisk, gateDiff); v.Blocking {
			t.Errorf("막연한 걱정으로 막았다: %s", v.Why)
		}
	})

	// 모델이 딴 데로 새서 diff 에 없는 파일을 들고 오는 일이 실제로 있었다.
	t.Run("이번 변경에 없는 자리는 근거가 아니다", func(t *testing.T) {
		v := ParseReviewerVerdict("FEEDBACK: internal/logger/slog.go:12 is wrong", gateDiff)
		if v.Blocking {
			t.Errorf("diff 밖 파일로 막았다: %s", v.Why)
		}
		if v.Why == "" {
			t.Error("왜 넘어갔는지 말해야 한다")
		}
	})

	t.Run("이번 변경 안의 자리를 대면 막는다", func(t *testing.T) {
		v := ParseReviewerVerdict("FEEDBACK: internal/domain/tag.go:3 always returns Unknown", gateDiff)
		if !v.Blocking {
			t.Errorf("근거를 댔는데 넘어갔다: %s", v.Why)
		}
		if len(v.Locations) != 1 {
			t.Errorf("자리를 못 뽑았다: %v", v.Locations)
		}
	})

	t.Run("엉뚱한 이야기는 막지 않는다", func(t *testing.T) {
		if ParseReviewerVerdict(offTopic, gateDiff).Blocking {
			t.Error("diff 와 무관한 지적으로 막았다")
		}
	})

	t.Run("승인과 빈 응답은 통과", func(t *testing.T) {
		if ParseCriticVerdict("APPROVED", gateDiff).Blocking {
			t.Error("승인을 막았다")
		}
		if ParseReviewerVerdict("   ", gateDiff).Blocking {
			t.Error("빈 응답을 거절로 읽었다")
		}
	})

	// diff 를 못 구했으면 자리 검사를 건너뛴다 — 그것 때문에 진짜 지적을 놓치면 안 된다.
	t.Run("diff 가 없으면 자리만 보고 판단한다", func(t *testing.T) {
		if !ParseCriticVerdict("RISK_DETECTED: a/b.go:1 leaks a token", "").Blocking {
			t.Error("diff 가 없다고 진짜 지적을 놓쳤다")
		}
	})
}

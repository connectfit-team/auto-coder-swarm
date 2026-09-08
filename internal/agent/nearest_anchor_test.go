package agent

import (
	"strings"
	"testing"
)

// 못 찾았을 때 원문에 무엇이 있는지 함께 알려 줘야 한다.
func TestNearestAnchorPointsAtRealCode(t *testing.T) {
	src := strings.Split(`package mariadb

func (r *ConnectRepository) DeleteAndDisconnectOnAccountDelete(ctx context.Context, userID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return nil
	})
}

func (r *ConnectRepository) HoldConnect(ctx context.Context, id int64) error {
	return nil
}
`, "\n")

	// 모델이 지어낸 것 — WithTransaction 이 붙은 함수는 없다.
	invented := "func (s *ConnectService) DeleteAndDisconnectOnAccountDeleteWithTransaction(ctx context.Context, req *connectv1.RequestDeleteAndDisconnectOnAccountDeleteWithTransaction) error {"
	near := nearestAnchor(src, invented)
	if near == "" {
		t.Fatal("가장 비슷한 곳을 못 짚었다 — 모델은 다시 같은 것을 지어낸다")
	}
	if !strings.Contains(near, "DeleteAndDisconnectOnAccountDelete") {
		t.Errorf("엉뚱한 곳을 짚었다:\n%s", near)
	}
	if !strings.Contains(near, "3줄") {
		t.Errorf("줄 번호를 안 알려 준다:\n%s", near)
	}

	// 아무 상관 없는 것에는 아무것도 짚지 않아야 한다 —
	// 엉뚱한 곳을 가리키면 없다고 말하는 것보다 나쁘다.
	if got := nearestAnchor(src, "func CalculateMonthlySalaryWithdrawal(payslip *PayslipDetail) decimal.Decimal {"); got != "" {
		t.Errorf("겹치지도 않는데 짚었다:\n%s", got)
	}
}

// 흔한 낱말로 겹침을 세면 아무 줄이나 이긴다.
func TestIdentSetSkipsCommonWords(t *testing.T) {
	got := identSet("func (s *ConnectService) Do(ctx context.Context) error { return nil }")
	for _, bad := range []string{"func", "ctx", "context", "error", "return", "nil"} {
		if got[bad] {
			t.Errorf("흔한 낱말 %q 를 셌다", bad)
		}
	}
	if !got["ConnectService"] {
		t.Errorf("셀 만한 이름을 놓쳤다: %v", sortedKeys(got))
	}
}

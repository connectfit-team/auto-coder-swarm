package orchestrator

import (
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
)

// proto 가 아직 배포되지 않아 나는 오류는 우리 탓이 아니다.
func TestPendingProtoSymbolIsExpected(t *testing.T) {
	plans := []insightclient.VariantRepoPlan{
		{
			Repo: "proto-gigpointapis", Publish: "protogen-make",
			Changes: []insightclient.VariantChange{{
				Block: []string{"  COUPON_STATUS_REFUNDED = 5; // 환불됨"},
			}},
		},
		{Repo: "gig-point", Publish: "pr"},
	}
	pending := PendingSymbols(plans)
	errs := []string{
		"internal/grpc/service_coupon.go:99:21: undefined: gigpointv1.CouponStatus_COUPON_STATUS_REFUNDED",
	}
	if got := UnexpectedBuildErrors(errs, pending); len(got) != 0 {
		t.Errorf("배포 전 이름을 우리 탓으로 봤다: %v", got)
	}
}

// 그 밖의 오류는 우리 편집이 틀렸다는 뜻이다.
func TestOtherBuildErrorIsOurs(t *testing.T) {
	pending := []string{"COUPON_STATUS_REFUNDED"}
	errs := []string{
		"internal/db/reward.go:148:31: stats.refundedPrice undefined (type Stats has no field or method refundedPrice)",
		"internal/x/y.go:12:3: cannot use a (variable of type int) as string value",
	}
	got := UnexpectedBuildErrors(errs, pending)
	if len(got) != 2 {
		t.Errorf("우리 오류를 걸렀다: %v", got)
	}
}

// 사람이 채워야 할 이름도 예상된 것이다.
func TestNeedsManualNameIsExpected(t *testing.T) {
	plans := []insightclient.VariantRepoPlan{{
		Repo: "worker", Publish: "pr",
		NeedsManual: []string{"IsInstagram"},
	}}
	pending := PendingSymbols(plans)
	// Go 는 없는 이름을 여러 모양으로 알린다. 어느 모양이든 걸러야 한다.
	for _, e := range []string{
		"a.go:383:20: social.Platform.IsInstagram undefined (type Platform has no field or method IsInstagram)",
		"a.go:1:1: undefined: IsInstagram",
	} {
		if got := UnexpectedBuildErrors([]string{e}, pending); len(got) != 0 {
			t.Errorf("사람이 채울 이름을 우리 탓으로 봤다: %v", got)
		}
	}
}

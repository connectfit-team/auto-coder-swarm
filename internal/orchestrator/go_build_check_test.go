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

// 알림 문장에서 낱말을 아무거나 뽑으면 흔한 낱말이 섞여 들어와
// 우리 편집이 틀려서 난 오류까지 덮는다.
func TestPendingSymbolsTakesOnlyNames(t *testing.T) {
	plans := []insightclient.VariantRepoPlan{
		{
			Repo: "proto-gigpointapis", Publish: "protogen-make",
			Changes: []insightclient.VariantChange{{
				Block: []string{"  // 쿠폰 사용 완료", "  COUPON_STATUS_REFUNDED = 5;"},
			}},
		},
		{
			Repo: "cms", Publish: "pr",
			NeedsManual: []string{
				"refundedPrice",
				"cms/a.svelte:141 — 설명이 consumed 것 그대로다: \"쿠폰 사용됨\"",
			},
		},
	}
	got := PendingSymbols(plans)
	want := map[string]bool{"COUPON_STATUS_REFUNDED": true, "refundedPrice": true}
	for _, g := range got {
		if !want[g] {
			t.Errorf("이름이 아닌 것을 담았다: %q (전체 %v)", g, got)
		}
	}
	if len(got) != 2 {
		t.Errorf("담은 이름 = %v, 둘이어야 한다", got)
	}
}

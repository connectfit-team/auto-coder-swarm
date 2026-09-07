package orchestrator

import (
	"fmt"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/korean"
)

// 값 추가 작업이 사람에게 보이는 글은 모두 여기 있다 — 로그 요약, 커밋 메시지,
// PR 본문, 작업 결과. 판단하는 코드와 섞으면 문구를 고치려고 흐름을 읽어야 한다.

func variantCommitMessage(req insightclient.VariantPlanRequest, p insightclient.VariantRepoPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s 더한다\n\n", korean.With(req.Label, "을", "를"))
	fmt.Fprintf(&b, "%s 가 있는 자리마다 %s 몫을 나란히 넣는다.\n", req.Seed, req.Value)
	if p.Note != "" {
		fmt.Fprintf(&b, "\n%s\n", p.Note)
	}
	if p.AlreadyThere > 0 {
		fmt.Fprintf(&b, "\n%d곳은 이미 값이 들어 있어 건드리지 않았다.\n", p.AlreadyThere)
	}
	if len(p.DepBumps) > 0 {
		b.WriteString("\n이 변경이 컴파일되려면 먼저 갱신해야 한다:\n")
		for _, d := range p.DepBumps {
			fmt.Fprintf(&b, "  %s\n", depBumpLine(d))
		}
	}
	if len(p.NeedsManual) > 0 {
		fmt.Fprintf(&b, "\n저장소에 없는 이름이 있다 — 사람이 채워야 한다:\n")
		for _, n := range p.NeedsManual {
			fmt.Fprintf(&b, "  %s\n", n)
		}
	}
	return b.String()
}

func planSummaryText(plans []insightclient.VariantRepoPlan) string {
	var b strings.Builder
	for _, p := range plans {
		fmt.Fprintf(&b, "%d) %s — 자리 %d곳", p.Order, p.Repo, len(p.Changes))
		if p.AlreadyThere > 0 {
			fmt.Fprintf(&b, " (이미 있음 %d곳)", p.AlreadyThere)
		}
		if p.Blocks {
			b.WriteString(" (이게 먼저 배포돼야 함)")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// depBumpLine 은 무엇을 어떻게 갱신해야 하는지 한 줄로 적는다.
func depBumpLine(d insightclient.DepBump) string {
	switch d.Kind {
	case "go":
		return fmt.Sprintf("%s — go get %s@<새 커밋> (%s 배포 뒤)", d.File, d.Module, d.From)
	case "pubspec":
		return fmt.Sprintf("%s — %s 의 ref 를 새 태그로 (%s 배포 뒤)", d.File, d.Module, d.From)
	case "vendored":
		return fmt.Sprintf("%s — %s 에서 다시 생성해 넣는다 (protogen 의 make)", d.File, d.Module)
	}
	return d.File + " — " + d.Module
}

// outcomeText 는 사람이 다음에 무엇을 할지 적는다 — 열린 PR 주소와, 사람이
// 직접 돌려야 하는 make 목표. 빈 문자열은 "아무것도 못 했다" 는 뜻이고
// 부르는 쪽이 그것을 실패로 읽는다.
func outcomeText(rs []VariantResult) string {
	var lines []string
	for _, r := range rs {
		switch {
		case r.PRURL != "":
			lines = append(lines, r.Repo+": "+r.PRURL)
		case r.MakeTarget != "":
			lines = append(lines, fmt.Sprintf("%s: protogen 에서 make %s (사람이 돌린다)", r.Repo, r.MakeTarget))
		}
	}
	return strings.Join(lines, "\n")
}

// doneHeadline 은 무엇으로 끝났는지 한 줄로 적는다.
// proto 는 PR 이 아니라 make 로 배포하므로 PR 만 세면 "0개" 로 보인다.
func doneHeadline(rs []VariantResult) string {
	var prs, makes int
	for _, r := range rs {
		switch {
		case r.PRURL != "":
			prs++
		case r.MakeTarget != "":
			makes++
		}
	}
	switch {
	case makes == 0:
		return fmt.Sprintf("PR %d개를 열었다", prs)
	case prs == 0:
		return fmt.Sprintf("protogen 의 make 목표 %d개를 남겼다 — 사람이 돌린다", makes)
	}
	return fmt.Sprintf("PR %d개를 열었고 protogen 의 make 목표 %d개를 남겼다", prs, makes)
}

// whyNoPR 은 PR 이 하나도 안 열린 까닭을 한 줄로 만든다.
// 실패가 있으면 그것을, 없으면 되돌린 자리를 적는다 — 되돌린 까닭을 삼키면
// "넣을 것이 하나도 없었다" 만 남아 무엇이 막았는지 알 수 없다.
func whyNoPR(rs []VariantResult) string {
	for _, r := range rs {
		if r.Err != "" {
			return r.Err
		}
	}
	var notes []string
	for _, r := range rs {
		notes = append(notes, r.NeedsManual...)
	}
	if len(notes) > 0 {
		return strings.Join(notes, " · ")
	}
	return "넣을 것이 하나도 없었다"
}

// protoPublishSteps 는 protogen 에서 무엇을 돌리고 무엇이 들어가는지 적는다.
func protoPublishSteps(p insightclient.VariantRepoPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "protogen 에서 `make %s` 을 돌린다. 그것이 컴파일·커밋·push 를 한다.\n", p.MakeTarget)
	b.WriteString("넣을 것:\n")
	for _, c := range p.Changes {
		fmt.Fprintf(&b, "  %s:%d 다음에\n", c.File, c.InsertAfter)
		for _, l := range c.Block {
			fmt.Fprintf(&b, "    %s\n", l)
		}
	}
	return b.String()
}

func askSummary(ask insightclient.VariantAskResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "씨앗 %s · 더할 값 %s (%s)\n", ask.Seed, ask.Value, ask.Label)
	for _, p := range ask.Plans {
		fmt.Fprintf(&b, "%d) %s — 자리 %d곳", p.Order, p.Repo, len(p.Changes))
		if p.AlreadyThere > 0 {
			fmt.Fprintf(&b, " (이미 있음 %d곳)", p.AlreadyThere)
		}
		if p.Blocks {
			b.WriteString(" (이게 먼저 배포돼야 함)")
		}
		if len(p.NeedsManual) > 0 {
			fmt.Fprintf(&b, " · 없는 이름: %s", strings.Join(p.NeedsManual, ", "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func resultSummary(rs []VariantResult) string {
	var b strings.Builder
	for _, r := range rs {
		fmt.Fprintf(&b, "%s — 넣음 %d · 건너뜀 %d", r.Repo, r.Inserted, r.Skipped)
		if len(r.Unverified) > 0 {
			fmt.Fprintf(&b, " · 문법 확인 못 함: %s", strings.Join(r.Unverified, " "))
		}
		if r.PRURL != "" {
			fmt.Fprintf(&b, " · %s", r.PRURL)
		}
		if r.MakeTarget != "" {
			fmt.Fprintf(&b, " · make %s", r.MakeTarget)
		}
		if r.Err != "" {
			fmt.Fprintf(&b, " · 실패: %s", r.Err)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// coverageTable 은 저장소마다 담았는지, 뺐으면 왜인지 적는다.
// 뺀 것을 보여 주지 않으면 놓친 것인지 사람이 알 수 없다.
func coverageTable(vs []insightclient.RepoVerdict) string {
	var b strings.Builder
	for _, v := range vs {
		if v.Planned > 0 {
			fmt.Fprintf(&b, "담음  %-24s %d곳 (낱말 %d회)\n", v.Repo, v.Planned, v.Hits)
			continue
		}
		fmt.Fprintf(&b, "뺌    %-24s 낱말 %d회 — %s\n", v.Repo, v.Hits, v.Reason)
		if v.Evidence != "" {
			fmt.Fprintf(&b, "        %s\n", v.Evidence)
		}
	}
	return b.String()
}

// blockerNote 는 먼저 머지돼야 하는 PR 을 본문 맨 앞에 적는다.
func blockerNote(blockers []string) string {
	if len(blockers) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("> 먼저 머지·배포돼야 이 PR 이 빌드된다:\n>\n")
	for _, x := range blockers {
		fmt.Fprintf(&b, "> - %s\n", x)
	}
	b.WriteString(">\n> 그때까지 초안으로 둔다.\n\n")
	return b.String()
}

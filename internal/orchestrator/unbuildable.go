package orchestrator

import (
	"fmt"
	"strings"
)

// 빌드가 안 되는 PR 은 그 사실을 머리에 적고 초안으로 연다.
//
// 없는 이름(undefined · has no field or method)은 새 proto 메시지·필드나 새
// 메서드가 필요하다는 뜻이고, 사람이 만들면 된다. 그래서 저장소를 통째로
// 버리지 않고 사람에게 올리는 것이 맞다 — 그 판단은 그대로 둔다.
//
// 틀렸던 것은 **보통 PR 로 열었다**는 것이다. 알림은 PR 본문 아래에 있고
// 사람은 제목을 보고 통과시킨다. 머지하면 빌드가 깨진다.
//
// 그리고 이 이름들은 proto 를 배포해도 안 생긴다 — 계획은 열거에 값 하나를
// 더할 뿐이고, atdv2.BLE 같은 것은 새 message 가 있어야 한다. "배포 뒤 생길
// 것" 으로 읽으면 안 된다.

const unbuildableShown = 8

// unbuildableNote 는 무엇이 없어서 빌드가 안 되는지 PR 머리에 적는다.
func unbuildableNote(missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("> **이대로는 빌드되지 않는다. 초안이다.**\n>\n")
	b.WriteString("> 아래 이름이 아직 없다. 새 proto 메시지·필드나 새 메서드가 있어야 한다 —\n")
	b.WriteString("> **proto 를 배포해도 저절로 생기지 않는다.** 사람이 만든 뒤에 초안을 푼다.\n>\n")
	for i, m := range missing {
		if i >= unbuildableShown {
			fmt.Fprintf(&b, "> - … 그 밖에 %d개\n", len(missing)-unbuildableShown)
			break
		}
		fmt.Fprintf(&b, "> - `%s`\n", firstLineOf(m))
	}
	b.WriteString(">\n> 값을 더한 자리 자체는 검증을 지났다 — 없는 이름만 채우면 된다.\n\n")
	return b.String()
}

// knowledgeNote 는 사내지식 없이 쓴 코드라는 것을 PR 머리에 적는다.
//
// 지식이 없어도 작업은 이어 가는 것이 맞다. 그러나 그 사실이 안 보이면
// 사람은 "정책까지 보고 쓴 코드" 로 읽는다 — 실제로는 404 를 받고 지식 없이
// 돌던 것이 계속이었다.
func knowledgeNote(why string) string {
	if why == "" {
		return ""
	}
	return "> **사내지식 없이 썼다.** " + why + "\n>\n" +
		"> 정책·과거 결정을 못 보고 코드만 보고 쓴 것이다. 그 값이나 규칙이\n" +
		"> 정책 표에 있는 것이라면 사람이 대조해야 한다.\n\n"
}

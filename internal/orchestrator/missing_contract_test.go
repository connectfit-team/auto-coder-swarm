package orchestrator

import (
	"strings"
	"testing"
)

// W-33064 가 실제로 낸 오류들이다.
const w33064 = `
/x/src/lib/server/data/connectcud.ts:120:9
Error: Cannot find name 'checkIfRequestIsPending'. 
/x/src/lib/server/data/connectcud.ts:131:9
Error: Cannot find name 'createPendingNotification'. 
/x/src/routes/(auth)/store/connect/+page.server.ts:40:21
Error: Property 'isPending' does not exist on type 'LaborContract'. 
/x/src/routes/(auth)/store/connect/+page.server.ts:52:9
Error: Module '"$lib/server/data/cabinet"' has no exported member 'updateConnectionStatus'. 
`

func TestMissingNames(t *testing.T) {
	got := missingContractNames(parseTypeErrors(w33064))
	want := []string{
		"LaborContract.isPending",
		"checkIfRequestIsPending",
		"createPendingNotification",
		"updateConnectionStatus",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("얻음 %v\n바람 %v", got, want)
	}
}

// 문법 오류나 값 불일치는 "없는 이름" 이 아니다 — 뒤쪽 저장소로 넘기면 안 된다.
func TestMissingNamesIgnoresOtherErrors(t *testing.T) {
	other := `
/x/src/a.ts:1:1
Error: Type 'string' is not assignable to type 'number'. 
/x/src/b.ts:2:2
Error: This comparison appears to be unintentional because the types have no overlap. 
`
	if got := missingContractNames(parseTypeErrors(other)); len(got) != 0 {
		t.Errorf("없는 이름이 아닌데 잡았다: %v", got)
	}
}

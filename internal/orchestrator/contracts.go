package orchestrator

import "fmt"

// tracedContract 는 따라가 닿은 계약 하나다.
type tracedContract struct {
	owner    string // 그 계약을 펴낸 저장소
	contract string // 계약 생성물의 저장소 안 경로
	why      string // 파일 → 공장 → 계약
	note     string // 그 계약에 적힌 말. 앱 공용·읽기 전용 같은 경고가 여기 있다.
}

// traceContracts 는 고칠 파일들에서 닿는 계약을 모은다.
//
// 저장소로 묶으면 안 된다. 한 저장소가 계약을 여럿 펴내므로, 나중에 훑은
// 파일의 계약이 앞의 것을 덮어쓰고 어느 .proto 를 고칠지가 훑는 순서로
// 정해진다.
func traceContracts(files []string, trace func(string) (owner, gen, why string)) ([]string, map[string]tracedContract) {
	seen := map[string]tracedContract{}
	var order []string
	for _, f := range files {
		owner, gen, why := trace(f)
		if owner == "" {
			continue
		}
		key := gen
		if key == "" {
			key = owner // 생성물 경로를 못 읽으면 저장소로 묶는 수밖에 없다
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = tracedContract{
			owner:    owner,
			contract: gen,
			why:      fmt.Sprintf("%s → %s", f, why),
		}
		order = append(order, key)
	}
	return order, seen
}

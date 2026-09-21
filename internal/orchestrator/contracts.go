package orchestrator

import "fmt"

// tracedContract 는 파일에서 따라가 닿은 계약 하나다.
type tracedContract struct {
	owner string // 그 계약을 펴낸 저장소
	why   string // 파일 → 공장 → 계약
}

// traceContracts 는 고칠 파일들에서 닿는 **계약**을 모은다.
//
// 후보를 저장소로 묶으면 안 된다. 한 저장소가 계약을 여럿 펴내므로, 나중에
// 훑은 파일의 계약이 앞의 것을 덮어썼고 **어느 .proto 를 고칠지가 훑는
// 순서로 정해졌다.** 그렇게 뽑힌 것이 스스로 「읽기 전용이다」 라고 적어 둔
// 조회 계약이었다 — 임자 저장소는 우연히 맞았다.
//
// 계약 경로로 묶으면 그 우연이 사라지고, 여럿이면 여럿으로 남아 고를 수 있다.
func traceContracts(files []string, trace func(string) (owner, why string)) ([]string, map[string]tracedContract) {
	seen := map[string]tracedContract{}
	var order []string
	for _, f := range files {
		owner, why := trace(f)
		if owner == "" {
			continue
		}
		key := lastProtosPath(why)
		if key == "" {
			key = owner // 생성물 경로를 못 읽으면 저장소로 묶는 수밖에 없다
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = tracedContract{owner: owner, why: fmt.Sprintf("%s → %s", f, why)}
		order = append(order, key)
	}
	return order, seen
}

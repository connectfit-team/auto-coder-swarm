package orchestrator

import "testing"

// 컴파일된 것도 생성물이다.
//
// W-65073 이 ceo 의 web_contents 안 main.dart.js 를 계획에 넣고 코더가
// 고쳤다. 그 파일은 flutter build 가 다시 만든다 — 고쳐도 조용히 사라진다.
func TestGeneratedPathCoversCompiled(t *testing.T) {
	generated := []string{
		"internal/web_contents/public/publishing/gig/main.dart.js",
		"internal/web_contents/payslip/main.dart.js",
		"static/app.min.js",
		"static/app.min.css",
		"static/bundle.js.map",
		"web/build/index.js",
		"web/dist/main.js",
		"api/message.pb.go",
		"lib/model/user.g.dart",
		"lib/model/user.freezed.dart",
		"lib/proto/message.pb.dart",
	}
	for _, p := range generated {
		if !isGeneratedPath(p) {
			t.Errorf("생성물인데 놓쳤다: %s", p)
		}
	}

	// 사람이 쓴 것은 막지 말아야 한다. 안 그러면 고칠 곳을 잃는다.
	handWritten := []string{
		"internal_v2/business/connect.go",
		"internal/rpc/v1.rpc.ceo.work.go",
		"src/lib/utils/workstamp.ts",
		"lib/model/user_profile.dart",
		"src/routes/users/+page.svelte",
		// 이름에 build·dist 가 들어가도 폴더가 아니면 사람 코드다.
		"internal/business/buildinfo.go",
		"src/lib/distance.ts",
	}
	for _, p := range handWritten {
		if isGeneratedPath(p) {
			t.Errorf("사람이 쓴 코드를 생성물로 봤다: %s", p)
		}
	}
}

package orchestrator

import "testing"

func diffOf(path string, lines ...string) string {
	out := "--- a/" + path + "\n+++ b/" + path + "\n@@\n"
	for _, l := range lines {
		out += "+" + l + "\n"
	}
	return out
}

func TestCheckToolLeak(t *testing.T) {
	cases := []struct {
		name  string
		diff  string
		block bool
	}{
		{"W-76095 의 그 수정", diffOf("src/business/connected_ceo_device_service.go",
			`	endpoint := "http://127.0.0.1:8000/v1/chat/completions"`), true},
		{"멀쩡한 연결보류", diffOf("src/lib/types/connectactions.ts",
			`	HOLD = 'hold',`), false},
		// 줄에 "test" 가 있으면 넘기던 때에는 이것이 통과했다.
		{"주소 안에 test 가 들어간 경우", diffOf("src/lib/api/client.ts",
			`	const base = "http://127.0.0.1:8000/v1/latest";`), true},
		// 앞에 host 가 없는 8000 은 주소가 아니다.
		{"timeout:8000 은 주소가 아니다", diffOf("src/lib/util/retry.ts",
			`	const opts = { timeout:8000, retries: 3 };`), false},
		{"host 가 붙은 포트", diffOf("src/lib/api/client.ts",
			`	const url = "//api.internal:8082/embed";`), true},
		{"시험 파일의 제 서버", diffOf("internal/handler/connect_test.go",
			`	srv := httptest.NewServer(h) // http://127.0.0.1:8000`), false},
		{"svelte 시험", diffOf("src/lib/api/client.spec.ts",
			`	const base = "http://localhost:8000";`), false},
		{"도구 이름", diffOf("src/lib/server/ai.ts",
			`	const model = await qdrant.search(q);`), true},
		{"제품의 평범한 포트", diffOf("src/lib/config.ts",
			`	const port = 3000;`), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bad := CheckToolLeak(c.diff)
			if c.block && len(bad) == 0 {
				t.Errorf("막아야 하는데 통과시켰다")
			}
			if !c.block && len(bad) != 0 {
				t.Errorf("멀쩡한 수정을 막았다: %v", bad)
			}
		})
	}
}

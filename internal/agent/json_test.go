package agent

import (
	"encoding/json"
	"testing"
)

// 모델이 실제로 뱉은 모양들이다. 하나라도 못 읽으면 상위에서는 엉뚱한 이유
// ("작업 규모 과다") 로 작업이 죽는다.
func TestExtractJSONHandlesRealModelOutput(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"펜스로 감쌈", "```json\n{\"is_feasible\": true}\n```"},
		{"펜스 뒤에 설명이 더 붙음", "```json\n{\"is_feasible\": true}\n```\n이 계획은 타당합니다. {참고}"},
		{"앞에 산문", "분석 결과입니다.\n{\"is_feasible\": true}"},
		{"객체 안 // 주석", "{\n  \"is_feasible\": true // 가능하다\n}"},
		{"블록 주석", "{ /* 판단 */ \"is_feasible\": true }"},
		{"목록 끝 쉼표", "{\"actionable_path\": [\"a\", \"b\",], \"is_feasible\": true,}"},
		{"중첩 객체", "{\"meta\": {\"n\": 1}, \"is_feasible\": true}"},
		{"뒤에 } 가 든 산문", "{\"is_feasible\": true}\n설명: 함수 f() { return } 를 고칩니다."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out struct {
				Feasible bool `json:"is_feasible"`
			}
			got := ExtractJSON(c.raw)
			if err := json.Unmarshal([]byte(got), &out); err != nil {
				t.Fatalf("파싱 실패: %v\n꺼낸 것: %s", err, got)
			}
			if !out.Feasible {
				t.Errorf("값이 안 실렸다: %s", got)
			}
		})
	}
}

// 문자열 안의 `//` 는 주석이 아니다. 지워 버리면 URL 이 든 값이 통째로 깨진다.
func TestExtractJSONKeepsURLsInStrings(t *testing.T) {
	var out struct {
		URL string `json:"url"`
	}
	got := ExtractJSON(`{"url": "https://github.com/connectfit-team/worker"}`)
	if err := json.Unmarshal([]byte(got), &out); err != nil {
		t.Fatalf("파싱 실패: %v (%s)", err, got)
	}
	if out.URL != "https://github.com/connectfit-team/worker" {
		t.Errorf("URL 이 잘렸다: %q", out.URL)
	}
}

func TestExtractJSONNoObject(t *testing.T) {
	if got := ExtractJSON("  형식을 못 지켰습니다  "); got != "형식을 못 지켰습니다" {
		t.Errorf("객체가 없으면 원문을 그대로 줘야 한다: %q", got)
	}
}

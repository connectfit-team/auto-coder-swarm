package main

import "testing"

func TestHostPort(t *testing.T) {
	cases := map[string]string{
		"http://localhost:8007":          "localhost:8007",
		"http://127.0.0.1:8005/api/v1":   "127.0.0.1:8005",
		"https://ckh.internal:443/x?y=1": "ckh.internal:443",
		// 포트가 없으면 기본을 쓴다 — 잘못 만든 주소로 영원히 기다리면 안 된다.
		"http://localhost": "1.2.3.4:9",
		"":                 "1.2.3.4:9",
	}
	for in, want := range cases {
		if got := hostPort(in, "1.2.3.4:9"); got != want {
			t.Errorf("%q → %q, 기대 %q", in, got, want)
		}
	}
}

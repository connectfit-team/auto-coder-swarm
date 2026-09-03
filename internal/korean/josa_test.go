package korean

import "testing"

func TestJosa(t *testing.T) {
	cases := []struct {
		word string
		want string
	}{
		{"인스타그램", "인스타그램을"}, // ㅁ 받침
		{"네이버", "네이버를"},
		{"카카오", "카카오를"},
		{"구글", "구글을"}, // ㄹ 받침
		{"애플", "애플을"},
		{"instagram", "instagram을"},
		{"naver", "naver를"}, // 네이버 로 읽힌다
		{"kakao", "kakao를"},
		{"google", "google을"}, // 구글
		{"apple", "apple을"},   // 애플
		{"cms", "cms를"},       // 씨엠에스
		{"v2", "v2를"},
		{"v1", "v1을"},
		{"이름 (설명)", "이름 (설명)을"},
	}
	for _, c := range cases {
		if got := With(c.word, "을", "를"); got != c.want {
			t.Errorf("With(%q) = %q, want %q", c.word, got, c.want)
		}
	}
}

func TestSubjectJosa(t *testing.T) {
	if got := With("naver", "이", "가"); got != "naver가" {
		t.Errorf("naver = %q", got)
	}
	if got := With("proto-userapis", "이", "가"); got != "proto-userapis가" {
		t.Errorf("proto-userapis = %q", got)
	}
}

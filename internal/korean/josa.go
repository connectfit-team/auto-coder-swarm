// Package korean 은 사람이 읽는 문장을 만들 때 쓰는 한국어 규칙을 모은다.
package korean

import "strings"

// 조사는 앞말의 받침에 따라 갈린다. "인스타그램를" 처럼 틀리면 PR 제목에 그대로 남는다.

// Josa 는 앞말 뒤에 붙일 조사를 고른다. withBatchim 이 받침 있을 때 쓰는 쪽이다.
// 예: Josa("인스타그램", "을", "를") == "을"
func Josa(word, withBatchim, without string) string {
	if hasBatchim(word) {
		return withBatchim
	}
	return without
}

// With 는 앞말과 조사를 붙여서 준다. 예: With("naver", "이", "가") == "naver가"
func With(word, withBatchim, without string) string {
	return word + Josa(word, withBatchim, without)
}

// hasBatchim 은 마지막 글자에 받침이 있는지 본다.
func hasBatchim(word string) bool {
	r := lastLetter(word)
	if r == 0 {
		return false
	}
	if r >= 0xAC00 && r <= 0xD7A3 {
		return (r-0xAC00)%28 != 0
	}
	// 숫자는 읽는 소리를 따른다. 영(0)·일(1)·삼(3)·육(6)·칠(7)·팔(8) 에 받침이 있다.
	if r >= '0' && r <= '9' {
		return strings.ContainsRune("013678", r)
	}
	// 로마자는 한국어로 읽는 소리를 따른다. 끝의 묵음 e 를 떼고 보면
	// google→구글(ㄹ), apple→애플(ㄹ) 처럼 받침이 드러난다. 반대로 끝이
	// r 이나 s 면 서버·에스처럼 받침 없이 읽힌다.
	if r >= 'A' && r <= 'Z' {
		r += 'a' - 'A'
	}
	if r >= 'a' && r <= 'z' {
		return strings.ContainsRune("lmn", r)
	}
	return false
}

// lastLetter 는 뒤에서부터 글자 하나를 찾는다. 괄호나 따옴표는 건너뛰고,
// 로마자 낱말 끝의 묵음 e 도 건너뛴다 (google 의 소리는 ㄹ 로 끝난다).
func lastLetter(word string) rune {
	rs := []rune(word)
	if n := len(rs); n >= 2 && (rs[n-1] == 'e' || rs[n-1] == 'E') && isLatin(rs[n-2]) {
		rs = rs[:n-1]
	}
	for i := len(rs) - 1; i >= 0; i-- {
		r := rs[i]
		if r >= 0xAC00 && r <= 0xD7A3 ||
			r >= '0' && r <= '9' ||
			r >= 'a' && r <= 'z' ||
			r >= 'A' && r <= 'Z' {
			return r
		}
	}
	return 0
}

func isLatin(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

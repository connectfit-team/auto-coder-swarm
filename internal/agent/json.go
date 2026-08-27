package agent

import "strings"

// ExtractJSON 은 모델이 뱉은 텍스트에서 JSON 객체 하나를 꺼낸다.
//
// 모델은 시키는 형식을 지키지 않는다. 실측으로 본 것: 마크다운 펜스로 감싸고,
// 객체 안에 `//` 주석을 넣고, 앞뒤에 설명 문장을 붙였다. 그때마다 파싱이
// 실패했는데 상위에서는 "작업 규모 과다" 로만 보여서 원인을 한참 못 찾았다.
//
// 앞뒤를 잘라내는 방식(첫 `{` ~ 마지막 `}`)으로는 부족하다 — 뒤쪽 설명 문장에
// `}` 가 하나라도 있으면 그 사이의 산문을 통째로 삼킨다. 짝을 세어 자른다.
func ExtractJSON(raw string) string {
	s := stripJSONComments(raw)
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return strings.TrimSpace(raw)
	}

	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case esc:
			esc = false
		case inStr && c == '\\':
			esc = true
		case c == '"':
			inStr = !inStr
		case inStr:
		case c == '{':
			depth++
		case c == '}':
			if depth--; depth == 0 {
				return stripTrailingCommas(s[start : i+1])
			}
		}
	}
	return stripTrailingCommas(s[start:])
}

// stripJSONComments 는 `//` 와 `/* */` 를 걷어낸다.
// 문자열 안은 건드리지 않는다 — 값에 든 `https://` 가 주석으로 잘리면 안 된다.
func stripJSONComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inStr, esc := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if esc {
			esc = false
			b.WriteByte(c)
			continue
		}
		if inStr {
			switch c {
			case '\\':
				esc = true
			case '"':
				inStr = false
			}
			b.WriteByte(c)
			continue
		}
		if c == '"' {
			inStr = true
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(s) {
			if s[i+1] == '/' {
				for i < len(s) && s[i] != '\n' {
					i++
				}
				b.WriteByte('\n')
				continue
			}
			if s[i+1] == '*' {
				j := strings.Index(s[i+2:], "*/")
				if j < 0 {
					return b.String()
				}
				i += 2 + j + 1
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// stripTrailingCommas 는 `}`·`]` 바로 앞의 쉼표를 지운다.
// 모델이 목록 끝에 습관적으로 붙이는데, Go 의 파서는 이걸 받지 않는다.
func stripTrailingCommas(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inStr, esc := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if esc {
			esc = false
			b.WriteByte(c)
			continue
		}
		if inStr {
			switch c {
			case '\\':
				esc = true
			case '"':
				inStr = false
			}
			b.WriteByte(c)
			continue
		}
		if c == '"' {
			inStr = true
			b.WriteByte(c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j < len(s) && (s[j] == '}' || s[j] == ']') {
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

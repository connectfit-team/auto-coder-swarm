package agent

import "strings"

// 자리 찾기 사다리의 아랫단들.
//
// hermes-agent 의 `tools/fuzzy_match.py` 는 아홉 단을 쌓는다. 우리에게 없던
// 세 단을 가져왔다 — 이스케이프, 블록 앵커, 줄별 유사도. 우리가 가장 자주
// 지는 자리가 「원문에 없는 내용을 찾으라고 했다」 라 아랫단이 값이 크다.
//
// **유사도로 찾은 자리는 딱 하나일 때만 쓴다.** 원본이 그렇게 한다 —
// 「approximately resemble old_string — fine for one unique replacement,
// never safe under replace_all」. 비슷한 자리 여럿을 한꺼번에 바꾸면
// 엉뚱한 곳이 함께 바뀐다.

// unescapeLiterals 는 모델이 진짜 줄바꿈 대신 적어 보낸 `\n`·`\t`·`\r` 를 편다.
//
// 흔한 실패다. 찾는 내용이 한 줄로 오고 그 안에 역슬래시 n 이 들어 있으면
// 원문과는 영영 안 맞는다. 바뀌는 것이 없으면 이 단은 아무 일도 안 한다.
func unescapeLiterals(s string) string {
	if !strings.Contains(s, `\n`) && !strings.Contains(s, `\t`) && !strings.Contains(s, `\r`) {
		return s
	}
	r := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\r`, "\r")
	return r.Replace(s)
}

// simRatio 는 두 글의 닮은 정도를 0~1 로 준다.
//
// 파이썬 difflib 의 `SequenceMatcher.ratio()` 와 같은 꼴이다 — 맞은 글자 수의
// 두 배를 길이 합으로 나눈다. 맞은 글자는 가장 긴 공통 부분열로 센다.
//
// 길면 잘라서 센다. 이 자리는 찾아바꾸기 블록의 가운데라 길어야 몇십 줄이고,
// 자르지 않으면 O(n·m) 이 튄다.
func simRatio(a, b string) float64 {
	const cap = 1500
	ar, br := []rune(a), []rune(b)
	if len(ar) > cap {
		ar = ar[:cap]
	}
	if len(br) > cap {
		br = br[:cap]
	}
	if len(ar) == 0 && len(br) == 0 {
		return 1
	}
	if len(ar) == 0 || len(br) == 0 {
		return 0
	}
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for i := 1; i <= len(ar); i++ {
		for j := 1; j <= len(br); j++ {
			if ar[i-1] == br[j-1] {
				cur[j] = prev[j-1] + 1
			} else if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
			} else {
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
		for j := range cur {
			cur[j] = 0
		}
	}
	return 2 * float64(prev[len(br)]) / float64(len(ar)+len(br))
}

// blockAnchorAt 는 첫 줄과 마지막 줄이 맞는 자리를 찾고 가운데를 닮은 정도로
// 판단한다.
//
// 문턱은 후보가 하나면 0.50, 여럿이면 0.70 이다. 원본이 적어 둔 그대로다 —
// 「0.10/0.30 은 상관없는 블록까지 맞았다. 이것이 안전한 바닥이다」.
// 후보가 여럿일수록 엄하게 보는 것이 핵심이다.
func blockAnchorAt(srcLines, wantLines []string) []int {
	n := len(wantLines)
	if n < 2 || n > len(srcLines) {
		return nil
	}
	first := strings.TrimSpace(normalizePunct(wantLines[0]))
	last := strings.TrimSpace(normalizePunct(wantLines[n-1]))

	var cand []int
	for i := 0; i+n <= len(srcLines); i++ {
		if strings.TrimSpace(normalizePunct(srcLines[i])) == first &&
			strings.TrimSpace(normalizePunct(srcLines[i+n-1])) == last {
			cand = append(cand, i)
		}
	}
	if len(cand) == 0 {
		return nil
	}
	if n <= 2 {
		return cand
	}

	threshold := 0.70
	if len(cand) == 1 {
		threshold = 0.50
	}
	wantMid := strings.Join(trimEach(wantLines[1:n-1]), "\n")
	var out []int
	for _, i := range cand {
		gotMid := strings.Join(trimEach(srcLines[i+1:i+n-1]), "\n")
		if simRatio(normalizePunct(gotMid), normalizePunct(wantMid)) >= threshold {
			out = append(out, i)
		}
	}
	return out
}

// contextAwareAt 는 마지막 단이다. 첫 줄·마지막 줄이 0.80 이상 닮고, 빈 줄이
// 아닌 모든 줄이 0.80 이상 닮은 자리만 받는다.
//
// 앞뒤를 닻으로 삼아 훑을 범위를 줄이고, 「모든 줄」 조건이 우연히 맞는 것을
// 막는다. 원본의 설명 그대로다.
func contextAwareAt(srcLines, wantLines []string) []int {
	n := len(wantLines)
	if n == 0 || n > len(srcLines) {
		return nil
	}
	const near = 0.80
	first := strings.TrimSpace(wantLines[0])
	last := strings.TrimSpace(wantLines[n-1])

	var out []int
	for i := 0; i+n <= len(srcLines); i++ {
		block := srcLines[i : i+n]
		if simRatio(first, strings.TrimSpace(block[0])) < near {
			continue
		}
		if simRatio(last, strings.TrimSpace(block[n-1])) < near {
			continue
		}
		ok := true
		for j := 0; j < n; j++ {
			w := strings.TrimSpace(wantLines[j])
			if w == "" {
				continue
			}
			if simRatio(w, strings.TrimSpace(block[j])) < near {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, i)
		}
	}
	return out
}

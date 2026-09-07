package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
)

// 문법이 맞아도 뜻이 안 맞는 편집이 있다.
//
// stats.refundedPrice 처럼 없는 필드에 값을 더하거나, 없는 메서드를 부르는
// 코드는 파서를 통과한다. 타입까지 보려면 빌드를 돌려야 한다.
//
// 다만 **proto 가 아직 배포되지 않아서 나는 오류**는 우리 탓이 아니다.
// 계획이 proto 에 더하는 이름을 미리 모아 두고, 그 이름을 말하는 오류만
// 예상된 것으로 본다. 그 밖의 오류는 우리 편집이 틀렸다는 뜻이다.

// Go 는 없는 이름을 여러 모양으로 알린다.
//
//	undefined: pkg.Name
//	x.Name undefined (type T has no field or method Name)
//	T has no field or method Name
var missingNameRes = []*regexp.Regexp{
	regexp.MustCompile(`undefined: (\S+)`),
	regexp.MustCompile(`(\S+) undefined \(`),
	regexp.MustCompile(`has no field or method (\w+)`),
}

// GoRepo 는 그 사본이 Go 저장소인지 본다.
func GoRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

// goBuildErrors 는 빌드 오류 줄들을 준다. 통과하면 빈 목록이다.
func goBuildErrors(dir string) []string {
	if _, err := exec.LookPath("go"); err != nil {
		return nil
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "go: ") {
			continue
		}
		out = append(out, l)
	}
	return out
}

// PendingSymbols 는 아직 배포되지 않아 없는 것이 당연한 이름들이다.
// proto 에 더하는 값과, 사람이 채워야 할 이름이 여기 든다.
func PendingSymbols(plans []insightclient.VariantRepoPlan) []string {
	var out []string
	for _, p := range plans {
		if p.Publish == "protogen-make" {
			for _, c := range p.Changes {
				for _, line := range c.Block {
					for _, id := range protoIdents(line) {
						out = appendOnceStr(out, id)
					}
				}
			}
		}
		for _, n := range p.NeedsManual {
			for _, id := range protoIdents(n) {
				out = appendOnceStr(out, id)
			}
		}
	}
	return out
}

var identRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{3,}`)

func protoIdents(line string) []string {
	var out []string
	for _, m := range identRe.FindAllString(line, -1) {
		out = append(out, m)
	}
	return out
}

// UnexpectedBuildErrors 는 예상된 것을 뺀 빌드 오류를 준다.
func UnexpectedBuildErrors(errs, pending []string) []string {
	var out []string
	for _, e := range errs {
		if expectedBuildError(e, pending) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func expectedBuildError(e string, pending []string) bool {
	for _, name := range missingNames(e) {
		for _, p := range pending {
			if len(name) < 4 || len(p) < 4 {
				continue
			}
			if strings.Contains(name, p) || strings.Contains(p, name) {
				return true
			}
		}
	}
	return false
}

// missingNames 는 오류에서 "없다" 고 지목된 이름들을 뽑는다.
func missingNames(e string) []string {
	var out []string
	for _, re := range missingNameRes {
		for _, m := range re.FindAllStringSubmatch(e, -1) {
			name := m[1]
			if i := strings.LastIndex(name, "."); i >= 0 {
				name = name[i+1:]
			}
			out = appendOnceStr(out, name)
		}
	}
	return out
}

// 빌드 오류에는 두 갈래가 있다.
//
// 없는 이름(undefined · has no field or method)은 **아직 안 만든 것**이다 —
// 새 proto 필드나 새 메서드가 필요하다는 뜻이고, 사람이 만들면 된다.
// 그 밖의 오류(타입 안 맞음 등)는 우리 편집이 **틀렸다**는 뜻이다.
//
// 앞의 것으로 저장소를 통째로 버리면 멀쩡한 자리까지 잃는다(② 에서 열한 곳).
// 사람에게 올리고 PR 은 초안으로 연다. 뒤의 것은 막는다.

// SplitBuildErrors 는 빌드 오류를 "사람이 만들면 되는 것" 과 "우리가 틀린 것"
// 으로 나눈다.
func SplitBuildErrors(errs []string) (missing, wrong []string) {
	for _, e := range errs {
		if len(missingNames(e)) > 0 {
			missing = append(missing, e)
			continue
		}
		wrong = append(wrong, e)
	}
	return missing, wrong
}

package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestInternalSignalNeverReachesHuman(t *testing.T) {
	tc := &taskContext{lastFeedback: "CODING_PARTIAL: a.ts 를 못 고쳤다"}

	got := tc.humanReason(fmt.Errorf("시도 3: %w", errRetryPlanning)).Error()
	if strings.Contains(got, errRetryPlanning.Error()) {
		t.Errorf("안쪽 신호가 그대로 사유가 됐다: %q", got)
	}
	if !strings.Contains(got, "a.ts 를 못 고쳤다") {
		t.Errorf("왜 막혔는지가 빠졌다: %q", got)
	}

	// 되먹임이 없어도 신호 문구를 쓰면 안 된다.
	if bare := (&taskContext{}).humanReason(errRetryPlanning).Error(); strings.Contains(bare, errRetryPlanning.Error()) {
		t.Errorf("되먹임이 없을 때 안쪽 신호가 그대로 나갔다: %q", bare)
	}

	// 바깥 오류와 nil 은 건드리지 않는다.
	real := fmt.Errorf("빌드가 오류 15개로 깨졌다")
	if tc.humanReason(real) != real {
		t.Errorf("바깥 오류를 바꿨다")
	}
	if tc.humanReason(nil) != nil {
		t.Errorf("nil 을 오류로 바꿨다")
	}
}

// 마지막 관문은 execute 에 걸려 있어야 한다. 떼면 신호가 다시 샌다.
func TestLastGateIsWired(t *testing.T) {
	src := readSource(t, "flow.go")
	if !strings.Contains(src, "err = t.humanReason(err)") {
		t.Error("execute 에 humanReason 관문이 없다")
	}
	// 마지막 시도에서 신호를 그대로 돌려주던 옛 모양.
	if strings.Contains(src, "errors.Is(err, errRetryPlanning) && attempt < 3") {
		t.Error("마지막 시도에서 안쪽 신호가 그대로 밖으로 나가는 옛 모양이 남아 있다")
	}
}

// 신호를 새로 만들었으면 internal_signal.go 에 적혀 있어야 한다.
// 사람이 봐도 되는 오류라면 그 까닭을 같은 파일에 적는다.
func TestEverySentinelIsAccountedFor(t *testing.T) {
	decl := regexp.MustCompile(`var (err[A-Za-z0-9_]*) = errors\.New\(`)
	reg := readSource(t, "internal_signal.go")

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range decl.FindAllStringSubmatch(string(b), -1) {
			if !strings.Contains(reg, m[1]) {
				t.Errorf("%s 의 %s 가 internal_signal.go 에 없다 — 안쪽 신호면 목록에 넣고, 아니면 까닭을 적어라", f, m[1])
			}
		}
	}
}

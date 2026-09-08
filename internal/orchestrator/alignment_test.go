package orchestrator

import (
	"strings"
	"testing"
)

const okDiff = `diff --git a/internal/domain/t.go b/internal/domain/t.go
--- a/internal/domain/t.go
+++ b/internal/domain/t.go
@@ -10,6 +10,8 @@
 	TypeDaily
+	// TypeBonus 상여금
+	TypeBonus
 )
`

func TestAlignmentPassesPlainAddition(t *testing.T) {
	if bad := CheckAlignment(AlignmentInput{Repo: "worker", Value: "bonus", Diff: okDiff}); len(bad) > 0 {
		t.Errorf("더하기만 한 편집을 막았다: %s", AlignmentNote(bad))
	}
}

// 생성물은 proto 배포가 다시 만든다. 손으로 고치면 그 변경은 사라진다.
func TestAlignmentBlocksGeneratedFiles(t *testing.T) {
	diff := `--- a/go/workstampv1/message.pb.go
+++ b/go/workstampv1/message.pb.go
@@
+	WorkStampType_BONUS WorkStampType = 12
`
	bad := CheckAlignment(AlignmentInput{Repo: "protogen", Value: "bonus", Diff: diff})
	if len(bad) == 0 || !strings.Contains(bad[0].Why, "생성물") {
		t.Fatalf("생성물을 안 막았다: %+v", bad)
	}
}

func TestAlignmentBlocksSensitiveFiles(t *testing.T) {
	for _, path := range []string{
		".env.production",
		".github/workflows/deploy.yml",
		"k8s/overlays/prod/kustomization.yaml",
		"internal/mariadb/migrations/003_add.sql",
	} {
		diff := "--- a/" + path + "\n+++ b/" + path + "\n@@\n+something\n"
		bad := CheckAlignment(AlignmentInput{Repo: "worker", Value: "bonus", Diff: diff})
		if len(bad) == 0 {
			t.Errorf("%s 를 안 막았다", path)
		}
	}
}

// 값을 더하는 일은 더하기만 한다. 지운 줄이 있으면 시킨 일이 아니다.
func TestAlignmentBlocksDeletions(t *testing.T) {
	diff := `--- a/internal/domain/t.go
+++ b/internal/domain/t.go
@@
-	TypeDaily
+	TypeBonus
`
	bad := CheckAlignment(AlignmentInput{Repo: "worker", Value: "bonus", Diff: diff})
	if len(bad) == 0 || !strings.Contains(bad[0].Why, "지웠다") {
		t.Fatalf("지운 줄을 안 막았다: %+v", bad)
	}
}

// gofmt 가 정렬을 다시 하면 지운 줄과 더한 줄이 짝으로 나온다. 그것은 막지 않는다.
func TestAlignmentAllowsReformatting(t *testing.T) {
	diff := `--- a/internal/domain/t.go
+++ b/internal/domain/t.go
@@
-	TypeDaily   Type = 3
+	TypeDaily Type = 3
+	TypeBonus Type = 4
`
	if bad := CheckAlignment(AlignmentInput{Repo: "worker", Value: "bonus", Diff: diff}); len(bad) > 0 {
		t.Errorf("정렬만 바뀐 줄을 막았다: %s", AlignmentNote(bad))
	}
}

// 요청이 저장소를 지목했으면 그 밖은 고치지 않는다. proto 는 예외다 —
// 그것이 배포돼야 나머지가 컴파일된다.
func TestAlignmentBlocksUnnamedRepo(t *testing.T) {
	in := AlignmentInput{Repo: "worker", Value: "bonus", Diff: okDiff, Named: []string{"cms"}}
	if bad := CheckAlignment(in); len(bad) == 0 {
		t.Error("지목하지 않은 저장소를 안 막았다")
	}
	in.Repo = "cms"
	if bad := CheckAlignment(in); len(bad) > 0 {
		t.Errorf("지목한 저장소를 막았다: %s", AlignmentNote(bad))
	}
	in.Repo, in.Blocker = "proto-workstampapis", true
	if bad := CheckAlignment(in); len(bad) > 0 {
		t.Errorf("먼저 배포돼야 하는 저장소를 막았다: %s", AlignmentNote(bad))
	}
}

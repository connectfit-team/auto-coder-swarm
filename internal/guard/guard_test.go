package guard

import (
	"strings"
	"testing"
)

const plainDiff = `diff --git a/internal/domain/t.go b/internal/domain/t.go
--- a/internal/domain/t.go
+++ b/internal/domain/t.go
@@ -10,6 +10,8 @@
 	TypeDaily
+	// TypeBonus 상여금
+	TypeBonus
 )
`

func TestBeforePushLetsOrdinaryWorkThrough(t *testing.T) {
	if bad := BeforePush(plainDiff, "feat/add-bonus-W-1"); len(bad) > 0 {
		t.Errorf("멀쩡한 편집을 막았다: %s", Note(bad))
	}
}

// 어느 흐름이든 사람이 지키는 가지로 바로 밀지 않는다.
func TestBeforePushBlocksProtectedBranches(t *testing.T) {
	for _, b := range []string{"master", "main", "develop", "release"} {
		if bad := BeforePush(plainDiff, b); len(bad) == 0 {
			t.Errorf("%s 로 바로 미는 것을 안 막았다", b)
		}
	}
}

func TestBeforePushBlocksGeneratedAndSensitive(t *testing.T) {
	blocked := map[string]string{
		"go/workstampv1/message.pb.go":         "생성물",
		"src/lib/server/protos/x/message.ts":   "생성물",
		"lib/model/work_stamp.pb.dart":         "생성물",
		".env.production":                      "설정",
		".github/workflows/deploy.yml":         "설정",
		"k8s/overlays/prod/kustomization.yaml": "설정",
		"deploy.sh":                            "설정",
		"internal/mariadb/migrations/003.sql":  "설정",
		"secrets.yaml":                         "설정",
		"certs/server.pem":                     "설정",
	}
	for path, want := range blocked {
		diff := "--- a/" + path + "\n+++ b/" + path + "\n@@\n+something\n"
		bad := BeforePush(diff, "feat/x")
		if len(bad) == 0 {
			t.Errorf("%s 를 안 막았다", path)
			continue
		}
		if !strings.Contains(bad[0].Why, want) {
			t.Errorf("%s: 까닭이 %q 인데 %q 를 바랐다", path, bad[0].Why, want)
		}
	}

	// 이름에 든 낱말로 막으면 멀쩡한 코드가 걸린다.
	// 실측: gig_mobile 의 secret.pin.manager.dart 는 앱의 PIN 화면이다.
	for _, path := range []string{
		"lib/infrastructure/global.config/secret.pin.manager.dart",
		"internal/deploy/deployment.go",
		"internal/domain/credential_store.go",
		"src/lib/utils/workstamp.ts",
	} {
		diff := "--- a/" + path + "\n+++ b/" + path + "\n@@\n+something\n"
		if bad := BeforePush(diff, "feat/x"); len(bad) > 0 {
			t.Errorf("%s 를 막았다: %s", path, Note(bad))
		}
	}
}

// 값을 더하는 일에만 쓰는 규칙. 버그 고치기는 지우는 것이 당연하다.
func TestInsertOnly(t *testing.T) {
	del := `--- a/t.go
+++ b/t.go
@@
-	TypeDaily
+	TypeBonus
`
	if bad := InsertOnly(del); len(bad) == 0 {
		t.Error("지운 줄을 안 막았다")
	}
	// gofmt 가 정렬을 다시 한 것은 막지 않는다.
	fmted := `--- a/t.go
+++ b/t.go
@@
-	TypeDaily   Type = 3
+	TypeDaily Type = 3
+	TypeBonus Type = 4
`
	if bad := InsertOnly(fmted); len(bad) > 0 {
		t.Errorf("정렬만 바뀐 줄을 막았다: %s", Note(bad))
	}
	// 한 줄로 적힌 열거를 늘린 것은 지운 것이 아니다.
	grown := `--- a/lib/model/user_profile.dart
+++ b/lib/model/user_profile.dart
@@
-enum CEOJobType { notSet, fullTime, daily, freelance, notYet }
+enum CEOJobType { notSet, fullTime, daily, freelance, contract, notYet }
`
	if bad := InsertOnly(grown); len(bad) > 0 {
		t.Errorf("줄을 늘린 것을 막았다: %s", Note(bad))
	}
	// 줄이 짧아지면 무언가 사라진 것이다.
	shrunk := `--- a/t.dart
+++ b/t.dart
@@
-enum T { a, b, c }
+enum T { a, b }
`
	if bad := InsertOnly(shrunk); len(bad) == 0 {
		t.Error("줄이 짧아진 것을 안 막았다")
	}
	if bad := InsertOnly(plainDiff); len(bad) > 0 {
		t.Errorf("더하기만 한 편집을 막았다: %s", Note(bad))
	}
}

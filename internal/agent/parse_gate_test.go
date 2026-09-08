package agent

import (
	"strings"
	"testing"
)

// 통짜로 다시 쓴 것이 뭉개졌으면 저장하지 않는다. W-77855 의 실제 모양이다.
func TestCheckSyntaxCatchesMangledGo(t *testing.T) {
	mangled := `package mariadb

func (r Connect) Get(ctx context.Context) error {
	if err := r.WithContext(ctx).First(&x).Error; err != nil {
		return err
	}
	cordNotFound
	return nil
`
	err := CheckSyntax("internal_v2/mariadb/connect.go", mangled)
	if err == nil {
		t.Fatal("뭉개진 Go 를 통과시켰다 — 그대로 저장된다")
	}
	if !strings.Contains(err.Error(), "문법에 안 맞는다") {
		t.Errorf("까닭을 안 말한다: %v", err)
	}

	good := "package p\n\nfunc A() error {\n\treturn nil\n}\n"
	if err := CheckSyntax("a.go", good); err != nil {
		t.Errorf("멀쩡한 Go 를 막았다: %v", err)
	}

	// 빈 내용은 저장하면 안 된다 — 파일이 사라진 것과 같다.
	if CheckSyntax("a.go", "   \n") == nil {
		t.Error("빈 내용을 통과시켰다")
	}

	// 검사할 수 없는 언어는 막지 않는다. 없는 검사를 실패로 만들면 안 된다.
	if err := CheckSyntax("a.dart", "이건 Dart 가 아니지만 여기서는 안 본다"); err != nil {
		t.Errorf("검사 못 하는 언어를 막았다: %v", err)
	}
}

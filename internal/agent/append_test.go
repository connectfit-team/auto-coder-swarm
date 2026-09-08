package agent

import (
	"strings"
	"testing"
)

// 새 메서드는 형제 뒤에 붙어야 한다. W-57730 이 이것으로 세 시도를 태웠다.
func TestAppendsNewMethodBesideSibling(t *testing.T) {
	src := `package grpc

// ConnectedWorkplaceIDListGet 은 연결이 끝난 근무지 ID 목록을 조회한다.
func (s *ConnectService) ConnectedWorkplaceIDListGet(ctx context.Context, _ *connectv1.RequestConnectedWorkplaceIDListGet) (*connectv1.ResponseConnectedWorkplaceIDListGet, error) {
	return s.biz.ConnectedWorkplaceIDListGet(ctx)
}
`
	// 코더가 낸 것 — 찾을 내용은 원문에 없다(주석 한 줄).
	search := "// CEOWorkConnectionUpdate 메서드를 구현합니다."
	replace := `// CEOWorkConnectionHold 은 연결을 보류 상태로 바꾼다.
func (s *ConnectService) CEOWorkConnectionHold(ctx context.Context, req *connectv1.RequestConnectionHold) (*connectv1.ResponseConnectionHold, error) {
	return s.biz.ConnectionHold(ctx, req.GetId())
}`

	got, err := replaceLoosely(src, search, replace)
	if err != nil {
		t.Fatalf("붙이지 못했다: %v", err)
	}
	if !strings.Contains(got, "CEOWorkConnectionHold") {
		t.Fatal("새 메서드가 안 들어갔다")
	}
	// 형제 **뒤**에 붙어야 한다. 함수 안에 들어가면 안 된다.
	iSib := strings.Index(got, "ConnectedWorkplaceIDListGet(ctx")
	iNew := strings.Index(got, "CEOWorkConnectionHold(ctx")
	if iNew < iSib {
		t.Error("형제 앞에 붙었다")
	}
	if strings.Count(got, "{") != strings.Count(got, "}") {
		t.Errorf("중괄호가 안 맞는다:\n%s", got)
	}
	if AppendedNote() == "" {
		t.Error("붙였다는 사실을 안 남긴다 — 사람이 diff 를 잘못 읽는다")
	}
}

// 조각은 붙이지 않는다. 문법이 깨진다.
func TestDoesNotAppendFragment(t *testing.T) {
	src := "package p\n\nfunc A() {\n\treturn\n}\n"
	if _, err := replaceLoosely(src, "없는 것", "if len(subscriptions) "); err == nil {
		t.Error("조각을 붙였다")
	}
}

// 이미 있는 이름은 붙이지 않는다. 두 번 선언된다.
func TestDoesNotAppendDuplicate(t *testing.T) {
	src := "package p\n\nfunc (s *S) Hold(ctx context.Context) error {\n\treturn nil\n}\n"
	dup := "func (s *S) Hold(ctx context.Context) error {\n\treturn nil\n}"
	if _, err := replaceLoosely(src, "없는 것", dup); err == nil {
		t.Error("이미 있는 이름을 또 붙였다")
	}
}

// 겹치는 것이 없으면 붙이지 않는다. 엉뚱한 곳에 붙이는 것은 실패보다 나쁘다.
func TestDoesNotAppendWhenUnrelated(t *testing.T) {
	src := "package p\n\nfunc CalculateTax(base int) int {\n\treturn base / 10\n}\n"
	newFn := "func (s *ConnectService) ConnectionHoldByRequestUserID(ctx context.Context, id string) error {\n\treturn nil\n}"
	if _, err := replaceLoosely(src, "없는 것", newFn); err == nil {
		t.Error("아무 상관 없는 곳에 붙였다")
	}
}

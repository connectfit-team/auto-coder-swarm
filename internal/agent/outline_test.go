package agent

import (
	"strings"
	"testing"
)

// 코더가 없는 필드와 이미 있는 이름을 지어내지 않게, 뼈대를 함께 줘야 한다.
func TestFileOutline(t *testing.T) {
	src := `package business

type Connect struct {
	repository ConnectRepository
	logger     *slog.Logger
}

type ConnectBusiness interface {
	CEOWorkConnectGet(ctx context.Context) error
}

const StateHeld = "HELD"

func (c *Connect) CEOWorkConnectUpdateState(ctx context.Context) error { return nil }
func (c *Connect) CEOWorkConnectGet(ctx context.Context) error        { return nil }
func New(r ConnectRepository) *Connect                                 { return &Connect{repository: r} }
`
	out := FileOutline("business/connect.go", src)
	for _, must := range []string{
		"repository",                // 진짜 필드 이름 (c.repo 를 지어내지 않게)
		"CEOWorkConnectUpdateState", // 이미 있는 메서드 (중복 선언을 안 하게)
		"ConnectBusiness",           // 인터페이스
		"StateHeld",                 // 상수
		"지어내지 마라",
	} {
		if !strings.Contains(out, must) {
			t.Errorf("뼈대에 %q 가 없다:\n%s", must, out)
		}
	}
	// 없는 필드는 없어야 한다.
	if strings.Contains(out, " repo,") || strings.Contains(out, "{ repo ") {
		t.Errorf("없는 필드가 들어갔다:\n%s", out)
	}

	// Go 가 아니면 아무것도 안 준다.
	if FileOutline("a.ts", "export const x = 1") != "" {
		t.Error("Go 아닌 파일에 뼈대를 만들었다")
	}
	// 뭉개진 파일의 뼈대는 믿을 수 없다.
	if FileOutline("a.go", "package p\nfunc (") != "" {
		t.Error("파싱 안 되는 파일의 뼈대를 만들었다")
	}
}

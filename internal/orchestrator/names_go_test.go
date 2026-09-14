package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func goFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	w := func(p, s string) {
		full := filepath.Join(dir, p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("go.mod", "module example.com/shop\n\ngo 1.22\n")
	w("internal/store/store.go", `package store

type Client struct{}

func New() *Client { return &Client{} }

// 메서드는 받는 타입과 함께 적어야 쓸모가 있다.
func (c *Client) ListInvites() error { return nil }
func (c *Client) AcceptRequest() error { return nil }
func (c *Client) hidden() {}

const MaxInvites = 50

var ErrGone = New()

type invisible struct{}
`)
	w("internal/store/store_test.go", "package store\n\nfunc HelperOnlyInTest() {}\n")
	w("internal/handler/connect.go", `package handler

import "example.com/shop/internal/store"

func Do() error { return store.New().ListInvites() }
`)
	return dir
}

func TestAvailableNamesGo(t *testing.T) {
	dir := goFixture(t)
	got := availableNamesGo(dir, "internal/handler/connect.go")

	for _, want := range []string{
		"example.com/shop/internal/store",
		"Client", "New", "MaxInvites", "ErrGone",
		"Client.ListInvites", "Client.AcceptRequest",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 쪽지에 없다:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"hidden", "invisible", "HelperOnlyInTest"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("%q 가 쪽지에 들어갔다", unwanted)
		}
	}
}

// 밖의 꾸러미만 들여오는 파일에는 붙일 것이 없다.
func TestAvailableNamesGoExternalOnly(t *testing.T) {
	dir := goFixture(t)
	p := filepath.Join(dir, "internal/handler/only.go")
	os.WriteFile(p, []byte("package handler\n\nimport \"fmt\"\n\nfunc F() { fmt.Println() }\n"), 0o644)
	if got := availableNamesGo(dir, "internal/handler/only.go"); got != "" {
		t.Errorf("붙일 것이 없어야 한다: %q", got)
	}
}

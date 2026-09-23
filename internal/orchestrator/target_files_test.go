package orchestrator

import (
	"strings"
	"testing"
)

func TestNonEmptyUnique(t *testing.T) {
	got := nonEmptyUnique("a.proto", "", "b.proto", "a.proto", "  ")
	if strings.Join(got, ",") != "a.proto,b.proto" {
		t.Fatalf("빈 것·겹치는 것을 못 걸렀다: %v", got)
	}
	if nonEmptyUnique("", " ") != nil {
		t.Fatal("다 비었는데 목록이 남는다")
	}
}

// 부모가 짚은 파일이 요청에 실려야 한다 — 글로만 적으면 자식이 다시 알아맞힌다.
func TestChainRequestCarriesTargetFiles(t *testing.T) {
	req := StatelessRequest{
		TargetRepo:  "proto-ceowebapis",
		TargetFiles: nonEmptyUnique("ceoweb/v1/connect.communication.proto", "ceoweb/v1/connect.service.proto"),
		AddsState:   true,
	}
	if len(req.TargetFiles) != 2 {
		t.Fatalf("두 자리를 다 싣지 못했다: %v", req.TargetFiles)
	}
	// 필드가 들어갈 자리가 먼저다 — 범위는 맨 앞을 쓴다.
	if !strings.HasSuffix(req.TargetFiles[0], "connect.communication.proto") {
		t.Fatalf("메시지가 있는 파일이 앞에 와야 한다: %v", req.TargetFiles)
	}
}

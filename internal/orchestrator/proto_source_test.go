package orchestrator

import (
	"strings"
	"testing"
)

func TestLastProtosPath(t *testing.T) {
	why := "src/lib/server/data/connectcud.ts → getConnectClient() → ConnectCEOWebDefinition → src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts"
	got := lastProtosPath(why)
	if !strings.HasSuffix(got, "connect.service.ts") || !strings.Contains(got, "/protos/") {
		t.Errorf("생성물 경로를 못 뽑았다: %q", got)
	}
	if lastProtosPath("아무 경로도 없다") != "" {
		t.Error("없는데 뽑았다")
	}
}

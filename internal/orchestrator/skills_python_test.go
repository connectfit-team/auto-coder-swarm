package orchestrator

import (
	"strings"
	"testing"
)

// 파이썬 protobuf 스텁도 생성물이다 — 손으로 고치면 다음 생성 때 덮어써진다.
// `gig_ai` 에만 240개 있다(code-insight-engine#88 에서 세었다).
func TestPythonStubsAreProcedureViolations(t *testing.T) {
	for _, p := range []string{
		"gen/alarmtype_pb2.py",
		"gen/service_pb2_grpc.py",
		"gen/alarmtype_pb2.pyi",
		"gen/service_pb2_grpc.pyi",
	} {
		v := checkProcedureViolations("+++ b/" + p + "\n+x = 1\n")
		if len(v) == 0 {
			t.Errorf("생성물인데 안 물었다: %s", p)
			continue
		}
		if !strings.Contains(v[0], p) {
			t.Errorf("어느 파일인지 안 밝힌다: %s", v[0])
		}
	}
}

// 사람이 쓴 파이썬은 그대로 둔다.
func TestOrdinaryPythonIsNotAProcedureViolation(t *testing.T) {
	for _, p := range []string{
		"worker/alarm/send_push.py",
		"scripts/build.py",
		"app/models/notification.py",
	} {
		if v := checkProcedureViolations("+++ b/" + p + "\n+x = 1\n"); len(v) > 0 {
			t.Errorf("사람이 쓴 파일을 물었다: %s (%s)", p, v[0])
		}
	}
}

// 계약 원본은 생성물이 아니다 — 이것까지 물면 할 일을 못 한다.
func TestProtoSourceIsNotAViolation(t *testing.T) {
	if v := checkProcedureViolations("+++ b/ceoweb/v1/connect.service.proto\n+  rpc X(A) returns (B) {}\n"); len(v) > 0 {
		t.Fatalf("계약 원본을 물었다: %s", v[0])
	}
}

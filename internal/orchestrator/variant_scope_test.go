package orchestrator

import (
	"strings"
	"testing"
)

// 제품 이름이 코드에 박혀 있으면, 그 제품에 대한 요청이 조용히 막힌다.
func TestVariantExcludes(t *testing.T) {
	// 기본은 아무것도 안 뺀다.
	t.Setenv("SWARM_VARIANT_EXCLUDE", "")
	if got := variantExcludes("근무 유형에 상여금 추가"); len(got) != 0 {
		t.Errorf("기본값인데 뺐다: %v", got)
	}

	t.Setenv("SWARM_VARIANT_EXCLUDE", "clockio, foo")
	got := variantExcludes("근무 유형에 상여금 추가")
	if strings.Join(got, ",") != "clockio,foo" {
		t.Errorf("설정대로 안 뺐다: %v", got)
	}

	// 요청이 그 이름을 말했으면 빼지 않는다.
	got = variantExcludes("clockio 의 근무 유형에 상여금 추가")
	if strings.Join(got, ",") != "foo" {
		t.Errorf("요청이 말한 것을 뺐다: %v", got)
	}
}

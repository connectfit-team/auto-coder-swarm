package workspace

import (
	"os"
	"path/filepath"
	"sort"
)

// 모델이 준 이름은 확인하고 쓴다.
//
// 범위 추출이 "고용주웹" 을 저장소 이름으로 냈다. 그런 저장소는 없다.
// 그대로 워크트리를 만들려다 실패했는데 그 실패를 무시해서, 빈 폴더에서
// 빌드를 돌리고 "이 저장소는 작업공간에서 빌드되지 않는다 — 환경 문제다" 로
// 끝났다(W-82668). 사람이 보는 까닭이 실제 까닭과 아무 관계가 없었다.
//
// 사본 폴더가 곧 진실이다 — 워크트리를 만들 때 쓰는 것과 같은 것을 본다.

// HasRepo 는 그 이름의 사본이 실제로 있는지 본다.
func (m *LocalManager) HasRepo(name string) bool {
	if name == "" {
		return false
	}
	// 경로 조각이 섞인 이름은 이름이 아니다.
	if filepath.Base(name) != name {
		return false
	}
	st, err := os.Stat(filepath.Join(m.masterRepos, name))
	return err == nil && st.IsDir()
}

// RepoPath 는 그 저장소 사본의 경로다. 없으면 빈 문자열이다.
func (m *LocalManager) RepoPath(name string) string {
	if !m.HasRepo(name) {
		return ""
	}
	return filepath.Join(m.masterRepos, name)
}

// Repos 는 사본이 있는 저장소 이름을 준다.
func (m *LocalManager) Repos() []string {
	es, err := os.ReadDir(m.masterRepos)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range es {
		if e.IsDir() && e.Name()[0] != '.' {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

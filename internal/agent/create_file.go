package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 없는 기능을 만들려면 **없는 파일을 만들 수 있어야 한다.**
//
// 코더는 "읽고 → 고치고 → 쓴다" 로만 일했다. 그래서 계획이 새 파일을 짚으면
// 첫 줄에서 죽었다(W-44018).
//
//	failed to read file: open …/internal_v2/business/pending_connection.go:
//	no such file or directory
//
// 계획 쪽은 이미 새 파일을 허용하고 있었다 — splitByRepoReality 가 "폴더가
// 있으면 살린다" 로 두었기 때문이다. 손이 그것을 못 받았을 뿐이다.
//
// 새 파일에는 찾아 바꿀 것이 없다. 통째로 받는다. 다만 받은 것을 그대로
// 믿지 않는다 — 문법을 보고, 폴더가 실제로 있는지 보고, 덮어쓰지 않는다.

// CreateFile 은 없는 파일을 새로 만든다.
func (a *CoderAgent) CreateFile(ctx context.Context, filePath, instructions, outline string) (string, error) {
	if _, err := os.Stat(filePath); err == nil {
		return "", fmt.Errorf("%s 는 이미 있다 — 새로 만들 것이 아니다", filepath.Base(filePath))
	}
	dir := filepath.Dir(filePath)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		// 폴더까지 지어내면 저장소에 없던 구조가 생긴다.
		return "", fmt.Errorf("%s 폴더가 없다 — 새 폴더까지 만드는 것은 사람이 정한다", dir)
	}

	prompt := fmt.Sprintf(a.conventions+
		"새 파일을 하나 만든다. 아래 지시를 그대로 따르라.\n\n"+
		"지켜야 할 것:\n"+
		"1. 파일 전체를 낸다. 설명하지 마라. 코드만 낸다.\n"+
		"2. 같은 폴더의 이웃 파일과 같은 꼴로 쓴다 — 꾸러미 이름, 이름 짓는 법, 오류 다루는 법.\n"+
		"3. 없는 타입·함수를 부르지 마라. 아래 뼈대에 있는 것만 쓴다.\n"+
		"4. 새 proto 메시지·필드가 필요하면 만들지 말고 주석으로 남겨라.\n\n"+
		"[만들 파일]\n%s\n\n%s\n[이웃 파일들]\n%s\n\n[지시]\n%s\n",
		filePath, outline, neighbourList(dir), instructions)

	raw, err := CallLLM(ctx, a.llm, a.Name(), prompt)
	if err != nil {
		return "", err
	}
	content := CleanCodeOutput(raw)
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("%s: 빈 파일을 냈다", filepath.Base(filePath))
	}
	if err := CheckSyntax(filePath, content); err != nil {
		return "", err
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("새 파일을 못 썼다: %w", err)
	}
	TidyFile(ctx, filePath)
	return fmt.Sprintf("Created %s", filePath), nil
}

// neighbourList 는 같은 폴더의 파일 이름들이다. 이름 짓는 꼴을 보라고 준다.
func neighbourList(dir string) string {
	es, err := os.ReadDir(dir)
	if err != nil {
		return "(못 읽었다)"
	}
	var names []string
	for _, e := range es {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
		if len(names) >= 30 {
			break
		}
	}
	return strings.Join(names, " ")
}

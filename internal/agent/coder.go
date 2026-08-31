package agent

import (
	"context"
	"fmt"
	"google.golang.org/adk/model"
	"os"
	"path/filepath"
	"strings"
)

type CoderAgent struct {
	llm model.LLM
	// 팀의 작업 절차. 힐러가 부르는 수리 경로도 같은 규약을 받아야 해서
	// 호출 인자가 아니라 필드다 — 한 군데서 빠뜨리면 그 경로만 규약 없이 돈다.
	conventions string
}

// SetConventions 는 이 작업에 적용할 절차를 심는다.
func (a *CoderAgent) SetConventions(s string) { a.conventions = s }

func NewCoderAgent(m model.LLM) *CoderAgent {
	return &CoderAgent{llm: m}
}

func (a *CoderAgent) Name() string {
	return "Coder"
}

func (a *CoderAgent) BuildRepairPrompt(filePath, content, instructions, buildError string) string {
	return fmt.Sprintf(a.conventions+"You are the Swarm Debugging Expert.\n"+
		"The previous attempt to modify the code caused a BUILD ERROR.\n"+
		"Your goal is to fix the code to resolve the error while still fulfilling the original instructions.\n\n"+
		"MANDATORY RULES:\n"+
		"1. Provide the FULL file content after repair.\n"+
		"2. Do not include any conversational text, ONLY the code.\n"+
		"3. Focus specifically on fixing the mentioned build error.\n\n"+
		"[Original Instructions]\n%s\n\n"+
		"[Build Error Output]\n%s\n\n"+
		"[Current Code State: %s]\n%s", instructions, buildError, filePath, content)
}

func (a *CoderAgent) RepairFile(ctx context.Context, filePath, instructions, buildError string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	prompt := a.BuildRepairPrompt(filePath, string(content), instructions, buildError)
	raw, err := CallLLM(ctx, a.llm, "Debugger", prompt)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filePath, []byte(CleanCodeOutput(raw)), 0644); err != nil {
		return "", err
	}
	TidyFile(ctx, filePath)
	return fmt.Sprintf("Repaired %s", filePath), nil
}

func (a *CoderAgent) ModifyFile(ctx context.Context, filePath string, instructions string) (string, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	original := string(raw)

	var updated string
	if len([]rune(original)) > wholeFileRewriteLimit {
		updated, err = a.editByBlocks(ctx, filePath, original, instructions)
	} else {
		updated, err = a.rewriteWholeFile(ctx, filePath, original, instructions)
	}
	if err != nil {
		return "", err
	}

	// **내주던 이름이 사라지면 쓰지 않는다.**
	//
	// 통짜로 다시 쓰게 하면 모델이 조용히 함수를 빠뜨린다. 실측으로
	// attendance.ts 에서 getAttendanceRecords 가 사라져 빌드가 깨졌고,
	// 자가 치유 세 번을 태운 뒤에야 드러났다. 여기서 막고 이유를 말한다.
	if lost := lostExports(original, updated); len(lost) > 0 {
		return "", fmt.Errorf("%s: 원래 있던 export 가 사라졌다 — %s. 고칠 줄만 바꾸고 나머지는 그대로 둬라",
			filepath.Base(filePath), strings.Join(lost, ", "))
	}

	if err := os.WriteFile(filePath, []byte(updated), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	// import 는 기계가 고친다. 모델에게 세 번 더 물어볼 일이 아니다.
	TidyFile(ctx, filePath)

	return fmt.Sprintf("Modified %s", filePath), nil
}

// rewriteWholeFile 은 작은 파일을 통째로 다시 받는다.
func (a *CoderAgent) rewriteWholeFile(ctx context.Context, filePath, original, instructions string) (string, error) {
	prompt := fmt.Sprintf(a.conventions+"You are the Swarm Coder.\n"+
		"Modify the following code based on the technical instructions.\n\n"+
		"MANDATORY RULES:\n"+
		"1. Provide the FULL file content after modification.\n"+
		"2. Do not include any conversational text, ONLY the code.\n"+
		"3. Maintain existing coding style.\n"+
		"4. Keep every existing export. Do not drop functions you were not asked to change.\n\n"+
		"[Technical Instructions]\n%s\n\n"+
		"[Original Code: %s]\n%s", instructions, filePath, original)

	raw, err := CallLLM(ctx, a.llm, a.Name(), prompt)
	if err != nil {
		return "", err
	}
	out := CleanCodeOutput(raw)
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("%s: 모델이 빈 내용을 냈다", filepath.Base(filePath))
	}
	return out, nil
}

// editByBlocks 는 큰 파일을 **고칠 자리만** 받아서 고친다.
//
// 창이 8,192 토큰이라 536줄짜리 파일을 넣고 다시 전부 출력하게 하면 반드시
// 잘린다. 원본은 읽히기만 하면 되므로 입력에는 들어가고, 출력은 바꿀 자리만
// 받으면 짧다. 어디를 고치는지도 기계로 확인된다.
func (a *CoderAgent) editByBlocks(ctx context.Context, filePath, original, instructions string) (string, error) {
	// **볼 것을 줄인다.**
	//
	// 426줄을 통째로 읽히고 "고칠 자리만 내라" 고 했더니 형식을 무시하고
	// 잘린 파일을 냈다. 지시문에 나온 이름이 있는 줄 둘레만 보여 주면
	// 볼 것이 줄고 어디를 고칠지가 분명해진다.
	shown, note := original, "[Original Code: "+filePath+"]"
	if r := relevantRegions(original, instructions); r != "" {
		shown = r
		note = "[관련 부분만 보여 준다. 앞의 숫자는 줄 번호이니 SEARCH 에는 빼고 적어라: " + filePath + "]"
	}

	// **형식 요구를 맨 끝에 둔다.**
	//
	// 앞에 두었더니 9B 가 코드를 읽는 동안 잊고 코드 리뷰 에세이를 썼다.
	// 모델은 끝에 있는 말을 더 잘 지킨다. 시작 글자까지 못 박는다.
	prompt := fmt.Sprintf("%s%s\n%s\n\n[고쳐야 할 것]\n%s\n\n%s",
		a.conventions,
		note, shown, instructions, editBlockRules)

	raw, err := CallLLM(ctx, a.llm, a.Name(), prompt)
	if err != nil {
		return "", err
	}

	// 형식을 안 지키면 짧게 한 번 더 묻는다. 긴 프롬프트에서 잊은 것뿐인 경우가 많다.
	if !editBlockRe.MatchString(raw) {
		retry := fmt.Sprintf("%s\n%s\n\n[고쳐야 할 것]\n%s\n\n%s",
			note, shown, instructions,
			"설명하지 마라. 아래 형식만 내라. 첫 글자는 < 여야 한다.\n"+editBlockFormat)
		if r2, e2 := CallLLM(ctx, a.llm, a.Name(), retry); e2 == nil && editBlockRe.MatchString(r2) {
			raw = r2
		}
	}

	updated, err := applyEditBlocks(original, raw)
	if err == nil {
		return updated, nil
	}

	// **블록을 안 내면 통짜로라도 받아 본다.**
	//
	// 형식을 못 지켰다고 바로 버리면 시도 한 번이 그냥 날아간다. 통짜로 온
	// 것이 원본만큼 길고 내주던 이름이 다 남아 있으면 쓸 수 있다 — 그
	// 판단은 부르는 쪽의 lostExports 가 한다.
	if whole := CleanCodeOutput(raw); shown == original && looksLikeWholeFile(original, whole) {
		return whole, nil
	}
	return "", fmt.Errorf("%s: %w", filepath.Base(filePath), err)
}

func (a *CoderAgent) GenerateTestFile(ctx context.Context, sourcePath string) (string, error) {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", err
	}

	ext := filepath.Ext(sourcePath)
	testPath := strings.TrimSuffix(sourcePath, ext)
	switch ext {
	case ".go":
		testPath += "_test.go"
	case ".ts", ".tsx":
		testPath += ".spec" + ext
	case ".dart":
		testPath += "_test.dart"
	default:
		return "", fmt.Errorf("unsupported extension for test generation: %s", ext)
	}

	prompt := fmt.Sprintf(a.conventions+"You are the Swarm Test Engineer.\n"+
		"Create a comprehensive unit test for the following source code.\n\n"+
		"MANDATORY RULES:\n"+
		"1. Provide the FULL test file content.\n"+
		"2. Do not include any conversational text, ONLY the code.\n"+
		"3. Ensure high coverage and test edge cases.\n\n"+
		"[Source Code: %s]\n%s", sourcePath, string(content))

	testCode, err := CallLLM(ctx, a.llm, "Tester", prompt)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(testPath, []byte(CleanCodeOutput(testCode)), 0644); err != nil {
		return "", err
	}
	TidyFile(ctx, testPath)

	return testPath, nil
}

func (a *CoderAgent) Process(ctx context.Context, input string) (string, error) {
	return "Use ModifyFile or GenerateTestFile", nil
}

// looksLikeWholeFile 은 통짜로 다시 쓴 결과로 볼 만한지 본다.
//
// 잘려 온 조각을 파일로 덮어쓰면 안 된다. 원본의 8할은 되어야 한다.
func looksLikeWholeFile(original, candidate string) bool {
	c := strings.TrimSpace(candidate)
	if c == "" {
		return false
	}
	return float64(len([]rune(c))) >= 0.8*float64(len([]rune(original)))
}

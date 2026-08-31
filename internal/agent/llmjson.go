package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"

	"google.golang.org/adk/model"
)

// jsonRetries 는 형식이 틀렸을 때 다시 물어보는 횟수다.
//
// 로컬 9B 모델은 "MANDATORY JSON FORMAT" 이라고 적어 줘도 마크다운 제목이나
// 설명 문단을 먼저 낸다. 한 번 실패했다고 작업을 죽이면, 모델이 흔들린 것과
// 진짜로 못 하는 것이 구별되지 않는다.
const jsonRetries = 3

const stricterJSON = "\n\n[출력 형식] 설명·인사·마크다운 없이 **JSON 객체 하나만** 출력하라. " +
	"```json 같은 펜스도 붙이지 마라. 첫 글자는 `{` 여야 한다."

// CallLLMJSON 은 JSON 을 받아 out 에 담는다. 형식이 틀리면 더 강하게 다시 묻는다.
//
// **모델의 규율에 기대지 않는다.** 실측으로 같은 프롬프트에 같은 모델이 어떤
// 때는 JSON 을, 어떤 때는 산문을 냈다. 형식은 부르는 쪽이 지켜 낸다.
//
// 마지막 원문을 함께 돌려준다 — 끝내 실패했을 때 무엇을 뱉었는지 봐야
// 프롬프트 문제인지 모델 문제인지 가릴 수 있다.
func CallLLMJSON(ctx context.Context, m model.LLM, name, prompt string, out any) (string, error) {
	return callJSONWith(ctx, prompt, out, name, func(p string) (string, error) {
		return CallLLM(ctx, m, name, p)
	})
}

// callJSONWith 는 형식만 책임진다. 호출을 함수로 받아 LLM 없이도 잴 수 있게 한다.
func callJSONWith(ctx context.Context, prompt string, out any, name string, call func(string) (string, error)) (string, error) {
	var raw string
	var lastErr error

	for i := 0; i < jsonRetries; i++ {
		p := prompt
		if i > 0 {
			p = prompt + stricterJSON
		}

		var err error
		if raw, err = call(p); err != nil {
			return raw, err // LLM 자체가 안 되는 것은 다시 물어도 같다
		}
		if err = unmarshalMaybeWrapped([]byte(ExtractJSON(raw)), out); err == nil {
			return raw, nil
		}
		lastErr = err
		log.Printf("⚠️ [%s] JSON 형식 실패 (%d/%d): %v", name, i+1, jsonRetries, err)

		if ctx.Err() != nil {
			return raw, ctx.Err()
		}
	}
	return raw, fmt.Errorf("%d회 시도했지만 JSON 이 아니다: %w", jsonRetries, lastErr)
}

// 모델이 답을 봉투에 넣어 보내는 일이 있다.
//
//	{"response": {"total_files": 3, "is_feasible": true, ...}}
//
// 이게 **오류 없이** 통과한다 — JSON 은 모르는 키를 그냥 무시하므로,
// 언마샬은 성공하고 구조체는 전 필드가 zero value 로 남는다. 그러면
// is_feasible 이 false 가 되어 상위에서 "작업 규모 과다" 로 죽는다.
// 모델은 제대로 답했는데 판단해 보지도 못하고 실패한 것이다(실측).
//
// 그래서 채워진 것이 하나도 없으면 봉투를 의심한다. 최상위 키가 하나이고
// 그 값이 객체면 한 겹 벗겨 다시 넣어 본다.
var envelopeKeys = map[string]bool{
	"response": true, "result": true, "data": true, "output": true,
	"answer": true, "content": true, "json": true,
}

func unmarshalMaybeWrapped(b []byte, out any) error {
	if err := json.Unmarshal(b, out); err != nil {
		return err
	}
	if !isZeroStruct(out) {
		return nil
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(b, &probe); err != nil {
		return nil // 객체가 아니면 벗길 것도 없다
	}
	for k, v := range probe {
		if len(probe) > 1 && !envelopeKeys[strings.ToLower(k)] {
			continue
		}
		inner := bytes.TrimSpace(v)
		if len(inner) == 0 || inner[0] != '{' {
			continue
		}
		if err := json.Unmarshal(inner, out); err == nil && !isZeroStruct(out) {
			log.Printf("[JSON] 봉투 %q 를 벗겨 읽었다", k)
			return nil
		}
	}
	return nil
}

// isZeroStruct 는 채워진 필드가 하나도 없는지 본다.
//
// 다시 언마샬해도 안전하도록, 여기서 판단만 하고 값은 건드리지 않는다.
func isZeroStruct(out any) bool {
	v := reflect.ValueOf(out)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false // 맵·슬라이스는 이 판단을 하지 않는다
	}
	return v.IsZero()
}

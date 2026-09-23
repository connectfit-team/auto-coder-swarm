package agent

import "errors"

// editBlockProblem 은 「고칠 자리를 못 찾았다」 류의 실패다.
//
// 이것을 「그 파일은 못 고친다」 와 섞으면 안 된다. 섞으면 되먹임이
// "고칠 수 있는 파일만 골라 다시 계획하라" 가 되어, 정작 고칠 수 있는 파일을
// 버린다. 자리를 못 찾은 것은 SEARCH 를 다시 적을 일이다.
type editBlockProblem struct{ err error }

func (e *editBlockProblem) Error() string { return e.err.Error() }
func (e *editBlockProblem) Unwrap() error { return e.err }

// IsEditBlockProblem 은 그 실패가 찾아바꾸기 자리 문제인지 알려준다.
func IsEditBlockProblem(err error) bool {
	var p *editBlockProblem
	return errors.As(err, &p)
}

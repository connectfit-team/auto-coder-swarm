package storage

import "time"

// 도는 도중에 사람이 방향을 더하는 길.
//
// 이미 나간 것은 되돌리지 않는다 — 열린 PR 은 열린 채로 남고, 시작된 검사는
// 멈추지 않는다. 다음 걸음부터 반영한다. 그러니 "받았다" 와 "반영했다" 는
// 다른 말이고, 화면에도 그렇게 적어야 한다.

// AddSteer 는 지시를 큐에 넣는다.
func (s *Storage) AddSteer(taskID, message string) (*TaskSteer, error) {
	st := &TaskSteer{TaskID: taskID, Message: message, CreatedAt: time.Now()}
	if err := s.DB.Create(st).Error; err != nil {
		return nil, err
	}
	return st, nil
}

// PendingSteers 는 아직 집어 가지 않은 지시를 오래된 것부터 준다.
func (s *Storage) PendingSteers(taskID string) ([]TaskSteer, error) {
	var out []TaskSteer
	err := s.DB.Where("task_id = ? AND applied_at IS NULL", taskID).
		Order("id asc").Find(&out).Error
	return out, err
}

// MarkSteersApplied 는 집어 간 것을 표시한다.
// 집어 갔다는 것이 그대로 반영했다는 뜻은 아니다 — 무엇을 했는지는 로그에 있다.
func (s *Storage) MarkSteersApplied(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now()
	return s.DB.Model(&TaskSteer{}).Where("id IN ?", ids).
		Update("applied_at", &now).Error
}

// CountSteers 는 그 작업에 들어온 지시 수를 준다. 화면이 "몇 개 받았다" 를 적는다.
func (s *Storage) CountSteers(taskID string) (int64, error) {
	var n int64
	err := s.DB.Model(&TaskSteer{}).Where("task_id = ?", taskID).Count(&n).Error
	return n, err
}

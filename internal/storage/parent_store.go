package storage

// 한 요청이 저장소 여럿에 걸치면 저장소마다 작업이 하나씩 생긴다.
// 그것들을 묶어 보여 주려면 뿌리 작업을 기억해야 한다.

// SetParentTask 는 이 작업을 낳은 작업을 적어 둔다.
func (s *Storage) SetParentTask(taskID, parentID string) error {
	if taskID == "" || parentID == "" || taskID == parentID {
		return nil
	}
	return s.DB.Model(&SwarmTask{}).Where("id = ?", taskID).
		Update("parent_task_id", parentID).Error
}

// ChildTasks 는 그 작업이 낳은 작업들이다. 만든 순서로 준다.
func (s *Storage) ChildTasks(parentID string) ([]SwarmTask, error) {
	var out []SwarmTask
	if parentID == "" {
		return out, nil
	}
	err := s.DB.Where("parent_task_id = ?", parentID).
		Order("created_at asc").Find(&out).Error
	return out, err
}

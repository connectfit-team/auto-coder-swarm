package storage

import (
	"time"

	"gorm.io/gorm"
)

type TaskStatus string

const (
	StatusPending         TaskStatus = "PENDING"
	StatusRunning         TaskStatus = "RUNNING"
	StatusWaitingApproval TaskStatus = "WAITING_APPROVAL"
	StatusApproved        TaskStatus = "APPROVED"
	StatusCompleted       TaskStatus = "COMPLETED"
	StatusFailed          TaskStatus = "FAILED"
	StatusCancelled       TaskStatus = "CANCELLED"
)

type SwarmTask struct {
	// 워커가 집을 때의 상태. DB 에 저장하지 않는다 — 집는 순간 status 는
	// RUNNING 으로 덮이므로, 그 전에 APPROVED 였는지를 여기 담아 둔다.
	ClaimedStatus TaskStatus `gorm:"-"`

	ID          string `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	UserRequest string         `gorm:"type:text"`
	RepoName    string         `gorm:"index"`
	Status      TaskStatus     `gorm:"index"`
	Result      string         `gorm:"type:text"`
	// 만들어진 PR 주소. Result 에도 같은 값이 들어가지만, 화면이 "PR 열기"
	// 단추를 띄우려면 이름 있는 자리가 필요하다 — Result 는 성공 문구가 올
	// 수도 있는 자리라 그것만 보고 링크를 만들 수 없다.
	PRURL         string `gorm:"column:pr_url;type:text"`
	ErrorLog      string `gorm:"type:text"`
	ProposedDiff  string `gorm:"type:text"`
	HumanFeedback string `gorm:"type:text"`
	ContextState  string `gorm:"type:text"`
	CIEWorkID     string `gorm:"index"`
	// 이 작업을 낳은 작업. 한 요청이 저장소 여럿에 걸치면 저장소마다 작업이
	// 하나씩 생기는데, 그것들을 묶어 보여 주려면 이름이 필요하다.
	// 화면이 "PR 열기" 단추를 하나만 띄우던 까닭이 이것이었다 — 작업 하나에
	// PR 하나이고, 형제 작업이 어디 있는지 아무도 몰랐다.
	ParentTaskID string `gorm:"index"`
}

// TaskSteer 는 작업이 도는 도중에 사람이 보낸 지시다.
//
// **DB 에 남긴다.** 이 기계는 하루 세 번 다시 뜨고 작업은 몇 분씩 돈다 —
// 메모리에만 두면 그 사이에 사라진다. 사라진 지시는 없는 것과 같은데, 사람은
// 말했다고 여긴다. 그것이 가장 나쁘다.
//
// AppliedAt 은 흐름이 그것을 집어 간 시각이다. 집어 갔다는 것이 "그대로
// 반영했다" 는 뜻은 아니다 — 무엇을 했는지는 로그에 적는다.
type TaskSteer struct {
	ID        uint   `gorm:"primaryKey"`
	TaskID    string `gorm:"index"`
	Message   string `gorm:"type:text"`
	CreatedAt time.Time
	AppliedAt *time.Time
}

type RepoLock struct {
	RepoName string `gorm:"primaryKey"`
	LockedAt time.Time
	TaskID   string
}

type TaskLog struct {
	ID        uint   `gorm:"primaryKey"`
	TaskID    string `gorm:"index"`
	Stage     string
	Message   string `gorm:"type:text"`
	Prompt    string `gorm:"type:text"`
	Summary   string `gorm:"type:text"`
	CreatedAt time.Time
}

type ThoughtLog struct {
	ID        uint   `gorm:"primaryKey"`
	TaskID    string `gorm:"index"`
	AgentName string
	Message   string `gorm:"type:text"`
	CreatedAt time.Time
}

type Setting struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"type:text"`
}

package storage

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Storage struct {
	DB *gorm.DB
}

func NewStorage(dbPath string) (*Storage, error) {
	db, err := gorm.Open(sqlite.Open(dbPath+"?_pragma=busy_timeout(5000)"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&SwarmTask{}, &RepoLock{}, &TaskLog{}, &ThoughtLog{}, &Setting{})
	return &Storage{DB: db}, nil
}

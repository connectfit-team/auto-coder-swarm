package storage

import (
	"log"
	"sync"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Storage struct {
	DB           *gorm.DB
	thoughtQueue chan *ThoughtLog
	cache        map[string]string
	cacheMu      sync.RWMutex
}

func NewStorage(dbPath string) (*Storage, error) {
	// [Performance] Enable WAL and set high busy timeout
	db, err := gorm.Open(sqlite.Open(dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&SwarmTask{}, &RepoLock{}, &TaskLog{}, &ThoughtLog{}, &Setting{})

	s := &Storage{
		DB:           db,
		thoughtQueue: make(chan *ThoughtLog, 1000), // Buffer for async writes
		cache:        make(map[string]string),
	}

	// Start background log writer
	go s.processLogQueue()

	return s, nil
}

func (s *Storage) processLogQueue() {
	log.Println("📥 [Storage] Async log writer started")
	for t := range s.thoughtQueue {
		// Bulk writing could be implemented here later for even higher throughput
		if err := s.DB.Create(t).Error; err != nil {
			log.Printf("⚠️ [Storage] Async write failed: %v", err)
		}
	}
}

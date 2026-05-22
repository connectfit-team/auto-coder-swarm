package storage

import (
	"log"
	"os"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
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
	var dialector gorm.Dialector
	dsn := os.Getenv("DATABASE_DSN")

	if dsn != "" {
		log.Println("🗄️ [Storage] Connecting to MariaDB/MySQL...")
		dialector = mysql.Open(dsn)
	} else {
		log.Println("🗄️ [Storage] Connecting to SQLite (Fallback)...")
		// Use standard sqlite dialector for broad compatibility
		dialector = sqlite.Open(dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&SwarmTask{}, &RepoLock{}, &TaskLog{}, &ThoughtLog{}, &Setting{})

	s := &Storage{
		DB:           db,
		thoughtQueue: make(chan *ThoughtLog, 1000),
		cache:        make(map[string]string),
	}

	go s.processLogQueue()

	return s, nil
}

func (s *Storage) processLogQueue() {
	log.Println("📥 [Storage] Async log writer started")
	for t := range s.thoughtQueue {
		if err := s.DB.Create(t).Error; err != nil {
			log.Printf("⚠️ [Storage] Async write failed: %v", err)
		}
	}
}

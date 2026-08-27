package storage

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Storage struct {
	DB           *gorm.DB
	thoughtQueue chan *ThoughtLog
	rdb          *redis.Client
}

func NewStorage(dbPath string, rdb *redis.Client) (*Storage, error) {
	var dialector gorm.Dialector
	dsn := os.Getenv("DATABASE_DSN")

	if dsn != "" {
		log.Println("🗄️ [Storage] Connecting to MariaDB/MySQL...")
		dialector = mysql.Open(dsn)
	} else {
		log.Println("🗄️ [Storage] Connecting to SQLite (Fallback)...")
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
		rdb:          rdb,
	}

	go s.processLogQueue()

	return s, nil
}

func (s *Storage) GetSetting(key string) string {
	if s.rdb != nil {
		val, err := s.rdb.Get(context.Background(), "setting:"+key).Result()
		if err == nil {
			return val
		}
	}

	var setting Setting
	if err := s.DB.Where("key = ?", key).First(&setting).Error; err != nil {
		return ""
	}

	if s.rdb != nil {
		s.rdb.Set(context.Background(), "setting:"+key, setting.Value, 1*time.Hour)
	}
	return setting.Value
}

func (s *Storage) SaveSetting(key, value string) error {
	err := s.DB.Save(&Setting{Key: key, Value: value}).Error
	if err == nil && s.rdb != nil {
		s.rdb.Set(context.Background(), "setting:"+key, value, 1*time.Hour)
		s.rdb.Publish(context.Background(), "system.settings.updated", key)
	}
	return err
}

func (s *Storage) processLogQueue() {
	log.Println("📥 [Storage] Async log writer started")
	for t := range s.thoughtQueue {
		if err := s.DB.Create(t).Error; err != nil {
			log.Printf("⚠️ [Storage] Async write failed: %v", err)
		}
	}
}

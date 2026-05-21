package storage

func (s *Storage) GetSetting(key string) string {
	var setting Setting
	if err := s.DB.Where("key = ?", key).First(&setting).Error; err != nil {
		return ""
	}
	return setting.Value
}

func (s *Storage) SaveSetting(key, value string) error {
	return s.DB.Save(&Setting{Key: key, Value: value}).Error
}

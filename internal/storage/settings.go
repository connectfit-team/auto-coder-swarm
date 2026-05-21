package storage

func (s *Storage) GetSetting(key string) string {
	s.cacheMu.RLock()
	val, ok := s.cache[key]
	s.cacheMu.RUnlock()
	if ok {
		return val
	}

	var setting Setting
	if err := s.DB.Where("key = ?", key).First(&setting).Error; err != nil {
		return ""
	}

	// Update cache
	s.cacheMu.Lock()
	s.cache[key] = setting.Value
	s.cacheMu.Unlock()

	return setting.Value
}

func (s *Storage) SaveSetting(key, value string) error {
	err := s.DB.Save(&Setting{Key: key, Value: value}).Error
	if err == nil {
		s.cacheMu.Lock()
		s.cache[key] = value
		s.cacheMu.Unlock()
	}
	return err
}

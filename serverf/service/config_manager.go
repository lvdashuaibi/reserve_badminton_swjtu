package service

import (
	"log"
	db "serverf/midware"
	"sync"
	"time"
)

// ReservationConfig 预订配置
type ReservationConfig struct {
	ReservationHour     int           `json:"reservationHour"`     // 预订时间-小时（0-23）
	ReservationMinute   int           `json:"reservationMinute"`   // 预订时间-分钟（0-59）
	ReservationSecond   int           `json:"reservationSecond"`   // 预订时间-秒（0-59）
	ReservationMillisec int           `json:"reservationMillisec"` // 预订时间-毫秒（0-999）
	RetryInterval       time.Duration `json:"retryInterval"`       // 重试间隔（分钟）
	RetryDuration       time.Duration `json:"retryDuration"`       // 重试持续时间（小时）
}

// ConfigManager 配置管理器
type ConfigManager struct {
	config ReservationConfig
	mu     sync.RWMutex
}

var (
	configManager     *ConfigManager
	configManagerOnce sync.Once
)

// GetConfigManager 获取配置管理器单例
func GetConfigManager() *ConfigManager {
	configManagerOnce.Do(func() {
		configManager = &ConfigManager{}
		// 从数据库加载配置
		if err := configManager.loadFromDB(); err != nil {
			log.Printf("从数据库加载配置失败，使用默认配置: %v", err)
			// 使用默认配置
			configManager.config = ReservationConfig{
				ReservationHour:     22,
				ReservationMinute:   30,
				ReservationSecond:   0,
				ReservationMillisec: 0,
				RetryInterval:       5 * time.Minute,
				RetryDuration:       24 * time.Hour,
			}
		}
	})
	return configManager
}

// loadFromDB 从数据库加载配置
func (cm *ConfigManager) loadFromDB() error {
	dbConfig, err := db.GetSystemConfig()
	if err != nil {
		return err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config = ReservationConfig{
		ReservationHour:     dbConfig.ReservationHour,
		ReservationMinute:   dbConfig.ReservationMinute,
		ReservationSecond:   dbConfig.ReservationSecond,
		ReservationMillisec: dbConfig.ReservationMillisec,
		RetryInterval:       time.Duration(dbConfig.RetryIntervalMinutes) * time.Minute,
		RetryDuration:       time.Duration(dbConfig.RetryDurationHours) * time.Hour,
	}

	log.Printf("已从数据库加载配置: %02d:%02d:%02d.%03d, 重试间隔:%d分钟, 重试时长:%d小时",
		cm.config.ReservationHour, cm.config.ReservationMinute,
		cm.config.ReservationSecond, cm.config.ReservationMillisec,
		dbConfig.RetryIntervalMinutes, dbConfig.RetryDurationHours)

	return nil
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig() ReservationConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// UpdateConfig 更新配置
func (cm *ConfigManager) UpdateConfig(hour, minute, second, millisec int, retryIntervalMinutes, retryDurationHours int) error {
	// 验证参数
	if hour < 0 || hour > 23 {
		hour = 22
	}
	if minute < 0 || minute > 59 {
		minute = 30
	}
	if second < 0 || second > 59 {
		second = 0
	}
	if millisec < 0 || millisec > 999 {
		millisec = 0
	}
	if retryIntervalMinutes < 1 {
		retryIntervalMinutes = 5
	}
	if retryDurationHours < 1 {
		retryDurationHours = 24
	}

	// 更新内存配置
	cm.mu.Lock()
	cm.config.ReservationHour = hour
	cm.config.ReservationMinute = minute
	cm.config.ReservationSecond = second
	cm.config.ReservationMillisec = millisec
	cm.config.RetryInterval = time.Duration(retryIntervalMinutes) * time.Minute
	cm.config.RetryDuration = time.Duration(retryDurationHours) * time.Hour
	cm.mu.Unlock()

	// 保存到数据库
	dbConfig := &db.SystemConfig{
		ReservationHour:      hour,
		ReservationMinute:    minute,
		ReservationSecond:    second,
		ReservationMillisec:  millisec,
		RetryIntervalMinutes: retryIntervalMinutes,
		RetryDurationHours:   retryDurationHours,
	}

	return db.UpdateSystemConfig(dbConfig)
}

// GetReservationTime 获取预订时间（小时、分钟、秒、毫秒）
func (cm *ConfigManager) GetReservationTime() (int, int, int, int) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config.ReservationHour, cm.config.ReservationMinute,
		cm.config.ReservationSecond, cm.config.ReservationMillisec
}

// GetRetryInterval 获取重试间隔
func (cm *ConfigManager) GetRetryInterval() time.Duration {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config.RetryInterval
}

// GetRetryDuration 获取重试持续时间
func (cm *ConfigManager) GetRetryDuration() time.Duration {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config.RetryDuration
}

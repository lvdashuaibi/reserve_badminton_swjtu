package db

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type User struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"unique"`
	Password    string `gorm:"size:255"`
	Token       string
	Email       string `gorm:"email"`
	RequestTime int    `json:"request_time" gorm:"request_time"`
}

// SystemConfig 系统配置表
type SystemConfig struct {
	ID                   uint `gorm:"primaryKey;autoIncrement"`
	ReservationHour      int  `gorm:"default:22"` // 预订时间-小时
	ReservationMinute    int  `gorm:"default:30"` // 预订时间-分钟
	ReservationSecond    int  `gorm:"default:0"`  // 预订时间-秒
	ReservationMillisec  int  `gorm:"default:0"`  // 预订时间-毫秒
	RetryIntervalMinutes int  `gorm:"default:5"`  // 重试间隔（分钟）
	RetryDurationHours   int  `gorm:"default:24"` // 重试持续时间（小时）
}

// RecurringReservation 定期预订配置表
type RecurringReservation struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	Username  string `gorm:"index"`        // 用户名
	Name      string `gorm:"size:100"`     // 配置名称
	Weekday   int    `gorm:"default:0"`    // 星期几（0=周日, 1=周一, ..., 6=周六）
	CourtNums string `gorm:"size:100"`     // 场地号列表，逗号分隔，如"7,8,9"
	TimeSlots string `gorm:"size:200"`     // 时间段列表，逗号分隔，如"19:00:00,20:00:00,21:00:00"
	Enabled   bool   `gorm:"default:true"` // 是否启用
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DatabaseConfig 用来存储数据库配置信息
type DatabaseConfig struct {
	Username  string
	Password  string
	Host      string
	Port      int
	DBName    string
	Charset   string
	ParseTime bool
	Loc       string
}

var (
	db     *gorm.DB
	dbOnce sync.Once
)

func loadConfig() (*DatabaseConfig, error) {
	// 读取环境变量中的密码，如果未设置则使用空密码
	password := os.Getenv("DB_PASSWORD")
	// 允许空密码，这在本地开发环境中很常见

	// 配置数据库连接信息
	config := &DatabaseConfig{
		Username:  "root",
		Password:  password,
		Host:      "127.0.0.1",
		Port:      3306,
		DBName:    "badminton",
		Charset:   "utf8mb4",
		ParseTime: true,
		Loc:       "Local",
	}

	return config, nil
}

func init() {
	openDB()
}

func openDB() {
	// 从配置文件加载数据库配置信息
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
		return
	}

	// 使用配置文件中的信息构建DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		config.Username, config.Password, config.Host, config.Port, config.DBName, config.Charset, config.ParseTime, config.Loc)

	// 打开数据库连接
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("连接数据库失败: %v", err)
		return
	}

	// 自动迁移
	err = db.AutoMigrate(&User{}, &SystemConfig{}, &RecurringReservation{})
	if err != nil {
		log.Printf("数据库自动迁移失败%v\n", err)
		return
	}

	// 初始化系统配置（如果不存在）
	var count int64
	db.Model(&SystemConfig{}).Count(&count)
	if count == 0 {
		defaultConfig := SystemConfig{
			ReservationHour:      22,
			ReservationMinute:    30,
			ReservationSecond:    0,
			ReservationMillisec:  0,
			RetryIntervalMinutes: 5,
			RetryDurationHours:   24,
		}
		db.Create(&defaultConfig)
		log.Println("已创建默认系统配置")
	}

	//fmt.Println("数据库连接成功")
}

func GetDb() *gorm.DB {
	dbOnce.Do(openDB)
	return db
}

func CloseDb() {
	if db != nil {
		sqlDb, err := db.DB()
		if err != nil {
			panic("获取数据库连接失败，错误：" + err.Error())
		}
		sqlDb.Close()
	}
}

func GetPassWordByUserName(username string) (string, error) {
	db := GetDb()
	user := User{}
	err := db.Where("Name = ?", username).First(&user).Error
	if err != nil {
		log.Printf("用户不存在a：%v", err.Error())
		return "", err
	}
	return user.Password, nil
}

func GetTokenByUserName(username string) (string, error) {
	db := GetDb()
	user := User{}
	err := db.Where("Name = ?", username).First(&user).Error
	if err != nil {
		log.Printf("用户不存在：%v", err.Error())
		return "", err
	}
	return user.Token, nil
}

func InsertUser(userName, password, token string) (User, error) {
	user := User{
		Name:     userName,
		Password: password,
		Token:    token,
	}
	result := db.Model(&User{}).Create(&user)
	if result.Error != nil {
		log.Printf("创建用户失败")
		return User{}, result.Error
	}

	return user, nil
}

func SetToken(username, newToken string) error {
	err := db.Model(&User{}).Where("Name = ?", username).Update("Token", newToken).Error
	return err
}

func ChangePassword(username, newPassword string) error {
	err := db.Model(&User{}).Where("Name = ?", username).Updates(map[string]interface{}{"password": newPassword}).Error
	return err
}

func FindUsername(username string) bool {
	err := db.Model(&User{}).Where("Name = ?", username)
	return err == nil
}

func GetEmailByUserName(username string) string {
	// 定义一个 User 结构体实例
	user := &User{}

	// 查询数据库，根据用户名获取用户记录
	result := db.Where("name = ?", username).First(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// 如果找不到记录，返回空字符串或根据需求返回默认值
			return ""
		}
		// 如果是其他错误，打印日志并返回空字符串
		log.Println("Error querying user:", result.Error)
		return ""
	}

	// 返回用户的邮箱
	return user.Email
}

func GetReqTimeByUserName(username string) (int, error) {
	// 定义一个 User 结构体实例
	user := &User{}

	// 查询数据库，根据用户名获取用户记录
	result := db.Where("name = ?", username).First(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// 如果找不到记录，返回空字符串或根据需求返回默认值
			return 0, errors.New("找不到记录")
		}
		// 如果是其他错误，打印日志并返回空字符串
		log.Println("Error querying user:", result.Error)
		return 0, result.Error
	}

	// 返回用户的邮箱
	return user.RequestTime, nil
}

// GetSystemConfig 获取系统配置
func GetSystemConfig() (*SystemConfig, error) {
	config := &SystemConfig{}
	result := db.First(config)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// 如果没有配置，创建默认配置
			config = &SystemConfig{
				ReservationHour:      22,
				ReservationMinute:    30,
				ReservationSecond:    0,
				ReservationMillisec:  0,
				RetryIntervalMinutes: 5,
				RetryDurationHours:   24,
			}
			db.Create(config)
			return config, nil
		}
		return nil, result.Error
	}
	return config, nil
}

// UpdateSystemConfig 更新系统配置
func UpdateSystemConfig(config *SystemConfig) error {
	// 获取第一条配置记录
	var existingConfig SystemConfig
	result := db.First(&existingConfig)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// 如果不存在，创建新配置
			return db.Create(config).Error
		}
		return result.Error
	}

	// 更新现有配置
	return db.Model(&existingConfig).Updates(config).Error
}

// GetRecurringReservations 获取用户的定期预订配置
func GetRecurringReservations(username string) ([]RecurringReservation, error) {
	var reservations []RecurringReservation
	result := db.Where("username = ?", username).Find(&reservations)
	return reservations, result.Error
}

// GetAllRecurringReservations 获取所有定期预订配置（管理员）
func GetAllRecurringReservations() ([]RecurringReservation, error) {
	var reservations []RecurringReservation
	result := db.Find(&reservations)
	return reservations, result.Error
}

// GetEnabledRecurringReservations 获取所有启用的定期预订配置
func GetEnabledRecurringReservations() ([]RecurringReservation, error) {
	var reservations []RecurringReservation
	result := db.Where("enabled = ?", true).Find(&reservations)
	return reservations, result.Error
}

// CreateRecurringReservation 创建定期预订配置
func CreateRecurringReservation(reservation *RecurringReservation) error {
	return db.Create(reservation).Error
}

// UpdateRecurringReservation 更新定期预订配置
func UpdateRecurringReservation(reservation *RecurringReservation) error {
	return db.Save(reservation).Error
}

// DeleteRecurringReservation 删除定期预订配置
func DeleteRecurringReservation(id uint) error {
	return db.Delete(&RecurringReservation{}, id).Error
}

// GetRecurringReservationByID 根据ID获取定期预订配置
func GetRecurringReservationByID(id uint) (*RecurringReservation, error) {
	var reservation RecurringReservation
	result := db.First(&reservation, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &reservation, nil
}

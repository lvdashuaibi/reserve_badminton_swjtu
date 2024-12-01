package db

import (
	"errors"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"os"
	"sync"
)

type User struct {
	ID       uint   `gorm:"primaryKey;autoIncrement"`
	Name     string `gorm:"unique"`
	Password string `gorm:"size:255"`
	Token    string
	Email    string `gorm:"email"`
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
	// 读取环境变量中的密码
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("环境变量 DB_PASSWORD 未设置")
	}

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
	err = db.AutoMigrate(&User{})
	if err != nil {
		log.Printf("数据库自动迁移失败%v\n", err)
		return
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

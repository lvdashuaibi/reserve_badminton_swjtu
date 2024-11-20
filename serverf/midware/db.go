package db

import (
	"errors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"sync"
)

type User struct {
	ID       uint   `gorm:"primaryKey;autoIncrement"`
	Name     string `gorm:"unique"`
	Password string `gorm:"size:255"`
	Token    string
	Email    string `gorm:"email"`
}

var (
	db     *gorm.DB
	dbOnce sync.Once
)

func init() {
	openDB()
}

func openDB() {
	var err error
	dsn := "root:100617YYzyq.@tcp(127.0.0.1:3306)/badminton?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("连接数据库失败:%v", err)
	}
	db.AutoMigrate(&User{})
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

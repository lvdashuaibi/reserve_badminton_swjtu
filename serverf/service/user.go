package service

import (
	"errors"
	"github.com/gin-gonic/gin"
	db "serverf/midware"
)

func Register(c *gin.Context, opUsername, Username, Password, badmintonToken string) error {

	if opUsername != "lwh" {
		return errors.New("你没有权限！！！！！")
	}

	if db.FindUsername(Username) {
		return errors.New("用户名已存在")
	}

	_, err := db.InsertUser(Username, Password, badmintonToken)
	if err != nil {
		return err
	}
	return nil
}

func Login(ctx *gin.Context, Username, Password string) (error, string) {

	storedPassword, err := db.GetPassWordByUserName(Username)
	if err != nil {
		return errors.New("用户不存在"), ""
	}

	if storedPassword != Password {
		return errors.New("密码错误"), ""
	}

	token, err := GenerateJWT(Username)
	if err != nil {
		return errors.New("Could not generate token"), ""
	}
	return nil, token
}

func ChangePassword(ctx *gin.Context, Username string, NewPassword string) error {
	if err := db.ChangePassword(Username, NewPassword); err != nil {
		return errors.New("修改失败")
	}
	return nil
}

package handler

import (
	"fmt"
	"net/http"
	"serverf/entity"
	"serverf/service"

	"github.com/gin-gonic/gin"
)

func HandleRegister(c *gin.Context) {
	registerData := &entity.RegisterData{}

	value, _ := c.Get("username")
	opUsername := value.(string)

	if err := c.BindJSON(&registerData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := service.Register(c, opUsername, registerData.Username, registerData.Password, registerData.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func HandleChangePassword(c *gin.Context) {
	data := &entity.ChangePasswordData{}

	value, _ := c.Get("username")
	username := value.(string)

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

	//fmt.Printf("Received new password: %s\n", data.NewPassword) // 调试信息

	err := service.ChangePassword(c, username, data.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "修改成功！"})
}

func HandleLogin(c *gin.Context) {
	loginData := &entity.LoginData{}
	err := c.Bind(loginData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Println(loginData)

	err, token := service.Login(c, loginData.Username, loginData.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

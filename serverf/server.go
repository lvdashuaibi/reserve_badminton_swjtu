package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"serverf/handler"
)

func main() {

	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"title": "posts/index",
		})
	})

	//db.InsertUser("lwh", "100617YYzyq.", "3c436ce8-e335-4c29-aedc-0e318e3b3583")

	r.POST("/login", handler.HandleLogin)
	r.POST("/register", handler.MiddleWare, handler.HandleRegister)
	r.POST("/changePassword", handler.MiddleWare, handler.HandleChangePassword)

	// 路由配置
	r.GET("/badminton", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "posts/index",
		})
	})
	r.POST("/reserve", handler.MiddleWare, handler.HandleReservation)
	r.GET("/getSessions", handler.MiddleWare, handler.HandleGetSessions)  // 新增获取场地信息的接口
	r.POST("/changeToken", handler.MiddleWare, handler.HandleChangeToken) // 新增更改 token 的接口

	// 启动服务器
	fmt.Println("Starting server on port 80...")
	r.Run(":80")
}

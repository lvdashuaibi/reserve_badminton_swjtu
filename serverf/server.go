package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"serverf/handler"

	"github.com/gin-gonic/gin"
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

	// 任务管理路由
	r.GET("/tasks-page", func(c *gin.Context) {
		c.HTML(http.StatusOK, "tasks.html", gin.H{
			"title": "任务管理",
		})
	})
	r.GET("/tasks", handler.MiddleWare, handler.HandleGetTasks)         // 获取任务列表
	r.POST("/cancelTask", handler.MiddleWare, handler.HandleCancelTask) // 取消任务

	// 配置管理路由（仅特权用户）
	r.GET("/config-page", func(c *gin.Context) {
		c.HTML(http.StatusOK, "config.html", gin.H{
			"title": "系统配置",
		})
	})
	r.GET("/config", handler.MiddleWare, handler.HandleGetConfig)     // 获取配置
	r.POST("/config", handler.MiddleWare, handler.HandleUpdateConfig) // 更新配置

	// 启动服务器
	fmt.Println("Starting server on port 8080...")
	r.Run(":8080")
}

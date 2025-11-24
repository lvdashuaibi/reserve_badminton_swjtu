package handler

import (
	"net/http"
	"serverf/service"

	"github.com/gin-gonic/gin"
)

// HandleGetTasks 获取任务列表
func HandleGetTasks(c *gin.Context) {
	value, _ := c.Get("username")
	username := value.(string)

	tm := service.GetTaskManager()

	var tasks []*service.ReservationTask
	// 如果是特权用户lwh，返回所有任务
	if username == "lwh" {
		tasks = tm.GetAllTasks()
	} else {
		// 普通用户只能看到自己的任务
		tasks = tm.GetUserTasks(username)
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":   tasks,
		"isAdmin": username == "lwh",
	})
}

// HandleCancelTask 取消任务
func HandleCancelTask(c *gin.Context) {
	value, _ := c.Get("username")
	username := value.(string)

	var req struct {
		TaskID string `json:"taskId"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	tm := service.GetTaskManager()

	// 获取任务
	task, exists := tm.GetTask(req.TaskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 检查权限：只有任务所有者或管理员可以取消
	if task.Username != username && username != "lwh" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限取消此任务"})
		return
	}

	// 取消任务
	if tm.CancelTask(req.TaskID) {
		c.JSON(http.StatusOK, gin.H{"message": "任务已取消"})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务无法取消（可能已完成或已取消）"})
	}
}

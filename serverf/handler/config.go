package handler

import (
	"net/http"
	"serverf/service"

	"github.com/gin-gonic/gin"
)

// HandleGetConfig 获取配置
func HandleGetConfig(c *gin.Context) {
	value, _ := c.Get("username")
	username := value.(string)

	// 只有特权用户可以访问
	if username != "lwh" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限访问配置"})
		return
	}

	cm := service.GetConfigManager()
	config := cm.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"reservationHour":      config.ReservationHour,
		"reservationMinute":    config.ReservationMinute,
		"reservationSecond":    config.ReservationSecond,
		"reservationMillisec":  config.ReservationMillisec,
		"retryIntervalMinutes": int(config.RetryInterval.Minutes()),
		"retryDurationHours":   int(config.RetryDuration.Hours()),
	})
}

// HandleUpdateConfig 更新配置
func HandleUpdateConfig(c *gin.Context) {
	value, _ := c.Get("username")
	username := value.(string)

	// 只有特权用户可以修改配置
	if username != "lwh" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限修改配置"})
		return
	}

	var req struct {
		ReservationHour      int `json:"reservationHour"`
		ReservationMinute    int `json:"reservationMinute"`
		ReservationSecond    int `json:"reservationSecond"`
		ReservationMillisec  int `json:"reservationMillisec"`
		RetryIntervalMinutes int `json:"retryIntervalMinutes"`
		RetryDurationHours   int `json:"retryDurationHours"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	cm := service.GetConfigManager()
	err := cm.UpdateConfig(
		req.ReservationHour,
		req.ReservationMinute,
		req.ReservationSecond,
		req.ReservationMillisec,
		req.RetryIntervalMinutes,
		req.RetryDurationHours,
	)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置更新成功"})
}

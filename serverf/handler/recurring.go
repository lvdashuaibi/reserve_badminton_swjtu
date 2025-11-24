package handler

import (
	"net/http"
	db "serverf/midware"

	"github.com/gin-gonic/gin"
)

// HandleGetRecurringReservations 获取定期预订配置列表
func HandleGetRecurringReservations(c *gin.Context) {
	value, _ := c.Get("username")
	username := value.(string)

	// 检查是否是管理员
	isAdmin := (username == "lwh")

	var reservations []db.RecurringReservation
	var err error

	if isAdmin {
		// 管理员可以查看所有配置
		reservations, err = db.GetAllRecurringReservations()
	} else {
		// 普通用户只能查看自己的配置
		reservations, err = db.GetRecurringReservations(username)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"isAdmin":      isAdmin,
		"reservations": reservations,
	})
}

// HandleCreateRecurringReservation 创建定期预订配置
func HandleCreateRecurringReservation(c *gin.Context) {
	value, _ := c.Get("username")
	username := value.(string)

	var req struct {
		Name      string   `json:"name"`
		Weekday   int      `json:"weekday"`
		CourtNums []string `json:"courtNums"`
		TimeSlots []string `json:"timeSlots"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// 验证参数
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "配置名称不能为空"})
		return
	}
	if req.Weekday < 0 || req.Weekday > 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "星期几必须在0-6之间"})
		return
	}
	if len(req.CourtNums) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少选择一个场地"})
		return
	}
	if len(req.TimeSlots) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少选择一个时间段"})
		return
	}

	// 将数组转换为逗号分隔的字符串
	courtNumsStr := ""
	for i, num := range req.CourtNums {
		if i > 0 {
			courtNumsStr += ","
		}
		courtNumsStr += num
	}

	timeSlotsStr := ""
	for i, slot := range req.TimeSlots {
		if i > 0 {
			timeSlotsStr += ","
		}
		timeSlotsStr += slot
	}

	reservation := &db.RecurringReservation{
		Username:  username,
		Name:      req.Name,
		Weekday:   req.Weekday,
		CourtNums: courtNumsStr,
		TimeSlots: timeSlotsStr,
		Enabled:   true,
	}

	if err := db.CreateRecurringReservation(reservation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "定期预订配置创建成功",
		"reservation": reservation,
	})
}

// HandleUpdateRecurringReservation 更新定期预订配置
func HandleUpdateRecurringReservation(c *gin.Context) {
	value, _ := c.Get("username")
	username := value.(string)

	var req struct {
		ID        uint     `json:"id"`
		Name      string   `json:"name"`
		Weekday   int      `json:"weekday"`
		CourtNums []string `json:"courtNums"`
		TimeSlots []string `json:"timeSlots"`
		Enabled   bool     `json:"enabled"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// 获取现有配置
	existing, err := db.GetRecurringReservationByID(req.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	// 检查权限
	if existing.Username != username && username != "lwh" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限修改此配置"})
		return
	}

	// 将数组转换为逗号分隔的字符串
	courtNumsStr := ""
	for i, num := range req.CourtNums {
		if i > 0 {
			courtNumsStr += ","
		}
		courtNumsStr += num
	}

	timeSlotsStr := ""
	for i, slot := range req.TimeSlots {
		if i > 0 {
			timeSlotsStr += ","
		}
		timeSlotsStr += slot
	}

	// 更新配置
	existing.Name = req.Name
	existing.Weekday = req.Weekday
	existing.CourtNums = courtNumsStr
	existing.TimeSlots = timeSlotsStr
	existing.Enabled = req.Enabled

	if err := db.UpdateRecurringReservation(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "定期预订配置更新成功",
		"reservation": existing,
	})
}

// HandleDeleteRecurringReservation 删除定期预订配置
func HandleDeleteRecurringReservation(c *gin.Context) {
	value, _ := c.Get("username")
	username := value.(string)

	var req struct {
		ID uint `json:"id"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// 获取现有配置
	existing, err := db.GetRecurringReservationByID(req.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	// 检查权限
	if existing.Username != username && username != "lwh" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限删除此配置"})
		return
	}

	if err := db.DeleteRecurringReservation(req.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "定期预订配置删除成功"})
}

package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"serverf/entity"
	db "serverf/midware"
	"serverf/service"
)

func HandleReservation(c *gin.Context) {
	req := entity.GetSessionIDRequest{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	_, err := service.Reservation(c, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed"})
		return
	}

	// 返回预定接收确认信息
	c.JSON(http.StatusOK, gin.H{"message": "Reservation request received. It will be processed at 22:30."})
}

// 新增处理函数，用于获取场地信息并返回给前端
func HandleGetSessions(c *gin.Context) {
	date := c.Query("date")

	value, _ := c.Get("username")
	username := value.(string)
	token, _ := db.GetTokenByUserName(username)

	sessions, err := service.GetSessions(token, entity.FieldID, date, entity.SportTypeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting sessions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

func HandleChangeToken(c *gin.Context) {
	var req struct {
		NewToken string `json:"newToken"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	//token = req.NewToken
	value, _ := c.Get("username")
	username := value.(string)
	db.SetToken(username, req.NewToken)
	c.JSON(http.StatusOK, gin.H{"message": "Token successfully updated!"})
}

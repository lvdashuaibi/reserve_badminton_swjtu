package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	db "serverf/midware"
)

const (
	baseURL            = "https://zhcg.swjtu.edu.cn/onesports-gateway"
	getSessionsPath    = "/wechat-c/api/wechat/memberBookController/weChatSessionsList"
	reserveSessionPath = "/business-service/orders/weChatSessionsReserve"
	defaultToken       = "a0833c5b-0acd-4bc1-82e9-e1296c44d4fa" // 默认 token
	fieldID            = "1462412671863504896"
	sportTypeID        = "2"
)

var jwtKey = []byte("lwh666")

// JWT Claim
type Claims struct {
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"exp"`
	jwt.StandardClaims
}

type SessionRequest struct {
	FieldID     string `json:"fieldId"`
	IsIndoor    string `json:"isIndoor"`
	PlaceTypeID string `json:"placeTypeId"`
	SearchDate  string `json:"searchDate"`
	SportTypeID string `json:"sportTypeId"`
}

type Session struct {
	ID             string `json:"id"`
	OpenDate       string `json:"openDate"`
	OpenStartTime  string `json:"openStartTime"`
	OpenEndTime    string `json:"openEndTime"`
	PlaceName      string `json:"placeName"`
	SessionsStatus string `json:"sessionsStatus"`
}

type ReserveRequest struct {
	Number        int              `json:"number"`
	OrderUseDate  string           `json:"orderUseDate"`
	RequestsList  []ReserveSession `json:"requestsList"`
	FieldName     string           `json:"fieldName"`
	FieldID       string           `json:"fieldId"`
	SiteName      string           `json:"siteName"`
	SportTypeName string           `json:"sportTypeName"`
	SportTypeID   string           `json:"sportTypeId"`
}

type ReserveSession struct {
	SessionsID string `json:"sessionsId"`
}

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

	r.POST("/login", handleLogin)
	r.POST("/register", MiddleWare, handleRegister)
	r.POST("changePassword", MiddleWare, handleChangePassword)

	// 路由配置
	r.GET("/badminton", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "posts/index",
		})
	})
	r.POST("/reserve", MiddleWare, handleReservation)
	r.GET("/getSessions", MiddleWare, handleGetSessions)  // 新增获取场地信息的接口
	r.POST("/changeToken", MiddleWare, handleChangeToken) // 新增更改 token 的接口

	// 启动服务器
	fmt.Println("Starting server on port 8080...")
	r.Run(":80")
}

func handleRegister(c *gin.Context) {
	var registerData struct {
		Username string `json:"name"`
		Password string `json:"password"`
		Token    string `json:"token"`
	}

	if err := c.BindJSON(&registerData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	value, _ := c.Get("username")
	admin := value.(string)
	fmt.Println(admin)
	if admin != "lwh" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "你没有权限！！！！！"})
		return
	}

	if db.FindUsername(registerData.Username) {
		c.JSON(http.StatusForbidden, gin.H{"error": "用户名已存在"})
		return
	}

	db.InsertUser(registerData.Username, registerData.Password, registerData.Token)
	c.JSON(http.StatusOK, gin.H{"message": "注册成功！"})
}

func handleChangePassword(c *gin.Context) {
	var data struct {
		NewPassword string `json:"new_password"`
	}

	value, _ := c.Get("username")
	username := value.(string)

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

	//fmt.Printf("Received new password: %s\n", data.NewPassword) // 调试信息

	if err := db.ChangePassword(username, data.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "修改失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "修改成功！"})
}

func handleLogin(c *gin.Context) {
	var loginData struct {
		Username string `json:"name"`
		Password string `json:"password"`
	}

	if err := c.BindJSON(&loginData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	storedPassword, err := db.GetPassWordByUserName(loginData.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户不存在"})
		return
	}

	if storedPassword != loginData.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	token, err := generateJWT(loginData.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func MiddleWare(c *gin.Context) {
	tokenString := c.GetHeader("Authorization")
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录！！！！"})
		c.Abort()
		return
	}
	// 去除 Bearer 前缀
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	// 解析 JWT
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 token"})
		c.Abort()
		return
	}

	// 将用户名添加到上下文中以供后续处理
	c.Set("username", claims.Username)

	c.Next()
}

func generateJWT(username string) (string, error) {
	expirationTime := time.Now().Add(30 * 24 * time.Hour)
	claims := &Claims{
		Username:  username,
		ExpiresAt: expirationTime,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

/*
	func handleReservation(c *gin.Context) {
		var req struct {
			Date     string `json:"date"`
			TimeSlot string `json:"timeSlot"`
			CourtNum string `json:"courtNum"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		value, _ := c.Get("username")
		username := value.(string)
		token, _ := db.GetTokenByUserName(username)

		// 获取场次ID
		sessionID, err := getSessionID(token, fieldID, req.Date, req.TimeSlot, req.CourtNum+"号羽毛球", sportTypeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting session ID: " + err.Error()})
			return
		}

		// 预定场地
		err = reserveCourt(token, sessionID, fieldID, req.Date, req.CourtNum+"号羽毛球", sportTypeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error reserving court: " + err.Error()})
			return
		}

		// 返回成功消息
		c.JSON(http.StatusOK, gin.H{"message": "Court successfully reserved!"})
	}
*/
func handleReservation(c *gin.Context) {
	var req struct {
		Date     string `json:"date"`
		TimeSlot string `json:"timeSlot"`
		CourtNum string `json:"courtNum"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	value, _ := c.Get("username")
	username := value.(string)
	token, _ := db.GetTokenByUserName(username)

	// 获取场次ID
	sessionID, err := getSessionID(token, fieldID, req.Date, req.TimeSlot, req.CourtNum+"号羽毛球", sportTypeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting session ID: " + err.Error()})
		return
	}

	// 获取当前时间，计算到今天22:30的时间差
	now := time.Now()
	nextReservationTime := time.Date(now.Year(), now.Month(), now.Day(), 22, 30, 0, 200000000, now.Location())

	if now.After(nextReservationTime) {
		// 如果当前时间已经过了当天22:30，直接预定
		go reserveCourtAtTime(token, sessionID, struct {
			Date     string
			TimeSlot string
			CourtNum string
		}(req))
	} else {
		// 等待到22:30再发起预定请求
		time.AfterFunc(nextReservationTime.Sub(now), func() {
			reserveCourtAtTime(token, sessionID, struct {
				Date     string
				TimeSlot string
				CourtNum string
			}(req))
		})
	}

	// 返回预定接收确认信息
	c.JSON(http.StatusOK, gin.H{"message": "Reservation request received. It will be processed at 22:30."})
}

func reserveCourtAtTime(token string, sessionID string, req struct {
	Date     string
	TimeSlot string
	CourtNum string
}) {
	// 预定场地
	err := reserveCourt(token, sessionID, fieldID, req.Date, req.CourtNum+"号羽毛球", sportTypeID)
	if err != nil {
		// 记录失败的日志或通知管理员
		return
	}
	// 记录成功的预定操作
}

func handleChangeToken(c *gin.Context) {
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

func getSessionID(token, fieldID, date, timeSlot, courtName, sportTypeID string) (string, error) {
	requestBody := SessionRequest{
		FieldID:     fieldID,
		IsIndoor:    "",
		PlaceTypeID: "",
		SearchDate:  date,
		SportTypeID: sportTypeID,
	}

	jsonBody, _ := json.Marshal(requestBody)
	req, err := http.NewRequest("POST", baseURL+getSessionsPath, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-UserToken", token)
	req.Header.Set("token", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return "", err
		}
		defer reader.(*gzip.Reader).Close()
	}

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}

	var sessions [][]Session
	if err := json.Unmarshal(body, &sessions); err != nil {
		return "", err
	}

	for _, sessionGroup := range sessions {
		for _, session := range sessionGroup {
			if session.PlaceName == courtName && session.OpenStartTime == timeSlot && session.SessionsStatus == "NO_RESERVED" {
				return session.ID, nil
			}
		}
	}

	return "", fmt.Errorf("no available sessions found")
}

func reserveCourt(token, sessionID, fieldID, date, courtName, sportTypeID string) error {
	// 定义最大重试时间为2小时
	retryDuration := 3 * time.Hour
	// 定义每次重试的间隔时间，例如每1分钟重试一次
	retryInterval := 85 * time.Second
	// 开始重试的时间
	startTime := time.Now()

	//log.Println("尝试预定...")

	// 将日期转换为时间戳
	layout := "2006-01-02"
	t, err := time.Parse(layout, date)
	if err != nil {
		return fmt.Errorf("日期解析失败: %v", err)
	}
	timestamp := t.UnixMilli() // 转换为毫秒时间戳

	requestBody := ReserveRequest{
		Number:        3,
		OrderUseDate:  fmt.Sprintf("%d", timestamp), // 传递时间戳
		RequestsList:  []ReserveSession{{SessionsID: sessionID}},
		FieldName:     "犀浦室内羽毛球馆",
		FieldID:       fieldID,
		SiteName:      courtName,
		SportTypeName: "羽毛球",
		SportTypeID:   sportTypeID,
	}

	jsonBody, _ := json.Marshal(requestBody)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s?timestamp=%d", baseURL, reserveSessionPath, timestamp), bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("请求创建失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-UserToken", token)
	req.Header.Set("token", token)
	req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Expires", "0")

	for time.Since(startTime) < retryDuration {
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Println("请求失败:", err)
			time.Sleep(retryInterval) // 请求失败时等待retryInterval后重试
			continue
		}
		defer resp.Body.Close()

		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		//log.Println("状态码:", resp.StatusCode, "响应内容:", string(bodyBytes))

		// 检查是否是 "预定日期未开放" 错误
		if strings.Contains(string(bodyBytes), "预定日期未开放，请勿超期预定") {
			log.Println("预定日期未开放，等待重试...")
			time.Sleep(retryInterval) // 等待retryInterval后重试
		} else if strings.Contains(string(bodyBytes), "已被占用") {
			log.Println("显示已被占用，等待重试...")
			time.Sleep(retryInterval) // 等待retryInterval后重试
		} else if strings.Contains(string(bodyBytes), "200") {
			log.Println("预定成功!")
			return nil
		} else {
			// 如果是其他错误，直接返回
			return fmt.Errorf("预定失败: %s", string(bodyBytes))
		}

		retryInterval += time.Second

	}

	return fmt.Errorf("超过最大重试时间，预定失败")
}

// 获取场地信息列表
func getSessions(token, fieldID, date, sportTypeID string) ([][]Session, error) {
	requestBody := SessionRequest{
		FieldID:     fieldID,
		IsIndoor:    "",
		PlaceTypeID: "",
		SearchDate:  date,
		SportTypeID: sportTypeID,
	}

	jsonBody, _ := json.Marshal(requestBody)
	req, err := http.NewRequest("POST", baseURL+getSessionsPath, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-UserToken", token)
	req.Header.Set("token", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer reader.(*gzip.Reader).Close()
	}

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var sessions [][]Session
	if err := json.Unmarshal(body, &sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

// 新增处理函数，用于获取场地信息并返回给前端
func handleGetSessions(c *gin.Context) {
	date := c.Query("date")

	value, _ := c.Get("username")
	username := value.(string)
	token, _ := db.GetTokenByUserName(username)

	sessions, err := getSessions(token, fieldID, date, sportTypeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting sessions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

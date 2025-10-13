package service

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"serverf/entity"
	db "serverf/midware"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func Reservation(c *gin.Context, req entity.GetSessionIDRequest) (interface{}, error) {
	value, _ := c.Get("username")
	username := value.(string)
	token, _ := db.GetTokenByUserName(username)

	var allSessionIDs []string

	// 遍历 courtNums 和 timeSlots，获取每一对的 sessionID
	for i := 0; i < len(req.CourtNums); i++ {
		sessionID, err := getSessionID(token, entity.FieldID, req.Date, req.TimeSlots[i], req.CourtNums[i]+"号羽毛球", entity.SportTypeID)
		if err != nil {
			return nil, err
		}
		allSessionIDs = append(allSessionIDs, sessionID)
	}

	now := time.Now()
	reqTime, err := db.GetReqTimeByUserName(username)
	if err != nil {
		return nil, err
	}
	nextReservationTime := time.Date(now.Year(), now.Month(), now.Day(), 22, 30, 0, reqTime, now.Location())

	if now.After(nextReservationTime) {
		go reserveCourtAtTime(token, allSessionIDs, username, req)
	} else {
		time.AfterFunc(nextReservationTime.Sub(now), func() {
			reserveCourtAtTime(token, allSessionIDs, username, req)
		})
	}
	return nil, nil
}
func getSessionID(token, fieldID, date, timeSlot, courtName, sportTypeID string) (string, error) {
	requestBody := entity.SessionRequest{
		FieldID:     fieldID,
		IsIndoor:    "",
		PlaceTypeID: "",
		SearchDate:  date,
		SportTypeID: sportTypeID,
	}

	jsonBody, _ := json.Marshal(requestBody)
	req, err := http.NewRequest("POST", entity.BaseURL+entity.GetSessionsPath, bytes.NewBuffer(jsonBody))
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

	var sessions [][]entity.Session
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

func reserveCourtAtTime(token string, sessionIDs []string, username string, req entity.GetSessionIDRequest) {
	// 预定场地
	err := reserveCourt(token, sessionIDs, entity.FieldID, req.Date, req.CourtNums[0]+"号羽毛球", entity.SportTypeID, username, req.TimeSlots[0])
	if err != nil {
		// 记录失败的日志或通知管理员
		return
	}
	// 记录成功的预定操作
}

func reserveCourt(token string, sessionIDs []string, fieldID, date, courtName, sportTypeID, username, timeSlot string) error {
	// 定义最大重试时间为2小时
	retryDuration := 3 * time.Hour
	// 定义每次重试的间隔时间，例如每1分钟重试一次
	retryInterval := time.Minute * 3
	// 开始重试的时间
	startTime := time.Now()

	// 将日期转换为时间戳
	layout := "2006-01-02"
	t, err := time.Parse(layout, date)
	if err != nil {
		return fmt.Errorf("日期解析失败: %v", err)
	}
	timestamp := t.UnixMilli() // 转换为毫秒时间戳

	// 构建 RequestsList
	var requestsList []entity.ReserveSession
	for _, sessionID := range sessionIDs {
		requestsList = append(requestsList, entity.ReserveSession{SessionsID: sessionID})
	}

	requestBody := entity.ReserveRequest{
		Number:        20,
		OrderUseDate:  fmt.Sprintf("%d", timestamp),
		RequestsList:  requestsList,
		FieldName:     "犀浦室内羽毛球馆",
		FieldID:       fieldID,
		SiteName:      courtName,
		SportTypeName: "羽毛球",
		SportTypeID:   sportTypeID,
	}

	jsonBody, _ := json.Marshal(requestBody)

	// 打印请求内容
	log.Printf("发送预订请求，内容：%s", string(jsonBody))

	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s?timestamp=%d", entity.BaseURL, entity.ReserveSessionPath, timestamp), bytes.NewBuffer(jsonBody))
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
		// 打印响应内容
		log.Printf("收到响应，状态码：%d，内容：%s", resp.StatusCode, string(bodyBytes))

		// 检查是否是 "预定日期未开放" 错误
		if strings.Contains(string(bodyBytes), "预定日期未开放，请勿超期预定") {
			log.Printf("%s，时间：%s ：预定日期未开放，等待重试...", courtName, timeSlot)
		} else if strings.Contains(string(bodyBytes), "已被占用") {
			log.Printf("\"%s，时间：%s ：显示已被占用，等待重试...", courtName, timeSlot)
		} else if strings.Contains(string(bodyBytes), "请勿重复请求") {
			log.Printf("\"%s，时间：%s ：请勿重复请求，等待重试...", courtName, timeSlot)
		} else if strings.Contains(string(bodyBytes), "200") {
			// 设置邮件正文
			err := SendMessage(db.GetEmailByUserName(username), date, courtName, timeSlot, "预定成功！请及时支付")
			if err != nil {
				log.Println("邮件发送失败")
			}
			log.Printf("\"%s，时间：%s ：预定成功!", courtName, timeSlot)
			return nil
		} else if strings.Contains(string(bodyBytes), "请求失败") {
			continue
		} else {
			// 如果是其他错误，直接返回
			log.Printf("Error:%s", string(bodyBytes))
		}

		time.Sleep(retryInterval)
		retryInterval += time.Second
	}

	return fmt.Errorf("超过最大重试时间，预定失败")
}

// 获取场地信息列表
func GetSessions(token, fieldID, date, sportTypeID string) ([][]entity.Session, error) {
	requestBody := entity.SessionRequest{
		FieldID:     fieldID,
		IsIndoor:    "",
		PlaceTypeID: "",
		SearchDate:  date,
		SportTypeID: sportTypeID,
	}

	jsonBody, _ := json.Marshal(requestBody)
	req, err := http.NewRequest("POST", entity.BaseURL+entity.GetSessionsPath, bytes.NewBuffer(jsonBody))
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

	var sessions [][]entity.Session
	if err := json.Unmarshal(body, &sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

package service

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"serverf/entity"
	db "serverf/midware"
	"strings"
	"time"
)

func Reservation(c *gin.Context, req entity.GetSessionIDRequest) (interface{}, error) {

	value, _ := c.Get("username")
	username := value.(string)
	token, _ := db.GetTokenByUserName(username)

	// 获取场次ID
	sessionID, err := getSessionID(token, entity.FieldID, req.Date, req.TimeSlot, req.CourtNum+"号羽毛球", entity.SportTypeID)
	if err != nil {
		return nil, err
	}

	// 获取当前时间，计算到今天22:30的时间差
	now := time.Now()
	reqTime, err := db.GetReqTimeByUserName(username)
	if err != nil {
		return nil, err
	}
	nextReservationTime := time.Date(now.Year(), now.Month(), now.Day(), 22, 30, 0, reqTime, now.Location())

	if now.After(nextReservationTime) {
		// 如果当前时间已经过了当天22:30，直接预定
		go reserveCourtAtTime(token, sessionID, username, struct {
			Date     string
			TimeSlot string
			CourtNum string
		}(req))
	} else {
		// 等待到22:30再发起预定请求
		time.AfterFunc(nextReservationTime.Sub(now), func() {
			reserveCourtAtTime(token, sessionID, username, struct {
				Date     string
				TimeSlot string
				CourtNum string
			}(req))
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

func reserveCourtAtTime(token string, sessionID string, username string, req struct {
	Date     string
	TimeSlot string
	CourtNum string
}) {
	// 预定场地
	err := reserveCourt(token, sessionID, entity.FieldID, req.Date, req.CourtNum+"号羽毛球", entity.SportTypeID, username, req.TimeSlot)
	if err != nil {
		// 记录失败的日志或通知管理员
		return
	}
	// 记录成功的预定操作
}

func reserveCourt(token, sessionID, fieldID, date, courtName, sportTypeID, username, timeSlot string) error {
	// 定义最大重试时间为2小时
	retryDuration := 3 * time.Hour
	// 定义每次重试的间隔时间，例如每1分钟重试一次
	retryInterval := time.Second * 3
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

	requestBody := entity.ReserveRequest{
		Number:        3,
		OrderUseDate:  fmt.Sprintf("%d", timestamp), // 传递时间戳
		RequestsList:  []entity.ReserveSession{{SessionsID: sessionID}},
		FieldName:     "犀浦室内羽毛球馆",
		FieldID:       fieldID,
		SiteName:      courtName,
		SportTypeName: "羽毛球",
		SportTypeID:   sportTypeID,
	}

	jsonBody, _ := json.Marshal(requestBody)
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
		//log.Println("状态码:", resp.StatusCode, "响应内容:", string(bodyBytes))

		// 检查是否是 "预定日期未开放" 错误
		if strings.Contains(string(bodyBytes), "预定日期未开放，请勿超期预定") {
			log.Printf("%s号，时间：%s ：预定日期未开放，等待重试...", courtName, timeSlot)
			time.Sleep(retryInterval) // 等待retryInterval后重试
		} else if strings.Contains(string(bodyBytes), "已被占用") {
			log.Printf("\"%s号，时间：%s ：显示已被占用，等待重试...", courtName, timeSlot)
			time.Sleep(retryInterval) // 等待retryInterval后重试
		} else if strings.Contains(string(bodyBytes), "200") {
			// 设置邮件正文
			err := SendMessage(db.GetEmailByUserName(username), date, courtName, timeSlot, "预定成功！请及时支付")
			if err != nil {
				log.Println("邮件发送失败")
			}
			log.Printf("\"%s号，时间：%s ：预定成功!", courtName, timeSlot)
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

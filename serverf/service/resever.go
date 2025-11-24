package service

import (
	"bytes"
	"compress/gzip"
	"context"
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
	"github.com/google/uuid"
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
	// 创建任务
	taskID := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())

	task := &ReservationTask{
		TaskID:     taskID,
		Username:   username,
		Date:       req.Date,
		CourtName:  req.CourtNums[0] + "号羽毛球",
		TimeSlot:   req.TimeSlots[0],
		Status:     "running",
		StartTime:  time.Now(),
		Message:    "正在尝试预订...",
		CancelFunc: cancel,
	}

	// 添加到任务管理器
	tm := GetTaskManager()
	tm.AddTask(task)

	log.Printf("[任务 %s] 开始预订：用户=%s, 场地=%s, 时间=%s", taskID, username, task.CourtName, task.TimeSlot)

	// 预定场地
	err := reserveCourtWithContext(ctx, token, sessionIDs, entity.FieldID, req.Date, req.CourtNums[0]+"号羽毛球", entity.SportTypeID, username, req.TimeSlots[0], taskID)
	if err != nil {
		// 记录失败的日志或通知管理员
		log.Printf("[任务 %s] 预订失败: %v", taskID, err)
		tm.UpdateTaskStatus(taskID, "failed", err.Error())
		return
	}
	// 记录成功的预定操作
	log.Printf("[任务 %s] 预订成功", taskID)
	tm.UpdateTaskStatus(taskID, "success", "预订成功！")
}

func reserveCourtWithContext(ctx context.Context, token string, sessionIDs []string, fieldID, date, courtName, sportTypeID, username, timeSlot, taskID string) error {
	tm := GetTaskManager()

	// 定义最大重试时间为24小时
	retryDuration := 24 * time.Hour
	// 定义固定的重试间隔时间为5分钟
	retryInterval := 5 * time.Minute
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
	log.Printf("[任务 %s] 发送预订请求，内容：%s", taskID, string(jsonBody))

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
		// 检查是否被取消
		select {
		case <-ctx.Done():
			log.Printf("[任务 %s] 任务已被取消", taskID)
			return fmt.Errorf("任务已被用户取消")
		default:
		}

		// 在每次重试前，先查询场地状态
		log.Printf("[任务 %s] 查询场地状态：%s，时间：%s", taskID, courtName, timeSlot)
		tm.UpdateTaskStatus(taskID, "running", fmt.Sprintf("查询场地状态..."))

		sessions, err := GetSessions(token, fieldID, date, sportTypeID)
		if err != nil {
			log.Printf("[任务 %s] 查询场地状态失败: %v，等待重试...", taskID, err)
			tm.UpdateTaskStatus(taskID, "running", fmt.Sprintf("查询失败，等待重试: %v", err))

			// 等待时检查取消
			select {
			case <-ctx.Done():
				return fmt.Errorf("任务已被用户取消")
			case <-time.After(retryInterval):
			}
			continue
		}

		// 检查对应的场地是否可预定
		isAvailable := false
		var sessionStatus string
		for _, sessionGroup := range sessions {
			for _, session := range sessionGroup {
				if session.PlaceName == courtName && session.OpenStartTime == timeSlot {
					sessionStatus = session.SessionsStatus
					if session.SessionsStatus == "NO_RESERVED" {
						isAvailable = true
					}
					break
				}
			}
			if sessionStatus != "" {
				break
			}
		}

		// 如果场地不可预定，发送邮件通知并停止重试
		if !isAvailable {
			log.Printf("[任务 %s] %s，时间：%s ：场地状态为 %s，不可预定", taskID, courtName, timeSlot, sessionStatus)
			email := db.GetEmailByUserName(username)
			if email != "" {
				statusMsg := ""
				switch sessionStatus {
				case "RESERVED":
					statusMsg = "已被预订"
				case "EXPIRED":
					statusMsg = "已过期"
				case "":
					statusMsg = "未找到该场地"
				default:
					statusMsg = sessionStatus
				}
				err := SendMessage(email, date, courtName, timeSlot, fmt.Sprintf("预订失败：场地状态为【%s】，无法预订", statusMsg))
				if err != nil {
					log.Printf("[任务 %s] 邮件发送失败: %v", taskID, err)
				} else {
					log.Printf("[任务 %s] 已发送失败通知邮件至: %s", taskID, email)
				}
			}
			return fmt.Errorf("场地状态为 %s，不可预定", sessionStatus)
		}

		log.Printf("[任务 %s] %s，时间：%s ：场地可预定，开始预订", taskID, courtName, timeSlot)
		tm.UpdateTaskStatus(taskID, "running", "场地可预定，正在发送预订请求...")

		// 场地可预定，发送预订请求
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[任务 %s] 请求失败: %v", taskID, err)
			tm.UpdateTaskStatus(taskID, "running", fmt.Sprintf("请求失败，等待重试: %v", err))

			// 等待时检查取消
			select {
			case <-ctx.Done():
				return fmt.Errorf("任务已被用户取消")
			case <-time.After(retryInterval):
			}
			continue
		}
		defer resp.Body.Close()

		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		// 打印响应内容
		log.Printf("[任务 %s] 收到响应，状态码：%d，内容：%s", taskID, resp.StatusCode, string(bodyBytes))

		// 检查是否是 "预定日期未开放" 错误
		if strings.Contains(string(bodyBytes), "预定日期未开放，请勿超期预定") {
			log.Printf("[任务 %s] %s，时间：%s ：预定日期未开放，等待重试...", taskID, courtName, timeSlot)
			tm.UpdateTaskStatus(taskID, "running", "预定日期未开放，等待重试...")
		} else if strings.Contains(string(bodyBytes), "已被占用") {
			log.Printf("[任务 %s] %s，时间：%s ：显示已被占用，等待重试...", taskID, courtName, timeSlot)
			tm.UpdateTaskStatus(taskID, "running", "场地已被占用，等待重试...")
		} else if strings.Contains(string(bodyBytes), "请勿重复请求") {
			log.Printf("[任务 %s] %s，时间：%s ：请勿重复请求，等待重试...", taskID, courtName, timeSlot)
			tm.UpdateTaskStatus(taskID, "running", "请勿重复请求，等待重试...")
		} else if strings.Contains(string(bodyBytes), "200") {
			// 设置邮件正文
			err := SendMessage(db.GetEmailByUserName(username), date, courtName, timeSlot, "预定成功！请及时支付")
			if err != nil {
				log.Printf("[任务 %s] 邮件发送失败: %v", taskID, err)
			}
			log.Printf("[任务 %s] %s，时间：%s ：预定成功!", taskID, courtName, timeSlot)
			return nil
		} else if strings.Contains(string(bodyBytes), "请求失败") {
			tm.UpdateTaskStatus(taskID, "running", "请求失败，等待重试...")
		} else {
			// 如果是其他错误，直接返回
			log.Printf("[任务 %s] Error:%s", taskID, string(bodyBytes))
			tm.UpdateTaskStatus(taskID, "running", fmt.Sprintf("未知错误: %s", string(bodyBytes)))
		}

		// 等待时检查取消
		select {
		case <-ctx.Done():
			return fmt.Errorf("任务已被用户取消")
		case <-time.After(retryInterval):
		}
	}

	return fmt.Errorf("超过最大重试时间，预定失败")
}

func reserveCourt(token string, sessionIDs []string, fieldID, date, courtName, sportTypeID, username, timeSlot string) error {
	// 定义最大重试时间为24小时
	retryDuration := 24 * time.Hour
	// 定义固定的重试间隔时间为5分钟
	retryInterval := 5 * time.Minute
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
		// 在每次重试前，先查询场地状态
		log.Printf("查询场地状态：%s，时间：%s", courtName, timeSlot)
		sessions, err := GetSessions(token, fieldID, date, sportTypeID)
		if err != nil {
			log.Printf("查询场地状态失败: %v，等待重试...", err)
			time.Sleep(retryInterval)
			continue
		}

		// 检查对应的场地是否可预定
		isAvailable := false
		var sessionStatus string
		for _, sessionGroup := range sessions {
			for _, session := range sessionGroup {
				if session.PlaceName == courtName && session.OpenStartTime == timeSlot {
					sessionStatus = session.SessionsStatus
					if session.SessionsStatus == "NO_RESERVED" {
						isAvailable = true
					}
					break
				}
			}
			if sessionStatus != "" {
				break
			}
		}

		// 如果场地不可预定，发送邮件通知并停止重试
		if !isAvailable {
			log.Printf("%s，时间：%s ：场地状态为 %s，不可预定", courtName, timeSlot, sessionStatus)
			email := db.GetEmailByUserName(username)
			if email != "" {
				statusMsg := ""
				switch sessionStatus {
				case "RESERVED":
					statusMsg = "已被预订"
				case "EXPIRED":
					statusMsg = "已过期"
				case "":
					statusMsg = "未找到该场地"
				default:
					statusMsg = sessionStatus
				}
				err := SendMessage(email, date, courtName, timeSlot, fmt.Sprintf("预订失败：场地状态为【%s】，无法预订", statusMsg))
				if err != nil {
					log.Printf("邮件发送失败: %v", err)
				} else {
					log.Printf("已发送失败通知邮件至: %s", email)
				}
			}
			return fmt.Errorf("场地状态为 %s，不可预定", sessionStatus)
		}

		log.Printf("%s，时间：%s ：场地可预定，开始预订", courtName, timeSlot)

		// 场地可预定，发送预订请求
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

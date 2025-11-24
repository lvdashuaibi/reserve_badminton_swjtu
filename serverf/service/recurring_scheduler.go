package service

import (
	"log"
	"serverf/entity"
	db "serverf/midware"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RecurringScheduler 定期任务调度器
type RecurringScheduler struct {
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
}

var (
	recurringScheduler     *RecurringScheduler
	recurringSchedulerOnce sync.Once
)

// GetRecurringScheduler 获取定期任务调度器单例
func GetRecurringScheduler() *RecurringScheduler {
	recurringSchedulerOnce.Do(func() {
		recurringScheduler = &RecurringScheduler{
			running: false,
			stopCh:  make(chan struct{}),
		}
	})
	return recurringScheduler
}

// Start 启动定期任务调度器
func (rs *RecurringScheduler) Start() {
	rs.mu.Lock()
	if rs.running {
		rs.mu.Unlock()
		return
	}
	rs.running = true
	rs.mu.Unlock()

	log.Println("[定期任务调度器] 启动")

	// 每天检查一次是否需要创建预订任务
	ticker := time.NewTicker(1 * time.Hour) // 每小时检查一次
	defer ticker.Stop()

	// 立即执行一次检查
	rs.checkAndCreateTasks()

	for {
		select {
		case <-ticker.C:
			rs.checkAndCreateTasks()
		case <-rs.stopCh:
			log.Println("[定期任务调度器] 停止")
			return
		}
	}
}

// Stop 停止定期任务调度器
func (rs *RecurringScheduler) Stop() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.running {
		close(rs.stopCh)
		rs.running = false
	}
}

// checkAndCreateTasks 检查并创建预订任务
func (rs *RecurringScheduler) checkAndCreateTasks() {
	log.Println("[定期任务调度器] 开始检查定期预订配置")

	// 获取所有启用的定期预订配置
	reservations, err := db.GetEnabledRecurringReservations()
	if err != nil {
		log.Printf("[定期任务调度器] 获取定期预订配置失败: %v", err)
		return
	}

	if len(reservations) == 0 {
		log.Println("[定期任务调度器] 没有启用的定期预订配置")
		return
	}

	log.Printf("[定期任务调度器] 找到 %d 个启用的定期预订配置", len(reservations))

	now := time.Now()

	// 遍历所有定期预订配置
	for _, reservation := range reservations {
		// 计算下一个符合条件的日期
		nextDate := rs.getNextReservationDate(now, reservation.Weekday)

		// 检查是否应该创建任务（提前两天）
		daysUntilReservation := int(nextDate.Sub(now).Hours() / 24)

		// 如果距离预订日期还有2天，则创建任务
		if daysUntilReservation == 2 {
			rs.createReservationTask(&reservation, nextDate)
		}
	}
}

// getNextReservationDate 获取下一个符合星期几条件的日期
func (rs *RecurringScheduler) getNextReservationDate(now time.Time, targetWeekday int) time.Time {
	// 计算当前是星期几（0=周日, 1=周一, ..., 6=周六）
	currentWeekday := int(now.Weekday())

	// 计算需要前进的天数
	daysToAdd := (targetWeekday - currentWeekday + 7) % 7
	if daysToAdd == 0 {
		daysToAdd = 7 // 如果是同一天，则取下周的这一天
	}

	nextDate := now.AddDate(0, 0, daysToAdd)
	return time.Date(nextDate.Year(), nextDate.Month(), nextDate.Day(), 0, 0, 0, 0, nextDate.Location())
}

// createReservationTask 创建预订任务（智能策略）
func (rs *RecurringScheduler) createReservationTask(reservation *db.RecurringReservation, reserveDate time.Time) {
	log.Printf("[定期任务调度器] 为用户 %s 创建预订任务：%s, 日期=%s",
		reservation.Username, reservation.Name, reserveDate.Format("2006-01-02"))

	// 解析场地号和时间段
	courtNums := strings.Split(reservation.CourtNums, ",")
	timeSlots := strings.Split(reservation.TimeSlots, ",")

	// 获取用户token
	token, err := db.GetTokenByUserName(reservation.Username)
	if err != nil {
		log.Printf("[定期任务调度器] 获取用户token失败: %v", err)
		return
	}

	// 查询可用场地
	dateStr := reserveDate.Format("2006-01-02")
	availableSlots := rs.getAvailableSlots(token, dateStr, courtNums, timeSlots)

	if len(availableSlots) == 0 {
		log.Printf("[定期任务调度器] 没有可用的场地和时间段")
		return
	}

	// 生成最优预订策略
	reservationGroups := rs.generateOptimalReservationGroups(availableSlots, courtNums, timeSlots)

	log.Printf("[定期任务调度器] 生成了 %d 个预订请求组", len(reservationGroups))

	// 为每个预订组创建任务
	for i, group := range reservationGroups {
		log.Printf("[定期任务调度器] 创建预订组 %d: 场地=%v, 时间=%v", i+1, group.CourtNums, group.TimeSlots)

		req := entity.GetSessionIDRequest{
			Date:      dateStr,
			CourtNums: group.CourtNums,
			TimeSlots: group.TimeSlots,
		}

		// 创建gin.Context
		c := &gin.Context{}
		c.Set("username", reservation.Username)

		// 调用预订函数
		_, err := Reservation(c, req)
		if err != nil {
			log.Printf("[定期任务调度器] 创建预订任务失败: %v", err)
		} else {
			log.Printf("[定期任务调度器] 成功创建预订任务")
		}
	}
}

// SlotInfo 场地时间段信息
type SlotInfo struct {
	CourtNum string
	TimeSlot string
}

// ReservationGroup 预订组
type ReservationGroup struct {
	CourtNums []string
	TimeSlots []string
}

// getAvailableSlots 获取可用的场地和时间段
func (rs *RecurringScheduler) getAvailableSlots(token, date string, courtNums, timeSlots []string) []SlotInfo {
	var availableSlots []SlotInfo

	// 查询场地信息
	sessions, err := GetSessions(token, entity.FieldID, date, entity.SportTypeID)
	if err != nil {
		log.Printf("[定期任务调度器] 查询场地信息失败: %v", err)
		return availableSlots
	}

	// 遍历所有场地和时间段组合，检查是否可用
	for _, courtNum := range courtNums {
		courtName := courtNum + "号羽毛球"
		for _, timeSlot := range timeSlots {
			// 检查这个场地和时间段是否可用
			for _, sessionGroup := range sessions {
				for _, session := range sessionGroup {
					if session.PlaceName == courtName &&
						session.OpenStartTime == timeSlot &&
						session.SessionsStatus == "NO_RESERVED" {
						availableSlots = append(availableSlots, SlotInfo{
							CourtNum: courtNum,
							TimeSlot: timeSlot,
						})
						break
					}
				}
			}
		}
	}

	log.Printf("[定期任务调度器] 找到 %d 个可用的场地时间段", len(availableSlots))
	return availableSlots
}

// generateOptimalReservationGroups 生成最优预订策略
// 策略：尽可能让每个场地预订连续的时间段，增加预订成功率
func (rs *RecurringScheduler) generateOptimalReservationGroups(availableSlots []SlotInfo, courtNums, timeSlots []string) []ReservationGroup {
	var groups []ReservationGroup

	// 按场地分组
	courtSlots := make(map[string][]string)
	for _, slot := range availableSlots {
		courtSlots[slot.CourtNum] = append(courtSlots[slot.CourtNum], slot.TimeSlot)
	}

	// 为每个场地生成连续时间段的预订组
	for courtNum, slots := range courtSlots {
		if len(slots) == 0 {
			continue
		}

		// 找出连续的时间段
		consecutiveGroups := rs.findConsecutiveTimeSlots(slots, timeSlots)

		// 为每个连续时间段组创建预订请求
		for _, consecutiveSlots := range consecutiveGroups {
			if len(consecutiveSlots) > 0 {
				groups = append(groups, ReservationGroup{
					CourtNums: []string{courtNum},
					TimeSlots: consecutiveSlots,
				})
			}
		}
	}

	// 如果某些时间段在多个场地都可用，创建额外的预订组（增加成功率）
	for _, timeSlot := range timeSlots {
		availableCourts := []string{}
		for _, slot := range availableSlots {
			if slot.TimeSlot == timeSlot {
				availableCourts = append(availableCourts, slot.CourtNum)
			}
		}

		// 如果这个时间段有多个场地可用，且还没有被单独预订，创建一个组合预订
		if len(availableCourts) > 1 {
			groups = append(groups, ReservationGroup{
				CourtNums: availableCourts,
				TimeSlots: []string{timeSlot},
			})
		}
	}

	return groups
}

// findConsecutiveTimeSlots 找出连续的时间段
func (rs *RecurringScheduler) findConsecutiveTimeSlots(availableSlots, allTimeSlots []string) [][]string {
	var result [][]string

	// 创建时间段索引映射
	timeSlotIndex := make(map[string]int)
	for i, slot := range allTimeSlots {
		timeSlotIndex[slot] = i
	}

	// 标记哪些时间段可用
	available := make(map[int]bool)
	for _, slot := range availableSlots {
		if idx, ok := timeSlotIndex[slot]; ok {
			available[idx] = true
		}
	}

	// 找出连续的时间段
	var currentGroup []string
	for i := 0; i < len(allTimeSlots); i++ {
		if available[i] {
			currentGroup = append(currentGroup, allTimeSlots[i])
		} else {
			if len(currentGroup) > 0 {
				result = append(result, currentGroup)
				currentGroup = []string{}
			}
		}
	}

	// 添加最后一组
	if len(currentGroup) > 0 {
		result = append(result, currentGroup)
	}

	return result
}

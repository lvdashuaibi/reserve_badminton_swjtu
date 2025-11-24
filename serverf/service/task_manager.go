package service

import (
	"context"
	"sync"
	"time"
)

// ReservationTask 预订任务结构
type ReservationTask struct {
	TaskID       string             `json:"taskId"`       // 任务ID
	Username     string             `json:"username"`     // 用户名
	Date         string             `json:"date"`         // 预订日期
	CourtName    string             `json:"courtName"`    // 场地名称
	TimeSlot     string             `json:"timeSlot"`     // 时间段
	Status       string             `json:"status"`       // 状态：running, success, failed, cancelled
	SubmitTime   time.Time          `json:"submitTime"`   // 提交时间（任务创建时间）
	StartTime    *time.Time         `json:"startTime"`    // 开始时间（开始执行预订的时间）
	ScheduleTime time.Time          `json:"scheduleTime"` // 计划执行时间
	Message      string             `json:"message"`      // 状态消息
	CancelFunc   context.CancelFunc `json:"-"`            // 取消函数
}

// TaskManager 任务管理器
type TaskManager struct {
	tasks map[string]*ReservationTask
	mu    sync.RWMutex
}

var (
	taskManager *TaskManager
	once        sync.Once
)

// GetTaskManager 获取任务管理器单例
func GetTaskManager() *TaskManager {
	once.Do(func() {
		taskManager = &TaskManager{
			tasks: make(map[string]*ReservationTask),
		}
	})
	return taskManager
}

// AddTask 添加任务
func (tm *TaskManager) AddTask(task *ReservationTask) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tasks[task.TaskID] = task
}

// GetTask 获取任务
func (tm *TaskManager) GetTask(taskID string) (*ReservationTask, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	task, exists := tm.tasks[taskID]
	return task, exists
}

// GetUserTasks 获取用户的所有任务
func (tm *TaskManager) GetUserTasks(username string) []*ReservationTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var userTasks []*ReservationTask
	for _, task := range tm.tasks {
		if task.Username == username {
			userTasks = append(userTasks, task)
		}
	}
	return userTasks
}

// GetAllTasks 获取所有任务（仅管理员）
func (tm *TaskManager) GetAllTasks() []*ReservationTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var allTasks []*ReservationTask
	for _, task := range tm.tasks {
		allTasks = append(allTasks, task)
	}
	return allTasks
}

// UpdateTaskStatus 更新任务状态
func (tm *TaskManager) UpdateTaskStatus(taskID, status, message string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[taskID]; exists {
		task.Status = status
		task.Message = message
	}
}

// UpdateTaskStartTime 更新任务开始时间
func (tm *TaskManager) UpdateTaskStartTime(taskID string, startTime time.Time) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[taskID]; exists {
		task.StartTime = &startTime
	}
}

// CancelTask 取消任务
func (tm *TaskManager) CancelTask(taskID string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[taskID]; exists {
		if task.Status == "running" && task.CancelFunc != nil {
			task.CancelFunc()
			task.Status = "cancelled"
			task.Message = "任务已被用户取消"
			return true
		}
	}
	return false
}

// RemoveTask 删除任务
func (tm *TaskManager) RemoveTask(taskID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.tasks, taskID)
}

// CleanupOldTasks 清理旧任务（超过24小时的已完成任务）
func (tm *TaskManager) CleanupOldTasks() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	for taskID, task := range tm.tasks {
		// 使用提交时间来判断任务是否过期
		if task.Status != "running" && now.Sub(task.SubmitTime) > 24*time.Hour {
			delete(tm.tasks, taskID)
		}
	}
}

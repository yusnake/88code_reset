package scheduler

import (
	"fmt"
	"time"

	"code88reset/internal/api"
	"code88reset/internal/models"
	"code88reset/internal/reset"
	"code88reset/internal/storage"
	"code88reset/pkg/logger"
)

const (
	// 北京时区
	BeijingTimezone = "Asia/Shanghai"

	// 重置时间配置
	FirstResetHour   = 18
	FirstResetMinute = 50

	SecondResetHour   = 23
	SecondResetMinute = 55

	// 最小间隔时间（5小时）
	MinResetInterval = 5 * time.Hour
)

// Scheduler 调度器
type Scheduler struct {
	apiClient          *api.Client
	storage            *storage.Storage
	location           *time.Location
	creditThresholdMax float64 // 额度上限百分比（0-100），当额度>上限时跳过重置
	creditThresholdMin float64 // 额度下限百分比（0-100），当额度<下限时才执行重置
	useMaxThreshold    bool    // true=使用上限模式，false=使用下限模式
	enableFirstReset   bool    // 是否启用18:55重置
	loop               *loopController
	accountUpdater     accountUpdater
	logAgg             *logAggregator
}

// NewScheduler 创建新的调度器
func NewScheduler(apiClient *api.Client, storage *storage.Storage, timezone string) (*Scheduler, error) {
	return NewSchedulerWithConfig(apiClient, storage, timezone, 83.0, 0, true, false)
}

// NewSchedulerWithConfig 创建带配置的调度器
func NewSchedulerWithConfig(apiClient *api.Client, storage *storage.Storage, timezone string, thresholdMax, thresholdMin float64, useMax bool, enableFirstReset bool) (*Scheduler, error) {
	// 使用配置的时区，如果未设置则使用默认时区
	if timezone == "" {
		timezone = BeijingTimezone
	}

	// 加载时区
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("加载时区失败 (%s): %w", timezone, err)
	}

	return &Scheduler{
		apiClient:          apiClient,
		storage:            storage,
		location:           loc,
		creditThresholdMax: thresholdMax,
		creditThresholdMin: thresholdMin,
		useMaxThreshold:    useMax,
		enableFirstReset:   enableFirstReset,
		loop:               newLoopController(),
		accountUpdater:     newAccountUpdater(storage),
		logAgg:             newLogAggregator("单账号调度器", 5*time.Minute),
	}, nil
}

// Start 启动调度器
func (s *Scheduler) Start() {
	logger.Info("========================================")
	logger.Info("调度器启动")
	logger.Info("时区: %s", s.location.String())
	if s.enableFirstReset {
		logger.Info("第一次重置时间: %02d:%02d (已启用)", FirstResetHour, FirstResetMinute)
	} else {
		logger.Info("第一次重置时间: %02d:%02d (已禁用)", FirstResetHour, FirstResetMinute)
	}
	logger.Info("第二次重置时间: %02d:%02d", SecondResetHour, SecondResetMinute)

	// 显示额度判断模式
	if s.useMaxThreshold && s.creditThresholdMax > 0 {
		logger.Info("额度判断模式: 上限模式 - 当额度 > %.1f%% 时跳过18点重置", s.creditThresholdMax)
	} else if !s.useMaxThreshold && s.creditThresholdMin > 0 {
		logger.Info("额度判断模式: 下限模式 - 当额度 < %.1f%% 时才执行18点重置", s.creditThresholdMin)
	} else {
		logger.Info("额度判断模式: 已禁用")
	}

	logger.Info("========================================")
	s.loop.run(s.checkAndExecute)
	s.logAgg.Flush()
	logger.Info("调度器已停止")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	logger.Info("正在停止调度器...")
	s.loop.Stop()
	s.logAgg.Flush()
}

// checkAndExecute 检查并执行重置任务
func (s *Scheduler) checkAndExecute() {
	now := time.Now().In(s.location)
	currentHour := now.Hour()
	currentMinute := now.Minute()

	s.logAgg.Add("检查时间: %s", now.Format("2006-01-02 15:04:05"))

	// 检查是否需要执行第一次重置（18:50）
	if currentHour == FirstResetHour && currentMinute == FirstResetMinute {
		s.logAgg.Flush()
		if !s.enableFirstReset {
			logger.Info("========================================")
			logger.Info("触发第一次重置检查（18:50）")
			logger.Info("第一次重置已禁用，跳过执行")
			logger.Info("========================================")
			return
		}
		s.executeReset("first")
		return
	}

	// 检查是否需要执行第二次重置（23:55）
	if currentHour == SecondResetHour && currentMinute == SecondResetMinute {
		s.logAgg.Flush()
		s.executeReset("second")
		return
	}
}

// executeReset 执行重置逻辑
func (s *Scheduler) executeReset(resetType string) {
	s.logAgg.Flush()
	resetTypeText := map[string]string{"first": "第一次", "second": "第二次"}[resetType]
	logger.Info("========================================")
	logger.Info("触发%s重置任务", resetTypeText)
	logger.Info("========================================")

	// 记录到系统日志
	s.storage.AddSystemLog("info", fmt.Sprintf("触发%s重置任务", resetTypeText))

	// 尝试获取锁
	operation := fmt.Sprintf("%s_reset", resetType)
	if err := s.storage.AcquireLock(operation); err != nil {
		logger.Warn("无法获取锁: %v", err)
		return
	}
	defer s.storage.ReleaseLock()

	// 加载状态
	status, err := s.storage.LoadStatus()
	if err != nil {
		logger.Error("加载状态失败: %v", err)
		return
	}

	// 检查今天是否已经执行过此次重置
	if resetType == "first" && status.FirstResetToday {
		logger.Info("今天已执行过第一次重置，跳过")
		s.storage.AddSystemLog("info", "今天已执行过第一次重置，跳过")
		return
	}
	if resetType == "second" && status.SecondResetToday {
		logger.Info("今天已执行过第二次重置，跳过")
		s.storage.AddSystemLog("info", "今天已执行过第二次重置，跳过")
		return
	}

	// 检查两次重置的时间间隔
	if resetType == "second" && status.LastFirstResetTime != nil {
		interval := time.Since(*status.LastFirstResetTime)
		if interval < MinResetInterval {
			logger.Warn("距离第一次重置时间不足5小时（%.1f小时），跳过", interval.Hours())
			return
		}
	}

	logger.Info("正在获取目标订阅信息...")
	runner := reset.NewRunner(
		s.apiClient,
		reset.Filter{TargetPlans: s.apiClient.TargetPlans, RequireMonthly: true},
		reset.Options{
			ResetType:          resetType,
			UseMaxThreshold:    s.useMaxThreshold,
			CreditThresholdMax: s.creditThresholdMax,
			CreditThresholdMin: s.creditThresholdMin,
		},
	)

	results, err := runner.Execute()
	if err != nil {
		logger.Error("执行重置失败: %v", err)
		s.storage.AddSystemLog("error", fmt.Sprintf("执行%s重置失败: %v", resetTypeText, err))
		s.recordFailure(status, err.Error(), resetType)
		return
	}

	if len(results) == 0 {
		logger.Warn("未找到需要处理的订阅")
		s.storage.AddSystemLog("warning", fmt.Sprintf("%s重置: 未找到需要处理的订阅", resetTypeText))
		s.recordSkip(status, resetType, "无匹配订阅")
		return
	}

	reset.LogResults(results)

	anySuccess := false
	anyError := false
	lastMessage := ""

	for _, res := range results {
		if res.Err != nil {
			anyError = true
			lastMessage = fmt.Sprintf("[%s] %v", res.Subscription.SubscriptionName, res.Err)
			s.storage.AddSystemLog("error", fmt.Sprintf("%s重置失败: %s", resetTypeText, lastMessage))
			continue
		}
		if res.Skipped {
			lastMessage = fmt.Sprintf("[%s] 跳过: %s", res.Subscription.SubscriptionName, res.SkipReason)
			s.storage.AddSystemLog("info", fmt.Sprintf("%s重置跳过: %s", resetTypeText, lastMessage))
			continue
		}

		anySuccess = true
		lastMessage = fmt.Sprintf("[%s] %s", res.Subscription.SubscriptionName, res.ResetResponse.Message)
		s.storage.AddSystemLog("success", fmt.Sprintf("%s重置成功: %s", resetTypeText, lastMessage))
		status.ResetTimesBeforeReset = res.BeforeResets
		status.CreditsBeforeReset = res.BeforeCredits
		status.ResetTimesAfterReset = res.AfterResets
		status.CreditsAfterReset = res.AfterCredits

		if res.UpdatedSubscription != nil {
			s.updateAccountInfo(res.UpdatedSubscription)
		} else {
			s.updateAccountInfo(&res.Subscription)
		}
	}

	now := time.Now()
	if resetType == "first" {
		status.FirstResetToday = true
		status.LastFirstResetTime = &now
	} else {
		status.SecondResetToday = true
		status.LastSecondResetTime = &now
	}

	status.LastResetMessage = lastMessage

	if anySuccess {
		status.LastResetSuccess = true
		status.ConsecutiveFailures = 0
	} else if anyError {
		status.LastResetSuccess = false
		status.ConsecutiveFailures++
	} else {
		status.LastResetSuccess = true
	}

	if err := s.storage.SaveStatus(status); err != nil {
		logger.Error("保存状态失败: %v", err)
	}

	logger.Info("========================================")
	logger.Info("%s重置任务完成", map[string]string{"first": "第一次", "second": "第二次"}[resetType])
	logger.Info("========================================")
}

// updateAccountInfo 更新账号信息
func (s *Scheduler) updateAccountInfo(sub *models.Subscription) {
	s.accountUpdater.UpdateGlobal(sub)
}

func (s *Scheduler) recordFailure(status *models.ExecutionStatus, message, resetType string) {
	now := time.Now()
	if resetType == "first" {
		status.FirstResetToday = true
		status.LastFirstResetTime = &now
	} else {
		status.SecondResetToday = true
		status.LastSecondResetTime = &now
	}
	status.LastResetSuccess = false
	status.LastResetMessage = message
	status.ConsecutiveFailures++
	if err := s.storage.SaveStatus(status); err != nil {
		logger.Error("保存状态失败: %v", err)
	}
}

func (s *Scheduler) recordSkip(status *models.ExecutionStatus, resetType string, reason string) {
	now := time.Now()
	if resetType == "first" {
		status.FirstResetToday = true
		status.LastFirstResetTime = &now
	} else {
		status.SecondResetToday = true
		status.LastSecondResetTime = &now
	}
	status.LastResetSuccess = true
	status.LastResetMessage = fmt.Sprintf("跳过: %s", reason)
	if err := s.storage.SaveStatus(status); err != nil {
		logger.Error("保存状态失败: %v", err)
	}
}

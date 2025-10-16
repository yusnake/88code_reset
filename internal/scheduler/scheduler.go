package scheduler

import (
	"context"
	"fmt"
	"time"

	"code88reset/internal/api"
	"code88reset/internal/models"
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

	// 订阅状态检查间隔（每小时检查一次）
	SubscriptionCheckInterval = 1 * time.Hour
)

// Scheduler 调度器
type Scheduler struct {
	apiClient              *api.Client
	storage                *storage.Storage
	location               *time.Location
	ctx                    context.Context
	cancel                 context.CancelFunc
	lastSubscriptionCheck  time.Time
	creditThresholdMax     float64 // 额度上限百分比（0-100），当额度>上限时跳过重置
	creditThresholdMin     float64 // 额度下限百分比（0-100），当额度<下限时才执行重置
	useMaxThreshold        bool    // true=使用上限模式，false=使用下限模式
	enableFirstReset       bool    // 是否启用18:55重置
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

	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		apiClient:             apiClient,
		storage:               storage,
		location:              loc,
		ctx:                   ctx,
		cancel:                cancel,
		lastSubscriptionCheck: time.Time{}, // 初始化为零值，确保首次检查
		creditThresholdMax:    thresholdMax,
		creditThresholdMin:    thresholdMin,
		useMaxThreshold:       useMax,
		enableFirstReset:      enableFirstReset,
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

	logger.Info("订阅状态检查间隔: %v", SubscriptionCheckInterval)
	logger.Info("========================================")

	// 启动时立即验证目标订阅
	go s.checkSubscriptionStatus()

	// 启动时立即检查一次重置任务
	go s.checkAndExecute()

	// 每分钟检查一次
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			logger.Info("调度器已停止")
			return
		case <-ticker.C:
			// 定期检查订阅状态
			s.periodicSubscriptionCheck()
			// 检查重置任务
			s.checkAndExecute()
		}
	}
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	logger.Info("正在停止调度器...")
	s.cancel()
}

// periodicSubscriptionCheck 定期检查订阅状态
func (s *Scheduler) periodicSubscriptionCheck() {
	now := time.Now()

	// 检查是否需要更新订阅状态（每小时一次）
	if now.Sub(s.lastSubscriptionCheck) >= SubscriptionCheckInterval {
		s.checkSubscriptionStatus()
		s.lastSubscriptionCheck = now
	}
}

// checkSubscriptionStatus 检查并验证目标订阅状态
func (s *Scheduler) checkSubscriptionStatus() {
	logger.Debug("检查目标订阅状态...")

	sub, err := s.apiClient.GetTargetSubscription()
	if err != nil {
		logger.Warn("无法获取目标订阅: %v", err)
		return
	}

	// 更新账号信息
	s.updateAccountInfo(sub)

	logger.Info("订阅状态: 名称=%s, 类型=%s, resetTimes=%d, 积分=%.4f/%.2f",
		sub.SubscriptionName,
		sub.SubscriptionPlan.PlanType,
		sub.ResetTimes,
		sub.CurrentCredits,
		sub.SubscriptionPlan.CreditLimit)

	// 警告：如果 resetTimes 不足
	if sub.ResetTimes < 2 {
		logger.Warn("当前 resetTimes=%d，不足以执行重置（需要 >= 2）", sub.ResetTimes)
	}

	// 警告：如果检测到 PAYGO 类型（理论上不应该出现，因为在 GetTargetSubscription 中已过滤）
	isPAYGO := sub.SubscriptionName == "PAYGO" ||
	           sub.SubscriptionPlan.SubscriptionName == "PAYGO" ||
	           sub.SubscriptionPlan.PlanType == "PAYGO" ||
	           sub.SubscriptionPlan.PlanType == "PAY_PER_USE"

	if isPAYGO {
		logger.Error("🚨 警告：检测到 PAYGO 类型订阅 (名称=%s, 类型=%s)，这不应该发生！",
			sub.SubscriptionName, sub.SubscriptionPlan.PlanType)
	}
}

// checkAndExecute 检查并执行重置任务
func (s *Scheduler) checkAndExecute() {
	now := time.Now().In(s.location)
	currentHour := now.Hour()
	currentMinute := now.Minute()

	logger.Debug("当前北京时间: %s", now.Format("2006-01-02 15:04:05"))

	// 检查是否需要执行第一次重置（18:50）
	if currentHour == FirstResetHour && currentMinute == FirstResetMinute {
		if !s.enableFirstReset {
			logger.Debug("18:55重置已禁用，跳过")
			return
		}
		s.executeReset("first")
		return
	}

	// 检查是否需要执行第二次重置（23:55）
	if currentHour == SecondResetHour && currentMinute == SecondResetMinute {
		s.executeReset("second")
		return
	}
}

// executeReset 执行重置逻辑
func (s *Scheduler) executeReset(resetType string) {
	logger.Info("========================================")
	logger.Info("触发%s重置任务", map[string]string{"first": "第一次", "second": "第二次"}[resetType])
	logger.Info("========================================")

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
		return
	}
	if resetType == "second" && status.SecondResetToday {
		logger.Info("今天已执行过第二次重置，跳过")
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

	// 获取目标订阅信息
	logger.Info("正在获取目标订阅信息...")
	freeSub, err := s.apiClient.GetTargetSubscription()
	if err != nil {
		logger.Error("获取目标订阅失败: %v", err)
		s.updateStatusAfterFailure(status, err.Error())
		return
	}

	// 更新账号信息
	s.updateAccountInfo(freeSub)

	// 检查当前额度百分比（仅在第一次重置时检查）
	if resetType == "first" && freeSub.SubscriptionPlan.PlanType == "MONTHLY" {
		creditPercent := 0.0
		if freeSub.SubscriptionPlan.CreditLimit > 0 {
			creditPercent = (freeSub.CurrentCredits / freeSub.SubscriptionPlan.CreditLimit) * 100
		}

		logger.Info("当前额度: %.4f / %.2f (%.2f%%)",
			freeSub.CurrentCredits,
			freeSub.SubscriptionPlan.CreditLimit,
			creditPercent)

		// 上限模式：当额度>上限时跳过重置
		if s.useMaxThreshold && s.creditThresholdMax > 0 {
			if creditPercent > s.creditThresholdMax {
				logger.Info("上限模式: 当前额度 %.2f%% > %.1f%%，跳过18点重置",
					creditPercent, s.creditThresholdMax)
				s.updateStatusAfterSkip(status, resetType, freeSub,
					fmt.Sprintf("额度充足(%.2f%% > %.1f%%)", creditPercent, s.creditThresholdMax))
				return
			}
			logger.Info("上限模式: 当前额度 %.2f%% <= %.1f%%，继续执行重置",
				creditPercent, s.creditThresholdMax)
		} else if !s.useMaxThreshold && s.creditThresholdMin > 0 {
			// 下限模式：当额度<下限时才执行重置
			if creditPercent >= s.creditThresholdMin {
				logger.Info("下限模式: 当前额度 %.2f%% >= %.1f%%，跳过18点重置",
					creditPercent, s.creditThresholdMin)
				s.updateStatusAfterSkip(status, resetType, freeSub,
					fmt.Sprintf("额度充足(%.2f%% >= %.1f%%)", creditPercent, s.creditThresholdMin))
				return
			}
			logger.Info("下限模式: 当前额度 %.2f%% < %.1f%%，继续执行重置",
				creditPercent, s.creditThresholdMin)
		}
	}

	// 检查 resetTimes
	logger.Info("当前 resetTimes: %d", freeSub.ResetTimes)

	// 第一次重置（18:50）需要至少2次机会，保证留一次给23:55
	// 第二次重置（23:55）只需要至少1次机会
	minRequired := 2
	if resetType == "second" {
		minRequired = 1
	}

	if freeSub.ResetTimes < minRequired {
		logger.Warn("resetTimes=%d < %d，重置次数不足，跳过重置", freeSub.ResetTimes, minRequired)
		s.updateStatusAfterSkip(status, resetType, freeSub, fmt.Sprintf("resetTimes不足(需要>=%d)", minRequired))
		return
	}

	// 记录重置前的状态
	status.ResetTimesBeforeReset = freeSub.ResetTimes
	status.CreditsBeforeReset = freeSub.CurrentCredits

	// 执行重置
	logger.Info("执行重置: subscriptionID=%d, 当前积分=%.4f, resetTimes=%d",
		freeSub.ID, freeSub.CurrentCredits, freeSub.ResetTimes)

	resetResp, err := s.apiClient.ResetCredits(freeSub.ID)
	if err != nil {
		logger.Error("重置失败: %v", err)
		s.updateStatusAfterFailure(status, err.Error())
		return
	}

	// 重置成功，等待几秒后再次获取订阅信息验证
	logger.Info("重置响应: %s", resetResp.Message)
	time.Sleep(3 * time.Second)

	// 验证重置结果
	logger.Info("验证重置结果...")
	freeSubAfter, err := s.apiClient.GetTargetSubscription()
	if err != nil {
		logger.Warn("验证重置结果时获取订阅信息失败: %v", err)
		// 即使验证失败，也认为重置成功（因为API返回成功）
		s.updateStatusAfterSuccess(status, resetType, freeSub, resetResp)
		return
	}

	// 记录重置后的状态
	status.ResetTimesAfterReset = freeSubAfter.ResetTimes
	status.CreditsAfterReset = freeSubAfter.CurrentCredits

	logger.Info("重置后状态: resetTimes=%d, 积分=%.4f",
		freeSubAfter.ResetTimes, freeSubAfter.CurrentCredits)

	// 更新账号信息和状态
	s.updateAccountInfo(freeSubAfter)
	s.updateStatusAfterSuccess(status, resetType, freeSubAfter, resetResp)

	logger.Info("========================================")
	logger.Info("%s重置任务完成", map[string]string{"first": "第一次", "second": "第二次"}[resetType])
	logger.Info("========================================")
}

// updateAccountInfo 更新账号信息
func (s *Scheduler) updateAccountInfo(sub *models.Subscription) {
	account := &models.AccountInfo{
		EmployeeID:         sub.EmployeeID,
		EmployeeName:       sub.EmployeeName,
		EmployeeEmail:      sub.EmployeeEmail,
		FreeSubscriptionID: sub.ID,
		CurrentCredits:     sub.CurrentCredits,
		CreditLimit:        sub.SubscriptionPlan.CreditLimit,
		ResetTimes:         sub.ResetTimes,
		LastCreditReset:    sub.LastCreditReset,
	}

	if err := s.storage.SaveAccountInfo(account); err != nil {
		logger.Error("保存账号信息失败: %v", err)
	} else {
		logger.Debug("账号信息已更新")
	}
}

// updateStatusAfterSuccess 重置成功后更新状态
func (s *Scheduler) updateStatusAfterSuccess(status *models.ExecutionStatus, resetType string, sub *models.Subscription, resp *models.ResetResponse) {
	now := time.Now()

	if resetType == "first" {
		status.FirstResetToday = true
		status.LastFirstResetTime = &now
	} else {
		status.SecondResetToday = true
		status.LastSecondResetTime = &now
	}

	status.LastResetSuccess = true
	status.LastResetMessage = resp.Message
	status.ConsecutiveFailures = 0

	if err := s.storage.SaveStatus(status); err != nil {
		logger.Error("保存状态失败: %v", err)
	}
}

// updateStatusAfterFailure 重置失败后更新状态
func (s *Scheduler) updateStatusAfterFailure(status *models.ExecutionStatus, errorMsg string) {
	status.LastResetSuccess = false
	status.LastResetMessage = errorMsg
	status.ConsecutiveFailures++

	if err := s.storage.SaveStatus(status); err != nil {
		logger.Error("保存状态失败: %v", err)
	}
}

// updateStatusAfterSkip 跳过重置后更新状态
func (s *Scheduler) updateStatusAfterSkip(status *models.ExecutionStatus, resetType string, sub *models.Subscription, reason string) {
	// 标记为已执行（即使跳过），避免重复检查
	now := time.Now()

	if resetType == "first" {
		status.FirstResetToday = true
		status.LastFirstResetTime = &now
	} else {
		status.SecondResetToday = true
		status.LastSecondResetTime = &now
	}

	status.LastResetMessage = fmt.Sprintf("跳过: %s", reason)

	if err := s.storage.SaveStatus(status); err != nil {
		logger.Error("保存状态失败: %v", err)
	}
}

// ManualReset 手动触发重置（用于测试）
func (s *Scheduler) ManualReset() error {
	logger.Info("========================================")
	logger.Info("手动触发重置任务")
	logger.Info("========================================")

	// 尝试获取锁
	if err := s.storage.AcquireLock("manual_reset"); err != nil {
		return fmt.Errorf("无法获取锁: %w", err)
	}
	defer s.storage.ReleaseLock()

	// 获取目标订阅信息
	freeSub, err := s.apiClient.GetTargetSubscription()
	if err != nil {
		return fmt.Errorf("获取目标订阅失败: %w", err)
	}

	logger.Info("目标订阅信息:")
	logger.Info("  名称: %s", freeSub.SubscriptionName)
	logger.Info("  ID: %d", freeSub.ID)
	logger.Info("  类型: %s", freeSub.SubscriptionPlan.PlanType)
	logger.Info("  当前积分: %.4f / %.2f", freeSub.CurrentCredits, freeSub.SubscriptionPlan.CreditLimit)
	logger.Info("  resetTimes: %d", freeSub.ResetTimes)

	if freeSub.ResetTimes < 2 {
		return fmt.Errorf("resetTimes=%d，不满足重置条件（需要 >= 2）", freeSub.ResetTimes)
	}

	logger.Info("\n⚠️  准备执行重置操作...")
	logger.Info("⚠️  这将消耗一次重置机会")
	logger.Info("⚠️  请在主程序中确认后再调用实际的重置接口\n")

	return nil
}

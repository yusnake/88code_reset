package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"code88reset/internal/api"
	"code88reset/internal/models"
	"code88reset/internal/storage"
	"code88reset/pkg/logger"
)

// MultiScheduler 多账号调度器
type MultiScheduler struct {
	activeAccounts         []models.AccountConfig  // 当前活跃的账号列表（从环境变量获取）
	storage                *storage.Storage
	baseURL                string
	targetPlans            []string
	location               *time.Location
	ctx                    context.Context
	cancel                 context.CancelFunc
	lastSubscriptionCheck  time.Time
}

// NewMultiSchedulerWithAccounts 创建新的多账号调度器（使用指定的账号列表）
func NewMultiSchedulerWithAccounts(storage *storage.Storage, baseURL string, activeAccounts []models.AccountConfig, targetPlans []string, timezone string) (*MultiScheduler, error) {
	// 使用配置的时区
	if timezone == "" {
		timezone = BeijingTimezone
	}

	// 加载时区
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("加载时区失败 (%s): %w", timezone, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &MultiScheduler{
		activeAccounts:        activeAccounts,
		storage:               storage,
		baseURL:               baseURL,
		targetPlans:           targetPlans,
		location:              loc,
		ctx:                   ctx,
		cancel:                cancel,
		lastSubscriptionCheck: time.Time{},
	}, nil
}

// Start 启动多账号调度器
func (s *MultiScheduler) Start() {
	logger.Info("========================================")
	logger.Info("多账号调度器启动")
	logger.Info("时区: %s", s.location.String())
	logger.Info("第一次重置时间: %02d:%02d", FirstResetHour, FirstResetMinute)
	logger.Info("第二次重置时间: %02d:%02d", SecondResetHour, SecondResetMinute)
	logger.Info("活跃账号数量: %d", len(s.activeAccounts))
	logger.Info("========================================")

	if len(s.activeAccounts) == 0 {
		logger.Warn("没有活跃的账号，调度器将空转")
	}

	// 启动时立即检查所有账号的订阅状态
	go s.checkAllAccountsStatus()

	// 启动时立即检查一次重置任务
	go s.checkAndExecute()

	// 每分钟检查一次
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			logger.Info("多账号调度器已停止")
			return
		case <-ticker.C:
			// 定期检查所有账号的订阅状态
			s.periodicSubscriptionCheck()
			// 检查重置任务
			s.checkAndExecute()
		}
	}
}

// Stop 停止调度器
func (s *MultiScheduler) Stop() {
	logger.Info("正在停止多账号调度器...")
	s.cancel()
}

// periodicSubscriptionCheck 定期检查所有账号的订阅状态
func (s *MultiScheduler) periodicSubscriptionCheck() {
	now := time.Now()

	// 每小时检查一次
	if now.Sub(s.lastSubscriptionCheck) >= SubscriptionCheckInterval {
		s.checkAllAccountsStatus()
		s.lastSubscriptionCheck = now
	}
}

// checkAllAccountsStatus 检查所有活跃账号的订阅状态
func (s *MultiScheduler) checkAllAccountsStatus() {
	if len(s.activeAccounts) == 0 {
		logger.Debug("没有活跃的账号")
		return
	}

	logger.Info("开始检查 %d 个账号的订阅状态...", len(s.activeAccounts))

	for i, acc := range s.activeAccounts {
		logger.Info("[%d/%d] 检查账号: %s (%s)",
			i+1, len(s.activeAccounts), acc.EmployeeEmail, acc.Name)

		// 创建客户端
		client := api.NewClient(s.baseURL, acc.APIKey, s.targetPlans)
		client.Storage = s.storage

		// 获取目标订阅
		sub, err := client.GetTargetSubscription()
		if err != nil {
			logger.Warn("账号 %s 无法获取目标订阅: %v", acc.EmployeeEmail, err)
			continue
		}

		// 更新账号信息
		s.updateAccountInfo(acc.EmployeeEmail, sub)

		logger.Info("  订阅状态: 名称=%s, 类型=%s, resetTimes=%d, 积分=%.4f/%.2f",
			sub.SubscriptionName,
			sub.SubscriptionPlan.PlanType,
			sub.ResetTimes,
			sub.CurrentCredits,
			sub.SubscriptionPlan.CreditLimit)

		// 警告：resetTimes 不足
		if sub.ResetTimes < 2 {
			logger.Warn("  账号 %s 的 resetTimes=%d，不足以执行重置",
				acc.EmployeeEmail, sub.ResetTimes)
		}
	}

	logger.Info("所有账号订阅状态检查完成")
}

// updateAccountInfo 更新账号信息
func (s *MultiScheduler) updateAccountInfo(employeeEmail string, sub *models.Subscription) {
	accountInfo := &models.AccountInfo{
		KeyID:              "",  // 从配置中获取
		APIKeyName:         "",  // 从配置中获取
		EmployeeID:         sub.EmployeeID,
		EmployeeName:       sub.EmployeeName,
		EmployeeEmail:      sub.EmployeeEmail,
		FreeSubscriptionID: sub.ID,
		CurrentCredits:     sub.CurrentCredits,
		CreditLimit:        sub.SubscriptionPlan.CreditLimit,
		ResetTimes:         sub.ResetTimes,
		LastCreditReset:    sub.LastCreditReset,
		LastUpdated:        time.Now(),
	}

	if err := s.storage.SaveAccountInfoByEmail(employeeEmail, accountInfo); err != nil {
		logger.Warn("保存账号信息失败 (Email=%s): %v", employeeEmail, err)
	}
}

// checkAndExecute 检查并执行重置任务
func (s *MultiScheduler) checkAndExecute() {
	now := time.Now().In(s.location)
	currentHour := now.Hour()
	currentMinute := now.Minute()

	logger.Debug("当前时间: %s", now.Format("2006-01-02 15:04:05"))

	// 检查是否需要执行第一次重置（18:50）
	if currentHour == FirstResetHour && currentMinute == FirstResetMinute {
		s.executeResetForAllAccounts("first")
		return
	}

	// 检查是否需要执行第二次重置（23:55）
	if currentHour == SecondResetHour && currentMinute == SecondResetMinute {
		s.executeResetForAllAccounts("second")
		return
	}
}

// executeResetForAllAccounts 为所有活跃账号执行重置
func (s *MultiScheduler) executeResetForAllAccounts(resetType string) {
	resetName := map[string]string{"first": "第一次", "second": "第二次"}[resetType]

	logger.Info("========================================")
	logger.Info("触发%s重置任务（多账号模式）", resetName)
	logger.Info("========================================")

	if len(s.activeAccounts) == 0 {
		logger.Warn("没有活跃的账号，跳过重置")
		return
	}

	logger.Info("开始为 %d 个账号执行%s重置...", len(s.activeAccounts), resetName)

	// 使用 WaitGroup 并发执行重置
	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	for i, acc := range s.activeAccounts {
		wg.Add(1)
		go func(index int, account models.AccountConfig) {
			defer wg.Done()

			logger.Info("[%d/%d] 开始重置账号: %s (%s)",
				index+1, len(s.activeAccounts), account.EmployeeEmail, account.Name)

			success := s.executeResetForAccount(account, resetType)

			mu.Lock()
			if success {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(i, acc)
	}

	// 等待所有重置完成
	wg.Wait()

	logger.Info("========================================")
	logger.Info("%s重置任务完成: 成功 %d 个，失败 %d 个",
		resetName, successCount, failCount)
	logger.Info("========================================")
}

// executeResetForAccount 为单个账号执行重置
func (s *MultiScheduler) executeResetForAccount(acc models.AccountConfig, resetType string) bool {
	employeeEmail := acc.EmployeeEmail

	// 加载账号的执行状态
	status, err := s.storage.LoadStatusByEmail(employeeEmail)
	if err != nil {
		logger.Error("账号 %s 加载状态失败: %v", employeeEmail, err)
		return false
	}

	// 检查今天是否已经执行过此次重置
	if resetType == "first" && status.FirstResetToday {
		logger.Info("账号 %s 今天已执行过第一次重置，跳过", employeeEmail)
		return true // 返回 true 因为已经完成
	}
	if resetType == "second" && status.SecondResetToday {
		logger.Info("账号 %s 今天已执行过第二次重置，跳过", employeeEmail)
		return true
	}

	// 检查时间间隔
	var lastResetTime *time.Time
	if resetType == "first" {
		lastResetTime = status.LastFirstResetTime
	} else {
		lastResetTime = status.LastSecondResetTime
	}

	if lastResetTime != nil && time.Since(*lastResetTime) < MinResetInterval {
		logger.Warn("账号 %s 距离上次重置时间不足 %v，跳过",
			employeeEmail, MinResetInterval)
		return false
	}

	// 创建客户端
	client := api.NewClient(s.baseURL, acc.APIKey, s.targetPlans)
	client.Storage = s.storage

	// 获取目标订阅
	sub, err := client.GetTargetSubscription()
	if err != nil {
		logger.Error("账号 %s 获取目标订阅失败: %v", employeeEmail, err)
		s.updateResetStatus(employeeEmail, status, resetType, false, err.Error())
		return false
	}

	// 记录重置前的状态
	status.ResetTimesBeforeReset = sub.ResetTimes
	status.CreditsBeforeReset = sub.CurrentCredits

	logger.Info("账号 %s 重置前: resetTimes=%d, credits=%.4f",
		employeeEmail, sub.ResetTimes, sub.CurrentCredits)

	// 执行重置
	resetResp, err := client.ResetCredits(sub.ID)
	if err != nil {
		logger.Error("账号 %s 重置失败: %v", employeeEmail, err)
		s.updateResetStatus(employeeEmail, status, resetType, false, err.Error())
		return false
	}

	logger.Info("账号 %s 重置成功: %s", employeeEmail, resetResp.Message)

	// 等待几秒后获取新状态
	time.Sleep(3 * time.Second)

	// 获取重置后的状态
	subAfter, err := client.GetTargetSubscription()
	if err != nil {
		logger.Warn("账号 %s 获取重置后状态失败: %v", employeeEmail, err)
	} else {
		status.ResetTimesAfterReset = subAfter.ResetTimes
		status.CreditsAfterReset = subAfter.CurrentCredits

		logger.Info("账号 %s 重置后: resetTimes=%d, credits=%.4f",
			employeeEmail, subAfter.ResetTimes, subAfter.CurrentCredits)

		// 更新账号信息
		s.updateAccountInfo(employeeEmail, subAfter)
	}

	// 更新状态
	s.updateResetStatus(employeeEmail, status, resetType, true, resetResp.Message)

	return true
}

// updateResetStatus 更新重置状态
func (s *MultiScheduler) updateResetStatus(employeeEmail string, status *models.ExecutionStatus, resetType string, success bool, message string) {
	now := time.Now()

	if resetType == "first" {
		status.FirstResetToday = true
		status.LastFirstResetTime = &now
	} else {
		status.SecondResetToday = true
		status.LastSecondResetTime = &now
	}

	status.LastResetSuccess = success
	status.LastResetMessage = message

	if success {
		status.ConsecutiveFailures = 0
	} else {
		status.ConsecutiveFailures++
	}

	if err := s.storage.SaveStatusByEmail(employeeEmail, status); err != nil {
		logger.Error("账号 %s 保存状态失败: %v", employeeEmail, err)
	}
}

// ManualResetAllAccounts 手动重置所有活跃账号
func (s *MultiScheduler) ManualResetAllAccounts() error {
	logger.Info("========================================")
	logger.Info("手动触发多账号重置")
	logger.Info("========================================")

	if len(s.activeAccounts) == 0 {
		return fmt.Errorf("没有活跃的账号")
	}

	logger.Info("将为 %d 个账号执行重置...", len(s.activeAccounts))

	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	for i, acc := range s.activeAccounts {
		wg.Add(1)
		go func(index int, account models.AccountConfig) {
			defer wg.Done()

			logger.Info("[%d/%d] 手动重置账号: %s (%s)",
				index+1, len(s.activeAccounts), account.EmployeeEmail, account.Name)

			success := s.manualResetForAccount(account)

			mu.Lock()
			if success {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(i, acc)
	}

	wg.Wait()

	logger.Info("========================================")
	logger.Info("手动重置完成: 成功 %d 个，失败 %d 个", successCount, failCount)
	logger.Info("========================================")

	if failCount > 0 {
		return fmt.Errorf("部分账号重置失败: 成功 %d 个，失败 %d 个", successCount, failCount)
	}

	return nil
}

// manualResetForAccount 手动重置单个账号
func (s *MultiScheduler) manualResetForAccount(acc models.AccountConfig) bool {
	employeeEmail := acc.EmployeeEmail

	// 创建客户端
	client := api.NewClient(s.baseURL, acc.APIKey, s.targetPlans)
	client.Storage = s.storage

	// 获取目标订阅
	sub, err := client.GetTargetSubscription()
	if err != nil {
		logger.Error("账号 %s 获取目标订阅失败: %v", employeeEmail, err)
		return false
	}

	logger.Info("账号 %s 重置前: resetTimes=%d, credits=%.4f",
		employeeEmail, sub.ResetTimes, sub.CurrentCredits)

	// 执行重置
	resetResp, err := client.ResetCredits(sub.ID)
	if err != nil {
		logger.Error("账号 %s 重置失败: %v", employeeEmail, err)
		return false
	}

	logger.Info("账号 %s 重置成功: %s", employeeEmail, resetResp.Message)

	// 等待几秒后获取新状态
	time.Sleep(3 * time.Second)

	// 获取重置后的状态
	subAfter, err := client.GetTargetSubscription()
	if err != nil {
		logger.Warn("账号 %s 获取重置后状态失败: %v", employeeEmail, err)
	} else {
		logger.Info("账号 %s 重置后: resetTimes=%d, credits=%.4f",
			employeeEmail, subAfter.ResetTimes, subAfter.CurrentCredits)

		// 更新账号信息
		s.updateAccountInfo(employeeEmail, subAfter)
	}

	return true
}

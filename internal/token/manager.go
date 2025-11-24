package token

import (
	"fmt"
	"time"

	"code88reset/internal/api"
	"code88reset/internal/models"
	"code88reset/internal/reset"
	"code88reset/pkg/logger"

	"github.com/google/uuid"
)

// Manager Token 管理器
type Manager struct {
	storage       *Storage
	baseURL       string
	systemStorage SystemStorage // 用于记录系统日志
}

// SystemStorage 系统存储接口
type SystemStorage interface {
	AddSystemLog(logType, message string) error
}

// NewManager 创建 Token 管理器
func NewManager(storage *Storage, baseURL string, systemStorage SystemStorage) *Manager {
	return &Manager{
		storage:       storage,
		baseURL:       baseURL,
		systemStorage: systemStorage,
	}
}

// AddToken 添加新 Token 并自动获取订阅详情
func (m *Manager) AddToken(apiKey, name string) (*models.Token, error) {
	// 验证 API Key
	client := api.NewClient(m.baseURL, apiKey, nil)
	if m.systemStorage != nil {
		client.Storage = m.systemStorage
	}

	// 获取订阅详情
	subs, err := client.GetSubscriptions()
	if err != nil {
		return nil, fmt.Errorf("获取订阅失败: %w", err)
	}

	if len(subs) == 0 {
		return nil, fmt.Errorf("该 API Key 没有关联的订阅")
	}

	// 筛选目标订阅（优先选择 MONTHLY 且非 PAYGO 的订阅）
	targetSub := findTargetSubscription(subs)
	if targetSub == nil {
		return nil, fmt.Errorf("未找到合适的订阅（需要 MONTHLY 类型且非 PAYGO）")
	}

	// 构建 Token 对象
	now := time.Now()
	token := &models.Token{
		ID:      uuid.New().String(),
		Name:    name,
		APIKey:  apiKey,
		Enabled: true,
		AddedAt: now,
		Subscription: &models.TokenSubscriptionInfo{
			ID:               targetSub.ID,
			SubscriptionName: targetSub.SubscriptionName,
			PlanType:         targetSub.SubscriptionPlan.PlanType,
			CurrentCredits:   targetSub.CurrentCredits,
			CreditLimit:      targetSub.SubscriptionPlan.CreditLimit,
			CreditPercent:    calculatePercent(targetSub.CurrentCredits, targetSub.SubscriptionPlan.CreditLimit),
			ResetTimes:       targetSub.ResetTimes,
			Status:           targetSub.SubscriptionStatus,
			RemainingDays:    targetSub.RemainingDays,
			EmployeeName:     targetSub.EmployeeName,
			EmployeeEmail:    targetSub.EmployeeEmail,
			StartDate:        targetSub.StartDate,
			EndDate:          targetSub.EndDate,
			LastCreditReset:  targetSub.LastCreditReset,
		},
		SubscriptionUpdatedAt: &now,
	}

	// 保存到存储
	if err := m.storage.Add(token); err != nil {
		return nil, err
	}

	logger.Info("添加 Token 成功: %s (%s - %s)", token.Name, token.Subscription.EmployeeName, token.Subscription.EmployeeEmail)
	return token, nil
}

// RefreshSubscription 刷新 Token 的订阅信息
func (m *Manager) RefreshSubscription(tokenID string) (*models.Token, error) {
	token, err := m.storage.Get(tokenID)
	if err != nil {
		return nil, err
	}

	// 获取最新订阅信息
	client := api.NewClient(m.baseURL, token.APIKey, nil)
	if m.systemStorage != nil {
		client.Storage = m.systemStorage
	}
	subs, err := client.GetSubscriptions()
	if err != nil {
		return nil, fmt.Errorf("获取订阅失败: %w", err)
	}

	targetSub := findTargetSubscription(subs)
	if targetSub == nil {
		return nil, fmt.Errorf("未找到合适的订阅")
	}

	// 更新订阅信息
	now := time.Now()
	token.Subscription = &models.TokenSubscriptionInfo{
		ID:               targetSub.ID,
		SubscriptionName: targetSub.SubscriptionName,
		PlanType:         targetSub.SubscriptionPlan.PlanType,
		CurrentCredits:   targetSub.CurrentCredits,
		CreditLimit:      targetSub.SubscriptionPlan.CreditLimit,
		CreditPercent:    calculatePercent(targetSub.CurrentCredits, targetSub.SubscriptionPlan.CreditLimit),
		ResetTimes:       targetSub.ResetTimes,
		Status:           targetSub.SubscriptionStatus,
		RemainingDays:    targetSub.RemainingDays,
		EmployeeName:     targetSub.EmployeeName,
		EmployeeEmail:    targetSub.EmployeeEmail,
		StartDate:        targetSub.StartDate,
		EndDate:          targetSub.EndDate,
		LastCreditReset:  targetSub.LastCreditReset,
	}
	token.SubscriptionUpdatedAt = &now

	if err := m.storage.Update(token); err != nil {
		return nil, err
	}

	logger.Info("刷新订阅成功: %s (积分: %.2f/%.2f, resetTimes: %d)",
		token.Name, token.Subscription.CurrentCredits, token.Subscription.CreditLimit, token.Subscription.ResetTimes)
	return token, nil
}

// ResetToken 手动重置指定 Token
func (m *Manager) ResetToken(tokenID string, resetType string, thresholdPercent float64) (*models.Token, error) {
	token, err := m.storage.Get(tokenID)
	if err != nil {
		return nil, err
	}

	if !token.Enabled {
		return nil, fmt.Errorf("Token 已禁用")
	}

	// 创建 API 客户端
	client := api.NewClient(m.baseURL, token.APIKey, nil)
	if m.systemStorage != nil {
		client.Storage = m.systemStorage
	}

	// 获取最新订阅信息
	subs, err := client.GetSubscriptions()
	if err != nil {
		return nil, fmt.Errorf("获取订阅失败: %w", err)
	}

	targetSub := findTargetSubscription(subs)
	if targetSub == nil {
		return nil, fmt.Errorf("未找到合适的订阅")
	}

	// 记录重置前状态
	beforeCredits := targetSub.CurrentCredits

	// 执行重置逻辑
	runner := reset.NewRunner(client, reset.Filter{
		TargetPlans:    []string{},
		RequireMonthly: true,
	}, reset.Options{
		ResetType:          resetType,
		UseMaxThreshold:    true,
		CreditThresholdMax: thresholdPercent,
		CreditThresholdMin: 0,
		SleepBetween:       3 * time.Second,
	})

	results, err := runner.Execute()
	if err != nil {
		return nil, fmt.Errorf("执行重置失败: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("没有订阅被重置")
	}

	result := results[0]
	now := time.Now()

	// 更新重置记录
	token.LastReset = &models.TokenResetRecord{
		ResetAt:       now,
		ResetType:     resetType,
		Success:       result.Err == nil && !result.Skipped,
		BeforeCredits: beforeCredits,
		AfterCredits:  result.AfterCredits,
		Message:       formatResetMessage(result),
	}

	// 更新订阅信息
	if result.UpdatedSubscription != nil {
		updatedSub := result.UpdatedSubscription
		token.Subscription = &models.TokenSubscriptionInfo{
			ID:               updatedSub.ID,
			SubscriptionName: updatedSub.SubscriptionName,
			PlanType:         updatedSub.SubscriptionPlan.PlanType,
			CurrentCredits:   updatedSub.CurrentCredits,
			CreditLimit:      updatedSub.SubscriptionPlan.CreditLimit,
			CreditPercent:    calculatePercent(updatedSub.CurrentCredits, updatedSub.SubscriptionPlan.CreditLimit),
			ResetTimes:       updatedSub.ResetTimes,
			Status:           updatedSub.SubscriptionStatus,
			RemainingDays:    updatedSub.RemainingDays,
			EmployeeName:     updatedSub.EmployeeName,
			EmployeeEmail:    updatedSub.EmployeeEmail,
			StartDate:        updatedSub.StartDate,
			EndDate:          updatedSub.EndDate,
			LastCreditReset:  updatedSub.LastCreditReset,
		}
		token.SubscriptionUpdatedAt = &now
	}

	if err := m.storage.Update(token); err != nil {
		return nil, err
	}

	if token.LastReset.Success {
		logger.Info("重置成功: %s (%.2f → %.2f)", token.Name, beforeCredits, token.LastReset.AfterCredits)
	} else {
		logger.Warn("重置失败: %s - %s", token.Name, token.LastReset.Message)
	}

	return token, nil
}

// ToggleToken 切换 Token 启用/禁用状态
func (m *Manager) ToggleToken(tokenID string) (*models.Token, error) {
	token, err := m.storage.Get(tokenID)
	if err != nil {
		return nil, err
	}

	token.Enabled = !token.Enabled

	if err := m.storage.Update(token); err != nil {
		return nil, err
	}

	status := "禁用"
	if token.Enabled {
		status = "启用"
	}
	logger.Info("Token 已%s: %s", status, token.Name)

	return token, nil
}

// DeleteToken 删除 Token
func (m *Manager) DeleteToken(tokenID string) error {
	token, err := m.storage.Get(tokenID)
	if err != nil {
		return err
	}

	if err := m.storage.Delete(tokenID); err != nil {
		return err
	}

	logger.Info("Token 已删除: %s", token.Name)
	return nil
}

// GetToken 获取单个 Token
func (m *Manager) GetToken(tokenID string) (*models.Token, error) {
	return m.storage.Get(tokenID)
}

// ListTokens 获取所有 Token
func (m *Manager) ListTokens() []*models.Token {
	return m.storage.List()
}

// ListEnabledTokens 获取所有启用的 Token
func (m *Manager) ListEnabledTokens() []*models.Token {
	return m.storage.ListEnabled()
}

// findTargetSubscription 查找目标订阅（优先选择 MONTHLY 且非 PAYGO）
func findTargetSubscription(subs []models.Subscription) *models.Subscription {
	// 优先选择 MONTHLY 类型
	for i := range subs {
		sub := &subs[i]
		planType := sub.SubscriptionPlan.PlanType

		// 排除 PAYGO
		if isPAYGO(sub) {
			continue
		}

		// 优先选择 MONTHLY
		if planType == "MONTHLY" {
			return sub
		}
	}

	// 如果没有 MONTHLY，返回第一个非 PAYGO 订阅
	for i := range subs {
		sub := &subs[i]
		if !isPAYGO(sub) {
			return sub
		}
	}

	return nil
}

// isPAYGO 检查是否为 PAYGO 订阅
func isPAYGO(sub *models.Subscription) bool {
	planType := sub.SubscriptionPlan.PlanType
	subscriptionName := sub.SubscriptionName
	planSubscriptionName := sub.SubscriptionPlan.SubscriptionName

	return planType == "PAYGO" ||
		planType == "PAY_PER_USE" ||
		subscriptionName == "PAYGO" ||
		planSubscriptionName == "PAYGO"
}

// calculatePercent 计算百分比
func calculatePercent(current, limit float64) float64 {
	if limit <= 0 {
		return 0
	}
	return (current / limit) * 100
}

// formatResetMessage 格式化重置消息
func formatResetMessage(result reset.Result) string {
	if result.Err != nil {
		return result.Err.Error()
	}
	if result.Skipped {
		return fmt.Sprintf("已跳过: %s", result.SkipReason)
	}
	return fmt.Sprintf("重置成功 (%.2f → %.2f, resetTimes: %d → %d)",
		result.BeforeCredits, result.AfterCredits, result.BeforeResets, result.AfterResets)
}

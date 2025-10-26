package models

import "time"

// Config 应用配置
type Config struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

// AccountConfig 单个账号配置
type AccountConfig struct {
	APIKey        string `json:"api_key"`
	KeyID         string `json:"key_id"`         // API Key 的唯一标识
	EmployeeID    int    `json:"employee_id"`    // 员工 ID
	EmployeeName  string `json:"employee_name"`  // 员工名称
	EmployeeEmail string `json:"employee_email"` // 员工邮箱
	Name          string `json:"name"`           // API Key 名称
	Enabled       bool   `json:"enabled"`        // 是否启用
	AddedAt       string `json:"added_at"`       // 添加时间
}

// MultiAccountConfig 多账号配置
type MultiAccountConfig struct {
	Accounts []AccountConfig `json:"accounts"`
}

// SubscriptionPlan 订阅计划
type SubscriptionPlan struct {
	ID                     int     `json:"id"`
	SubscriptionName       string  `json:"subscriptionName"`
	PlanType               string  `json:"planType"`
	CreditLimit            float64 `json:"creditLimit"`
	Cost                   float64 `json:"cost"`
	BillingCycle           string  `json:"billingCycle"`
	Features               string  `json:"features"`
	ConcurrencyLimit       int     `json:"concurrencyLimit"`
	EnableModelRestriction bool    `json:"enableModelRestriction"`
	RestrictedModels       *string `json:"restrictedModels"`
}

// Subscription 订阅信息
type Subscription struct {
	ID                 int              `json:"id"`
	EmployeeID         int              `json:"employeeId"`
	EmployeeName       string           `json:"employeeName"`
	EmployeeEmail      string           `json:"employeeEmail"`
	SubscriptionPlanID int              `json:"subscriptionPlanId"`
	SubscriptionName   string           `json:"subscriptionPlanName"`
	CurrentCredits     float64          `json:"currentCredits"`
	StartDate          string           `json:"startDate"`
	EndDate            string           `json:"endDate"`
	IsActive           bool             `json:"isActive"`
	AutoRenew          bool             `json:"autoRenew"`
	AutoResetWhenZero  bool             `json:"autoResetWhenZero"`
	Cost               float64          `json:"cost"`
	BillingCycle       string           `json:"billingCycle"`
	BillingCycleDesc   string           `json:"billingCycleDesc"`
	CreatedAt          string           `json:"createdAt"`
	UpdatedAt          string           `json:"updatedAt"`
	LastCreditReset    *string          `json:"lastCreditReset"`
	LastCreditUpdate   *string          `json:"lastCreditUpdate"`
	ResetTimes         int              `json:"resetTimes"`
	RemainingDays      int              `json:"remainingDays"`
	SubscriptionStatus string           `json:"subscriptionStatus"`
	SubscriptionPlan   SubscriptionPlan `json:"subscriptionPlan"`
}

// UsageResponse 用量响应
type UsageResponse struct {
	ID                     int            `json:"id"`
	KeyID                  string         `json:"keyId"` // API Key 的唯一标识
	Name                   string         `json:"name"`  // API Key 名称
	EmployeeID             int            `json:"employeeId"`
	SubscriptionID         int            `json:"subscriptionId"`
	SubscriptionName       string         `json:"subscriptionName"`
	CurrentCredits         float64        `json:"currentCredits"`
	CreditLimit            float64        `json:"creditLimit"`
	SubscriptionEntityList []Subscription `json:"subscriptionEntityList"`
	CreatedAt              string         `json:"createdAt"`
	UpdatedAt              string         `json:"updatedAt"`
}

// ResetResponse 重置响应
type ResetResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		SubscriptionID int     `json:"subscriptionId"`
		NewCredits     float64 `json:"newCredits"`
		ResetAt        string  `json:"resetAt"`
	} `json:"data,omitempty"`
	Error *ErrorResponse `json:"error,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

// AccountInfo 账号信息（持久化到 account.json）
type AccountInfo struct {
	KeyID              string    `json:"key_id"`       // API Key 的唯一标识
	APIKeyName         string    `json:"api_key_name"` // API Key 名称
	EmployeeID         int       `json:"employee_id"`
	EmployeeName       string    `json:"employee_name"`
	EmployeeEmail      string    `json:"employee_email"`
	FreeSubscriptionID int       `json:"free_subscription_id"`
	CurrentCredits     float64   `json:"current_credits"`
	CreditLimit        float64   `json:"credit_limit"`
	ResetTimes         int       `json:"reset_times"`
	LastCreditReset    *string   `json:"last_credit_reset"`
	LastUpdated        time.Time `json:"last_updated"`
}

// ExecutionStatus 执行状态（持久化到 status.json）
type ExecutionStatus struct {
	LastCheckTime         time.Time  `json:"last_check_time"`
	LastFirstResetTime    *time.Time `json:"last_first_reset_time"`
	LastSecondResetTime   *time.Time `json:"last_second_reset_time"`
	FirstResetToday       bool       `json:"first_reset_today"`
	SecondResetToday      bool       `json:"second_reset_today"`
	LastResetSuccess      bool       `json:"last_reset_success"`
	LastResetMessage      string     `json:"last_reset_message"`
	ConsecutiveFailures   int        `json:"consecutive_failures"`
	TodayDate             string     `json:"today_date"` // YYYY-MM-DD 格式
	ResetTimesBeforeReset int        `json:"reset_times_before_reset"`
	ResetTimesAfterReset  int        `json:"reset_times_after_reset"`
	CreditsBeforeReset    float64    `json:"credits_before_reset"`
	CreditsAfterReset     float64    `json:"credits_after_reset"`
}

// LockFile 锁文件
type LockFile struct {
	PID       int       `json:"pid"`
	StartTime time.Time `json:"start_time"`
	Operation string    `json:"operation"`
	Hostname  string    `json:"hostname"`
}

// ResetConfig 重置配置
type ResetConfig struct {
	Enabled          bool    `json:"enabled"`
	Hour             int     `json:"hour"`
	Minute           int     `json:"minute"`
	ThresholdPercent float64 `json:"threshold_percent"`
}

// DynamicConfig 动态配置（可热重载）
type DynamicConfig struct {
	FirstReset  ResetConfig `json:"first_reset"`
	SecondReset ResetConfig `json:"second_reset"`
	Timezone    string      `json:"timezone"`
	WebPort     int         `json:"web_port"`
}

// TokenSubscriptionInfo Token的订阅详情
type TokenSubscriptionInfo struct {
	ID                 int     `json:"id"`
	SubscriptionName   string  `json:"subscription_name"`
	PlanType           string  `json:"plan_type"`
	CurrentCredits     float64 `json:"current_credits"`
	CreditLimit        float64 `json:"credit_limit"`
	CreditPercent      float64 `json:"credit_percent"`
	ResetTimes         int     `json:"reset_times"`
	Status             string  `json:"status"`
	RemainingDays      int     `json:"remaining_days"`
	EmployeeName       string  `json:"employee_name"`
	EmployeeEmail      string  `json:"employee_email"`
	StartDate          string  `json:"start_date"`
	EndDate            string  `json:"end_date"`
	LastCreditReset    *string `json:"last_credit_reset"`
}

// TokenResetRecord Token的重置记录
type TokenResetRecord struct {
	ResetAt       time.Time `json:"reset_at"`
	ResetType     string    `json:"reset_type"` // "first" or "second"
	Success       bool      `json:"success"`
	BeforeCredits float64   `json:"before_credits"`
	AfterCredits  float64   `json:"after_credits"`
	Message       string    `json:"message"`
}

// Token API Token 信息
type Token struct {
	ID                    string                 `json:"id"`
	Name                  string                 `json:"name"`
	APIKey                string                 `json:"api_key"`
	Enabled               bool                   `json:"enabled"`
	AddedAt               time.Time              `json:"added_at"`
	Subscription          *TokenSubscriptionInfo `json:"subscription,omitempty"`
	SubscriptionUpdatedAt *time.Time             `json:"subscription_updated_at,omitempty"`
	LastReset             *TokenResetRecord      `json:"last_reset,omitempty"`
}

// TokenStorage Token 存储结构
type TokenStorage struct {
	Tokens []Token `json:"tokens"`
}

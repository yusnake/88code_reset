package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"code88reset/internal/account"
	"code88reset/internal/app"
	"code88reset/internal/config"
	appconfig "code88reset/internal/config"
	"code88reset/internal/models"
	"code88reset/internal/storage"
	"code88reset/internal/token"
	"code88reset/internal/web"
	"code88reset/pkg/logger"
)

const (
	Version = "v1.6.1" // 应用版本号
)

var (
	mode               = flag.String("mode", "web", "运行模式: web(Web管理模式), test(测试), run(自动调度器), list(列出历史账号)")
	apiKey             = flag.String("apikey", "", "API Key，支持单个或多个（逗号分隔），仅在run/test模式使用")
	apiKeys            = flag.String("apikeys", "", "多个 API Keys（逗号分隔），与 -apikey 等效")
	baseURL            = flag.String("baseurl", appconfig.DefaultBaseURL, "API Base URL")
	dataDir            = flag.String("datadir", appconfig.DefaultDataDir, "数据目录")
	logDir             = flag.String("logdir", appconfig.DefaultLogDir, "日志目录")
	planNames          = flag.String("plans", "", "要重置的订阅计划名称（匹配 subscriptionName），多个用逗号分隔；留空表示所有 MONTHLY 套餐")
	timezone           = flag.String("timezone", "", "时区设置 (例如: Asia/Shanghai, Asia/Hong_Kong, UTC)")
	creditThresholdMax = flag.Float64("threshold-max", 0, "额度上限百分比(0-100)，当额度>上限时跳过18点重置，0表示使用环境变量或默认值83")
	creditThresholdMin = flag.Float64("threshold-min", 0, "额度下限百分比(0-100)，当额度<下限时才执行18点重置，0表示不使用下限")
	enableFirstReset   = flag.Bool("first-reset", false, "是否启用18:55重置，默认关闭（仅run模式）")
	webPort            = flag.Int("webport", 8966, "Web 服务器端口（仅web模式）")
)

func main() {
	flag.Parse()

	if err := logger.Init(*logDir); err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	logger.Info("========================================")
	logger.Info("88code FREE 订阅重置工具")
	logger.Info("========================================")
	logger.Info("运行模式: %s", *mode)

	// 初始化存储
	store, err := storage.NewStorage(*dataDir)
	if err != nil {
		logger.Error("初始化存储失败: %v", err)
		os.Exit(1)
	}

	switch *mode {
	case "web":
		runWebMode(store)
	case "test", "run", "list":
		runLegacyMode(store)
	default:
		logger.Error("未知的运行模式: %s", *mode)
		logger.Error("支持的模式: web, test, run, list")
		os.Exit(1)
	}
}

// runWebMode 运行 Web 管理模式
func runWebMode(store *storage.Storage) {
	logger.Info("启动 Web 管理模式...")

	// 获取管理员 Token
	adminToken := os.Getenv("WEB_ADMIN_TOKEN")
	if adminToken == "" {
		adminToken = "admin123" // 默认管理员密码，生产环境应该修改
		logger.Warn("未设置 WEB_ADMIN_TOKEN 环境变量，使用默认值: admin123")
		logger.Warn("请通过环境变量 WEB_ADMIN_TOKEN 设置自定义管理员密码")
	}

	// 初始化配置管理器
	configMgr, err := config.NewDynamicConfigManager(*dataDir)
	if err != nil {
		logger.Error("初始化配置管理器失败: %v", err)
		os.Exit(1)
	}

	// 初始化 Token 存储和管理器
	tokenStorage, err := token.NewStorage(*dataDir)
	if err != nil {
		logger.Error("初始化 Token 存储失败: %v", err)
		os.Exit(1)
	}

	tokenMgr := token.NewManager(tokenStorage, *baseURL, store)

	// 创建 Web 服务器
	port := *webPort
	if envPort := os.Getenv("WEB_PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", &port)
	}

	webServer := web.NewServer(port, tokenMgr, configMgr, store, adminToken, Version)

	// 启动 Web 服务器（在 goroutine 中）
	go func() {
		if err := webServer.Start(); err != nil {
			logger.Error("Web 服务器启动失败: %v", err)
			os.Exit(1)
		}
	}()

	// 启动定时重置调度器（基于 Token 管理器）
	go runTokenBasedScheduler(tokenMgr, configMgr, store)

	// 等待中断信号
	logger.Info("========================================")
	logger.Info("系统已启动")
	logger.Info("Web 管理界面: http://localhost:%d", port)
	logger.Info("管理员Token: %s", adminToken)
	logger.Info("按 Ctrl+C 停止")
	logger.Info("========================================")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("\n正在关闭服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := webServer.Stop(ctx); err != nil {
		logger.Error("Web 服务器关闭失败: %v", err)
	}

	logger.Info("服务已停止")
}

// runTokenBasedScheduler 基于 Token 管理器运行调度器
func runTokenBasedScheduler(tokenMgr *token.Manager, configMgr *config.DynamicConfigManager, store *storage.Storage) {
	logger.Info("启动定时重置调度器...")

	// 每分钟检查一次
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		checkAndExecuteReset(tokenMgr, configMgr, store)
	}
}

// checkAndExecuteReset 检查并执行重置
func checkAndExecuteReset(tokenMgr *token.Manager, configMgr *config.DynamicConfigManager, store *storage.Storage) {
	cfg := configMgr.GetConfig()

	// 加载时区
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		logger.Error("加载时区失败: %v", err)
		return
	}

	now := time.Now().In(loc)
	currentHour := now.Hour()
	currentMinute := now.Minute()

	// 检查第一次重置
	if cfg.FirstReset.Enabled && currentHour == cfg.FirstReset.Hour && currentMinute == cfg.FirstReset.Minute {
		logger.Info("========================================")
		logger.Info("触发第一次重置任务")
		logger.Info("========================================")
		executeTokenReset(tokenMgr, "first", cfg.FirstReset.ThresholdPercent, store)
		return
	}

	// 检查第二次重置
	if cfg.SecondReset.Enabled && currentHour == cfg.SecondReset.Hour && currentMinute == cfg.SecondReset.Minute {
		logger.Info("========================================")
		logger.Info("触发第二次重置任务")
		logger.Info("========================================")
		executeTokenReset(tokenMgr, "second", cfg.SecondReset.ThresholdPercent, store)
		return
	}
}

// executeTokenReset 执行 Token 重置
func executeTokenReset(tokenMgr *token.Manager, resetType string, threshold float64, store *storage.Storage) {
	// 获取锁
	operation := fmt.Sprintf("%s_reset", resetType)
	if err := store.AcquireLock(operation); err != nil {
		logger.Warn("无法获取锁: %v", err)
		return
	}
	defer store.ReleaseLock()

	// 检查今天是否已经执行过
	status, err := store.LoadStatus()
	if err != nil {
		logger.Warn("加载状态失败: %v", err)
		status = &models.ExecutionStatus{}
	}

	today := time.Now().Format("2006-01-02")
	if status.TodayDate != today {
		// 新的一天，重置标志
		status.TodayDate = today
		status.FirstResetToday = false
		status.SecondResetToday = false
	}

	if resetType == "first" && status.FirstResetToday {
		logger.Info("今天已执行过第一次重置，跳过")
		return
	}

	if resetType == "second" && status.SecondResetToday {
		logger.Info("今天已执行过第二次重置，跳过")
		return
	}

	// 获取所有启用的 Token
	tokens := tokenMgr.ListEnabledTokens()
	if len(tokens) == 0 {
		logger.Warn("没有启用的 Token")
		return
	}

	logger.Info("开始重置 %d 个启用的 Token...", len(tokens))

	successCount := 0
	failCount := 0

	for _, t := range tokens {
		logger.Info("[%d/%d] 重置 Token: %s", successCount+failCount+1, len(tokens), t.Name)

		updatedToken, err := tokenMgr.ResetToken(t.ID, resetType, threshold)
		if err != nil {
			logger.Error("  重置失败: %v", err)
			failCount++
			continue
		}

		if updatedToken.LastReset != nil && updatedToken.LastReset.Success {
			logger.Info("  ✅ 重置成功: %.2f → %.2f",
				updatedToken.LastReset.BeforeCredits,
				updatedToken.LastReset.AfterCredits)
			successCount++
		} else {
			logger.Warn("  ⚠️ 重置跳过: %s",
				updatedToken.LastReset.Message)
			failCount++
		}
	}

	logger.Info("========================================")
	logger.Info("重置完成: 成功 %d, 失败/跳过 %d", successCount, failCount)
	logger.Info("========================================")

	// 更新状态
	if resetType == "first" {
		status.FirstResetToday = true
		now := time.Now()
		status.LastFirstResetTime = &now
	} else {
		status.SecondResetToday = true
		now := time.Now()
		status.LastSecondResetTime = &now
	}

	status.LastResetSuccess = successCount > 0
	status.LastResetMessage = fmt.Sprintf("成功: %d, 失败: %d", successCount, failCount)

	if err := store.SaveStatus(status); err != nil {
		logger.Error("保存状态失败: %v", err)
	}
}

// runLegacyMode 运行传统模式（兼容旧版本）
func runLegacyMode(store *storage.Storage) {
	// 解析配置
	tz := appconfig.GetTimezone(*timezone)
	thresholdMax, thresholdMin, useMax := appconfig.GetCreditThresholds(*creditThresholdMax, *creditThresholdMin)
	firstReset := appconfig.GetEnableFirstReset(*enableFirstReset)

	logger.Info("Base URL: %s", *baseURL)
	logger.Info("时区设置: %s", tz)
	logger.Info("数据目录: %s", *dataDir)
	logger.Info("日志目录: %s", *logDir)
	if strings.TrimSpace(*planNames) == "" {
		logger.Info("目标套餐: 所有 MONTHLY 套餐")
	} else {
		logger.Info("目标套餐: %s", *planNames)
	}
	if useMax {
		logger.Info("额度判断模式: 上限模式 - 当额度 > %.1f%% 时跳过18点重置", thresholdMax)
	} else if thresholdMin > 0 {
		logger.Info("额度判断模式: 下限模式 - 当额度 < %.1f%% 时才执行18点重置", thresholdMin)
	} else {
		logger.Info("额度判断模式: 已禁用")
	}
	logger.Info("18:55重置: %v", firstReset)

	plans := appconfig.ParsePlans(*planNames)
	keys := appconfig.GetAllAPIKeys(*apiKey, *apiKeys)

	accountMgr := account.NewManager(store, *baseURL)

	cfg := appconfig.Settings{
		Mode:               *mode,
		APIKeys:            keys,
		BaseURL:            *baseURL,
		DataDir:            *dataDir,
		LogDir:             *logDir,
		Plans:              plans,
		Timezone:           tz,
		CreditThresholdMax: thresholdMax,
		CreditThresholdMin: thresholdMin,
		UseMaxThreshold:    useMax,
		EnableFirstReset:   firstReset,
	}

	application := app.New(cfg, store, accountMgr)
	if err := application.Run(); err != nil {
		logger.Error("程序运行失败: %v", err)
		os.Exit(1)
	}
}

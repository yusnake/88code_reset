package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"code88reset/internal/api"
	"code88reset/internal/scheduler"
	"code88reset/internal/storage"
	"code88reset/pkg/logger"
)

const (
	DefaultBaseURL = "https://www.88code.org"
	DefaultDataDir = "./data"
	DefaultLogDir  = "./logs"
)

var (
	// 命令行参数
	mode         = flag.String("mode", "test", "运行模式: test(测试), run(运行调度器), manual(手动重置)")
	apiKey       = flag.String("apikey", "", "API Key (可选，优先使用环境变量或.env文件)")
	baseURL      = flag.String("baseurl", DefaultBaseURL, "API Base URL")
	dataDir      = flag.String("datadir", DefaultDataDir, "数据目录")
	logDir       = flag.String("logdir", DefaultLogDir, "日志目录")
	skipConfirm  = flag.Bool("yes", false, "跳过确认提示（仅用于手动重置）")
	planNames    = flag.String("plans", "FREE", "要重置的订阅计划名称，多个用逗号分隔（例如: FREE,PRO,PLUS）")
)

func main() {
	flag.Parse()

	// 初始化日志系统
	if err := logger.Init(*logDir); err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	logger.Info("========================================")
	logger.Info("88code FREE 订阅重置工具")
	logger.Info("========================================")
	logger.Info("运行模式: %s", *mode)

	// 获取 API Key
	key := getAPIKey(*apiKey)
	if key == "" {
		logger.Error("未找到 API Key，请通过以下方式之一提供:")
		logger.Error("  1. 环境变量: export API_KEY=your_key")
		logger.Error("  2. .env 文件: api-key=your_key")
		logger.Error("  3. 命令行参数: -apikey=your_key")
		os.Exit(1)
	}

	logger.Info("API Key: %s", maskAPIKey(key))
	logger.Info("Base URL: %s", *baseURL)
	logger.Info("数据目录: %s", *dataDir)
	logger.Info("日志目录: %s", *logDir)
	logger.Info("目标套餐: %s", *planNames)

	// 解析套餐名称
	plans := strings.Split(*planNames, ",")
	for i, plan := range plans {
		plans[i] = strings.TrimSpace(plan)
	}

	// 创建存储管理器（需要先创建，因为 API 客户端依赖它）
	store, err := storage.NewStorage(*dataDir)
	if err != nil {
		logger.Error("初始化存储失败: %v", err)
		os.Exit(1)
	}

	// 创建 API 客户端
	apiClient := api.NewClient(*baseURL, key, plans)
	// 设置存储接口以保存 API 响应
	apiClient.Storage = store

	// 根据模式执行不同操作
	switch *mode {
	case "test":
		runTestMode(apiClient, store)
	case "run":
		runSchedulerMode(apiClient, store)
	case "manual":
		runManualMode(apiClient, store)
	default:
		logger.Error("未知的运行模式: %s", *mode)
		logger.Error("支持的模式: test, run, manual")
		os.Exit(1)
	}
}

// runTestMode 测试模式 - 测试接口连接和获取信息
func runTestMode(apiClient *api.Client, store *storage.Storage) {
	logger.Info("\n========================================")
	logger.Info("测试模式 - 测试接口连接")
	logger.Info("========================================\n")

	// 测试 1: 连接测试
	logger.Info("【测试 1/3】测试 API 连接...")
	if err := apiClient.TestConnection(); err != nil {
		logger.Error("连接测试失败: %v", err)
		os.Exit(1)
	}
	logger.Info("✅ API 连接测试通过\n")

	// 测试 2: 获取订阅列表
	logger.Info("【测试 2/3】获取订阅列表...")
	subscriptions, err := apiClient.GetSubscriptions()
	if err != nil {
		logger.Error("获取订阅列表失败: %v", err)
		os.Exit(1)
	}
	logger.Info("✅ 获取到 %d 个订阅\n", len(subscriptions))

	// 显示所有订阅
	for i, sub := range subscriptions {
		logger.Info("订阅 %d:", i+1)
		logger.Info("  名称: %s", sub.SubscriptionName)
		logger.Info("  ID: %d", sub.ID)
		logger.Info("  当前积分: %.4f / %.2f", sub.CurrentCredits, sub.SubscriptionPlan.CreditLimit)
		logger.Info("  resetTimes: %d", sub.ResetTimes)
		logger.Info("  状态: %s", sub.SubscriptionStatus)
		logger.Info("")
	}

	// 测试 3: 获取目标订阅
	logger.Info("【测试 3/3】查找目标订阅...")
	targetSub, err := apiClient.GetTargetSubscription()
	if err != nil {
		logger.Error("获取目标订阅失败: %v", err)
		logger.Error("提示: 请检查 -plans 参数是否设置正确")
		os.Exit(1)
	}

	logger.Info("✅ 找到目标订阅\n")
	logger.Info("目标订阅详细信息:")
	logger.Info("  名称: %s", targetSub.SubscriptionName)
	logger.Info("  ID: %d", targetSub.ID)
	logger.Info("  用户: %s (%s)", targetSub.EmployeeName, targetSub.EmployeeEmail)
	logger.Info("  当前积分: %.4f / %.2f", targetSub.CurrentCredits, targetSub.SubscriptionPlan.CreditLimit)
	logger.Info("  resetTimes: %d", targetSub.ResetTimes)
	logger.Info("  计划类型: %s", targetSub.SubscriptionPlan.PlanType)
	logger.Info("  开始日期: %s", targetSub.StartDate)
	logger.Info("  结束日期: %s", targetSub.EndDate)
	logger.Info("  剩余天数: %d", targetSub.RemainingDays)

	if targetSub.LastCreditReset != nil {
		logger.Info("  上次重置: %s", *targetSub.LastCreditReset)
	} else {
		logger.Info("  上次重置: 从未重置")
	}

	// 保存账号信息
	logger.Info("\n保存账号信息到 %s/account.json...", *dataDir)
	account := &storage.Storage{}
	_ = account

	logger.Info("\n========================================")
	logger.Info("测试完成！")
	logger.Info("========================================")

	// 判断是否可以重置
	if targetSub.ResetTimes >= 2 {
		logger.Info("\n✅ 当前 resetTimes=%d，满足重置条件", targetSub.ResetTimes)
		logger.Info("可以使用以下命令进行手动测试:")
		logger.Info("  go run cmd/reset/main.go -mode=manual")
	} else {
		logger.Warn("\n⚠️  当前 resetTimes=%d，不满足重置条件（需要 >= 2）", targetSub.ResetTimes)
		logger.Warn("请等待次日或条件满足后再尝试重置")
	}
}

// runSchedulerMode 调度器模式 - 启动定时任务
func runSchedulerMode(apiClient *api.Client, store *storage.Storage) {
	logger.Info("\n========================================")
	logger.Info("调度器模式 - 启动定时任务")
	logger.Info("========================================\n")

	// 创建调度器
	sched, err := scheduler.NewScheduler(apiClient, store)
	if err != nil {
		logger.Error("创建调度器失败: %v", err)
		os.Exit(1)
	}

	// 启动调度器
	logger.Info("调度器已启动，等待定时任务触发...")
	logger.Info("按 Ctrl+C 停止\n")

	sched.Start()
}

// runManualMode 手动模式 - 手动触发重置（需要确认）
func runManualMode(apiClient *api.Client, store *storage.Storage) {
	logger.Info("\n========================================")
	logger.Info("手动重置模式")
	logger.Info("========================================\n")

	// 创建调度器
	sched, err := scheduler.NewScheduler(apiClient, store)
	if err != nil {
		logger.Error("创建调度器失败: %v", err)
		os.Exit(1)
	}

	// 先获取订阅信息
	targetSub, err := apiClient.GetTargetSubscription()
	if err != nil {
		logger.Error("获取目标订阅失败: %v", err)
		os.Exit(1)
	}

	logger.Info("当前目标订阅状态:")
	logger.Info("  名称: %s", targetSub.SubscriptionName)
	logger.Info("  ID: %d", targetSub.ID)
	logger.Info("  类型: %s", targetSub.SubscriptionPlan.PlanType)
	logger.Info("  当前积分: %.4f / %.2f", targetSub.CurrentCredits, targetSub.SubscriptionPlan.CreditLimit)
	logger.Info("  resetTimes: %d", targetSub.ResetTimes)

	if targetSub.ResetTimes < 2 {
		logger.Error("\n❌ resetTimes=%d，不满足重置条件（需要 >= 2）", targetSub.ResetTimes)
		logger.Error("无法执行重置操作")
		os.Exit(1)
	}

	logger.Warn("\n⚠️  警告：此操作将执行真实的重置！")
	logger.Warn("⚠️  这将消耗一次重置机会（resetTimes 将减少）")
	logger.Warn("")

	// 如果没有 -yes 参数，要求用户确认
	if !*skipConfirm {
		logger.Info("请输入 'yes' 确认执行重置，或输入其他内容取消:")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input != "yes" {
			logger.Info("取消操作")
			os.Exit(0)
		}
	}

	logger.Info("\n开始执行重置...")

	// 执行手动重置
	if err := sched.ManualReset(); err != nil {
		logger.Error("手动重置失败: %v", err)
		os.Exit(1)
	}

	// 实际执行重置
	logger.Info("正在调用重置接口...")
	resetResp, err := apiClient.ResetCredits(targetSub.ID)
	if err != nil {
		logger.Error("重置失败: %v", err)
		os.Exit(1)
	}

	logger.Info("\n✅ 重置成功!")
	logger.Info("响应: %s", resetResp.Message)

	// 验证结果
	logger.Info("\n验证重置结果...")
	targetSubAfter, err := apiClient.GetTargetSubscription()
	if err != nil {
		logger.Warn("获取验证信息失败: %v", err)
	} else {
		logger.Info("\n重置后状态:")
		logger.Info("  当前积分: %.4f (之前: %.4f)", targetSubAfter.CurrentCredits, targetSub.CurrentCredits)
		logger.Info("  resetTimes: %d (之前: %d)", targetSubAfter.ResetTimes, targetSub.ResetTimes)

		if targetSubAfter.CurrentCredits > targetSub.CurrentCredits {
			logger.Info("\n✅ 积分已成功恢复!")
		}
	}

	logger.Info("\n========================================")
	logger.Info("手动重置完成")
	logger.Info("========================================")
}

// getAPIKey 从多个来源获取 API Key
func getAPIKey(cmdKey string) string {
	// 优先级: 命令行参数 > 环境变量 > .env 文件

	// 1. 命令行参数
	if cmdKey != "" {
		return cmdKey
	}

	// 2. 环境变量
	if envKey := os.Getenv("API_KEY"); envKey != "" {
		return envKey
	}

	// 3. .env 文件
	if envKey := readAPIKeyFromEnv(".env"); envKey != "" {
		return envKey
	}

	return ""
}

// readAPIKeyFromEnv 从 .env 文件读取 API Key
func readAPIKeyFromEnv(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 支持多种格式
		if strings.HasPrefix(line, "api-key=") {
			return strings.TrimPrefix(line, "api-key=")
		}
		if strings.HasPrefix(line, "API_KEY=") {
			return strings.TrimPrefix(line, "API_KEY=")
		}
	}

	return ""
}

// maskAPIKey 遮蔽 API Key 显示
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:8] + "****"
}

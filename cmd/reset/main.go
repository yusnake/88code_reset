package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"code88reset/internal/account"
	"code88reset/internal/api"
	"code88reset/internal/scheduler"
	"code88reset/internal/storage"
	"code88reset/pkg/logger"
)

const (
	DefaultBaseURL  = "https://www.88code.org"
	DefaultDataDir  = "./data"
	DefaultLogDir   = "./logs"
	DefaultTimezone = "Asia/Shanghai" // 默认使用北京/上海时区 (UTC+8)
)

var (
	// 命令行参数
	mode         = flag.String("mode", "test", "运行模式: test(测试), run(自动调度器), manual(手动重置), list(列出历史账号)")
	apiKey       = flag.String("apikey", "", "API Key，支持单个或多个（逗号分隔），优先使用环境变量或.env文件")
	apiKeys      = flag.String("apikeys", "", "多个 API Keys（逗号分隔），与 -apikey 等效")
	baseURL      = flag.String("baseurl", DefaultBaseURL, "API Base URL")
	dataDir      = flag.String("datadir", DefaultDataDir, "数据目录")
	logDir       = flag.String("logdir", DefaultLogDir, "日志目录")
	skipConfirm  = flag.Bool("yes", false, "跳过确认提示（仅用于手动重置）")
	planNames    = flag.String("plans", "FREE", "要重置的订阅计划名称，多个用逗号分隔（例如: FREE,PRO,PLUS）")
	timezone     = flag.String("timezone", "", "时区设置 (例如: Asia/Shanghai, Asia/Hong_Kong, UTC)")
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

	// 获取时区配置
	tz := getTimezone(*timezone)

	logger.Info("Base URL: %s", *baseURL)
	logger.Info("时区设置: %s", tz)
	logger.Info("数据目录: %s", *dataDir)
	logger.Info("日志目录: %s", *logDir)
	logger.Info("目标套餐: %s", *planNames)

	// 解析套餐名称
	plans := strings.Split(*planNames, ",")
	for i, plan := range plans {
		plans[i] = strings.TrimSpace(plan)
	}

	// 创建存储管理器
	store, err := storage.NewStorage(*dataDir)
	if err != nil {
		logger.Error("初始化存储失败: %v", err)
		os.Exit(1)
	}

	// 创建账号管理器
	accountMgr := account.NewManager(store, *baseURL)

	// 获取所有 API Keys（统一处理单个和多个）
	keys := getAllAPIKeys(*apiKey, *apiKeys)
	if len(keys) == 0 && *mode != "list" {
		logger.Error("未找到 API Key，请通过以下方式之一提供:")
		logger.Error("  1. 环境变量: export API_KEY=your_key 或 export API_KEYS=key1,key2")
		logger.Error("  2. .env 文件: API_KEY=your_key 或 API_KEYS=key1,key2")
		logger.Error("  3. 命令行参数: -apikey=your_key 或 -apikeys=key1,key2")
		os.Exit(1)
	}

	// 根据模式执行不同操作
	switch *mode {
	case "list":
		runListMode(accountMgr)
	case "test":
		// 测试模式：只测试第一个 API Key
		if len(keys) == 0 {
			logger.Error("测试模式需要至少一个 API Key")
			os.Exit(1)
		}
		logger.Info("测试第一个 API Key: %s", maskAPIKey(keys[0]))
		apiClient := api.NewClient(*baseURL, keys[0], plans)
		apiClient.Storage = store
		runTestMode(apiClient, store, tz)
	case "run":
		// 自动调度器模式：支持单个或多个账号
		if len(keys) == 1 {
			// 单账号模式
			logger.Info("单账号模式 - API Key: %s", maskAPIKey(keys[0]))
			apiClient := api.NewClient(*baseURL, keys[0], plans)
			apiClient.Storage = store
			runSchedulerMode(apiClient, store, tz)
		} else {
			// 多账号模式
			logger.Info("多账号模式 - 检测到 %d 个 API Key", len(keys))
			runMultiAccountMode(accountMgr, store, keys, plans, tz)
		}
	case "manual":
		// 手动重置模式：只重置第一个账号
		if len(keys) == 0 {
			logger.Error("手动重置模式需要至少一个 API Key")
			os.Exit(1)
		}
		if len(keys) > 1 {
			logger.Warn("检测到 %d 个 API Key，手动模式只会重置第一个账号", len(keys))
		}
		logger.Info("手动重置账号 - API Key: %s", maskAPIKey(keys[0]))
		apiClient := api.NewClient(*baseURL, keys[0], plans)
		apiClient.Storage = store
		runManualMode(apiClient, store, tz)
	default:
		logger.Error("未知的运行模式: %s", *mode)
		logger.Error("支持的模式: test, run, manual, list")
		os.Exit(1)
	}
}

// runTestMode 测试模式 - 测试接口连接和获取信息
func runTestMode(apiClient *api.Client, store *storage.Storage, timezone string) {
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
func runSchedulerMode(apiClient *api.Client, store *storage.Storage, timezone string) {
	logger.Info("\n========================================")
	logger.Info("调度器模式 - 启动定时任务")
	logger.Info("========================================\n")

	// 创建调度器
	sched, err := scheduler.NewScheduler(apiClient, store, timezone)
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
func runManualMode(apiClient *api.Client, store *storage.Storage, timezone string) {
	logger.Info("\n========================================")
	logger.Info("手动重置模式")
	logger.Info("========================================\n")

	// 创建调度器
	sched, err := scheduler.NewScheduler(apiClient, store, timezone)
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

// getTimezone 从多个来源获取时区配置
func getTimezone(cmdTimezone string) string {
	// 优先级: 命令行参数 > 环境变量 > .env 文件 > 默认值

	// 1. 命令行参数
	if cmdTimezone != "" {
		return cmdTimezone
	}

	// 2. 环境变量 TZ
	if envTZ := os.Getenv("TZ"); envTZ != "" {
		return envTZ
	}

	// 3. 环境变量 TIMEZONE
	if envTimezone := os.Getenv("TIMEZONE"); envTimezone != "" {
		return envTimezone
	}

	// 4. .env 文件
	if tzFromEnv := readTimezoneFromEnv(".env"); tzFromEnv != "" {
		return tzFromEnv
	}

	// 5. 默认值
	return DefaultTimezone
}

// readTimezoneFromEnv 从 .env 文件读取时区配置
func readTimezoneFromEnv(filename string) string {
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
		if strings.HasPrefix(line, "TZ=") {
			return strings.TrimPrefix(line, "TZ=")
		}
		if strings.HasPrefix(line, "TIMEZONE=") {
			return strings.TrimPrefix(line, "TIMEZONE=")
		}
	}

	return ""
}


// runListMode 列表模式 - 列出所有账号
func runListMode(accountMgr *account.Manager) {
	logger.Info("\n========================================")
	logger.Info("账号列表")
	logger.Info("========================================\n")

	accounts, err := accountMgr.ListAccounts()
	if err != nil {
		logger.Error("获取账号列表失败: %v", err)
		os.Exit(1)
	}

	if len(accounts) == 0 {
		logger.Info("暂无账号，请先导入账号:")
		logger.Info("  go run cmd/reset/main.go -mode=import -apikeys=key1,key2,key3")
		return
	}

	total, enabled, disabled, _ := accountMgr.GetAccountCount()
	logger.Info("账号统计: 总计 %d 个，启用 %d 个，禁用 %d 个\n", total, enabled, disabled)

	for i, acc := range accounts {
		status := "✅ 启用"
		if !acc.Enabled {
			status = "❌ 禁用"
		}

		logger.Info("账号 %d: %s", i+1, status)
		logger.Info("  邮箱: %s", acc.EmployeeEmail)
		logger.Info("  名称: %s", acc.EmployeeName)
		logger.Info("  员工ID: %d", acc.EmployeeID)
		logger.Info("  API Key 名称: %s", acc.Name)
		logger.Info("  API Key ID: %s", acc.KeyID)
		logger.Info("  API Key: %s", maskAPIKey(acc.APIKey))
		logger.Info("  添加时间: %s", acc.AddedAt)
		logger.Info("")
	}

	logger.Info("========================================")
}

// runMultiAccountMode 多账号模式 - 启动多账号调度器
func runMultiAccountMode(accountMgr *account.Manager, store *storage.Storage, apiKeys []string, plans []string, timezone string) {
	logger.Info("\n========================================")
	logger.Info("多账号模式 - 启动多账号调度器")
	logger.Info("========================================\n")

	// 步骤 1: 同步账号信息到持久化配置
	logger.Info("步骤 1/3: 同步账号信息...")
	if err := accountMgr.SyncAccountsFromAPIKeys(apiKeys, plans); err != nil {
		logger.Error("同步账号失败: %v", err)
		os.Exit(1)
	}

	// 步骤 2: 获取当前活跃的账号（仅限当前 API Keys 列表中的）
	logger.Info("\n步骤 2/3: 获取活跃账号...")
	activeAccounts, err := accountMgr.GetActiveAccountsFromAPIKeys(apiKeys)
	if err != nil {
		logger.Error("获取活跃账号失败: %v", err)
		os.Exit(1)
	}

	if len(activeAccounts) == 0 {
		logger.Error("没有活跃的账号，请检查 API Keys 是否正确")
		os.Exit(1)
	}

	// 显示账号统计
	total, _, _, _ := accountMgr.GetAccountCount()
	logger.Info("账号统计: 历史总计 %d 个，当前活跃 %d 个\n", total, len(activeAccounts))

	// 显示活跃账号列表
	logger.Info("活跃账号列表:")
	for i, acc := range activeAccounts {
		logger.Info("  [%d] %s (%s)", i+1, acc.EmployeeEmail, acc.EmployeeName)
	}

	// 步骤 3: 创建并启动多账号调度器
	logger.Info("\n步骤 3/3: 启动调度器...")
	multiSched, err := scheduler.NewMultiSchedulerWithAccounts(store, *baseURL, activeAccounts, plans, timezone)
	if err != nil {
		logger.Error("创建多账号调度器失败: %v", err)
		os.Exit(1)
	}

	// 启动调度器
	logger.Info("\n========================================")
	logger.Info("多账号调度器已启动")
	logger.Info("将为 %d 个账号执行定时重置", len(activeAccounts))
	logger.Info("按 Ctrl+C 停止")
	logger.Info("========================================\n")

	multiSched.Start()
}

// getAllAPIKeys 从多个来源获取所有 API Keys（统一处理单个和多个）
func getAllAPIKeys(cmdKey, cmdKeys string) []string {
	var allKeys []string

	// 优先级: 命令行参数 > 环境变量 > .env 文件

	// 1. 从 -apikeys 参数获取
	if cmdKeys != "" {
		keys := strings.Split(cmdKeys, ",")
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key != "" {
				allKeys = append(allKeys, key)
			}
		}
	}

	// 2. 从 -apikey 参数获取（可能是单个或逗号分隔的多个）
	if cmdKey != "" {
		keys := strings.Split(cmdKey, ",")
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key != "" {
				allKeys = append(allKeys, key)
			}
		}
	}

	// 3. 如果命令行参数都没有，尝试环境变量
	if len(allKeys) == 0 {
		// 尝试 API_KEYS
		if envKeys := os.Getenv("API_KEYS"); envKeys != "" {
			keys := strings.Split(envKeys, ",")
			for _, key := range keys {
				key = strings.TrimSpace(key)
				if key != "" {
					allKeys = append(allKeys, key)
				}
			}
		}

		// 尝试 API_KEY（可能是单个或逗号分隔的多个）
		if len(allKeys) == 0 {
			if envKey := os.Getenv("API_KEY"); envKey != "" {
				keys := strings.Split(envKey, ",")
				for _, key := range keys {
					key = strings.TrimSpace(key)
					if key != "" {
						allKeys = append(allKeys, key)
					}
				}
			}
		}
	}

	// 4. 如果还是没有，尝试 .env 文件
	if len(allKeys) == 0 {
		// 尝试 API_KEYS
		if keysStr := readAPIKeysFromEnv(".env"); keysStr != "" {
			keys := strings.Split(keysStr, ",")
			for _, key := range keys {
				key = strings.TrimSpace(key)
				if key != "" {
					allKeys = append(allKeys, key)
				}
			}
		}

		// 尝试 API_KEY
		if len(allKeys) == 0 {
			if keyStr := readAPIKeyFromEnv(".env"); keyStr != "" {
				keys := strings.Split(keyStr, ",")
				for _, key := range keys {
					key = strings.TrimSpace(key)
					if key != "" {
						allKeys = append(allKeys, key)
					}
				}
			}
		}
	}

	return allKeys
}

// getAPIKeys 从多个来源获取多个 API Keys（保留向后兼容）
func getAPIKeys(cmdKeys string) []string {
	return getAllAPIKeys("", cmdKeys)
}

// readAPIKeysFromEnv 从 .env 文件读取多个 API Keys
func readAPIKeysFromEnv(filename string) string {
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
		if strings.HasPrefix(line, "api-keys=") {
			return strings.TrimPrefix(line, "api-keys=")
		}
		if strings.HasPrefix(line, "API_KEYS=") {
			return strings.TrimPrefix(line, "API_KEYS=")
		}
	}

	return ""
}

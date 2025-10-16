package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"code88reset/internal/api"
	appconfig "code88reset/internal/config"
	"code88reset/internal/models"
	"code88reset/internal/scheduler"
	"code88reset/internal/storage"
	"code88reset/pkg/logger"
)

type accountManager interface {
	ListAccounts() ([]models.AccountConfig, error)
	GetAccountCount() (total, enabled, disabled int, err error)
	SyncAccountsFromAPIKeys(apiKeys []string, targetPlans []string) error
	GetActiveAccountsFromAPIKeys(apiKeys []string) ([]models.AccountConfig, error)
}

type apiClient interface {
	TestConnection() error
	GetSubscriptions() ([]models.Subscription, error)
	GetTargetSubscription() (*models.Subscription, error)
	ResetCredits(subscriptionID int) (*models.ResetResponse, error)
}

type dependencies struct {
	newClient          func(store *storage.Storage, baseURL, apiKey string, plans []string) apiClient
	runSingleScheduler func(app *App, client apiClient) error
	runMultiScheduler  func(app *App, accounts []models.AccountConfig) error
	manualReset        func(app *App, client apiClient) error
	sleep              func(d time.Duration)
}

// App 负责根据配置协调运行模式
type App struct {
	Config     appconfig.Settings
	Store      *storage.Storage
	AccountMgr accountManager
	deps       dependencies
}

// New 创建新的应用实例
func New(cfg appconfig.Settings, store *storage.Storage, accountMgr accountManager) *App {
	return &App{
		Config:     cfg,
		Store:      store,
		AccountMgr: accountMgr,
		deps:       defaultDependencies(),
	}
}

func defaultDependencies() dependencies {
	return dependencies{
		newClient: func(store *storage.Storage, baseURL, apiKey string, plans []string) apiClient {
			client := api.NewClient(baseURL, apiKey, plans)
			client.Storage = store
			return client
		},
		runSingleScheduler: func(app *App, client apiClient) error {
			realClient, ok := client.(*api.Client)
			if !ok {
				return fmt.Errorf("unsupported api client type %T", client)
			}
			sched, err := scheduler.NewSchedulerWithConfig(
				realClient,
				app.Store,
				app.Config.Timezone,
				app.Config.CreditThresholdMax,
				app.Config.CreditThresholdMin,
				app.Config.UseMaxThreshold,
				app.Config.EnableFirstReset,
			)
			if err != nil {
				return err
			}

			sched.Start()
			return nil
		},
		runMultiScheduler: func(app *App, accounts []models.AccountConfig) error {
			multiSched, err := scheduler.NewMultiSchedulerWithConfig(
				app.Store,
				app.Config.BaseURL,
				accounts,
				app.Config.Plans,
				app.Config.Timezone,
				app.Config.CreditThresholdMax,
				app.Config.CreditThresholdMin,
				app.Config.UseMaxThreshold,
				app.Config.EnableFirstReset,
			)
			if err != nil {
				return err
			}

			multiSched.Start()
			return nil
		},
		manualReset: func(app *App, client apiClient) error {
			realClient, ok := client.(*api.Client)
			if !ok {
				return fmt.Errorf("unsupported api client type %T", client)
			}
			sched, err := scheduler.NewSchedulerWithConfig(
				realClient,
				app.Store,
				app.Config.Timezone,
				app.Config.CreditThresholdMax,
				app.Config.CreditThresholdMin,
				app.Config.UseMaxThreshold,
				app.Config.EnableFirstReset,
			)
			if err != nil {
				return err
			}
			return sched.ManualReset()
		},
		sleep: time.Sleep,
	}
}

// Run 根据配置执行对应模式
func (a *App) Run() error {
	keys := a.Config.APIKeys

	if len(keys) == 0 && a.Config.Mode != "list" {
		logger.Error("未找到 API Key，请通过以下方式之一提供:")
		logger.Error("  1. 环境变量: export API_KEY=your_key 或 export API_KEYS=key1,key2")
		logger.Error("  2. .env 文件: API_KEY=your_key 或 API_KEYS=key1,key2")
		logger.Error("  3. 命令行参数: -apikey=your_key 或 -apikeys=key1,key2")
		return fmt.Errorf("missing api keys")
	}

	switch a.Config.Mode {
	case "list":
		return a.runListMode()
	case "test":
		if len(keys) == 0 {
			logger.Error("测试模式需要至少一个 API Key")
			return fmt.Errorf("missing api key for test mode")
		}
		logger.Info("测试第一个 API Key: %s", appconfig.MaskAPIKey(keys[0]))
		client := a.newAPIClient(keys[0])
		return a.runTestMode(client)
	case "run":
		if len(keys) == 1 {
			logger.Info("单账号模式 - API Key: %s", appconfig.MaskAPIKey(keys[0]))
			client := a.newAPIClient(keys[0])
			return a.runSchedulerMode(client)
		}

		logger.Info("多账号模式 - 检测到 %d 个 API Key", len(keys))
		return a.runMultiAccountMode(keys)
	case "manual":
		if len(keys) == 0 {
			logger.Error("手动重置模式需要至少一个 API Key")
			return fmt.Errorf("missing api key for manual mode")
		}
		if len(keys) > 1 {
			logger.Warn("检测到 %d 个 API Key，手动模式只会重置第一个账号", len(keys))
		}
		logger.Info("手动重置账号 - API Key: %s", appconfig.MaskAPIKey(keys[0]))
		client := a.newAPIClient(keys[0])
		return a.runManualMode(client)
	default:
		logger.Error("未知的运行模式: %s", a.Config.Mode)
		logger.Error("支持的模式: test, run, manual, list")
		return fmt.Errorf("unknown mode %s", a.Config.Mode)
	}
}

func (a *App) runTestMode(client apiClient) error {
	logger.Info("\n========================================")
	logger.Info("测试模式 - 测试接口连接")
	logger.Info("========================================\n")

	// 测试 1: 连接测试
	logger.Info("【测试 1/3】测试 API 连接...")
	if err := client.TestConnection(); err != nil {
		logger.Error("连接测试失败: %v", err)
		return err
	}
	logger.Info("✅ API 连接测试通过\n")

	// 测试 2: 获取订阅列表
	logger.Info("【测试 2/3】获取订阅列表...")
	subscriptions, err := client.GetSubscriptions()
	if err != nil {
		logger.Error("获取订阅列表失败: %v", err)
		return err
	}
	logger.Info("✅ 获取到 %d 个订阅\n", len(subscriptions))

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
	targetSub, err := client.GetTargetSubscription()
	if err != nil {
		logger.Error("获取目标订阅失败: %v", err)
		logger.Error("提示: 请检查 -plans 参数是否设置正确")
		return err
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

	logger.Info("\n保存账号信息到 %s/account.json...", a.Config.DataDir)
	logger.Info("\n========================================")
	logger.Info("测试完成！")
	logger.Info("========================================")

	if targetSub.ResetTimes >= 2 {
		logger.Info("\n✅ 当前 resetTimes=%d，满足重置条件", targetSub.ResetTimes)
		logger.Info("可以使用以下命令进行手动测试:")
		logger.Info("  go run cmd/reset/main.go -mode=manual")
	} else {
		logger.Warn("\n⚠️  当前 resetTimes=%d，不满足重置条件（需要 >= 2）", targetSub.ResetTimes)
		logger.Warn("请等待次日或条件满足后再尝试重置")
	}

	return nil
}

func (a *App) runSchedulerMode(client apiClient) error {
	logger.Info("\n========================================")
	logger.Info("调度器模式 - 启动定时任务")
	logger.Info("========================================\n")

	logger.Info("调度器已启动，等待定时任务触发...")
	logger.Info("按 Ctrl+C 停止\n")

	if err := a.deps.runSingleScheduler(a, client); err != nil {
		logger.Error("创建调度器失败: %v", err)
		return err
	}

	return nil
}

func (a *App) runManualMode(client apiClient) error {
	logger.Info("\n========================================")
	logger.Info("手动重置模式")
	logger.Info("========================================\n")

	targetSub, err := client.GetTargetSubscription()
	if err != nil {
		logger.Error("获取目标订阅失败: %v", err)
		return err
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
		return fmt.Errorf("reset times below threshold")
	}

	logger.Warn("\n⚠️  警告：此操作将执行真实的重置！")
	logger.Warn("⚠️  这将消耗一次重置机会（resetTimes 将减少）")
	logger.Warn("")

	if !a.Config.SkipConfirm {
		logger.Info("请输入 'yes' 确认执行重置，或输入其他内容取消:")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input != "yes" {
			logger.Info("取消操作")
			return nil
		}
	}

	logger.Info("\n开始执行重置...")

	if err := a.deps.manualReset(a, client); err != nil {
		logger.Error("手动重置失败: %v", err)
		return err
	}

	logger.Info("正在调用重置接口...")
	resetResp, err := client.ResetCredits(targetSub.ID)
	if err != nil {
		logger.Error("重置失败: %v", err)
		return err
	}

	logger.Info("\n✅ 重置成功!")
	logger.Info("响应: %s", resetResp.Message)

	logger.Info("\n验证重置结果...")
	a.deps.sleep(3 * time.Second)
	targetSubAfter, err := client.GetTargetSubscription()
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

	return nil
}

func (a *App) runListMode() error {
	logger.Info("\n========================================")
	logger.Info("账号列表")
	logger.Info("========================================\n")

	accounts, err := a.AccountMgr.ListAccounts()
	if err != nil {
		logger.Error("获取账号列表失败: %v", err)
		return err
	}

	if len(accounts) == 0 {
		logger.Info("暂无账号，请先导入账号:")
		logger.Info("  go run cmd/reset/main.go -mode=import -apikeys=key1,key2,key3")
		return nil
	}

	total, enabled, disabled, err := a.AccountMgr.GetAccountCount()
	if err != nil {
		logger.Error("统计账号数量失败: %v", err)
		return err
	}
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
		logger.Info("  API Key: %s", appconfig.MaskAPIKey(acc.APIKey))
		logger.Info("  添加时间: %s", acc.AddedAt)
		logger.Info("")
	}

	logger.Info("========================================")
	return nil
}

func (a *App) runMultiAccountMode(apiKeys []string) error {
	logger.Info("\n========================================")
	logger.Info("多账号模式 - 启动多账号调度器")
	logger.Info("========================================\n")

	logger.Info("步骤 1/3: 同步账号信息...")
	if err := a.AccountMgr.SyncAccountsFromAPIKeys(apiKeys, a.Config.Plans); err != nil {
		logger.Error("同步账号失败: %v", err)
		return err
	}

	logger.Info("\n步骤 2/3: 获取活跃账号...")
	activeAccounts, err := a.AccountMgr.GetActiveAccountsFromAPIKeys(apiKeys)
	if err != nil {
		logger.Error("获取活跃账号失败: %v", err)
		return err
	}

	if len(activeAccounts) == 0 {
		logger.Error("没有活跃的账号，请检查 API Keys 是否正确")
		return fmt.Errorf("no active accounts")
	}

	total, _, _, err := a.AccountMgr.GetAccountCount()
	if err != nil {
		logger.Error("统计账号数量失败: %v", err)
		return err
	}

	logger.Info("账号统计: 历史总计 %d 个，当前活跃 %d 个\n", total, len(activeAccounts))
	logger.Info("活跃账号列表:")
	for i, acc := range activeAccounts {
		logger.Info("  [%d] %s (%s)", i+1, acc.EmployeeEmail, acc.EmployeeName)
	}

	logger.Info("\n步骤 3/3: 启动调度器...")
	logger.Info("\n========================================")
	logger.Info("多账号调度器已启动")
	logger.Info("将为 %d 个账号执行定时重置", len(activeAccounts))
	logger.Info("按 Ctrl+C 停止")
	logger.Info("========================================\n")

	if err := a.deps.runMultiScheduler(a, activeAccounts); err != nil {
		logger.Error("创建多账号调度器失败: %v", err)
		return err
	}

	return nil
}

func (a *App) newAPIClient(key string) apiClient {
	return a.deps.newClient(a.Store, a.Config.BaseURL, key, a.Config.Plans)
}

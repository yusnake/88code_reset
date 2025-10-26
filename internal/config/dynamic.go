package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"code88reset/internal/models"
	"code88reset/pkg/logger"
)

const (
	DefaultFirstResetHour      = 18
	DefaultFirstResetMinute    = 50
	DefaultSecondResetHour     = 23
	DefaultSecondResetMinute   = 55
	DefaultFirstThreshold      = 70.0
	DefaultSecondThreshold     = 100.0
	DefaultWebPort             = 8966
)

// DynamicConfigManager 动态配置管理器
type DynamicConfigManager struct {
	configPath string
	config     models.DynamicConfig
	mu         sync.RWMutex
	listeners  []chan<- models.DynamicConfig
}

// NewDynamicConfigManager 创建动态配置管理器
func NewDynamicConfigManager(dataDir string) (*DynamicConfigManager, error) {
	configPath := filepath.Join(dataDir, "config.json")

	mgr := &DynamicConfigManager{
		configPath: configPath,
		listeners:  make([]chan<- models.DynamicConfig, 0),
	}

	// 加载或创建默认配置
	if err := mgr.loadOrCreateDefault(); err != nil {
		return nil, err
	}

	return mgr, nil
}

// loadOrCreateDefault 加载配置或创建默认配置
func (m *DynamicConfigManager) loadOrCreateDefault() error {
	// 尝试加载现有配置
	if _, err := os.Stat(m.configPath); err == nil {
		return m.load()
	}

	// 创建默认配置
	m.config = models.DynamicConfig{
		FirstReset: models.ResetConfig{
			Enabled:          false, // 默认禁用第一次重置
			Hour:             DefaultFirstResetHour,
			Minute:           DefaultFirstResetMinute,
			ThresholdPercent: DefaultFirstThreshold,
		},
		SecondReset: models.ResetConfig{
			Enabled:          true, // 默认启用第二次重置
			Hour:             DefaultSecondResetHour,
			Minute:           DefaultSecondResetMinute,
			ThresholdPercent: DefaultSecondThreshold,
		},
		Timezone: DefaultTimezone,
		WebPort:  DefaultWebPort,
	}

	// 保存默认配置
	return m.save()
}

// load 加载配置
func (m *DynamicConfigManager) load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config models.DynamicConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	logger.Info("配置已加载: %s", m.configPath)
	return nil
}

// save 保存配置
func (m *DynamicConfigManager) save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	logger.Info("配置已保存: %s", m.configPath)
	return nil
}

// GetConfig 获取当前配置（只读）
func (m *DynamicConfigManager) GetConfig() models.DynamicConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置并通知监听器
func (m *DynamicConfigManager) UpdateConfig(newConfig models.DynamicConfig) error {
	// 验证配置
	if err := m.validateConfig(newConfig); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	m.mu.Lock()
	m.config = newConfig
	m.mu.Unlock()

	// 保存到文件
	if err := m.save(); err != nil {
		return err
	}

	// 通知所有监听器
	m.notifyListeners(newConfig)

	logger.Info("配置已更新并通知监听器")
	return nil
}

// validateConfig 验证配置有效性
func (m *DynamicConfigManager) validateConfig(config models.DynamicConfig) error {
	// 验证第一次重置配置
	if config.FirstReset.Hour < 0 || config.FirstReset.Hour > 23 {
		return fmt.Errorf("第一次重置小时必须在 0-23 之间")
	}
	if config.FirstReset.Minute < 0 || config.FirstReset.Minute > 59 {
		return fmt.Errorf("第一次重置分钟必须在 0-59 之间")
	}
	if config.FirstReset.ThresholdPercent < 0 || config.FirstReset.ThresholdPercent > 100 {
		return fmt.Errorf("第一次重置阈值必须在 0-100 之间")
	}

	// 验证第二次重置配置
	if config.SecondReset.Hour < 0 || config.SecondReset.Hour > 23 {
		return fmt.Errorf("第二次重置小时必须在 0-23 之间")
	}
	if config.SecondReset.Minute < 0 || config.SecondReset.Minute > 59 {
		return fmt.Errorf("第二次重置分钟必须在 0-59 之间")
	}
	if config.SecondReset.ThresholdPercent < 0 || config.SecondReset.ThresholdPercent > 100 {
		return fmt.Errorf("第二次重置阈值必须在 0-100 之间")
	}

	// 验证 Web 端口
	if config.WebPort < 1 || config.WebPort > 65535 {
		return fmt.Errorf("Web 端口必须在 1-65535 之间")
	}

	// 验证时区
	if config.Timezone == "" {
		return fmt.Errorf("时区不能为空")
	}

	return nil
}

// Subscribe 订阅配置变更
func (m *DynamicConfigManager) Subscribe(listener chan<- models.DynamicConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, listener)
}

// notifyListeners 通知所有监听器
func (m *DynamicConfigManager) notifyListeners(config models.DynamicConfig) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, listener := range m.listeners {
		select {
		case listener <- config:
			// 成功发送
		default:
			// 监听器缓冲区满，跳过
			logger.Warn("配置变更通知发送失败：监听器缓冲区满")
		}
	}
}

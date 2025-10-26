package token

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"code88reset/internal/models"
	"code88reset/pkg/logger"
)

// Storage Token 存储管理器
type Storage struct {
	filePath string
	mu       sync.RWMutex
	tokens   map[string]*models.Token // key: Token ID
}

// NewStorage 创建 Token 存储管理器
func NewStorage(dataDir string) (*Storage, error) {
	filePath := filepath.Join(dataDir, "tokens.json")

	s := &Storage{
		filePath: filePath,
		tokens:   make(map[string]*models.Token),
	}

	// 加载现有数据
	if err := s.load(); err != nil {
		// 如果文件不存在，创建空文件
		if os.IsNotExist(err) {
			if err := s.save(); err != nil {
				return nil, fmt.Errorf("创建 tokens.json 失败: %w", err)
			}
		} else {
			return nil, err
		}
	}

	return s, nil
}

// load 从文件加载 Token 数据
func (s *Storage) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var storage models.TokenStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return fmt.Errorf("解析 tokens.json 失败: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 转换为 map
	s.tokens = make(map[string]*models.Token)
	for i := range storage.Tokens {
		token := &storage.Tokens[i]
		s.tokens[token.ID] = token
	}

	logger.Info("已加载 %d 个 Token", len(s.tokens))
	return nil
}

// save 保存 Token 数据到文件
func (s *Storage) save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 转换为数组
	tokens := make([]models.Token, 0, len(s.tokens))
	for _, token := range s.tokens {
		tokens = append(tokens, *token)
	}

	storage := models.TokenStorage{
		Tokens: tokens,
	}

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 Token 数据失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("写入 tokens.json 失败: %w", err)
	}

	return nil
}

// Add 添加 Token
func (s *Storage) Add(token *models.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tokens[token.ID]; exists {
		return fmt.Errorf("Token ID 已存在: %s", token.ID)
	}

	s.tokens[token.ID] = token
	return s.saveUnlocked()
}

// Update 更新 Token
func (s *Storage) Update(token *models.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tokens[token.ID]; !exists {
		return fmt.Errorf("Token 不存在: %s", token.ID)
	}

	s.tokens[token.ID] = token
	return s.saveUnlocked()
}

// Delete 删除 Token
func (s *Storage) Delete(tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tokens[tokenID]; !exists {
		return fmt.Errorf("Token 不存在: %s", tokenID)
	}

	delete(s.tokens, tokenID)
	return s.saveUnlocked()
}

// Get 获取单个 Token
func (s *Storage) Get(tokenID string) (*models.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, exists := s.tokens[tokenID]
	if !exists {
		return nil, fmt.Errorf("Token 不存在: %s", tokenID)
	}

	// 返回副本，避免外部修改
	tokenCopy := *token
	return &tokenCopy, nil
}

// List 获取所有 Token
func (s *Storage) List() []*models.Token {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokens := make([]*models.Token, 0, len(s.tokens))
	for _, token := range s.tokens {
		// 返回副本
		tokenCopy := *token
		tokens = append(tokens, &tokenCopy)
	}

	return tokens
}

// ListEnabled 获取所有启用的 Token
func (s *Storage) ListEnabled() []*models.Token {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokens := make([]*models.Token, 0)
	for _, token := range s.tokens {
		if token.Enabled {
			// 返回副本
			tokenCopy := *token
			tokens = append(tokens, &tokenCopy)
		}
	}

	return tokens
}

// Count 获取 Token 总数
func (s *Storage) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tokens)
}

// CountEnabled 获取启用的 Token 数量
func (s *Storage) CountEnabled() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, token := range s.tokens {
		if token.Enabled {
			count++
		}
	}
	return count
}

// saveUnlocked 保存数据（不加锁版本，内部使用）
func (s *Storage) saveUnlocked() error {
	// 转换为数组
	tokens := make([]models.Token, 0, len(s.tokens))
	for _, token := range s.tokens {
		tokens = append(tokens, *token)
	}

	storage := models.TokenStorage{
		Tokens: tokens,
	}

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 Token 数据失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("写入 tokens.json 失败: %w", err)
	}

	return nil
}

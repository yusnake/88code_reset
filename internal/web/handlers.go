package web

import (
	"fmt"
	"net/http"
	"strings"

	"code88reset/internal/models"
	"code88reset/pkg/logger"
)

// handleTokens Token 列表管理
func (s *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListTokens(w, r)
	case http.MethodPost:
		s.handleAddToken(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleListTokens 获取 Token 列表
func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	tokens := s.tokenManager.ListTokens()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tokens": tokens,
		"count":  len(tokens),
	})
}

// handleAddToken 添加新 Token
func (s *Server) handleAddToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey string `json:"api_key"`
		Name   string `json:"name"`
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if req.APIKey == "" {
		writeError(w, http.StatusBadRequest, "API Key is required")
		return
	}

	if req.Name == "" {
		req.Name = "Token " + req.APIKey[:8]
	}

	token, err := s.tokenManager.AddToken(req.APIKey, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to add token: "+err.Error())
		return
	}

	logger.Info("通过 Web API 添加 Token: %s", token.Name)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Token added successfully",
		"token":   token,
	})
}

// handleBatchAddTokens 批量添加 Token
func (s *Server) handleBatchAddTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		APIKeys string `json:"api_keys"` // 多个 API Key，每行一个
		Prefix  string `json:"prefix"`   // Token 名称前缀，可选
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if req.APIKeys == "" {
		writeError(w, http.StatusBadRequest, "API Keys are required")
		return
	}

	// 按行分割
	lines := strings.Split(req.APIKeys, "\n")
	var results []map[string]interface{}
	successCount := 0
	failCount := 0

	for i, line := range lines {
		apiKey := strings.TrimSpace(line)
		if apiKey == "" {
			continue // 跳过空行
		}

		// 生成 Token 名称
		name := fmt.Sprintf("%s%d", req.Prefix, i+1)
		if req.Prefix == "" {
			name = fmt.Sprintf("Token-%d", i+1)
		}

		// 添加 Token
		token, err := s.tokenManager.AddToken(apiKey, name)
		if err != nil {
			results = append(results, map[string]interface{}{
				"api_key": apiKey[:10] + "...",
				"name":    name,
				"success": false,
				"error":   err.Error(),
			})
			failCount++
			logger.Warn("批量添加 Token 失败: %s - %v", name, err)
		} else {
			results = append(results, map[string]interface{}{
				"api_key": apiKey[:10] + "...",
				"name":    name,
				"success": true,
				"token":   token,
			})
			successCount++
			logger.Info("通过批量添加 Token: %s", token.Name)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"message":       fmt.Sprintf("批量添加完成: 成功 %d, 失败 %d", successCount, failCount),
		"success_count": successCount,
		"fail_count":    failCount,
		"results":       results,
	})
}

// handleTokenDetail Token 详情操作
func (s *Server) handleTokenDetail(w http.ResponseWriter, r *http.Request) {
	// 提取 Token ID
	path := strings.TrimPrefix(r.URL.Path, "/api/tokens/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "Token ID is required")
		return
	}

	tokenID := parts[0]

	switch r.Method {
	case http.MethodGet:
		s.handleGetToken(w, r, tokenID)
	case http.MethodDelete:
		s.handleDeleteToken(w, r, tokenID)
	case http.MethodPut:
		// 根据路径判断操作类型
		if len(parts) > 1 {
			switch parts[1] {
			case "toggle":
				s.handleToggleToken(w, r, tokenID)
			case "refresh":
				s.handleRefreshToken(w, r, tokenID)
			case "reset":
				s.handleResetToken(w, r, tokenID)
			default:
				writeError(w, http.StatusNotFound, "Unknown operation")
			}
		} else {
			writeError(w, http.StatusBadRequest, "Operation is required")
		}
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleGetToken 获取单个 Token
func (s *Server) handleGetToken(w http.ResponseWriter, r *http.Request, tokenID string) {
	token, err := s.tokenManager.GetToken(tokenID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Token not found")
		return
	}

	writeJSON(w, http.StatusOK, token)
}

// handleDeleteToken 删除 Token
func (s *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request, tokenID string) {
	if err := s.tokenManager.DeleteToken(tokenID); err != nil {
		writeError(w, http.StatusNotFound, "Token not found")
		return
	}

	logger.Info("通过 Web API 删除 Token: %s", tokenID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Token deleted successfully",
	})
}

// handleToggleToken 切换 Token 启用/禁用
func (s *Server) handleToggleToken(w http.ResponseWriter, r *http.Request, tokenID string) {
	token, err := s.tokenManager.ToggleToken(tokenID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Token not found")
		return
	}

	logger.Info("通过 Web API 切换 Token 状态: %s (enabled=%v)", tokenID, token.Enabled)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Token status toggled",
		"token":   token,
	})
}

// handleRefreshToken 刷新 Token 订阅信息
func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request, tokenID string) {
	token, err := s.tokenManager.RefreshSubscription(tokenID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to refresh subscription: "+err.Error())
		return
	}

	logger.Info("通过 Web API 刷新 Token 订阅: %s", tokenID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Subscription refreshed",
		"token":   token,
	})
}

// handleResetToken 手动重置单个 Token
func (s *Server) handleResetToken(w http.ResponseWriter, r *http.Request, tokenID string) {
	var req struct {
		ResetType string `json:"reset_type"` // "first" or "second"
	}

	if err := readJSON(r, &req); err != nil {
		// 如果没有请求体，使用默认值
		req.ResetType = "second"
	}

	if req.ResetType != "first" && req.ResetType != "second" {
		req.ResetType = "second"
	}

	// 获取配置以确定阈值
	cfg := s.configMgr.GetConfig()
	var threshold float64
	if req.ResetType == "first" {
		threshold = cfg.FirstReset.ThresholdPercent
	} else {
		threshold = cfg.SecondReset.ThresholdPercent
	}

	token, err := s.tokenManager.ResetToken(tokenID, req.ResetType, threshold)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Reset failed: "+err.Error())
		return
	}

	logger.Info("通过 Web API 手动重置 Token: %s (type=%s)", tokenID, req.ResetType)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Reset %s completed", req.ResetType),
		"token":   token,
	})
}

// handleManualReset 手动触发所有 Token 重置
func (s *Server) handleManualReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		ResetType string `json:"reset_type"` // "first" or "second"
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if req.ResetType != "first" && req.ResetType != "second" {
		writeError(w, http.StatusBadRequest, "Invalid reset_type, must be 'first' or 'second'")
		return
	}

	// 获取配置以确定阈值
	cfg := s.configMgr.GetConfig()
	var threshold float64
	if req.ResetType == "first" {
		threshold = cfg.FirstReset.ThresholdPercent
	} else {
		threshold = cfg.SecondReset.ThresholdPercent
	}

	// 获取所有启用的 Token
	tokens := s.tokenManager.ListEnabledTokens()
	if len(tokens) == 0 {
		writeError(w, http.StatusBadRequest, "No enabled tokens")
		return
	}

	// 执行重置
	type resetResult struct {
		TokenID string                 `json:"token_id"`
		Name    string                 `json:"name"`
		Success bool                   `json:"success"`
		Message string                 `json:"message"`
		Before  float64                `json:"before_credits,omitempty"`
		After   float64                `json:"after_credits,omitempty"`
		Last    *models.TokenResetRecord `json:"last_reset,omitempty"`
	}

	results := make([]resetResult, 0, len(tokens))

	for _, token := range tokens {
		updatedToken, err := s.tokenManager.ResetToken(token.ID, req.ResetType, threshold)

		result := resetResult{
			TokenID: token.ID,
			Name:    token.Name,
		}

		if err != nil {
			result.Success = false
			result.Message = err.Error()
		} else if updatedToken.LastReset != nil {
			result.Success = updatedToken.LastReset.Success
			result.Message = updatedToken.LastReset.Message
			result.Before = updatedToken.LastReset.BeforeCredits
			result.After = updatedToken.LastReset.AfterCredits
			result.Last = updatedToken.LastReset
		}

		results = append(results, result)
	}

	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	logger.Info("通过 Web API 手动触发批量重置: type=%s, success=%d/%d", req.ResetType, successCount, len(results))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"reset_type":    req.ResetType,
		"total":         len(results),
		"success_count": successCount,
		"results":       results,
	})
}

// handleSystemLogs 系统日志管理
func (s *Server) handleSystemLogs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetSystemLogs(w, r)
	case http.MethodDelete:
		s.handleClearSystemLogs(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleGetSystemLogs 获取系统日志
func (s *Server) handleGetSystemLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.storage.LoadSystemLogs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load system logs: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs.Logs,
		"count": len(logs.Logs),
	})
}

// handleClearSystemLogs 清空系统日志
func (s *Server) handleClearSystemLogs(w http.ResponseWriter, r *http.Request) {
	if err := s.storage.ClearSystemLogs(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to clear system logs: "+err.Error())
		return
	}

	logger.Info("通过 Web API 清空系统日志")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "System logs cleared successfully",
	})
}

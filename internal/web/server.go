package web

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"code88reset/internal/config"
	"code88reset/internal/models"
	"code88reset/internal/storage"
	"code88reset/internal/token"
	"code88reset/pkg/logger"
)

//go:embed static/*
var staticFiles embed.FS

// Server Web 服务器
type Server struct {
	httpServer   *http.Server
	tokenManager *token.Manager
	configMgr    *config.DynamicConfigManager
	storage      *storage.Storage
	adminToken   string
	version      string
}

// NewServer 创建 Web 服务器
func NewServer(port int, tokenManager *token.Manager, configMgr *config.DynamicConfigManager, storage *storage.Storage, adminToken string, version string) *Server {
	s := &Server{
		tokenManager: tokenManager,
		configMgr:    configMgr,
		storage:      storage,
		adminToken:   adminToken,
		version:      version,
	}

	// 创建路由
	mux := http.NewServeMux()

	// 静态文件 - 从 static 子目录提供服务
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("无法创建静态文件系统: %v", err))
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// API 路由
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/version", s.handleVersion)
	mux.HandleFunc("/api/status", s.withAuth(s.handleGetStatus))
	mux.HandleFunc("/api/config", s.withAuth(s.handleConfig))
	mux.HandleFunc("/api/tokens", s.withAuth(s.handleTokens))
	mux.HandleFunc("/api/tokens/batch", s.withAuth(s.handleBatchAddTokens))
	mux.HandleFunc("/api/tokens/", s.withAuth(s.handleTokenDetail))
	mux.HandleFunc("/api/reset/trigger", s.withAuth(s.handleManualReset))
	mux.HandleFunc("/api/system-logs", s.withAuth(s.handleSystemLogs))

	// 创建 HTTP 服务器
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      s.withCORS(s.withLogging(mux)),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// Start 启动 Web 服务器
func (s *Server) Start() error {
	logger.Info("========================================")
	logger.Info("Web 管理服务器启动")
	logger.Info("监听地址: %s", s.httpServer.Addr)
	logger.Info("========================================")

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("Web 服务器启动失败: %w", err)
	}

	return nil
}

// Stop 停止 Web 服务器
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("正在停止 Web 服务器...")
	return s.httpServer.Shutdown(ctx)
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// handleVersion 获取版本号
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version": s.version,
	})
}

// handleGetStatus 获取系统状态
func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	cfg := s.configMgr.GetConfig()
	tokens := s.tokenManager.ListTokens()

	enabledCount := 0
	for _, t := range tokens {
		if t.Enabled {
			enabledCount++
		}
	}

	// 计算下次重置时间
	now := time.Now()
	loc, _ := time.LoadLocation(cfg.Timezone)
	nowInTZ := now.In(loc)

	var nextResetTime time.Time
	var nextResetType string

	// 检查第一次重置
	if cfg.FirstReset.Enabled {
		firstToday := time.Date(nowInTZ.Year(), nowInTZ.Month(), nowInTZ.Day(),
			cfg.FirstReset.Hour, cfg.FirstReset.Minute, 0, 0, loc)
		if nowInTZ.Before(firstToday) {
			nextResetTime = firstToday
			nextResetType = "first"
		}
	}

	// 检查第二次重置
	if cfg.SecondReset.Enabled {
		secondToday := time.Date(nowInTZ.Year(), nowInTZ.Month(), nowInTZ.Day(),
			cfg.SecondReset.Hour, cfg.SecondReset.Minute, 0, 0, loc)
		if nowInTZ.Before(secondToday) {
			if nextResetTime.IsZero() || secondToday.Before(nextResetTime) {
				nextResetTime = secondToday
				nextResetType = "second"
			}
		}
	}

	// 如果今天没有下次重置，计算明天的
	if nextResetTime.IsZero() {
		tomorrow := nowInTZ.AddDate(0, 0, 1)
		if cfg.FirstReset.Enabled {
			nextResetTime = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
				cfg.FirstReset.Hour, cfg.FirstReset.Minute, 0, 0, loc)
			nextResetType = "first"
		} else if cfg.SecondReset.Enabled {
			nextResetTime = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
				cfg.SecondReset.Hour, cfg.SecondReset.Minute, 0, 0, loc)
			nextResetType = "second"
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"current_time":     now.Format(time.RFC3339),
		"timezone":         cfg.Timezone,
		"next_reset_time":  nextResetTime.Format(time.RFC3339),
		"next_reset_type":  nextResetType,
		"total_tokens":     len(tokens),
		"enabled_tokens":   enabledCount,
		"first_reset":      cfg.FirstReset,
		"second_reset":     cfg.SecondReset,
	})
}

// handleConfig 配置管理
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetConfig(w, r)
	case http.MethodPut:
		s.handleUpdateConfig(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleGetConfig 获取配置
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.configMgr.GetConfig()
	writeJSON(w, http.StatusOK, cfg)
}

// handleUpdateConfig 更新配置
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig models.DynamicConfig
	if err := readJSON(r, &newConfig); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if err := s.configMgr.UpdateConfig(newConfig); err != nil {
		writeError(w, http.StatusBadRequest, "Configuration update failed: "+err.Error())
		return
	}

	logger.Info("配置已通过 Web API 更新")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Configuration updated successfully",
		"config":  newConfig,
	})
}

package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"penetration-testing-ai/internal/agent"
	"penetration-testing-ai/internal/config"
	"penetration-testing-ai/internal/database"
	"penetration-testing-ai/internal/einoobserve"
	"penetration-testing-ai/internal/handler"
	"penetration-testing-ai/internal/logger"
	"penetration-testing-ai/internal/mcp"
	"penetration-testing-ai/internal/monitor"
	"penetration-testing-ai/internal/multiagent"
	"penetration-testing-ai/internal/security"
	"penetration-testing-ai/internal/skillpackage"
	"penetration-testing-ai/internal/toolguard"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

// App 应用
type App struct {
	config         *config.Config
	logger         *logger.Logger
	router         *gin.Engine
	mcpServer      *mcp.Server
	externalMCPMgr *mcp.ExternalMCPManager
	agent          *agent.Agent
	executor       *security.Executor
	db             *database.DB
	agentHandler   *handler.AgentHandler
}

// New 创建新应用
func New(cfg *config.Config, log *logger.Logger, configPath string) (*App, error) {
	toolGuard, err := toolguard.NewManager(cfg.EffectiveToolGuard())
	if err != nil {
		return nil, fmt.Errorf("初始化调用拦截规则: %w", err)
	}
	if err := multiagent.InitADK(); err != nil {
		return nil, fmt.Errorf("初始化 Eino ADK: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// CORS中间件
	router.Use(corsMiddleware(cfg.Server.CORSAllowedOrigins))

	// 初始化数据库
	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = "data/conversations.db"
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := database.NewDB(dbPath, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	monitorRetention := monitor.NewService(db, cfg, log.Logger)
	monitorRetention.PurgeExpired()
	monitor.StartRetentionLoop(monitorRetention, log.Logger)

	// 创建MCP服务器（带数据库持久化）
	mcpServer := mcp.NewServerWithStorage(log.Logger, db)
	mcpServer.SetToolGuard(toolGuard)
	mcpServer.ConfigureHTTPToolCallTimeoutFromAgentMinutes(cfg.Agent.ToolTimeoutMinutes)
	mcpServer.ConfigureToolWaitTimeoutSeconds(cfg.Agent.ToolWaitTimeoutSeconds)
	mcpServer.ConfigureToolResultMaxBytes(cfg.MultiAgent.EinoMiddleware.ReductionMaxLengthForTruncEffective())
	mcpServer.ConfigureToolResultSpillRoot(cfg.MultiAgent.EinoMiddleware.ReductionRootDir)

	// 创建安全工具执行器
	executor := security.NewExecutor(&cfg.Security, mcpServer, log.Logger)
	executor.SetShellNoOutputTimeoutSeconds(cfg.Agent.ShellNoOutputTimeoutSeconds)
	executor.SetToolOutputMaxBytes(cfg.MultiAgent.EinoMiddleware.ReductionMaxLengthForTruncEffective())
	executor.SetToolOutputSpillRoot(cfg.MultiAgent.EinoMiddleware.ReductionRootDir)

	// 注册工具
	executor.RegisterTools(mcpServer)

	// 注册漏洞记录工具
	registerVulnerabilityTools(mcpServer, db, log.Logger)

	// 创建外部MCP管理器（使用与内部MCP服务器相同的存储）
	externalMCPMgr := mcp.NewExternalMCPManagerWithStorage(log.Logger, db)
	externalMCPMgr.SetToolGuard(toolGuard)
	externalMCPMgr.ConfigureToolWaitTimeoutSeconds(cfg.Agent.ToolWaitTimeoutSeconds)
	externalMCPMgr.ConfigureToolResultMaxBytes(cfg.MultiAgent.EinoMiddleware.ReductionMaxLengthForTruncEffective())
	externalMCPMgr.ConfigureToolResultSpillRoot(cfg.MultiAgent.EinoMiddleware.ReductionRootDir)
	externalMCPMgr.ConfigureResilience(mcp.ExternalMCPResilienceConfig{
		MaxConcurrentPerServer:  cfg.Agent.ExternalMCPMaxConcurrentPerServer,
		MaxConcurrentTotal:      cfg.Agent.ExternalMCPMaxConcurrentTotal,
		CircuitFailureThreshold: cfg.Agent.ExternalMCPCircuitFailureThreshold,
		CircuitCooldown:         time.Duration(cfg.Agent.ExternalMCPCircuitCooldownSeconds) * time.Second,
	})
	mcp.RegisterExecutionControlTools(mcpServer, externalMCPMgr)
	if cfg.ExternalMCP.Servers != nil {
		externalMCPMgr.LoadConfigs(&cfg.ExternalMCP)
		externalMCPMgr.StartAllEnabled()
	}

	execReconciler := monitor.NewExecutionReconciler(db, mcpServer, externalMCPMgr, log.Logger)
	execReconciler.ReconcileOnStartup()
	monitor.StartStaleRunningReconcileLoop(execReconciler, log.Logger)

	// 创建Agent
	maxIterations := cfg.Agent.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 30
	}
	ag := agent.NewAgent(&cfg.OpenAI, &cfg.Agent, mcpServer, externalMCPMgr, log.Logger, maxIterations)
	ag.UpdateToolDescriptionMode(cfg.Security.ToolDescriptionMode)

	// 配置文件路径必须由入口传入（与 flag -config 一致）
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		configPath = "config.yaml"
	}

	skillsDir := skillpackage.SkillsRootFromConfig(cfg.SkillsDir, configPath)
	log.Logger.Debug("Skills 目录", zap.String("skillsDir", skillsDir))
	configDir := filepath.Dir(configPath)
	plantaskRel := strings.TrimSpace(cfg.MultiAgent.EinoMiddleware.PlantaskRelDir)
	if plantaskRel == "" {
		plantaskRel = ".eino/plantask"
	}
	plantaskBase := filepath.Join(skillsDir, plantaskRel)
	checkpointBase := strings.TrimSpace(cfg.MultiAgent.EinoMiddleware.CheckpointDir)
	reductionRoot := strings.TrimSpace(cfg.MultiAgent.EinoMiddleware.ReductionRootDir)
	workspaceRoot := strings.TrimSpace(cfg.Agent.WorkspaceRootDir)
	db.SetEinoConversationDirs(plantaskBase, checkpointBase, reductionRoot, workspaceRoot)

	ag.SetPromptBaseDir(configDir)

	// 创建处理器
	agentHandler := handler.NewAgentHandler(ag, db, cfg, log.Logger)
	monitorHandler := handler.NewMonitorHandler(mcpServer, executor, db, log.Logger)
	monitorHandler.SetMonitorRetention(monitorRetention)
	monitorHandler.SetExternalMCPManager(externalMCPMgr)
	monitorHandler.SetTaskManager(agentHandler.TaskManager())
	monitorHandler.SetAgentHandler(agentHandler)

	vulnerabilityHandler := handler.NewVulnerabilityHandler(db, log.Logger)
	vulnerabilityHandler.SetTaskStateProvider(agentHandler)

	configHandler := handler.NewConfigHandler(configPath, cfg, mcpServer, executor, ag, externalMCPMgr, log.Logger)
	configHandler.SetDB(db)
	configHandler.SetToolGuard(toolGuard)
	configHandler.SetVulnerabilityToolRegistrar(func() error {
		registerVulnerabilityTools(mcpServer, db, log.Logger)
		return nil
	})
	configHandler.SetSkillsToolRegistrar(func() error { return nil })

	externalMCPHandler := handler.NewExternalMCPHandler(externalMCPMgr, cfg, configPath, log.Logger)
	roleHandler := handler.NewRoleHandler(cfg, configPath, log.Logger)
	roleHandler.SetDB(db)
	skillsHandler := handler.NewSkillsHandler(cfg, configPath, log.Logger)
	skillsHandler.SetDB(db)

	conversationHandler := handler.NewConversationHandler(db, log.Logger)
	conversationHandler.SetTaskStopper(agentHandler)
	conversationHandler.SetTaskStateProvider(agentHandler)

	app := &App{
		config:         cfg,
		logger:         log,
		router:         router,
		mcpServer:      mcpServer,
		externalMCPMgr: externalMCPMgr,
		agent:          ag,
		executor:       executor,
		db:             db,
		agentHandler:   agentHandler,
	}

	setupRoutes(
		router,
		agentHandler,
		monitorHandler,
		conversationHandler,
		configHandler,
		externalMCPHandler,
		vulnerabilityHandler,
		roleHandler,
		skillsHandler,
		mcpServer,
		app,
	)

	return app, nil
}

// mcpHandlerWithAuth 转发到 MCP 处理。当前为匿名单用户模式，不再校验鉴权头。
func (a *App) mcpHandlerWithAuth(w http.ResponseWriter, r *http.Request) {
	a.mcpServer.HandleHTTP(w, r)
}

// Run 启动应用（向后兼容，不支持优雅关闭）
func (a *App) Run() error {
	return a.RunWithContext(context.Background())
}

// RunWithContext 启动应用，支持通过 context 取消来优雅关闭
func (a *App) RunWithContext(ctx context.Context) error {
	// 启动MCP服务器（如果启用）
	var mcpServer *http.Server
	if a.config.MCP.Enabled {
		mcpAddr := fmt.Sprintf("%s:%d", a.config.MCP.Host, a.config.MCP.Port)
		a.logger.Info("启动MCP服务器", zap.String("address", mcpAddr))

		mux := http.NewServeMux()
		mux.HandleFunc("/mcp", a.mcpHandlerWithAuth)

		mcpServer = &http.Server{Addr: mcpAddr, Handler: mux}
		go func() {
			if err := mcpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				a.logger.Error("MCP服务器启动失败", zap.Error(err))
			}
		}()
	}

	// 启动主服务器（可选 HTTPS + HTTP/2，见 config server.tls_*）
	addr := fmt.Sprintf("%s:%d", a.config.Server.Host, a.config.Server.Port)
	tlsMode, tlsConf, certFile, keyFile, tlsErr := prepareMainServerTLS(&a.config.Server)
	if tlsErr != nil {
		return tlsErr
	}

	srv := &http.Server{Addr: addr, Handler: a.router}
	var mainMux *mainServerMux
	httpRedirect := config.ServerHTTPRedirectEnabled(&a.config.Server)
	if tlsMode != mainTLSOff {
		srv.TLSConfig = tlsConf
		if err := http2.ConfigureServer(srv, &http2.Server{}); err != nil {
			return fmt.Errorf("主服务 HTTP/2 配置失败: %w", err)
		}
		switch tlsMode {
		case mainTLSFromFiles:
			a.logger.Debug("启动 HTTPS 主服务（已启用 HTTP/2 协商）",
				zap.String("address", addr),
				zap.String("cert", certFile),
			)
		case mainTLSInMemorySelfSigned:
			a.logger.Debug("启动 HTTPS 主服务（内存自签证书，仅测试；已启用 HTTP/2 协商）",
				zap.String("address", addr),
			)
		}
		if httpRedirect {
			a.logger.Debug("已启用 HTTP→HTTPS 自动跳转（同端口嗅探分流）", zap.String("address", addr))
		}
	} else {
		a.logger.Debug("启动 HTTP 主服务", zap.String("address", addr))
	}

	// 监听 context 取消，优雅关闭 HTTP 服务器
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if mainMux != nil {
			if err := mainMux.Shutdown(shutdownCtx); err != nil {
				a.logger.Error("HTTP/HTTPS 分流服务器关闭失败", zap.Error(err))
			}
		} else if err := srv.Shutdown(shutdownCtx); err != nil {
			a.logger.Error("HTTP服务器关闭失败", zap.Error(err))
		}
		if mcpServer != nil {
			if err := mcpServer.Shutdown(shutdownCtx); err != nil {
				a.logger.Error("MCP服务器关闭失败", zap.Error(err))
			}
		}
	}()

	var err error
	switch {
	case tlsMode != mainTLSOff && httpRedirect:
		var tlsConfReady *tls.Config
		tlsConfReady, err = ensureMainTLSConfigCerts(tlsMode, tlsConf, certFile, keyFile)
		if err != nil {
			return fmt.Errorf("加载 TLS 证书: %w", err)
		}
		srv.TLSConfig = tlsConfReady
		var ln net.Listener
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		mainMux = newMainServerMux(ln, srv, portFromListenAddr(addr), a.logger.Logger)
		err = mainMux.Serve()
	case tlsMode == mainTLSOff:
		err = srv.ListenAndServe()
	case tlsMode == mainTLSFromFiles:
		err = srv.ListenAndServeTLS(certFile, keyFile)
	case tlsMode == mainTLSInMemorySelfSigned:
		var ln net.Listener
		ln, err = tls.Listen("tcp", addr, srv.TLSConfig)
		if err == nil {
			err = srv.Serve(ln)
		}
	default:
		err = srv.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown 关闭应用
func (a *App) Shutdown() {
	if a.agentHandler != nil {
		a.agentHandler.ShutdownTasks()
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = einoobserve.ShutdownOtel(shutdownCtx)
	shutdownCancel()

	// 停止所有外部MCP客户端
	if a.externalMCPMgr != nil {
		a.externalMCPMgr.StopAll()
	}

	// 关闭主数据库连接
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.logger.Logger.Warn("关闭主数据库连接失败", zap.Error(err))
		}
	}
}

// setupRoutes 注册 HTTP 路由。当前为匿名单用户模式，不做登录与权限校验。
func setupRoutes(
	router *gin.Engine,
	agentHandler *handler.AgentHandler,
	monitorHandler *handler.MonitorHandler,
	conversationHandler *handler.ConversationHandler,
	configHandler *handler.ConfigHandler,
	externalMCPHandler *handler.ExternalMCPHandler,
	vulnerabilityHandler *handler.VulnerabilityHandler,
	roleHandler *handler.RoleHandler,
	skillsHandler *handler.SkillsHandler,
	mcpServer *mcp.Server,
	app *App,
) {
	api := router.Group("/api")

	protected := api.Group("")
	protected.Use(security.AnonymousSessionMiddleware())
	{
		// Agent 执行
		protected.POST("/eino-agent", agentHandler.EinoSingleAgentLoop)
		protected.POST("/eino-agent/runs", agentHandler.SubmitSingleAgentRun)
		protected.GET("/eino-agent/runs/:runId", agentHandler.GetAgentRun)
		protected.POST("/eino-agent/stream", agentHandler.EinoSingleAgentLoopStream)
		protected.POST("/agent-loop/cancel", agentHandler.CancelAgentLoop)
		protected.GET("/agent-loop/tasks", agentHandler.ListAgentTasks)
		protected.GET("/agent-loop/task-events", agentHandler.SubscribeAgentTaskEvents)
		protected.GET("/agent-loop/tasks/completed", agentHandler.ListCompletedTasks)

		// 对话与消息
		protected.GET("/usage/tokens", conversationHandler.GetTokenUsageStats)
		protected.POST("/conversations", conversationHandler.CreateConversation)
		protected.GET("/conversations", conversationHandler.ListConversations)
		protected.GET("/conversations/:id", conversationHandler.GetConversation)
		protected.GET("/conversations/:id/token-usage", conversationHandler.GetConversationTokenUsageStats)
		protected.GET("/conversations/:id/plan-tasks", conversationHandler.GetConversationPlanTasks)
		protected.GET("/messages/:id/process-details", conversationHandler.GetMessageProcessDetails)
		protected.GET("/process-details/:id", conversationHandler.GetProcessDetail)
		protected.PUT("/conversations/:id", conversationHandler.UpdateConversation)
		protected.DELETE("/conversations/:id", conversationHandler.DeleteConversation)
		protected.POST("/conversations/:id/delete-turn", conversationHandler.DeleteConversationTurn)
		protected.PUT("/conversations/:id/pinned", conversationHandler.UpdateConversationPinned)

		// 工具执行监控
		protected.GET("/monitor", monitorHandler.Monitor)
		protected.GET("/monitor/execution/:id", monitorHandler.GetExecution)
		protected.POST("/monitor/execution/:id/cancel", monitorHandler.CancelExecution)
		protected.POST("/monitor/executions/names", monitorHandler.BatchGetToolNames)
		protected.DELETE("/monitor/execution/:id", monitorHandler.DeleteExecution)
		protected.DELETE("/monitor/executions", monitorHandler.DeleteExecutions)
		protected.GET("/monitor/stats", monitorHandler.GetStats)
		protected.GET("/monitor/calls-timeline", monitorHandler.GetCallsTimeline)

		// 配置与调用拦截
		protected.GET("/config", configHandler.GetConfig)
		protected.GET("/tool-guard", configHandler.GetToolGuard)
		protected.PUT("/tool-guard", configHandler.UpdateToolGuard)
		protected.POST("/tool-guard/test", configHandler.TestToolGuard)
		protected.GET("/config/tools", configHandler.GetTools)
		protected.GET("/config/tools/:name/schema", configHandler.GetToolSchema)
		protected.PUT("/config", configHandler.UpdateConfig)
		protected.POST("/config/apply", configHandler.ApplyConfig)
		protected.POST("/config/test-openai", configHandler.TestOpenAI)
		protected.POST("/config/test-typesafe", configHandler.TestTypeSafe)
		protected.POST("/config/list-models", configHandler.ListModels)

		// 外部 MCP
		protected.GET("/external-mcp", externalMCPHandler.GetExternalMCPs)
		protected.GET("/external-mcp/stats", externalMCPHandler.GetExternalMCPStats)
		protected.GET("/external-mcp/:name", externalMCPHandler.GetExternalMCP)
		protected.PUT("/external-mcp/:name", externalMCPHandler.AddOrUpdateExternalMCP)
		protected.DELETE("/external-mcp/:name", externalMCPHandler.DeleteExternalMCP)
		protected.POST("/external-mcp/:name/start", externalMCPHandler.StartExternalMCP)
		protected.POST("/external-mcp/:name/stop", externalMCPHandler.StopExternalMCP)

		// 漏洞管理
		protected.GET("/vulnerabilities", vulnerabilityHandler.ListVulnerabilities)
		protected.GET("/vulnerabilities/export", vulnerabilityHandler.ExportVulnerabilities)
		protected.DELETE("/vulnerabilities/batch", vulnerabilityHandler.BatchDeleteVulnerabilities)
		protected.GET("/vulnerabilities/filter-options", vulnerabilityHandler.GetVulnerabilityFilterOptions)
		protected.GET("/vulnerabilities/stats", vulnerabilityHandler.GetVulnerabilityStats)
		protected.GET("/vulnerabilities/:id", vulnerabilityHandler.GetVulnerability)
		protected.GET("/vulnerabilities/:id/download", vulnerabilityHandler.DownloadVulnerability)
		protected.POST("/vulnerabilities", vulnerabilityHandler.CreateVulnerability)
		protected.POST("/vulnerabilities/:id/retest", vulnerabilityHandler.CreateRetest)
		protected.PUT("/vulnerabilities/:id", vulnerabilityHandler.UpdateVulnerability)
		protected.DELETE("/vulnerabilities/:id", vulnerabilityHandler.DeleteVulnerability)

		// 角色化测试
		protected.GET("/roles", roleHandler.GetRoles)
		protected.GET("/roles/:name", roleHandler.GetRole)
		protected.POST("/roles", roleHandler.CreateRole)
		protected.PUT("/roles/:name", roleHandler.UpdateRole)
		protected.DELETE("/roles/:name", roleHandler.DeleteRole)

		// Skills 管理（具体路径需注册在 /skills/:name 之前）
		protected.GET("/skills", skillsHandler.GetSkills)
		protected.GET("/skills/stats", skillsHandler.GetSkillStats)
		protected.DELETE("/skills/stats", skillsHandler.ClearSkillStats)
		protected.GET("/skills/:name/files", skillsHandler.ListSkillPackageFiles)
		protected.GET("/skills/:name/file", skillsHandler.GetSkillPackageFile)
		protected.PUT("/skills/:name/file", skillsHandler.PutSkillPackageFile)
		protected.GET("/skills/:name/bound-roles", skillsHandler.GetSkillBoundRoles)
		protected.POST("/skills", skillsHandler.CreateSkill)
		protected.PUT("/skills/:name", skillsHandler.UpdateSkill)
		protected.DELETE("/skills/:name", skillsHandler.DeleteSkill)
		protected.DELETE("/skills/:name/stats", skillsHandler.ClearSkillStatsByName)
		protected.GET("/skills/:name", skillsHandler.GetSkill)

		// MCP端点
		protected.POST("/mcp", func(c *gin.Context) {
			mcpServer.HandleHTTP(c.Writer, c.Request)
		})
	}

	// 静态文件
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/*")

	// 前端页面
	router.GET("/", func(c *gin.Context) {
		version := app.config.Version
		if version == "" {
			version = "v1.0.0"
		}
		c.HTML(http.StatusOK, "index.html", gin.H{"Version": version})
	})
}

// corsMiddleware 允许同源请求与操作者显式配置的来源。
func corsMiddleware(configuredOrigins []string) gin.HandlerFunc {
	allowedOrigins := make(map[string]struct{}, len(configuredOrigins))
	for _, origin := range configuredOrigins {
		if normalized, ok := normalizeCORSOrigin(origin); ok {
			allowedOrigins[normalized] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			c.Writer.Header().Add("Vary", "Origin")
			normalized, valid := normalizeCORSOrigin(origin)
			_, explicitlyAllowed := allowedOrigins[normalized]
			parsed, _ := url.Parse(origin)
			sameHost := valid && strings.EqualFold(parsed.Host, c.Request.Host)
			if !sameHost && !explicitlyAllowed {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-origin request denied"})
				return
			}
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Max-Age", "600")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// normalizeCORSOrigin validates and canonicalizes a serialized origin. CORS
// origins never contain credentials, paths, query strings, or fragments.
func normalizeCORSOrigin(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" || strings.EqualFold(raw, "null") {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), true
}

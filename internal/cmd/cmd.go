package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"

	"github.com/qianfree/team-api/internal/consts"
	adminController "github.com/qianfree/team-api/internal/controller/admin"
	captchaController "github.com/qianfree/team-api/internal/controller/captcha"
	docsController "github.com/qianfree/team-api/internal/controller/docs"
	openController "github.com/qianfree/team-api/internal/controller/open"
	settingsController "github.com/qianfree/team-api/internal/controller/settings"
	tenantController "github.com/qianfree/team-api/internal/controller/tenant"
	"github.com/qianfree/team-api/internal/dispatchadapter"
	"github.com/qianfree/team-api/internal/logic/admin"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/monitor"
	relayLogic "github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/logic/task"
	"github.com/qianfree/team-api/internal/logic/tenant"
	"github.com/qianfree/team-api/internal/logic/update"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/response"
	"github.com/qianfree/team-api/internal/utility/crypto"

	adminHandler "github.com/qianfree/team-api/internal/handler/admin"
	"github.com/qianfree/team-api/internal/handler/landing"
	"github.com/qianfree/team-api/internal/handler/public"
	"github.com/qianfree/team-api/internal/handler/relay"
	setupHandler "github.com/qianfree/team-api/internal/handler/setup"
	"github.com/qianfree/team-api/internal/plugin"
	"github.com/qianfree/team-api/web"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start http server of team-api",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			s := g.Server()

			printBanner()

			// Auto-migrate database schema
			if err := runAutoMigrate(ctx); err != nil {
				return err
			}

			// Health check: Redis
			if err := checkRedis(ctx); err != nil {
				g.Log().Fatalf(ctx, "Redis 连接检查失败，程序退出: %v", err)
			}
			g.Log().Info(ctx, "Redis 连接正常")

			// Initialize JWT secret
			common.InitJWTSecret(ctx)

			// 启动阶段校验加密密钥，避免运行时请求路径 panic（#3）
			if hexKey := g.Cfg().MustGet(ctx, "crypto.encryptionKey").String(); hexKey == "" {
				g.Log().Fatal(ctx, "crypto.encryptionKey 未配置，程序退出（运行 openssl rand -hex 32 生成）")
			} else if _, keyErr := crypto.GetEncryptionKey(hexKey); keyErr != nil {
				g.Log().Fatalf(ctx, "crypto.encryptionKey 配置无效，程序退出: %v", keyErr)
			}
			g.Log().Info(ctx, "加密密钥校验通过")

			// System initialization: env auto-init or detect setup mode
			autoInit, initErr := admin.AutoInitAdmin(ctx)
			if initErr != nil {
				g.Log().Errorf(ctx, "auto init admin: %v", initErr)
			}
			if !autoInit {
				exists, checkErr := admin.AdminExists(ctx)
				if checkErr != nil {
					g.Log().Errorf(ctx, "check admin existence: %v", checkErr)
				} else if !exists {
					setupHandler.SetSetupMode(true)
					g.Log().Info(ctx, "系统未初始化，进入设置模式 — 请访问 /setup 完成初始化")
				}
			}

			// Initialize config service
			common.Config().Warmup(ctx)
			common.Config().StartSubscriber(ctx)

			// C1: 通用 Cache 的跨实例 L1 失效订阅（cache:invalidate）
			common.StartCacheInvalidationSubscriber(ctx)

			// Initialize content filter engine
			common.InitContentFilter(ctx)

			// Initialize monitoring collector
			monitor.InitCollector(ctx)
			monitor.InitRequestTracker()
			monitor.InitRelaykitTracker()
			monitor.InitDispatchTracker()
			dispatchadapter.SetBreakerOpenHook(monitor.TrackDispatchBreakerOpen)

			// Ensure partitioned tables have current+future partitions
			if partitionErr := common.EnsurePartitions(ctx); partitionErr != nil {
				g.Log().Errorf(ctx, "ensure partitions: %v", partitionErr)
			}

			// Initialize async usage log writer
			common.InitUsageLogWriter()

			// Initialize async channel error writer
			common.InitChannelErrorWriter()

			// Initialize async audit log writer
			common.InitAuditLogWriter()

			// Initialize async error log writer
			response.InitErrorLogWriter()

			// Register cron jobs
			common.InitCronScheduler()
			cs := common.GetCronScheduler()
			registerCronJobs(cs)
			cs.StartBackground(ctx)

			// 启动钱包物化器（boot goroutine，秒级刷新 Redis 权威钱包状态到 DB；不走 cron）
			billing.StartWalletMaterializer(ctx)

			// Initialize update manager
			update.InitManager(ctx)
			update.CheckPendingVerification(ctx)

			// Initialize plugin system (must be after CronScheduler init)
			pluginApp := &plugin.App{
				Server: s,
				DB:     g.DB(),
				Redis:  g.Redis(),
				Hook:   plugin.GlobalEmitter(),
			}
			if pluginErr := plugin.Bootstrap(ctx, pluginApp); pluginErr != nil {
				g.Log().Errorf(ctx, "plugin bootstrap: %v", pluginErr)
			}

			// Global middleware
			s.Use(middleware.Recovery)
			s.Use(middleware.RequestId)
			s.Use(middleware.ServiceName)

			// Setup mode guard: block all requests until initialization is complete
			s.Use(func(r *ghttp.Request) {
				if !setupHandler.IsSetupMode() {
					r.Middleware.Next()
					return
				}
				// Double-check DB in case setup was completed by another request/instance
				exists, _ := admin.AdminExists(r.Context())
				if exists {
					setupHandler.SetSetupComplete()
					r.Middleware.Next()
					return
				}
				// Allow setup and health endpoints
				if r.URL.Path == "/setup" || r.URL.Path == "/api/setup/initialize" || r.URL.Path == "/api/setup/status" || r.URL.Path == "/api/health" {
					r.Middleware.Next()
					return
				}
				// Block everything else
				accept := r.Header.Get("Accept")
				if len(accept) >= 4 && accept[:4] == "text" {
					r.Response.RedirectTo("/setup")
				} else {
					r.Response.WriteJson(g.Map{
						"code":       consts.CodeSetupNotInitialized,
						"message":    consts.MsgSetupNotInitialized,
						"data":       nil,
						"request_id": r.GetCtxVar("RequestId"),
					})
				}
				r.Exit()
			})

			// Setup routes (always registered, guard decides if accessible)
			s.Group("/", func(group *ghttp.RouterGroup) {
				group.GET("/setup", setupHandler.HandleSetupPage)
			})
			s.Group("/api/setup", func(group *ghttp.RouterGroup) {
				group.Middleware(middleware.ErrorHandler)
				group.GET("/status", setupHandler.HandleSetupStatus)
				group.POST("/initialize", setupHandler.HandleSetupInitialize)
			})

			// Health check endpoint (always accessible, used by Docker/K8s probes)
			s.Group("/api", func(group *ghttp.RouterGroup) {
				group.GET("/health", func(r *ghttp.Request) {
					r.Response.WriteJson(g.Map{
						"status":  "ok",
						"version": consts.Version,
					})
				})
			})

			// Register route groups
			s.Group("/api", func(group *ghttp.RouterGroup) {
				group.Middleware(middleware.MiddlewareHandlerResponse)

				// Admin — public endpoints use g.Meta middleware:"-" to skip auth
				group.Group("/admin", func(g *ghttp.RouterGroup) {
					g.Middleware(middleware.DemoMode, middleware.AdminAuth, middleware.AdminPermissionGuard, middleware.OperationLog)
					g.Bind(adminController.NewV1())
				})

				// Tenant — public endpoints use g.Meta middleware:"-" to skip auth
				group.Group("/tenant", func(g *ghttp.RouterGroup) {
					g.Middleware(middleware.DemoMode, middleware.MaintenanceMode, middleware.TenantAuth, middleware.Idempotency)
					g.Bind(tenantController.NewV1())
				})

				// Payment callbacks — manual registration (raw string response, not JSON)
				group.Middleware(middleware.ErrorHandler)
				registerPaymentCallbacks(group)

				// Captcha — public, no auth required (shared by admin + tenant)
				group.Group("/captcha", func(g *ghttp.RouterGroup) {
					g.Bind(captchaController.NewV1())
				})
				// Settings — public settings, no auth required
				group.Group("/settings", func(g *ghttp.RouterGroup) {
					g.Bind(settingsController.NewV1())
				})

				// Docs — public OpenAPI spec
				group.Group("/docs", func(g *ghttp.RouterGroup) {
					g.Bind(docsController.NewV1())
				})
			})

			// File streaming serve — admin auth required, bypasses JSON response wrapper.
			// 流式返回二进制（local 存储模式 + 未来应用层代理下载），不能走统一 JSON 响应格式，
			// 故独立注册路由组（不套 MiddlewareHandlerResponse），只套鉴权与审计中间件。
			s.Group("/api/admin/files", func(group *ghttp.RouterGroup) {
				group.Middleware(middleware.AdminAuth, middleware.AdminPermissionGuard, middleware.OperationLog)
				group.GET("/{id}/serve", adminHandler.HandleFileServe)
			})

			// Open Platform API — HMAC-SHA256 authentication
			s.Group("/api/open", func(group *ghttp.RouterGroup) {
				group.Middleware(middleware.MiddlewareHandlerResponse)
				group.Middleware(middleware.OpenPlatformAuth)
				group.Bind(openController.NewV1())
			})

			// AI proxy endpoints (OpenAI compatible, /v1/xxx)
			registerRelayRoutes(s)

			// Register plugin routes
			plugin.RegisterAllRoutes(ctx, s)

			// Embedded frontend SPA serving
			registerFrontendRoutes(s)

			// Landing page for backend root when frontend is not embedded
			registerLandingPage(s)

			// Migrate encryption key (legacy default → configured key)
			relayLogic.MigrateEncryptionKey(ctx)

			// Initialize active task count and start polling
			task.InitActiveCount(ctx)
			task.StartAsyncPolling(ctx)

			// Start sync-image async worker pool (wraps synchronous image providers as async tasks)
			task.StartSyncImageWorkers(ctx)

			// Start webhook dispatcher (event-driven delivery)
			tenant.InitWebhookDispatcher(ctx)

			// 注入 webhook 发布函数到 billing 包（解耦循环依赖）
			billing.SetPublishWebhookEventFn(tenant.PublishWebhookEvent)

			// Graceful shutdown order (defers run LIFO after s.Run returns):
			// 1) 先排空任务池（StopSyncImageWorkers / StopAsyncPolling）——它们收尾时会写
			//    用量日志 / 审计 / 结算；此时异步 Writer 必须仍存活。
			// 2) 再 flush 关闭异步 Writer。
			// 3) 最后关 webhook / plugin。
			// 因此把两个 task.Stop* 注册在最后（最先执行），Writer 关闭注册在前（后执行）。
			defer plugin.Shutdown(ctx)
			defer tenant.ShutdownWebhookDispatcher()
			defer common.CloseAuditLogWriter()
			defer common.CloseChannelErrorWriter()
			defer common.CloseUsageLogWriter()
			defer response.CloseErrorLogWriter()
			defer task.StopAsyncPolling()
			defer task.StopSyncImageWorkers()

			s.Run()
			return nil
		},
	}
)

// printBanner prints the startup banner with copyright information.
func printBanner() {
	cyan := "\x1b[36;1m"
	green := "\x1b[32m"
	dim := "\x1b[2m"
	reset := "\x1b[0m"

	fmt.Println()
	fmt.Printf("  %s████████╗███████╗ █████╗ ███╗   ███╗       █████╗ ██████╗ ██╗%s\n", cyan, reset)
	fmt.Printf("  %s╚══██╔══╝██╔════╝██╔══██╗████╗ ████║      ██╔══██╗██╔══██╗██║%s\n", cyan, reset)
	fmt.Printf("  %s   ██║   █████╗  ███████║██╔████╔██║█████╗███████║██████╔╝██║%s\n", cyan, reset)
	fmt.Printf("  %s   ██║   ██╔══╝  ██╔══██║██║╚██╔╝██║╚════╝██╔══██║██╔═══╝ ██║%s\n", cyan, reset)
	fmt.Printf("  %s   ██║   ███████╗██║  ██║██║ ╚═╝ ██║      ██║  ██║██║     ██║%s\n", cyan, reset)
	fmt.Printf("  %s   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝      ╚═╝  ╚═╝╚═╝     ╚═╝%s\n", cyan, reset)
	fmt.Printf("  %sTeam-API%s %s%s%s  %s|  %s%s企业级大模型 API 网关系统%s\n", cyan, reset, green, consts.Version, reset, dim, reset, dim, reset)
	fmt.Printf("  %shttps://github.com/qianfree/team-api%s\n", dim, reset)
	fmt.Println()
	fmt.Printf("  %sAGPL v3.0 开源协议  |  Copyright © 2025-2026 Team-API Contributors%s\n", dim, reset)
	fmt.Println()
}

// registerCronJobs 集中注册所有定时任务，避免散落在主启动流程中。
// 新增 cron 任务时在此追加一项即可；任务名（用于分布式锁 key）需保持唯一且稳定。
func registerCronJobs(cs *common.CronScheduler) {
	cs.Register("ops_system_collector", "系统指标采集", "* * * * *", func(ctx context.Context) error {
		return monitor.CollectSystemMetrics(ctx)
	})
	cs.Register("ops_alert_detector", "告警检测", "* * * * *", func(ctx context.Context) error {
		return monitor.RunAlertDetection(ctx)
	})
	cs.Register("ops_metrics_cleanup", "指标数据清理", "0 3 * * *", func(ctx context.Context) error {
		return monitor.CleanupOldMetrics(ctx)
	})
	cs.Register("partition_ensure", "分区维护", "0 2 * * *", func(ctx context.Context) error {
		return common.EnsurePartitions(ctx)
	})
	cs.Register("health_snapshot", "渠道健康快照", "*/5 * * * *", func(ctx context.Context) error {
		return task.SnapshotHealthScores(ctx)
	})
	cs.Register("channel_auto_test", "渠道自动测试", "*/5 * * * *", func(ctx context.Context) error {
		if common.Config().GetBool(ctx, "channel_auto_test_enabled") {
			task.AutoTestChannels(ctx)
		}
		return nil
	})
	cs.Register("model_sunset_check", "模型下线检查", "0 0 * * *", func(ctx context.Context) error {
		return task.CheckModelSunset(ctx)
	})
	cs.Register("data_cleanup", "过期数据清理", "0 3 * * *", func(ctx context.Context) error {
		return admin.CleanupExpiredData(ctx)
	})
	cs.Register("export_file_cleanup", "导出文件清理", "0 4 * * *", func(ctx context.Context) error {
		return admin.CleanupExpiredExportFiles(ctx)
	})
	cs.Register("file_retention_check", "文件保留检查", "0 5 * * *", func(ctx context.Context) error {
		return admin.CheckFileRetention(ctx)
	})
	cs.Register("task_timeout_check", "任务超时检查", "*/10 * * * *", func(ctx context.Context) error {
		return admin.MarkStuckTasksFailed(ctx)
	})
	cs.Register("task_executor", "异步任务执行", "*/1 * * * *", func(ctx context.Context) error {
		task.RunPendingTasks(ctx)
		return nil
	})
	cs.Register("project_budget_check", "项目预算检查", "*/5 * * * *", func(ctx context.Context) error {
		return tenant.CheckBudgetExhausted(ctx)
	})
	cs.Register("usage_log_cleanup", "用量日志清理", "0 3 * * *", func(ctx context.Context) error {
		retentionDays := common.Config().GetInt(ctx, "usage_log_retention_days")
		if retentionDays == 0 {
			retentionDays = 90
		}
		return task.ScheduleAutoCleanup(ctx, retentionDays)
	})
	cs.Register("usage_daily_aggregate", "用量日报表聚合", "0 1 * * *", func(ctx context.Context) error {
		// 用量日维度聚合：将 bil_usage_logs 聚合进 bil_usage_daily，供流量桑基图与趋势分析。
		// 自愈：每次重算最近 3 个完整天，覆盖短暂宕机；ON CONFLICT 保证幂等。
		// end 取今天 00:00（开区间），永不聚合当天进行中的数据。
		end := time.Now().Format("2006-01-02")
		start := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
		return task.AggregateUsageRange(ctx, start, end)
	})
	cs.Register("oauth_token_refresh", "OAuth 令牌刷新", "*/10 * * * *", func(ctx context.Context) error {
		return task.RefreshExpiringOAuthTokens(ctx)
	})
	cs.Register("prededuct_sweep", "预扣清扫", "*/2 * * * *", func(ctx context.Context) error {
		billing.PredeductSweep(ctx)
		return nil
	})
	cs.Register("billing_daily_reconciliation", "计费日对账", "20 5 * * *", func(ctx context.Context) error {
		// 日对账：聚合对账（bil_records vs bil_transactions）+ 交叉对账（usage_logs 反连接
		// 发现漏结算免单请求）+ 冻结余额一致性校验，结果经日志告警
		_, err := billing.RunDailyReconciliation(ctx)
		return err
	})
	cs.Register("update_check", "系统更新检查", "0 */6 * * *", func(ctx context.Context) error {
		if common.Config().GetBool(ctx, "update_auto_check_enabled") {
			return update.BackgroundCheck(ctx)
		}
		return nil
	})
	cs.Register("order_expiration", "订单过期处理", "*/5 * * * *", func(ctx context.Context) error {
		return task.ExpirePendingOrders(ctx)
	})
}

// registerPaymentCallbacks registers payment callback routes.
func registerPaymentCallbacks(group *ghttp.RouterGroup) {
	group.Group("/payment", func(g *ghttp.RouterGroup) {
		g.POST("/callback/{channel}", public.HandlePaymentCallback)
		g.GET("/callback/{channel}", public.HandlePaymentCallback)
		g.GET("/epay/return", public.HandlePaymentEpayReturn)
		g.POST("/epay/return", public.HandlePaymentEpayReturn)
	})
}

// registerRelayRoutes registers AI proxy routes (/v1/xxx).
func registerRelayRoutes(server *ghttp.Server) {
	server.Group("/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.ApiMaintenance, middleware.MaintenanceMode, middleware.ApiKeyAuth, middleware.ContentFilter)

		group.POST("/chat/completions", relay.HandleChatCompletions)
		group.GET("/models", relay.HandleModels)
		group.GET("/models/{model_id}", relay.HandleModelDetail)
		group.POST("/embeddings", relay.HandleEmbeddings)
		group.POST("/images/generations", relay.HandleImagesGenerations)
		group.POST("/completions", relay.HandleCompletions)
		group.POST("/responses", relay.HandleResponses)
		group.POST("/responses/compact", relay.HandleResponses)
		// Responses 生命周期端点（Redis 路由还原渠道后透传上游）
		group.GET("/responses/{id}", relay.HandleResponsesRetrieve)
		group.POST("/responses/{id}/cancel", relay.HandleResponsesCancel)
		group.DELETE("/responses/{id}", relay.HandleResponsesDelete)
		group.POST("/messages", relay.HandleMessages)
		group.POST("/audio/speech", relay.HandleAudioSpeech)
		group.POST("/audio/transcriptions", relay.HandleAudioTranscription)
		group.POST("/audio/translations", relay.HandleAudioTranslation)
		group.POST("/rerank", relay.HandleRerank)
		group.POST("/moderations", relay.HandleModerations)
		group.POST("/images/edits", relay.HandleImagesEdits)
		group.GET("/realtime", relay.HandleRealtime)
	})

	// Gemini 兼容路由（/v1beta/models/{model}:generateContent）
	server.Group("/v1beta", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.ApiMaintenance, middleware.MaintenanceMode, middleware.ApiKeyAuth, middleware.ContentFilter)
		group.GET("/models", relay.HandleGeminiModels)
		group.GET("/models/{model}", relay.HandleGeminiModelDetail)
		group.POST("/models/{model}", relay.HandleGeminiGenerateContent)
	})

	// 异步任务端点（视频/音乐生成）
	server.Group("/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.ApiMaintenance, middleware.MaintenanceMode, middleware.ApiKeyAuth, middleware.ContentFilter)
		group.POST("/video/generations", relay.HandleTaskSubmit)
		group.GET("/video/generations/{task_id}", relay.HandleTaskFetch)
	})

	// 异步图片生成端点（阿里云 DashScope 等）
	server.Group("/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.ApiMaintenance, middleware.MaintenanceMode, middleware.ApiKeyAuth, middleware.ContentFilter)
		group.POST("/images/generations/async", relay.HandleAliImageSubmit)
		group.GET("/images/generations/async/{task_id}", relay.HandleTaskFetch)
	})

	// Suno 端点
	server.Group("/suno", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.ApiMaintenance, middleware.MaintenanceMode, middleware.ApiKeyAuth, middleware.ContentFilter)
		group.POST("/submit/{action}", relay.HandleTaskSubmit)
		group.POST("/fetch", relay.HandleSunoFetchBatch)
		group.GET("/fetch/{task_id}", relay.HandleTaskFetch)
	})
}

// registerLandingPage 在未内嵌前端（非 embedweb 构建）时，为后端根路径注册项目介绍页，
// 避免浏览器直接访问后端地址时显示 404。内嵌前端时根路径由租户 SPA 承接，此页不注册。
func registerLandingPage(s *ghttp.Server) {
	if web.Enabled {
		return
	}
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.GET("/", landing.HandleLanding)
		group.GET("/index.html", landing.HandleLanding)
	})
}

// registerFrontendRoutes serves embedded frontend SPA assets.
// Admin console at /admin, tenant console at / (catch-all).
// Existing API routes take priority over these wildcard routes.
// When built without the "embedweb" tag, this is a no-op.
func registerFrontendRoutes(s *ghttp.Server) {
	if !web.Enabled {
		return
	}

	adminSub, _ := fs.Sub(web.AdminFS, "admin/dist")
	tenantSub, _ := fs.Sub(web.TenantFS, "tenant/dist")

	// Admin SPA: /admin/* → web/admin/dist/
	s.Group("/admin", func(group *ghttp.RouterGroup) {
		group.ALL("/*any", ghttp.WrapF(spaHandler(adminSub, "/admin")))
	})

	// Tenant SPA: /* → web/tenant/dist/ (lowest priority catch-all)
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.ALL("/*any", ghttp.WrapF(spaHandler(tenantSub, "")))
	})
}

// spaHandler returns an http.HandlerFunc that serves static files from the
// given filesystem, falling back to index.html for SPA client-side routing.
func spaHandler(root fs.FS, prefix string) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(root))
	if prefix != "" {
		fileServer = http.StripPrefix(prefix, fileServer)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Resolve the file path within the embedded FS
		path := strings.TrimPrefix(r.URL.Path, prefix)
		path = strings.TrimPrefix(path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to open the file; if it exists, serve it directly
		if f, err := root.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html (SPA fallback)
		indexBytes, err := fs.ReadFile(root, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexBytes)
	}
}

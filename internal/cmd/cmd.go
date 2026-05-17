package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

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
	"github.com/qianfree/team-api/internal/logic/admin"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/internal/logic/task"
	"github.com/qianfree/team-api/internal/logic/tenant"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/response"

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

			// Initialize JWT secret
			common.InitJWTSecret(ctx)

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

			// Initialize content filter engine
			common.InitContentFilter(ctx)

			// Initialize monitoring collector
			monitor.InitCollector(ctx)
			monitor.InitRequestTracker()

			// Ensure partitioned tables have current+future partitions
			if partitionErr := common.EnsurePartitions(ctx); partitionErr != nil {
				g.Log().Errorf(ctx, "ensure partitions: %v", partitionErr)
			}

			// Initialize async usage log writer
			common.InitUsageLogWriter()

			// Initialize async error log writer
			response.InitErrorLogWriter()

			// Register cron jobs
			common.InitCronScheduler()
			cs := common.GetCronScheduler()
			cs.Register("ops_system_collector", "* * * * *", func(ctx context.Context) error {
				return monitor.CollectSystemMetrics(ctx)
			})
			cs.Register("ops_alert_detector", "* * * * *", func(ctx context.Context) error {
				return monitor.RunAlertDetection(ctx)
			})
			cs.Register("ops_metrics_cleanup", "0 3 * * *", func(ctx context.Context) error {
				return monitor.CleanupOldMetrics(ctx)
			})
			cs.Register("partition_ensure", "0 2 * * *", func(ctx context.Context) error {
				return common.EnsurePartitions(ctx)
			})
			cs.Register("health_snapshot", "*/5 * * * *", func(ctx context.Context) error {
				return task.SnapshotHealthScores(ctx)
			})
			cs.Register("channel_auto_test", "*/5 * * * *", func(ctx context.Context) error {
				task.AutoTestChannels(ctx)
				return nil
			})
			cs.Register("model_sunset_check", "0 0 * * *", func(ctx context.Context) error {
				return task.CheckModelSunset(ctx)
			})
			cs.Register("data_cleanup", "0 3 * * *", func(ctx context.Context) error {
				return admin.CleanupExpiredData(ctx)
			})
			cs.Register("export_file_cleanup", "0 4 * * *", func(ctx context.Context) error {
				return admin.CleanupExpiredExportFiles(ctx)
			})
			cs.Register("file_retention_check", "0 5 * * *", func(ctx context.Context) error {
				return admin.CheckFileRetention(ctx)
			})
			cs.Register("task_timeout_check", "*/10 * * * *", func(ctx context.Context) error {
				return admin.MarkStuckTasksFailed(ctx)
			})
			cs.Register("task_executor", "*/1 * * * *", func(ctx context.Context) error {
				task.RunPendingTasks(ctx)
				return nil
			})
			cs.Register("usage_log_cleanup", "0 3 * * *", func(ctx context.Context) error {
				retentionDays := common.Config().GetInt(ctx, "usage_log_retention_days")
				if retentionDays == 0 {
					retentionDays = 90
				}
				return task.ScheduleAutoCleanup(ctx, retentionDays)
			})
			cs.Register("oauth_token_refresh", "*/10 * * * *", func(ctx context.Context) error {
				return task.RefreshExpiringOAuthTokens(ctx)
			})
			cs.Register("cron_execution_cleanup", "30 3 * * *", func(ctx context.Context) error {
				retentionDays := common.Config().GetInt(ctx, "cron_execution_retention_days")
				if retentionDays == 0 {
					retentionDays = 30
				}
				_, err := g.DB().Ctx(ctx).Exec(ctx,
					"DELETE FROM sys_cron_job_executions WHERE created_at < NOW() - ($1 || ' days')::interval",
					retentionDays,
				)
				return err
			})
			cs.StartBackground(ctx)

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
					g.Middleware(middleware.DemoMode, middleware.AdminAuth, middleware.OperationLog)
					g.Bind(adminController.NewV1())
				})

				// Tenant — public endpoints use g.Meta middleware:"-" to skip auth
				group.Group("/tenant", func(g *ghttp.RouterGroup) {
					g.Middleware(middleware.DemoMode, middleware.MaintenanceMode, middleware.TenantAuth)
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

			// Initialize active task count and start polling
			task.InitActiveCount(ctx)
			go task.StartAsyncPolling(ctx)

			// Start webhook dispatcher (event-driven delivery)
			tenant.InitWebhookDispatcher(ctx)

			// Flush usage log writer on shutdown (s.Run blocks until server stops)
			defer plugin.Shutdown(ctx)
			defer common.CloseUsageLogWriter()
			defer response.CloseErrorLogWriter()

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

// registerPaymentCallbacks registers payment callback routes.
func registerPaymentCallbacks(group *ghttp.RouterGroup) {
	group.Group("/payment", func(g *ghttp.RouterGroup) {
		g.POST("/callback/{channel_id}", public.HandlePaymentCallback)
		g.GET("/callback/{channel_id}", public.HandlePaymentCallback)
		g.GET("/epay/return/{channel_id}", public.HandlePaymentEpayReturn)
		g.POST("/stripe/webhook/{channel_id}", public.HandlePaymentStripeWebhook)
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

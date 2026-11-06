# AI请求分析：插件 → 核心功能迁移

## 背景

`plugins/ai-req-analyze`（约 2600 行 Go + 4 个 Vue 页面）是挂在插件框架上的 AI 使用行为审计功能：从审计日志采样会话单元 → 调独立分析 AI → 产出风险报告。现要求改为系统原生功能：删除插件实现，代码按核心模块规范（api → controller → service → logic → dao 分层、goose 迁移、静态 RBAC、sys_options 配置）迁入 `internal/`，存量数据/配置/角色授权无缝迁移。

**已确认决策**：权限点重命名为 `ai_analysis:view/analyze/config`（迁移脚本改写存量授权）；配置保留功能页内抽屉（底层换 sys_options 存储 + AES-256 加密）。

## 命名对照

| 项 | 插件（旧） | 核心（新） |
|---|---|---|
| 表 | `plg_ai_req_analyze_reports/findings/jobs` | `aud_ai_analysis_reports/findings/jobs`（aud_ 审计域前缀） |
| 路由 | `/api/admin/plugins/ai-req-analyze/*` | `/api/admin/ai-analysis/*` |
| 权限点 | `ai-req-analyze:view/analyze/config` | `ai_analysis:view/analyze/config` |
| 配置 | `sys_plugins.config` JSON + `plugin.ai-req-analyze.*` | `sys_options` 键 `ai_analysis_*`（api key 延续 AES-256-GCM） |
| cron | `plugin_ai-req-analyze_patrol/_retention_cleanup` | `ai_analysis_patrol` / `ai_analysis_retention_cleanup` |

## 一、数据库迁移 `migrations/000030_ai_analysis.sql`（goose）

**Up**（幂等，覆盖三种部署状态：装过插件有数据 / 装过无数据 / 从未安装）：
1. DO 块：若 `plg_ai_req_analyze_reports` 存在 → 三表 `RENAME TO aud_ai_analysis_*` + 重命名 7 个索引 + 更新表注释（去掉"插件"字样）；否则 → 按核心规范 CREATE 三表（DDL 取自 `plugins/ai-req-analyze/installer.go`，含全部列注释、BRIN/组合索引）
2. 配置迁移：`INSERT INTO sys_options ... SELECT 'ai_analysis_<key>', config::jsonb->>'<key>' FROM sys_plugins WHERE name='ai-req-analyze'`，映射 15 个键（`ai_base_url`→`ai_analysis_base_url` 等；`ai_api_key` 原样搬运，`v1:` 密文与新读取方兼容）+ `ON CONFLICT (key) DO NOTHING`
3. 权限点改写：`UPDATE sys_admin_role_perms SET permission_point = 'ai_analysis:' || split_part(permission_point, ':', 2) WHERE permission_point LIKE 'ai-req-analyze:%'`
4. 清理插件痕迹：`DELETE FROM sys_plugins / tnt_tenant_plugins`（name='ai-req-analyze'）、`DELETE FROM sys_options WHERE key LIKE 'plugin.ai-req-analyze.%'`

**Down**：表名改回 plg_ 前缀（IF EXISTS 防护）、权限点反向改写、删除 `ai_analysis_%` 配置键（尽力恢复，注释说明）。

## 二、配置体系扩展（sys_options + AES 加密）

- `internal/logic/common/settings_registry.go`：`Registry` 新增 15 个 `ai_analysis_*` 键（不加入 `Categories`，配置界面走功能页自有端点）；`SettingDef` 新增 `Encrypted bool` 字段
- `internal/logic/common/config.go`：
  - `SetOption`：`def.Encrypted` 且值非 `v1:` 密文时，经 `uc.EncryptString`（`internal/utility/crypto`，密钥 `crypto.encryptionKey`，复用 `internal/plugin/config.go:32` 的取键方式）加密后落库
  - 新增 `GetSecret(ctx, key)`：读取并解密（`v1:` 前缀判断，解密失败回空串、明文历史值原样返回，对齐 `plugin.decryptSecretFields` 语义）
  - `UpdateCategory` 已有的 `******` 掩码跳过逻辑直接复用

## 三、后端引擎迁移 `internal/logic/aianalysis/`（新包）

按文件平移 `plugins/ai-req-analyze/`，逐项适配：

| 源文件 | 适配点 |
|---|---|
| `analyzer.go` / `sampler.go` / `aiclient.go` | 基本原样；表名常量、日志前缀、包名；继续用 `lcommon.AuditModelCtx/LoadConversationView/MaskSensitiveData/GetAuditLevels/LogSensitiveAccess` |
| `model.go` | 优先 `gf gen dao` 生成 `AudAiAnalysisReports` 等 dao/do/entity 后改用 dao；CLI 不可用则保留手写 DO（`g.Meta orm:"table:aud_ai_analysis_xxx"` 方式） |
| `config.go` | `LoadConfig` 改读 `common.Config().GetOption/GetSecret`，键名换 `ai_analysis_*`；`configFromMap` 纯函数保留供测试（键名同步改） |
| `jobrunner.go` | 原样（惰性 worker 池、条件更新认领）；`markStaleJobsFailed` 导出为 `RecoverStaleJobs(ctx)` 供启动调用 |
| `patrol.go` | 拆出 `RunPatrol(ctx) error` / `RunRetentionCleanup(ctx) error` 普通函数供 cron 注册；去掉 Cronable/CronJobDef 壳 |
| `installer.go` / `plugin.go` / `permission.go` / `routes.go` / `api.go` | 删除（职责转移到迁移脚本、新 api/logic、rbac.go） |

cron 表达式热更新（改善插件版需禁用→启用的限制）：参照 `cmd.go:396 watchChannelAutoTestInterval` 模式，监听 `ai_analysis_patrol_cron` 变更 `Reschedule`。

## 四、API / 控制器 / 服务层（按 CLAUDE.md 生成顺序）

1. **`api/admin/v1/ai_analysis.go`**（手写）：8 个端点 Req/Res，g.Meta 路由注解：
   - `POST /ai-analysis/analyze`、`GET /ai-analysis/reports`、`GET /ai-analysis/reports/{id}`、`GET /ai-analysis/reports/{id}/conversation?finding_id=`、`DELETE /ai-analysis/reports/{id}`、`GET /ai-analysis/jobs`、`GET/PUT /ai-analysis/config`、`POST /ai-analysis/test`
   - 参数解析逻辑（时间窗校验、31 天上限、分页 clamp）从插件 `api.go`/`routes.go` 迁入；config GET 用 `Config().GetCategoryWithValues`（敏感掩码），PUT 用 `Config().UpdateCategory`（掩码跳过）
2. **`internal/logic/admin/ai_analysis.go`**：`sAdmin` 方法（CreateAiAnalysisJob / ListAiAnalysisReports / GetAiAnalysisReportDetail / GetAiAnalysisReportConversation / DeleteAiAnalysisReport / ListAiAnalysisJobs / GetAiAnalysisConfig / UpdateAiAnalysisConfig / TestAiAnalysisAI），业务委托 `aianalysis` 包；敏感访问日志 `LogSensitiveAccess` 调用保留
3. **代码生成**：优先 `go install github.com/gogf/gf/cmd/gf/v2@v2.10.2` 后执行 `gf gen service` + `gf gen ctrl -m`；CLI 装不上则按现有生成物格式手写追加 `api/admin/admin.go`（IAdminV1）、`internal/service/admin.go`、`internal/controller/admin/admin_v1_ai_analysis.go`，随后 `grep CodeNotImplemented` 验证接线
4. **`internal/cmd/cmd.go`**：`registerCronJobs` 追加两个任务 + patrol_cron 配置监听；启动流程（cron 初始化附近）调用 `aianalysis.RecoverStaleJobs(ctx)`；`plugins/registry.go` 移除 ai-req-analyze 导入

## 五、权限体系

- `internal/middleware/rbac.go` 静态规则（默认拒绝，漏配即 403）：
  - `POST /api/admin/ai-analysis/analyze`、`DELETE prefix /api/admin/ai-analysis/reports/` → `ai_analysis:analyze`
  - `GET prefix /api/admin/ai-analysis/reports`、`GET /api/admin/ai-analysis/jobs` → `ai_analysis:view`
  - `GET/PUT /api/admin/ai-analysis/config`、`POST /api/admin/ai-analysis/test` → `ai_analysis:config`
- `internal/logic/admin/permission.go`：`predefinedPermissionGroups` 新增 `ai_analysis` 组（3 权限点）
- `internal/logic/admin/role.go`：`tierPermissions["ai_analysis"]` 三档（read=view；operate=+analyze；full=+config），与插件档位语义一致

## 六、前端 `web/admin`

- `views/plugin/ai-req-analyze/` → `views/security/ai-analysis/`（4 个组件全部保留）：
  - API 路径 `/admin/plugins/ai-req-analyze/*` → `/admin/ai-analysis/*`（含 ConfigDrawer 的 config 读写与 `ConversationDrawer` 的 fetchUrl）
  - 权限点字符串三处更新（路由 meta / 菜单 / 按钮 hasPermission）
  - 删除 `pluginUnavailable` 404 降级与"前往插件管理"引导（核心功能常驻）
  - 删除空目录 `views/plugin/`
- `router/admin.ts:190`：路由 `path: 'ai-analysis'`、`meta.perm: 'ai_analysis:view'`
- `views/AdminLayout.vue`：菜单项 perm 更新、注释改为核心模块说明

## 七、测试迁移与清理

- 6 个插件测试文件 → `internal/logic/aianalysis/`（`config_test` 键名改 `ai_analysis_*`，其余为纯函数测试基本原样）
- 删除 `plugins/ai-req-analyze/` 整目录；检查 `internal/logic/admin/plugin_permission_test.go`、`internal/middleware/rbac_coverage_test.go` 是否引用旧权限点并同步更新
- `CLAUDE.md` 项目结构中 plugins 目录描述更新（去掉 ai-req-analyze，仅剩 example）

## 验证

1. **迁移**：`go run . migrate status` → `migrate up`；对已装插件的库验证表重命名+数据保留+配置/权限点迁移，对空库验证建表分支
2. **后端**：`go build ./...`；`go test ./internal/logic/aianalysis/... ./internal/logic/admin/... ./internal/middleware/...`；`grep -r "ai-req-analyze" --include="*.go"` 应仅剩迁移脚本
3. **生成物**：`grep -r CodeNotImplemented internal/controller/` 无新增桩
4. **端到端**（本地起服务）：配置抽屉读写（api key 掩码保持语义、连通性测试）→ 发起分析 → 任务进度 → 报告详情/会话还原 → 删除；无 `ai_analysis:view` 权限的账号访问返回 403；插件管理页不再出现 AI请求分析
5. **前端**：`cd web/admin && bun run build`（或 type-check）通过
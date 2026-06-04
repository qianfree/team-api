export const meta = {
  name: "team-api-code-audit",
  description: "Multi-agent code audit of team-api: quality, style consistency, GoFrame conventions, DB operations, API style, and bugs across hand-written logic/ and relay/ code.",
  phases: ["review", "synthesis"],
};

// ---- Shared rubric injected into every reviewer ----
const RUBRIC = `
你是资深 Go / GoFrame v2 代码审查员，正在审查多租户大模型 API 网关 SaaS 项目 team-api。

## 第一步（必做）
先阅读以下文件建立评审基线，不要跳过：
- CLAUDE.md（项目规范：分层、命名、响应格式、货币规则、多租户隔离、DoD）
- docs/reference/goframe-conventions.md（GoFrame 使用规范 + 末尾"已修复的框架使用错误记录"——这是高频反模式清单，重点据此排查）
- docs/reference/api-format-reference.md（响应格式 / 错误映射，仅 API 风格相关时需要）

## 审查维度（6 项，逐项覆盖）
1. **代码质量**：可读性、重复代码、过长函数、错误处理完整性、资源泄漏（未关闭的 rows/file/ctx）、并发安全（map 竞态、缺锁）、panic 风险（解引用未判 nil、越界、类型断言无 ok）。
2. **代码风格统一**：是否与周边代码一致；命名（蛇形 URL、驼峰 Go 标识符）；tab 缩进；注释密度；是否偏离 GoFrame 标准分层（业务逻辑只应在 logic/cmd/handler/middleware/utility/consts/relay 中）。
3. **GoFrame 规范**：DAO 模式（dao.Xxx.Ctx(ctx) 而非 g.DB().Model("table")）；DO 结构体而非 Data(g.Map{})；时间用 gtime；错误用 gerror；日志用 g.Log()；上下文传递；自动维护 created_at/updated_at（不手动 set）。
4. **数据库操作**：SQL 注入（fmt.Sprintf 拼 SQL → 必须参数化 ?）；写操作错误被静默丢弃（_, _ = ...）；Scan 必须用指针类型且改指针后要判 nil 防 panic；事务边界是否正确（资金/多表写是否在事务内）；N+1 查询；缺索引的高频查询；多租户隔离（tenant_id 过滤是否遗漏——越权读写其他租户数据是严重安全问题）。
5. **接口风格**：统一响应格式 {code,message,data,request_id}（管理类接口必须走 internal/response，禁止裸 g.Map）；代理接口 /v1 等禁止统一响应、需原生透传；分页结构 {list,total,page,page_size}；业务错误码 >=10000；message 必须中文且不暴露技术细节；权限校验是否覆盖端点。
6. **Bug / 隐患**：逻辑错误、边界条件、货币精度（NUMERIC(20,10)，展示 6 位；USD/CNY 混用错误是严重 bug）、计费预扣/结算/退款一致性、缓存失效与脏读、JWT/鉴权绕过、幂等性、TOCTOU 竞态。

## 输出要求
聚焦真实问题，避免吹毛求疵的风格噪音。每条 finding 给出：
- **严重级别**：Critical（安全/资金/数据损坏/panic 崩溃）/ High（功能 bug/明显隐患）/ Medium（规范违反/质量问题）/ Low（风格/小优化）
- **维度**：上面 6 项之一
- **位置**：file:line（必须精确，可点击）
- **问题**：简述
- **建议**：怎么修

## 写报告
将完整 findings 以 Markdown 写入文件：{REPORT_PATH}
报告结构：标题 + 一句话模块概述 + 「按严重级别分组的 findings 表/列表」+ 「亮点（做得好的地方，可选）」。
最后向调用者返回一段不超过 200 字的中文摘要：Critical/High 数量 + 最关键的 1-3 个问题。
`;

function reviewer(title, scopeDesc, paths, reportFile, model) {
  const prompt = RUBRIC
    .replace("{REPORT_PATH}", `docs/code-review/${reportFile}`)
    + `\n\n## 本次审查范围：${title}\n${scopeDesc}\n\n重点审查以下路径（只读这些范围内的源码，generated 代码 controller/dao/model/service 无需评审除非发现手改痕迹）：\n${paths}\n`;
  return agent(prompt, {
    description: `审查 ${title}`,
    model: model || "sonnet",
  });
}

const reviewers = [
  () => reviewer(
    "计费引擎 (billing)",
    "资金正确性最高优先级：预扣/结算/退款一致性、钱包冻结余额并发、USD/CNY 换算、精度、限流原子性、对账。",
    "internal/logic/billing/ 全部 *.go（含 wallet/settlement/pricing/reconciliation/ratelimit/currency/member_quota/level/scope/snapshot/summary/task_billing/alert/provider）。测试文件也看一眼覆盖是否充分。",
    "01-billing.md",
    "opus",
  ),
  () => reviewer(
    "管理后台-渠道与模型 (admin A)",
    "渠道管理、Key 轮询、模型定价、租户等级/乘数、模型导入导出。",
    "internal/logic/admin/channel.go channel_error.go channel_oauth.go channel_test_request.go channel_testing.go model.go model_group.go model_import_export.go tenant.go tenant_level.go tenant_model.go dashboard.go",
    "02-admin-channel-model.md",
  ),
  () => reviewer(
    "管理后台-鉴权与安全 (admin B)",
    "管理员账号、登录鉴权、审计、安全设置、RBAC 权限、系统设置、初始化、数据治理、日志清理、定时任务。",
    "internal/logic/admin/admin_user.go auth.go audit.go security.go permission.go settings.go setup.go data_governance.go error_log.go cron_job.go usage_log_cleanup.go admin.go",
    "03-admin-auth-security.md",
  ),
  () => reviewer(
    "管理后台-运营业务 (admin C)",
    "套餐、优惠码、兑换码、订单、成员、插件、工单、反馈、帮助中心、更新日志、通知、任务管理。",
    "internal/logic/admin/plan.go promo_code.go redemption.go order.go member.go plugin.go ticket.go feedback.go help_center.go changelog.go notification.go task_management.go",
    "04-admin-ops.md",
  ),
  () => reviewer(
    "租户控制台-身份与成员 (tenant A)",
    "登录/OAuth、成员管理、成员导入、模型范围、成员额度、邀请、组织、项目预算。多租户隔离与越权风险重点关注。",
    "internal/logic/tenant/auth.go oauth.go member.go member_import.go member_model_scope.go member_quota.go invitation.go organization.go project.go tenant.go lifecycle.go",
    "05-tenant-identity.md",
  ),
  () => reviewer(
    "租户控制台-计费与开放平台 (tenant B)",
    "API Key、租户侧计费、订单、套餐、优惠码、兑换码、开放平台、Webhook 分发与 worker、安全设置。",
    "internal/logic/tenant/api_key.go billing.go order.go plan.go promo_code.go redemption.go open_platform.go webhook_dispatcher.go webhook_worker.go security.go",
    "06-tenant-billing-open.md",
  ),
  () => reviewer(
    "租户控制台-业务功能 (tenant C)",
    "通知、playground、模型与对比、仪表盘、审计配置、邮件、反馈、帮助、工单、插件、任务。",
    "internal/logic/tenant/notification.go playground.go model.go model_comparison.go dashboard.go personal_dashboard.go audit_config.go email.go feedback.go help_center.go ticket.go plugin.go task.go",
    "07-tenant-features.md",
  ),
  () => reviewer(
    "公共逻辑 (common)",
    "双层缓存、配置、安全/加密、JWT、邮件、验证码、会话、分页、批量写、内容过滤、存储工厂(S3/OSS/COS)、定时、事件、特性开关、用量日志写入。安全与缓存一致性重点。",
    "internal/logic/common/ 全部 *.go 及 oauth/ 子目录",
    "08-common.md",
  ),
  () => reviewer(
    "Relay 供应商适配器 (relay/channel)",
    "各供应商适配器：请求/响应协议转换正确性、流式处理、错误透传、token 计量、边界。",
    "relay/channel/ 全部子目录（openai/claude/gemini/ali/zhipu/vertex/tencent/minimax/dify/coze/ollama 等）与 registry.go",
    "09-relay-channels.md",
  ),
  () => reviewer(
    "Relay 核心 (relay core)",
    "请求处理器、流式 helper、渠道调度/亲和/重试、override 改写、dto、taskchannel，以及 internal/logic/relay。代理接口必须原生透传、禁止统一响应包装；流式 ResponseWriter 操作正确性。",
    "relay/handler/ relay/helper/ relay/scheduler/ relay/override/ relay/common/ relay/dto/ relay/constant/ relay/taskchannel/ internal/logic/relay/ internal/handler/relay/",
    "10-relay-core.md",
    "opus",
  ),
  () => reviewer(
    "横切层 (middleware/handler/cmd/response/utility/consts)",
    "中间件（鉴权、租户注入、统一响应、限流、recover）、特殊端点 handler（支付回调/setup）、路由注册、统一响应包、工具（crypto/totp/turnstile/export）、常量。typed-nil interface 陷阱、中间件顺序、recover 覆盖。",
    "internal/middleware/ internal/handler/public/ internal/handler/setup/ internal/cmd/ internal/response/ internal/utility/ internal/consts/ internal/plugin/",
    "11-cross-cutting.md",
  ),
  () => reviewer(
    "API 定义与接口风格 (api/)",
    "Req/Res 结构体与路由注解：URL 蛇形小写、统一响应约定、分页结构、字段命名、tag 一致性、鉴权中间件分组是否齐全、代理接口与管理接口的区分。",
    "api/ 全部 *.go（admin/v1, tenant/v1, open/v1, captcha, docs, settings 等）。重点看路由注解与请求结构体一致性，不必逐字段。",
    "12-api-style.md",
  ),
  () => reviewer(
    "数据库迁移与 Schema (migrations) + 其余逻辑",
    "迁移脚本：PostgreSQL 语法、表前缀规范、字段注释、NUMERIC(20,10) 金额、无外键、BRIN 索引用于追加表、id/created_at/updated_at 齐全、goose up/down 幂等。另含 monitor/task/payment/open/docs/settings 逻辑。",
    "migrations/*.sql 全部；internal/logic/monitor/ internal/logic/task/ internal/logic/payment/ internal/logic/open/ internal/logic/docs/ internal/logic/settings/",
    "13-db-and-misc.md",
  ),
];

// NOTE: parallel() takes an array of THUNKS (() => agent(...)), not invoked promises.
// phase() callbacks do not execute in this runtime, so run the work directly at top level.
await parallel(reviewers);

await agent(
    `你是首席工程师，负责汇总 team-api 全项目代码审查结果。

13 个审查 agent 已将各自 findings 写入 docs/code-review/ 目录下的 01-*.md 至 13-*.md。

任务：
1. 读取 docs/code-review/ 下所有 01~13 编号的报告文件。
2. 去重、归并跨模块的同类问题（例如多个模块都有的 SQL 注入 / 缺 tenant_id 过滤 / 静默丢错），识别系统性模式。
3. 产出一份高质量中文汇总报告，写入 docs/code-review/SUMMARY.md，结构：
   - # team-api 代码审查汇总
   - ## 总体评价（2-3 段：整体工程质量、规范遵循度、主要风险面）
   - ## 严重问题清单（Critical + High，按优先级排序的表格：级别 | 维度 | 模块 | 位置 file:line | 问题 | 建议）
   - ## 系统性模式（反复出现的问题类别，给出受影响模块清单与统一整改建议）
   - ## 分模块评分（每模块一行：模块 | Critical | High | Medium | Low | 简评）
   - ## 各维度小结（代码质量 / 风格统一 / GoFrame 规范 / 数据库 / 接口风格 / Bug 六个维度各一段）
   - ## 优先整改建议（Top 10 行动项，按性价比排序）
4. 向调用者返回一段中文执行摘要（控制在 400 字内）：整体结论 + Critical/High 总数 + 最该先修的 3-5 件事。

注意：尊重各 agent 给出的 file:line，不要臆造位置。若某报告缺失或为空，在汇总中说明。`,
    { description: "汇总审查报告", model: "opus" },
);

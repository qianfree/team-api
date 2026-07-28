# Team-API 后端代码质量分析报告

> 审查范围：**全部后端手写 Go 代码**（~200 个文件），不含 `gf` 脚手架自动生成的目录（`internal/controller/`、`internal/service/`、`internal/model/`、`internal/dao/`）。
>
> 审查日期：2026-07-20

---

## ✅ 修复进度总览（复核日期：2026-07-22）

对全部 47 项逐条对照当前代码复核，状态已标注在下方**每个问题标题后**。图例：✅ 已修复 ｜ 🟡 部分修复 ｜ ❌ 未修复。

**总计：✅ 已修复 17 ｜ 🟡 部分修复 5 ｜ ❌ 未修复 25**

| 优先级 | ✅ 已修复 | 🟡 部分修复 | ❌ 未修复 |
|--------|----------|-------------|-----------|
| 🔴 高（14） | 9 | 1 | 4 |
| 🟡 中（17） | 8 | 3 | 6 |
| 🟢 低（16） | 0 | 1 | 15 |
| **合计（47）** | **17** | **5** | **25** |

---

## 审查模块概览

| 模块 | 文件数 | 说明 |
|------|--------|------|
| `internal/logic/billing/` | ~18 | 计费引擎：定价、结算、钱包、对账、告警 |
| `internal/logic/payment/` | ~5 | 支付逻辑：回调处理、订单履约、渠道设置 |
| `internal/logic/admin/` | ~25 | 管理后台业务：租户、渠道、模型、工单、通知等 |
| `internal/logic/tenant/` | ~12 | 租户控制台业务：成员、密钥、工单、帮助中心等 |
| `internal/logic/task/` | ~10 | 异步任务：图片同步、健康快照、定时任务 |
| `internal/logic/monitor/` | ~10 | 监控告警：指标采集、告警规则、通知 |
| `internal/logic/common/` | ~20 | 公共逻辑：JWT、缓存、会话、安全、审计 |
| `internal/logic/relay/` | ~4 | Relay 业务：亲和性、健康检查 |
| `internal/middleware/` | ~14 | 中间件：认证、RBAC、限流、日志、幂等 |
| `internal/handler/` | ~7 | 特殊端点：Relay 代理、支付回调、系统初始化 |
| `internal/response/` | 2 | 统一响应封装 |
| `internal/consts/` | 1 | 业务状态码和常量 |
| `internal/utility/` | ~8 | 工具函数：加密、导出、TOTP、Turnstile |
| `relay/` | ~100+ | AI 模型代理层：渠道适配器、调度器、协议转换 |

---

## 🔴 高优先级问题（14 个）

### 安全隐患

#### 1. 无 CORS 配置 ❌ 未修复
- **文件**：`internal/cmd/cmd.go`
- **问题**：整个项目没有任何 CORS 中间件配置，未设置 `Access-Control-Allow-Origin` 等响应头
- **影响**：所有基于浏览器的 Web 客户端（前端 SPA、浏览器扩展、AI Playground）无法跨域调用 API
- **建议**：添加 CORS 中间件，配置允许的来源域名白名单

#### 2. 登录端点无频率限制 🟡 部分修复（有账号锁定，缺 IP/路由限流）
- **文件**：`internal/cmd/cmd.go:260-268`
- **问题**：`/api/admin/auth/login` 和 `/api/tenant/auth/login` 没有速率限制中间件
- **影响**：攻击者可对管理员/租户登录接口进行暴力破解
- **建议**：对登录端点添加 IP 级别和全局级别的速率限制

#### 3. `getEncKey()` 运行时 panic ✅ 已修复（2026-07-22）
- **文件**：`internal/middleware/open_auth.go:202-206`、`internal/logic/common/security.go:491-496`
- **问题**：加密密钥获取函数在请求处理链中调用，密钥配置错误时直接 `panic`，导致整个服务进程崩溃
- **影响**：配置错误可使生产服务宕机
- **建议**：将密钥验证移至启动阶段，运行时返回 error 而非 panic
- **修复**：crypto 包新增非 panic 的 `GetEncryptionKey(hexKey) ([]byte, error)`，`MustGetEncryptionKey` 改为复用它；`internal/cmd/cmd.go:65-71` 在启动阶段（JWT 初始化后）校验 `crypto.encryptionKey`，缺失/非法即 `Fatal` 拒绝启动。密钥为配置文件静态值，启动即校验后运行时不会再触发 panic

#### 4. Open Platform nonce 未防重放 ✅ 已修复
- **文件**：`internal/middleware/open_auth.go:58`
- **问题**：读取 `X-Nonce` 头部但未将 nonce 存入 Redis 进行去重，仅依赖时间戳偏移检查（5 分钟窗口）
- **影响**：在 5 分钟窗口内，攻击者可重放已签名的请求
- **建议**：将 nonce 存入 Redis，TTL 匹配时间戳偏移窗口

#### 5. 刷新令牌无 Token Family 模式 ❌ 未修复
- **文件**：`internal/logic/common/session.go:101-129`
- **问题**：`RefreshSession` 只轮换刷新令牌哈希，但未实现 token family 重用检测
- **影响**：若攻击者窃取刷新令牌并在合法用户之前使用，攻击者获得新访问令牌，合法用户的旧令牌静默失效，但攻击者会话仍活跃
- **建议**：实现 token family 模式，被盗令牌使整个 family 失效

#### 6. `ParseProvisionalToken`/`ParseConfirmToken` 不验证签名算法 ✅ 已修复（2026-07-22）
- **文件**：`internal/logic/common/security.go:66-68, 98-100`
- **问题**：这两个函数未检查 `token.Method.(*jwt.SigningMethodHMAC)`，与 `ParseAccessToken`（`jwt.go:129-131`）不一致
- **影响**：攻击者可构造 `"alg": "none"` 令牌绕过认证
- **建议**：添加相同的签名方法检查
- **修复**：两个 keyfunc 均加入 `if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok { return nil, gerror.Newf(...) }`，与 `ParseAccessToken` 一致，拒绝非 HMAC/`alg:none` 令牌

#### 7. 幂等性中间件已实现但未注册 ✅ 已修复
- **文件**：`internal/middleware/idempotency.go`
- **问题**：完整的幂等性中间件（冲突检测、响应缓存、数据库持久化）已实现，但在 `internal/cmd/cmd.go` 中未注册到任何路由组
- **影响**：死代码，支付/充值等关键变更接口无幂等保护

#### 8. 查询参数 Token 回退对所有路由生效 ✅ 已修复
- **文件**：`internal/middleware/admin_auth.go:213`
- **问题**：`ExtractBearerToken` 允许通过 URL 查询参数传递 Token（为 WebSocket 设计），但此回退对所有路由生效
- **影响**：Token 出现在服务器日志、代理日志和浏览器历史中
- **建议**：限制查询参数 Token 回退仅在 WebSocket 路由（检查 `Upgrade: websocket` 头）

#### 9. SSRF 风险：BaseURL 不验证内网 IP ❌ 未修复
- **文件**：所有 `relay/channel/*/adaptor.go`
- **问题**：渠道配置中的 `BaseURL` 直接用于上游 HTTP 请求，无内部 IP 验证
- **影响**：若攻击者获得渠道配置权限，可将 `BaseURL` 设为 `http://169.254.169.254/latest/meta-data/`（AWS 元数据服务）等内网地址
- **建议**：验证 BaseURL 是否指向内网 IP，或在渠道创建时进行校验

#### 10. WebSocket CheckOrigin 允许所有来源 ✅ 已修复
- **文件**：`relay/handler/realtime_handler.go:22`
- **问题**：`CheckOrigin: func(r *http.Request) bool { return true }` 允许任意来源的 WebSocket 连接
- **影响**：跨站 WebSocket 劫持风险增加
- **建议**：验证 Origin 头，至少检查非空

### 数据一致性与并发安全

#### 11. `preDeductDB` 未使用事务 — SELECT FOR UPDATE 和 UPDATE 分离执行 ✅ 已修复
- **文件**：`internal/logic/billing/wallet.go:205-237`
- **问题**：`LockUpdate()`（SELECT FOR UPDATE）和后续 `UPDATE` 是两个独立语句，不在事务中。行锁在 SELECT 完成后立即释放（自动提交模式），两个并发预扣可以同时通过余额检查
- **影响**：并发预扣可超额冻结，钱包余额被击穿，造成资损
- **修复**：将 SELECT FOR UPDATE 和 UPDATE 包装在 `g.DB().Transaction()` 中

#### 12. `SettleWithUsage` 吞掉所有错误返回 nil ✅ 已修复（2026-07-22）
- **文件**：`internal/logic/billing/provider.go:44-48`
- **问题**：`SettleWithUsage` 返回错误时直接 `return nil`，调用方无法区分"结算失败"和"无结果"
- **影响**：DB 死锁/钱包不存在等场景下预扣金丢失，无任何告警
- **修复**：`BillingProvider` 接口签名无 error 返回值（不可改），故在 provider 包装层出错时记 `Error` 级日志告警，带 `request_id / tenant_id / model / pre_deduct` 对账关键字段，明确提示"预扣冻结金可能仍被冻结、需对账追回"，不再静默丢弃

#### 13. 任务结算退款失败静默忽略 ✅ 已修复
- **文件**：`internal/logic/billing/task_billing.go:269-273`
- **问题**：`SettleTaskSuccess` 调整退款失败时仅 `g.Log().Warningf`，调用方收到成功结果但实际退款未执行
- **影响**：钱包 `frozen_balance` 永久偏高，减少可用余额
- **修复**：退款失败时返回错误给调用方

### 资源管理

#### 14. 无请求体大小限制 ❌ 未修复（代码层无显式限制，靠框架默认 ~8MB）
- **文件**：`internal/handler/relay/relay.go` 所有 handler 函数
- **问题**：所有 handler 调用 `r.GetBody()` 无大小限制
- **影响**：恶意客户端可发送多 GB 请求体耗尽服务器内存
- **建议**：添加 `io.LimitReader` 或配置 GoFrame 的 `server.MaxRequestBodySize`

---

## 🟡 中优先级问题（17 个）

### 并发与事务

#### 15. `unfreezeDB` 未使用事务 ✅ 已修复
- **文件**：`internal/logic/billing/wallet.go:278-296`
- **问题**：与 `preDeductDB` 相同模式 — SELECT 后独立 UPDATE，无事务保护
- **影响**：两个并发解冻可能重复扣减 `frozen_balance`，导致其变为负数

#### 16. Redis/DB 双写不一致 ❌ 未修复
- **文件**：`internal/logic/billing/wallet.go:240-247`
- **问题**：`preDeductSyncDB` 中 Redis Lua 脚本成功后 DB 同步可能失败。进程崩溃时冻结金额在 Redis（有 TTL）但不在 DB
- **影响**：Redis 和 DB 的 `frozen_balance` 漂移，TTL 过期后冻结数据丢失

#### 17. 每次操作 spawn goroutine ❌ 未修复
- **文件**：
  - `internal/logic/billing/wallet.go:199` — `trackPreDeduct`
  - `internal/logic/billing/member_quota.go:72` — `incrMemberQuotaDB`
  - `internal/logic/billing/api_key_quota.go:48` — 匿名 goroutine
- **问题**：每次预扣/配额更新创建一个新 goroutine，高并发下创建数万 goroutine
- **建议**：使用 worker pool 或 buffered channel

#### 18. 会话淘汰缺少 `FOR UPDATE` 🟡 部分修复（已包事务，仍无行锁）
- **文件**：`internal/logic/common/session.go:54`
- **问题**：强制最大会话数时，SELECT 计数和删除不在事务中加锁
- **影响**：并发登录可能删除超过必要数量的会话

### 数据一致性

#### 19. `RemoveMember` 无事务保护 ✅ 已修复
- **文件**：`internal/logic/tenant/member.go:557-593`
- **问题**：撤销 API Key → 删除成员范围 → 匿名化用户，三步操作无事务
- **影响**：进程崩溃导致部分状态不一致（如 Key 已撤销但用户未匿名化）

#### 20. `DisableMember`/`EnableMember` 无事务保护 ✅ 已修复
- **文件**：`internal/logic/tenant/member.go:597-679`
- **问题**：撤销 API Key 和更新用户状态无事务保护

#### 21. `SetModelPricing` 无事务保护 ✅ 已修复
- **文件**：`internal/logic/admin/model.go:417-440`
- **问题**：DELETE 后循环 INSERT 不在事务中
- **影响**：中途失败导致定价记录被删但未全部重新插入

#### 22. `DeleteModel` 四表删除无事务保护 ✅ 已修复
- **文件**：`internal/logic/admin/model.go:341-365`
- **问题**：pricing / tenant_models / group_models / model 四表删除不在事务中
- **影响**：中途失败留下孤儿记录

### 性能问题

#### 23. `ListTenants` N+1 查询 ✅ 已修复（2026-07-22）
- **文件**：`internal/logic/admin/tenant.go`
- **问题**：每行租户触发 3 次独立查询（owner 名称、成员数、钱包余额）
- **影响**：20 行 = 1 主查询 + 60 次 N+1 = 61 次查询
- **修复**：新增 `batchTenantAggregates`，用 `WhereIn` + `GROUP BY tenant_id` 三条聚合查询批量取 owner 名称/成员数/钱包余额，循环内改为 map 查表。查询数从 1+3N 降为 1+3（常数）

#### 24. `ExportTenants` N+1 查询 ✅ 已修复（2026-07-22）
- **文件**：`internal/logic/admin/tenant.go`
- **问题**：`fetchTenantRow` 每行执行 owner 名称、成员数、钱包余额查询
- **影响**：1000 行 = 3000+ 次额外查询
- **修复**：移除 `fetchTenantRow` 闭包，改为每页（1000 行）调用一次 `batchTenantAggregates` 批量聚合，再从 map 组装导出行。每页额外查询从 3000 降为 3

#### 25. `AffinityStore` 无自动清理 ✅ 已修复
- **文件**：`relay/scheduler/affinity.go:116`
- **问题**：`CleanExpired()` 方法存在但无后台 goroutine 定期调用
- **影响**：长期运行过期条目无限积累
- **修复**：`internal/cmd/cmd.go:171` 注册定时任务 `affinity_cache_cleanup`（cron `*/5 * * * *`），后台每 5 分钟调用 `CleanExpired()`

#### 26. 模型变更后缓存未失效 🟡 部分修复（`DeleteModel` 有失效，`SetModelPricing` 仍无）
- **文件**：
  - `internal/logic/admin/model.go:341` — `DeleteModel`
  - `internal/logic/admin/model.go:417` — `SetModelPricing`
- **问题**：删除模型/更新定价后未调用 `InvalidateModelCache()`
- **影响**：变更在模型缓存 TTL（600s）过期后才生效

### 安全问题

#### 27. 无安全响应头 ❌ 未修复
- **文件**：全局
- **问题**：未设置 `X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、`Strict-Transport-Security`、`Content-Security-Policy`、`Referrer-Policy`
- **影响**：缺乏纵深防御

#### 28. `OperationLog` goroutine 无超时 ❌ 未修复
- **文件**：`internal/middleware/operation_log.go:155`
- **问题**：异步审计日志使用 `context.Background()` 无超时
- **影响**：DB 慢查询时 goroutine 无限积累

### TaskChannel 适配器

#### 29. `DoRequest` 忽略 context ❌ 未修复
- **文件**：4 个 `relay/taskchannel/*/adaptor.go`
- **问题**：所有 taskchannel adapter 的 `DoRequest` 接受 `ctx` 参数但使用 `_` 丢弃，使用 `http.NewRequest` 而非 `http.NewRequestWithContext`
- **影响**：上游请求无超时/取消传播

#### 30. TaskChannel 不使用连接池 🟡 部分修复（仅 gemini 用连接池）
- **文件**：4 个 `relay/taskchannel/*/adaptor.go`
- **问题**：每次请求创建新 `http.Client`，不使用 `common.NewPooledClient`
- **影响**：无连接复用，每次新建 TCP/TLS 连接

#### 31. `SettleFailed` 错误在 8 处被 `_ =` 丢弃 ❌ 未修复（实为 11 处）
- **文件**：`relay/handler/relay_handler.go:448,463,500,561,587,692,699,713`
- **问题**：多处 `_ = billing.SettleFailed(...)` 丢弃错误
- **影响**：Redis 宕机时预扣款静默丢失，无日志无告警

---

## 🟢 低优先级问题 / 代码质量改进（16 个）

### 代码重复

#### 32. `DoRequest` 在 14 个 adapter 中结构完全相同 ❌ 未修复
- **文件**：`relay/channel/{aws,baidu_v2,cloudflare,codex,coze,moonshot,minimax,xai,ali,ollama,vertex,deepseek,tencent,gemini}/adaptor.go`
- **问题**：构建 URL → 创建请求 → 设置 header → 应用覆盖 → 获取超时 → 获取客户端 → 发送请求 的流程完全一致
- **建议**：提取 `DoRequest` 公共实现到 `relay/common/`，adapter 只覆盖 `PreDoRequest` hook

#### 33. `convertClaudeRequest` 在 4 个 adapter 中逐字复制 ❌ 未修复
- **文件**：`relay/channel/{moonshot,minimax,ali,deepseek}/adaptor.go`
- **问题**：完全相同的 Claude 请求转换逻辑复制了 4 次
- **建议**：提取到公共工具函数

#### 34. 错误写入函数三份几乎相同的代码 ❌ 未修复
- **文件**：`relay/handler/{chat_handler,claude_handler,gemini_handler}.go`
- **问题**：`WriteRelayError`、`WriteClaudeRelayError`、`WriteGeminiRelayError` 约 80% 逻辑相同，仅 JSON 响应格式不同
- **建议**：提取公共 `extractErrorInfo` 函数

#### 35. 结算事务逻辑三处重复 ✅ 已修复
- **文件**：`internal/logic/billing/settlement.go`（`Settle` 和 `SettleWithUsage`）、`internal/logic/billing/task_billing.go`（`SettleTaskSuccess`）
- **问题**：钱包更新 → 余额读取 → 计费记录 → 交易记录 → 跟踪标记 的相同模式重复三次
- **建议**：提取公共结算事务方法
- **修复**：在 `settlement.go` 提取公共骨架 `executeSettlementTx(ctx, settlementTxParams)`，统一封装「钱包扣款 → 事务内读准确余额 → 创建计费记录 → 记录消费流水 → 标记预扣追踪已结算」五步事务，并复用 `readWalletBalanceTx`、`markPredeductSettledTx` 两个辅助函数。三处结算的差异部分（计费记录构造、流水构造、预扣追踪 request_id 集合含 task 的 `_adjust`）通过 `settlementTxParams` 的闭包/字段注入。幂等（唯一冲突回滚）与错误前缀行为保持不变，`go vet` 与现有单测通过。

#### 36. `relayModeString` 与 `RelayMode.String()` 重复 ❌ 未修复
- **文件**：`relay/handler/relay_handler.go:893-932`、`relay/constant/relay_mode.go:114-163`
- **问题**：两份实现有差异（handler 版本有更多 case），已分化
- **建议**：合并到 `RelayMode.String()`

#### 37. `HandleRealtime` 复制 ~150 行 RelayHandler 验证逻辑 ❌ 未修复
- **文件**：`relay/handler/realtime_handler.go`
- **问题**：scope 检查、IP 白名单、频率限制、模型验证、并发限制等在 `RelayHandler` 中已有实现
- **建议**：提取共享验证函数

#### 38. `parseInt` helper 在两个 taskchannel 中重复 ❌ 未修复
- **文件**：`relay/taskchannel/{ali,kling}/adaptor.go`
- **建议**：移到公共包

### 输入验证

#### 39. `CreateTenant` 未验证 MaxConcurrency 负值 ❌ 未修复
- **文件**：`internal/logic/admin/tenant.go:116-124`
- **建议**：添加 `>= 0` 校验

#### 40. `CreateChannel` 未验证 Priority/Weight 负值 🟡 部分修复（Weight 有 tag，Priority 无校验）
- **文件**：`internal/logic/admin/channel.go:232`
- **建议**：添加非负校验

#### 41. `ListChannels` 未调用 `NormalizePagination` ❌ 未修复
- **文件**：`internal/logic/admin/channel.go:46-56`
- **建议**：统一使用 `common.NormalizePagination` 规范化分页参数

### 错误处理

#### 42. `EstimatePreDeductAmount` 错误时返回 $0.01 兜底 ❌ 未修复
- **文件**：`internal/logic/billing/pricing.go:395-448`
- **问题**：3 个错误路径均返回 $0.01，对昂贵模型几乎无预扣保护作用
- **建议**：根据模型默认价格档次设置更合理的兜底值

#### 43. `RecalculateByTokens` 错误时返回 0 ❌ 未修复
- **文件**：`internal/logic/billing/task_billing.go:393-401`
- **问题**：模型价格加载失败时返回 0 成本，使用原始估算值
- **建议**：记录告警日志

#### 44. `response.Error()` 内部错误用 Warning 级别 ❌ 未修复
- **文件**：`internal/response/response.go:59,78`
- **问题**：内部错误（应告警处理）记录为 `Warning` 级别
- **建议**：改为 `Error` 级别

### 其他

#### 45. 对账日期范围丢失最后一秒 ❌ 未修复
- **文件**：`internal/logic/billing/reconciliation.go:39-40`
- **问题**：使用 `23:59:59` 结束时间，丢失 `23:59:59.001 ~ 23:59:59.999`
- **建议**：使用 `< next_day 00:00:00`

#### 46. `ChannelMeta` 全 adapter 未做 nil 检查 ❌ 未修复
- **文件**：所有 `relay/channel/*/adaptor.go`
- **问题**：所有 adapter 直接访问 `info.ChannelMeta.BaseURL` 等字段，若调度器未填充则 panic
- **建议**：在 relay handler 入口添加 nil guard


#### 47. codex adapter JSON 解析错误静默忽略 ❌ 未修复
- **文件**：`relay/channel/codex/adaptor.go:33`
- **问题**：`_ = json.Unmarshal(...)` 丢弃错误，认证凭据可能为空
- **建议**：记录错误日志

---

## ✅ 代码亮点

审查中也发现了许多值得肯定的优秀实践：

### 架构设计
- **两阶段缓存架构**：`internal/logic/common/cache.go` — L1 内存 + L2 Redis + Pub/Sub 失效，设计优良
- **错误分类体系**：`relay/constant/errors.go` — 层次化错误类型 + `Unwrap()` 链 + `IsRetryable` 智能重试判断
- **StreamScanner**：`relay/helper/stream_scanner.go` — 三 goroutine 架构 + WaitGroup 关闭 + writeMutex 序列化

### 代码质量
- **`sync_image_worker.go`**：CAS 竞态保护 + 优雅关闭（graceful wait + timeout）+ 计费重试，生产级代码
- **租户隔离一致性强**：所有租户侧查询一致使用 `Where("tenant_id", tenantID)`，无遗漏
- **审计日志脱敏**：`ApplyAuditLevel` + `truncateBody` 分级过滤敏感信息
- **多处事务使用正确**：`CreateTenant`、`DeleteChannel`、`SetChannelAbilities` 等正确使用 `g.DB().Transaction()`

### 计费设计
- **Redis 原子预扣 + DB 异步同步**：Lua 脚本保证 Redis 层原子性
- **钱包冻结余额**：`frozen_balance` 防止并发超扣
- **五层额度模型**：租户钱包 → 套餐额度 → 成员额度 → 项目预算 → Key 额度，层次清晰

---

## 📊 统计汇总

| 严重程度 | 数量 | 涉及模块 |
|---------|------|---------|
| 🔴 高优先级 | 14 | 安全(8)、数据一致性(3)、资源管理(1)、认证(2) |
| 🟡 中优先级 | 17 | 并发/事务(4)、数据一致性(4)、性能(4)、安全(2)、TaskChannel(3) |
| 🟢 低优先级 | 16 | 代码重复(7)、输入验证(3)、错误处理(3)、其他(3) |
| **合计** | **47** | |

### 按模块分布

| 模块 | 高 | 中 | 低 |
|------|----|----|-----|
| Billing / Payment | 3 | 6 | 4 |
| Middleware / Auth | 6 | 2 | 0 |
| Relay 层 | 2 | 3 | 7 |
| Admin / Tenant Logic | 1 | 4 | 3 |
| TaskChannel | 1 | 3 | 1 |
| 全局 | 2 | 1 | 1 |

---

## 🔧 修复优先级建议

### 第一批（立即修复 — 安全漏洞）
1. 添加 CORS 配置（#1）
2. 登录端点添加频率限制（#2）
3. 修复 `"alg":"none"` 令牌绕过（#6）
4. 限制查询参数 Token 回退范围（#8）
5. Open Platform nonce 防重放（#4）
6. WebSocket CheckOrigin 限制（#10）

### 第二批（尽快修复 — 数据一致性）
7. `preDeductDB` 加事务保护（#11）
8. `SettleWithUsage` 错误传播（#12）
9. 任务结算退款失败处理（#13）
10. `unfreezeDB` 加事务保护（#15）
11. 四个无事务保护的成员/模型操作（#19-#22）

### 第三批（计划修复 — 性能与可靠性）
12. N+1 查询优化（#23, #24）
13. goroutine-per-op 改为 worker pool（#17）
14. 模型变更后缓存失效（#26）
15. `getEncKey()` 启动时验证（#3）
16. TaskChannel context 传播（#29）

### 第四批（持续改进 — 代码质量）
17. 消除 adapter 代码重复（#32-#38）
18. 输入验证完善（#39-#41）
19. 错误处理规范化（#42-#44）
20. 边界条件修正（#45-#47）

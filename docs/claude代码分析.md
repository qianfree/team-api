# team-api 后端代码质量分析报告

> 分析日期:2026-07-20
> 分析范围:后端手写代码(`relay/`、`internal/logic/`、`internal/handler/`、`internal/middleware/`、`internal/utility/`、`internal/cmd/`、`internal/consts/`、`internal/response/`、`api/`)
> 明确排除:`internal/controller/`、`internal/service/`、`internal/model/`、`internal/dao/`(GoFrame 脚手架自动生成)、前端代码(`web/`)、参考项目(`new-api/`、`sub2api/`)

分析方式:按子系统拆分为 6 个并行审查(relay/channel+taskchannel 供应商适配器、relay 其余模块、billing+payment+monitor 计费核心、admin+tenant 业务逻辑、common+task+notification+docs+open 公共依赖、handler+middleware+utility+cmd+api 基础设施),逐一走查源码后汇总。

---

## 总体评价

架构分层清晰,核心设计(Redis Lua 原子预扣、事务化结算、双层缓存、供应商适配器接口统一、JWT 算法白名单、bcrypt cost=12、AES-256-GCM 加密)总体扎实,未发现能被外部直接一击利用的严重漏洞。但集中暴露三类系统性问题:

1. **权限校验"各自为战"导致的真实越权漏洞** —— 每个 logic 文件各自手写角色/归属判断,没有统一的鉴权网关兜底,已经出现真实遗漏
2. **分布式场景下"看似生效实则失效"的机制** —— 缓存失效订阅、cron 分布式锁、重试统计字段等已修复(见修复记录);仍残留进程内 `walletCache` 充值后不失效、任务领取非原子 CAS 等多副本隐患
3. **测试覆盖率与项目自身 DoD(核心逻辑 ≥80%)差距巨大** —— 恰恰是计费、支付、权限、退款这些最需要保护的核心路径覆盖率最低,本次发现的多数高危问题现有测试完全没有捕获

---

## 🔴 高危 — 建议优先处理

### 越权 / 权限漏洞

| 位置 | 问题 |
|------|------|
| `internal/middleware/rbac.go:318-349`(`AdminPermissionGuard`) | `matchPermission()` 找不到路由对应规则时返回空字符串,此时**默认放行**已认证的非超管请求(fail-open),新增管理端点忘记登记权限规则即无保护 |
| `internal/logic/tenant/team_guard.go` | 文件名/`requireTeamEnabled` 命名暗示是通用授权守卫,实际只检查团队功能是否启用,不做资源归属或 RBAC 校验,容易被误认为已提供统一防护 |

### 资金正确性

| 位置 | 问题 |
|------|------|
| `internal/logic/admin/order.go:63-112`(`RefundOrder`) | 仅将订单状态改为 `refunding`、插入 `ord_refunds`(`status: approved`)记录后直接返回,**从未调用支付渠道退款 API,也未冲正钱包余额**。全仓库搜索确认没有任何后续代码消费 `refunding` 状态,退款流程是半成品,订单永久卡住,用户实际收不到退款 |
| `internal/logic/billing/currency.go` | 同时维护 `payment_exchange_rate_cny_to_usd`(默认 0.14)和 `payment_exchange_rate_usd_to_cny`(默认 7.25)两个**独立**汇率配置,`1/0.14=7.143 ≠ 7.25`,非严格倒数,**直接违反 CLAUDE.md"只维护单一方向汇率,禁止再配置第二个独立汇率"的强约束** |
| `internal/model/entity/bil_wallets.go` 及 `internal/logic/billing/wallet.go` 全文 | `Balance`/`FrozenBalance`/`WarningThreshold`/`CumulativeRecharge` 等金额字段在 Go 层全部是 `float64`,数据库列虽是 `NUMERIC(20,10)`,但应用层所有加减法(预扣/结算/退款/汇率换算)均在有损精度上进行,**违反 CLAUDE.md"金额字段不用 FLOAT"的规则**,长期运行存在累计误差风险 |

### 安全

| 位置 | 问题 | 状态 |
|------|------|------|
| `internal/response/error_log.go` + `response.go:71-92` | 系统级错误发生时,原始请求体(截断至 2000 字符)未做任何脱敏就写入 `sys_error_logs`,**登录/改密接口一旦触发 500 会把明文密码存进错误日志表** | ✅ 已修复:新增 `sanitizeRequestBody`,对 JSON 请求体递归脱敏 password/secret/token/code 等敏感字段,非 JSON 体不落原文 |
| `internal/logic/common/verify_code.go:117-169` | `VerifyCode` 仅做一次字符串比较,无失败次数计数或锁定机制,10 分钟有效期内理论上可被暴力枚举 6 位数字验证码(发送侧有 60s 冷却/5 次每小时限流,验证侧无节流) | ✅ 已修复:验证侧增加 Redis 失败计数,10 分钟内失败 5 次即锁定需重新获取,成功后清零 |
| `internal/utility/totp/totp.go:34-36`(`ValidateCode`) | 校验是无状态纯函数,调用方也未记录已使用的时间步,默认 ±1 周期(约 90 秒)窗口内同一验证码**可被无限次重复使用** | ✅ 已修复:`security.go` 新增 `validateTOTPOnce` 防重放守卫(Redis `SET NX EX 90`),4 处调用方统一接入,Redis 故障时 fail-open |
| `api/admin/v1/admin_user.go:40,53` | 创建/更新管理员的 `Role` 字段无 `v:"in:super_admin,admin"` 字段级白名单校验(逻辑层 `common.ValidateAdminRole` 已兜底拦截非法角色,实际注入风险已消除,仅剩 API 层校验一致性问题) | ✅ 已修复:补上字段级 `v:"in:super_admin,admin"`,与逻辑层口径一致(纵深防御) |
| `api/admin/v1/promo.go:44,54` | 优惠码创建/更新请求体直接用 `map[string]interface{}` 承载,完全绕开 GoFrame 字段级校验 | ⏸️ 非 bug:逻辑层 `promo_code.go:41-72` 的 `CreatePromoCode`/`UpdatePromoCode` **均已有字段白名单**,内部字段注入已被拦截;剩余仅为"缺少 GoFrame 字段级类型校验"的设计一致性问题,改成 typed struct 会变动前端契约、收益低,暂不改 |

### 核心链路测试覆盖率(实测)

| 模块 | 覆盖率 |
|------|--------|
| `internal/logic/billing` | 20.1%(虽新增多个 `_test.go`,但集中在 money/currency 辅助逻辑,settlement/pricing 主干仍薄弱) |
| `internal/logic/payment` | 6.6% |
| `internal/logic/monitor` | **0%**(无任何 `_test.go`) |
| `internal/logic/admin`、`internal/logic/tenant` | 合计约 76 个文件,**零测试**(权限/隔离/资金代码) |

均远低于 CLAUDE.md 要求的核心计费/调度/权限逻辑 ≥80% 覆盖率标准,且本次发现的越权漏洞、退款半成品、进程内缓存失效等问题现有测试**完全没有覆盖**。

---

## 🟡 中等 — 建议排期处理

- **`internal/logic/tenant/project.go:289-291, 337-341, 370-374`** 等多处:`ProjectUpdate`/`ProjectArchive`/`ProjectUnarchive` 先用 `WHERE id=? AND tenant_id=?` 做归属校验(SELECT),但紧接着的 UPDATE 只用 `WHERE id=req.Id`,丢失了 tenant_id 过滤。当前靠前置 SELECT 把关,不构成立即可利用漏洞,但破坏了"写操作必须带 tenant_id"的一致性约定
- **`relay/taskchannel/{ali,sora,suno,volcengine,midjourney}/adaptor.go`**:6/7 个任务渠道适配器各自新建裸 `http.Client`(而非复用 `common.NewPooledClient`),**忽略渠道代理配置**;且 4/7 处丢弃 `BuildRequestHeader` 的错误返回值。与 `relay/channel/`(聊天适配器)统一走连接池的实践不一致
- **`internal/logic/billing/api_key_quota.go`/`member_quota.go`**:`CheckApiKeyQuota`/`CheckMemberQuota` 是"读取当前值→比较→放行"的非原子检查,增量在结算后异步执行,与钱包层基于 Lua 脚本的真正原子预留形成明显的架构不一致,并发请求可同时通过检查从而累计超出配额上限
- **`internal/handler/public/payment.go:21-26`**:支付回调处理返回的任何错误(签名失败、金额不符、订单不存在等)被统一丢弃且**不记录日志**,签名伪造/篡改尝试在服务端完全不可追溯
- **`internal/logic/payment/fulfill.go`(`creditWalletTx`)**:充值成功后只调用了 `billing.InvalidateWalletRedis`,未清除 `billing` 包内私有的进程内 `walletCache`(300s TTL,无跨包失效接口),充值后最长 300 秒内 `GetWallet()` 可能仍返回旧余额
- **`internal/logic/billing/settlement.go`**:补扣场景(`actualCost > preDeductAmount`)下 `UPDATE bil_wallets SET balance = balance - ?` 没有非负校验,理论上可将余额扣成负数
- **`internal/logic/task/task.go:103-135`(`ExecuteTask`)**:任务领取用"先 SELECT 再 UPDATE"而非原子 CAS(对比同目录 `async_provider.go` 中正确的 `UpdateTaskCAS`),多副本同时跑可能并发领取并重复执行同一任务
- **`internal/utility/crypto/crypto.go:21-78`**:AES-GCM 的 `Seal`/`Open` 未使用 AAD 参数(传 nil),密文未与所属记录上下文(如 tenant_id)绑定
- **`internal/utility/turnstile/turnstile.go:29-64`**:未校验 siteverify 返回的 `Hostname`/`Action` 字段,同一 site key 跨域场景下 token 可能被跨站复用;整个包无单元测试
- **`internal/middleware/rbac.go:365-380`(`matchPermission`)**:后缀匹配用 `strings.Contains` 子串搜索而非分段匹配,例如规则后缀 `/test` 会误匹配路径 `.../latest`
- **`internal/middleware/admin_auth.go:73-75`/`tenant_auth.go:78-80`**:无 `jti` 的旧 token 跳过会话吊销检查,强制登出/踢人对这类 token 无效直到自然过期
- **`internal/logic/admin/channel.go:126-227`(`CloneChannel`)、`channel_oauth.go:248-307`**:渠道创建→密钥创建跨表写入未包在事务里,密钥插入失败会留下孤儿渠道记录
- **`internal/logic/admin/promo_code.go:38-48`(`CreatePromoCode`)**:`req.Data map[string]interface{}` 未做字段白名单校验直接 Insert,而同文件 `UpdatePromoCode` 有白名单,两者处理不一致
- **`internal/handler/setup/setup.go` + `internal/logic/admin/setup.go:49-56`**:系统初始化判断依赖"先查后插入"非原子操作,并发发起的两个初始化请求可能都通过"未初始化"判断,各自创建一条 super_admin 记录

---

## 🟢 低优先级 / 观察项

- `internal/logic/common/notification.go:444-456` 手写 `containsAny` 子串匹配循环,可直接用 `strings.Contains`
- `internal/logic/docs/openapi.go:44` 向 `Config().GetString(nil, ...)` 传入 `nil` context
- `internal/logic/common/batch_writer.go:74-77` 批量写入缓冲区满时静默丢弃最早记录,无计数/告警,用于 `bil_usage_logs` 等计费相关高频写入场景
- `internal/logic/common/session.go:132-137`(`RevokeSession`)只做 DB 删除、不写 Redis 黑名单,全靠调用方手动配对调用 `MarkSessionRevoked`,函数签名未强制这一点
- `internal/logic/monitor/alert_engine.go` 的 `"eq"` 条件分支用 `float64` 直接相等比较,对 CPU%/延迟/QPS 等连续型指标几乎永远不触发
- `internal/logic/billing/wallet.go` 的 `CheckBalance` 全仓库无调用方,是死代码
- `internal/logic/tenant/open_platform.go:405-445`(`WebhookConfigUpdate`)不检查受影响行数,ID 不存在时静默返回成功
- `internal/logic/tenant/member_import.go:63-68` 用 `context.Background()` 启动导入 goroutine,丢失请求的 trace/request_id
- `internal/logic/admin/auth.go:371-385`(`RevokeSession`)未像其他函数一样限定 `user_type=admin`,该表是 admin/tenant 共用表

---

## 正面观察(值得保留的设计)

- 钱包预扣使用 Redis Lua 脚本做到真正的原子操作(`billing.PreDeduct`),限流(QPS/并发)同样是很好的原子设计范例
- `relay/channel/vertex/auth.go` 的 OAuth2 token 缓存用 `sync.Map` 双重检查 + `singleflight.Group` 合并并发刷新请求,是本次审查中并发处理写得最好的一处
- `relay/scheduler/scheduler.go` 的全局降级逻辑(健康度全部过低时降级使用全部候选渠道)设计合理且有专门测试覆盖
- `relay/dto` 各供应商 DTO 保持独立而非强行抽象共享基类,是合理的选择(协议差异本身很大)
- `internal/logic/task/async_polling.go` 对"崩溃窗口内成功但未结算"和"失败需退款"的补偿逻辑设计成熟,注释详细记录了历史踩坑(如 CAS 谓词 bug)
- JWT 实现(`internal/logic/common/jwt.go`)显式拒绝 `none` 算法且有单测覆盖;密码哈希(bcrypt cost=12)、AES-256-GCM 加密选型均规范
- `relay/taskchannel/` 7 个适配器严格实现同一组 `TaskAdaptor` 接口方法,接口层面一致性好
- 错误包裹整体规范,抽样统计 `fmt.Errorf` 使用 `%w` 196 处 vs `%v` 仅 3 处(且都是合理场景),未发现系统性丢失错误上下文

---

## 建议的修复优先级

1. **权限漏洞收尾**(`AdminPermissionGuard` fail-open)—— `member_model_scope`、IP 白名单越权已修复(补 owner/admin 角色校验),剩余 fail-open 影响面明确、修复成本低
2. **退款流程补全** —— `RefundOrder` 仍是半成品,直接影响资金正确性
3. **汇率双配置合并为单一方向** + **金额字段 float64 → decimal**(后者改动面大,可分阶段推进)
4. **安全硬化项**(错误日志脱敏、验证码防暴力破解、TOTP 防重放、Kling JWT 签名失败处理)—— 单项成本都不高,建议一次性排期
5. **补齐核心模块单测**(billing/payment/admin/tenant),优先把本次发现的并发/越权场景写成回归测试,防止再次引入

# Go 与 GoFrame 代码开发规范 符合度审计报告

> **审计对象**：《docs/Go与GoFrame代码开发规范.md》（v1.0，2026-11-07）
> **审计范围**：internal/、relay/、relaykit/、plugins/ 非测试代码（api/、internal/cmd/、migrations/ 一并覆盖）
> **审计日期**：2026-11-07
> **复审日期**：2026-11-19（结论见 §9 复审记录）
> **审计方式**：规范全文逐条对照 + 五维度深查（错误处理 / 分层与 DAO / context·并发·时间 / 缓存·日志·配置·安全 / API 定义·测试·工程化）+ 全仓机械扫描
> **文档用途**：缺陷登记簿。修复时按条目勾选 `[ ]` → `[x]`，每项均已附 文件:行 证据与修复建议。

---

## 1. 总体结论

**架构级合规度高，执行级一致性差，另有 1 项安全级紧急问题。**

- 规范的核心架构约束（分层、统一响应、错误码集中定义、多租户双键校验、日志脱敏体系、Redis Lua 原子）落地质量高于一般项目，多处达到示范级。
- 三类系统性风险：**资金路径静默吞错**、**后台 goroutine 缺 recover**、**质量工具链零支撑**（无 lint / CI 测试 / 覆盖率）。
- 1 项安全紧急项：**真实数据库凭据已随 config.yaml 进入 git 仓库**。

## 2. 符合度矩阵

| 规范章节 | 符合度 | 摘要 |
|---|---|---|
| §3.1/§3.4 分层架构 | ✅ 符合 | logic 零 import ghttp/controller，controller 零越界，全单行委托 |
| §3.6 错误码 | ✅ 基本符合 | consts 集中分段定义质量高，仅 3 处裸数字 |
| §3.9 缓存 Redis | ✅ 基本符合 | 裸 Set 无 TTL = 0 处，Lua 原子完备，key 命名为项目自有体系 |
| §3.5 DAO/ORM | ⚠️ 部分符合 | 双键 >90%、事务规范、N+1 仅 1 处；但魔法列名是全仓主流（0/440 用 Columns()） |
| §2.6 并发 | ⚠️ 部分符合 | 工程化示范级（worker 池/优雅停机/panic 重启）；~28 处 goroutine 无 recover |
| §2.4 错误处理 | ⚠️ 部分符合 | 主流程 errors.Is/As 到位；资金路径 23 处吞错 |
| §3.7 日志 | ⚠️ 部分符合 | 89% 带 ctx、脱敏工具齐全；4 个泄漏点 |
| §3.2/§3.3 API 定义 | ⚠️ 部分符合 | g.Meta 100% 齐全；dc:/v: 中文消息覆盖不足 |
| §2.5 context | ⚠️ 部分符合 | ~8-10 处请求链路中段 Background |
| §2.7 时间与时长 | ⚠️ 部分符合 | 时刻比较干净；2 处 Sleep 做同步 |
| §4 测试 | ⚠️ 部分符合 | 核心模块覆盖广；表驱动仅 ~40%，无覆盖率度量 |
| §6 工程化 | ❌ 不符合 | 无 lint / CI 测试 / 覆盖率任何工具支撑 |
| §5 安全 | ❌ 有紧急项 | config.yaml 真实凭据入 git；默认密钥不拒绝 |

---

## 3. P0 紧急缺陷（安全 / 资金正确性，立即处理）

### 3.1 [x] 真实数据库凭据随 config.yaml 进入 git 仓库

> ✅ 2026-11-19 复审：主体已修复——文件已不被 git 跟踪、已列入 .gitignore，`git log --all` 无该文件历史。遗留：口令轮换属运维动作需自行确认；config.example.yaml:79 的 encryptionKey 仍为固定值（若线上沿用同款密钥则 §3.2 风险仍在）。

- **违反**：§3.8「敏感配置优先环境变量注入，禁止明文提交进仓库」；§5
- **证据**：`manifest/config/config.yaml` 被 git 跟踪（.gitignore 虽列有该项，但历史已跟踪），含：
  - `:43` 主库连接串（真实口令，此处不复制，以 `PgDB` 开头）+ `:52` 审计库同口令
  - `:74` Redis 地址 `192.168.50.22:16380`
  - `:84` JWT secret（弱默认值 `...change-me`）
  - `:94` encryptionKey（与 config.example.yaml 完全同款）
  - `:19` 开发者本机路径 `F:\logs`
- **危害**：任何拿到仓库的人可直接连内网库；若仓库将来公开/泄露，全部租户 API Key 密文可被同款 key 解密。
- **修复**：
  1. 立即轮换数据库口令（主库 + 审计库）与 Redis；
  2. `git rm --cached manifest/config/config.yaml`，本地配置改环境变量 / config.local.yaml（gitignore）注入；
  3. 用 `git filter-repo` 或 BFG 从历史清除该文件；
  4. config.example.yaml 中的示例值全部改为明显占位符。

### 3.2 [ ] 默认密钥不拒绝（2026-10 安全审计遗留项，仍未修）

> ❌ 2026-11-19 复审：未修复（jwt.go:46-48 仍只拒绝空 secret，无默认值黑名单）。

- **违反**：§3.8「配置缺失不得 panic（启动必需项 fail-fast）」的精神——默认弱密钥属同等启动校验缺失
- **证据**：
  - `internal/logic/common/jwt.go:46-48` 只拒绝**空** JWT secret；
  - `internal/cmd/cmd.go:76-82` 只拒绝**空/格式错** encryptionKey；
  - `config.yaml:84` 的 `-change-me` secret 与 `:94` 与 example 同款的 AES key 均可通过校验正常启动。
- **危害**：拿仓库即可伪造 JWT / 解密库内 API Key。
- **修复**：启动校验增加「已知默认值黑名单」（`change-me`、与 example 相同的 key、过短密钥），生产模式下直接 fail-fast。

### 3.3 [x] 计费退款失败被静默吞掉（约 23 处，零日志）

> ✅ 2026-11-19 复审后当日修复：剩余 20 处全部收敛——relay_handler 13 处 + realtime 4 处统一走 `releasePreDeduct`（检查错误 + Errorf 带 request/tenant/frozen 金额），sync_image 3 处走 `releaseSyncImagePreDeduct`。

- **违反**：§2.4-1「每个 error 都必须处理」；附录 A「禁止 `_ =` 吞错」
- **证据**：`_ = billing.SettleFailed(ctx, ...)` / `SettleTaskFailed` 模式：
  - `relay/handler/relay_handler.go:650/654/679/717/728/744/768/808/833/843/1177/1189/1204`（13 处）
  - `relay/handler/realtime_handler.go:288/292/305/318`（4 处）
  - `internal/handler/relay/sync_image.go:117/169/204/205`（同路径有主错误日志，半合规）
- **危害**：`SettleFailed` → `UnfreezePreDeduct`（`internal/logic/billing/settlement.go:433-441`）失败意味着**预扣冻结款未释放**，租户资金被错误占用，且无任何日志可排障。
- **修复**：每处至少 `Warningf`（带 tenantID/requestID/frozen 金额）；确认 `prededuct_sweep` cron（每 2 分钟按「重算 frozen = Σ 幸存预扣」自愈）覆盖此类失败场景，未覆盖则补对账补偿。

### 3.4 [ ] 系统错误日志打印全量请求体且未脱敏

> ❌ 2026-11-19 复审：未修复（handler_response.go:36 仍 `r.GetBodyString()` 全量入日志，未复用 maskSensitiveFields、无截断）。

- **违反**：§3.7「敏感字段必须脱敏后入日志」；§5
- **证据**：`internal/middleware/handler_response.go:36` —— 5xx 路径 `r.GetBodyString()` 全量入日志，未过脱敏（`internal/middleware/operation_log.go:253-283` 已有 maskSensitiveFields 却未复用）。
- **危害**：请求体中的密码 / API Key 明文直接落日志文件。
- **修复**：复用 maskSensitiveFields，另加长度截断（如 2KB）。

---

## 4. P1 系统性缺陷（专项治理）

### 4.1 [x] 质量三件套零工具支撑

> ✅ 2026-11-19 修复：落地 `.golangci.yml`（v2，主模块 + relaykit 子模块）+ Makefile `fmt/lint/test/cov` 目标 + `.github/workflows/ci.yml`（gofmt 检查与单元测试阻塞、golangci-lint 首期报告模式）；7 个 gofmt 偏差文件已修。lint 首跑存量：主模块 457 项 / relaykit 31 项（errcheck/goimports 为主），清零后转阻塞门禁。

- **违反**：§2.1（gofmt/goimports CI 强制、golangci-lint）、§4（go test CI 全绿、覆盖率 ≥80%）、§6.1（提交前三件套）
- **证据**：
  - 全项目无 `.golangci.yml`（规范附录 B 有现成参考配置）；
  - Makefile 无 lint / test 目标（仅 run/build/ctrl/dao/migrate/docker）；
  - `.github/workflows/docker.yml`、`release.yml` 均无 go test / golangci-lint / gofmt 步骤（纯构建发布）；
  - 无任何覆盖率（coverprofile）配置；
  - `gofmt -l` 实测 **7 个文件格式偏差**：`relaykit/dto/audio.go`（struct 对齐）、`relaykit/relayconvert/internal/goldentest/goldentest.go`、`relaykit/relayconvert/kitutil/kitutil_test.go`、两个 `benchmark_test.go`、`registry_test_helpers_test.go` 等。
- **修复**：落地附录 B 的 `.golangci.yml`（按项目实际裁剪，生成目录 controller/service/dao/model 可 exclude）；Makefile 加 `lint`/`test`/`cov` 目标；CI 加格式检查 + lint + test 步骤；修 7 个 gofmt 偏差文件。

### 4.2 [x] 后台 goroutine 无 defer recover（约 28/48 处，58%）

> ✅ 2026-11-19 修复：新增统一包装 `internal/utility/safego`（Run/Guard：defer recover + Error 级日志带堆栈），点名 24 处 + 复审新增 #1/#2/#4 全部收口；artifact worker 池逐 job Guard、recordTaskSubmitAudit 族统一走 `goRecordTaskSubmitAudit` 入口；email.go 两处 sender.Send 返回值改记日志。新代码一律用 safego.Run 启动后台 goroutine。

- **违反**：§2.4-6「每个自行启动的 goroutine 顶层必须有 defer recover」
- **证据**（长期后台 worker，panic 即崩整个多租户进程）：
  - `internal/logic/task/async_polling.go:56-70` —— 异步任务核心轮询循环
  - `internal/dispatchadapter/state_redis.go:79-88` —— 调度健康上报 worker
  - `internal/dispatchadapter/config.go:117-135` —— 路由策略热更新循环
  - `internal/logic/common/cache.go:349`、`internal/logic/common/config.go:376` —— Redis Pub/Sub 重连循环
  - `internal/logic/common/content_filter.go:59` —— 订阅循环
  - `internal/logic/billing/sweeper.go:43-56` —— 钱包物化定时器（资金相关）
  - `internal/logic/billing/settlement.go:253/401`、`task_billing.go:324` —— `go CheckBalanceWarning`（叠加 Background + 无并发上限）
  - `internal/logic/tenant/email.go:181-205` —— 无 recover 且两次 `sender.Send` 返回值直接丢弃（双重违规）
  - `internal/logic/task/task.go:295-302` —— worker 池无 recover（同文件 :313/:339 的有，不一致）
  - 一次性通知类：`task/model_sunset.go:66`、`admin/model.go:471`、`monitor/alert_notify.go:40`、`billing/alert.go:62`、`dispatchadapter/maintenance.go:295`
  - 其他：`middleware/operation_log.go:160`、`plugin/hooks.go:120`、`tenant/member_import.go:70`、`update/update.go:104`、`update/graceful_exit.go:34`、`relay/handler/dispatch_glue.go:151`、`relay/channel/openai/realtime.go:133/163`、`relay/helper/stream.go:458`
- **修复范本**（仓内已有）：`internal/logic/common/cron.go:288` `runHandlerSafely`（含高质量注释）、`internal/logic/admin/workbench.go:241-252`。建议抽公共 `gopanic.Run(ctx, name, fn)` 包装统一收口。

### 4.3 [x] 计费/业务决策的 DB 查询失败后零值继续（约 8 处）

> ✅ 2026-11-19 修复：点名 9 处全部改为检查错误 + Warningf 留痕（空值/默认值降级语义不变）——level.go 两处（金额相关最关键）、callback.go、tenant_model.go、maintenance.go、member.go、security.go、alert.go。

- **违反**：§2.4-1「禁止下划线静默吞错后继续用零值」
- **证据**：
  - `internal/logic/billing/level.go:39,77` —— 租户等级配置查询失败 → config 为 nil → **按默认等级计费，直接影响金额**（最严重）
  - `internal/logic/payment/callback.go:183` —— 支付回调中订单更新失败静默
  - `internal/logic/common/tenant_model.go:127` —— 分组查询吞错
  - `internal/dispatchadapter/maintenance.go:290` —— 渠道名 Scan 吞错 → 通知发空名
  - `internal/logic/tenant/member.go:449`、`internal/logic/tenant/security.go:313`、`internal/logic/billing/alert.go:70`
- **修复**：逐处改为「返回错误 / Errorf 记日志后走显式降级分支」，禁止零值隐式参与决策。

### 4.4 [x] 请求链路中段 context.Background()（约 8-10 处，掐断取消 + 丢 request_id）

> ✅ 2026-11-19 修复：点名 11 处 + 实现侧 provider.go RecordAudit/UpdateTaskAudit 内部 Background 全部改 `context.WithoutCancel(ctx)`；recordFailedUsage 族补 ctx 参数；DryRun/Skill.Validate 接口 ctx 化（复审新增 #3）。批次 F 另完成 OAuth 族 9 处 + taskchannel 3 处 + buildProxyClient/relayClientMaxBodySize 的彻底 ctx 化（NewRequestWithContext）。

- **违反**：§2.5-4「只有任务根/测试才 Background，禁止调用链中段新开」
- **证据**：
  - `internal/logic/billing/settlement.go:253/401`、`internal/logic/billing/task_billing.go:324` —— `go CheckBalanceWarning(context.Background(), tenantID)`
  - `internal/middleware/operation_log.go:160-161` —— 操作日志落库 goroutine（请求中间件路径）
  - `internal/handler/relay/task.go:499` —— RecordAudit 用 Background（调用方本身有 r.Context()）
  - `relay/handler/relay_handler.go:1326/1372`、`relay/handler/realtime_handler.go:450` —— RecordUsage 用 Background
  - `internal/logic/billing/wallet.go:167` —— doRebuildWalletFromDB
  - `internal/middleware/body_limit.go:31`、`internal/logic/common/oauth/client.go:49` —— 请求路径读配置
  - （`relay/handler/relay_handler.go:437` 有注释 + cancel，可接受，建议改 WithoutCancel）
- **修复**：仓内已有 8+ 处正确范式 `context.WithoutCancel(ctx)`（`verify_code.go:84`、`notification.go:150`、`member_import.go:72` 等），统一替换即可；同类场景两种写法并存是执行不一致。

### 4.5 [ ] common 垃圾抽屉包（规范明令五类命中四类）

- **违反**：§1.2-1「禁止 util/common/misc/helper/shared 垃圾抽屉包名」
- **证据**：
  - `internal/logic/common/`：65 个 .go，横跨 audit/captcha/cron/email/jwt/notification/ip_blacklist/pagination 等十余功能域，被 **141 个文件**引用；
  - `relay/common/`：26 个 .go（http_client/billing/usage/cache_hash 等）；
  - `relay/helper/`：22 文件（被 26 文件引用）；`relaykit/relayconvert/internal/shared/`：13 文件（职责尚收敛）；`relaykit/relayconvert/kitutil/`：5 文件杂烩；
  - `internal/utility/` 目录名边缘违规，但子包均有语义名（corpus/crypto/export/ssrf/totp/turnstile）——半合规。
- **修复**：common 按职责拆分（cron/email/jwt/audit 各自成包），影响面 141 文件，需专门分支渐进迁移；新代码一律不再往 common 加文件。

### 4.6 [ ] 列名魔法字符串完全取代 Columns()（全仓主流风格 vs 规范字面冲突）

- **违反**：§3.5-2「列名引用 dao.Xxx.Columns().Field，禁止魔法字符串列名」
- **证据**：logic 层 `dao.Columns()` 采用率 **0/440** —— `.Where("` 440 处/75 文件、`.Order/OrderDesc` 62 处、`.Fields` 64 处、`g.DB().Model("表名字符串")` 19 处。
- **缓解**：全部为小写常量列名 + 占位符参数化，无注入风险；实际危害是列改名时的静默漂移。
- **修复决策**：全量改造代价大（440+ 处），二选一——(a) 按规范逐步改造（触碰文件时顺手改）；(b) 修订规范承认现状（约定「列名字符串 + code review 把关」），配套在 gf gen dao 后加列名一致性测试兜底。

### 4.7 [ ] 字段级文档「最后一公里」覆盖不足

- **违反**：§3.2「每个字段写 dc:」；§3.3「错误消息中文化」；§1.4「导出类型必须有 godoc、注释统一中文」
- **证据**：
  - dc: 覆盖率：Req 字段 953/1342 = **71%**，Res 字段 78/1111 = **7%**；重灾区 `api/admin/v1/plan.go` 34 个 json 字段仅 2 个 dc:（PlanListReq 全无 dc: 也无 v:）；
  - v: 校验 tag 589 个中 **390 个（66%）无 `#` 中文定制消息**（校验失败输出 GoFrame 默认英文直达客户端）；已定制的 199 个 100% 中文且质量高；
  - api/ 中 Req/Res/Item 类型 949 个缺 godoc **614 个（64.7%）**；
  - 英文注释残留：`internal/consts/consts.go`（53 行）、`internal/response/response.go:23,40`、`internal/middleware/idempotency.go:28`、`api/docs/v1/openapi.go:5-10`、`internal/logic/admin/security.go:141,182`；
  - logic 层导出函数 godoc 缺失仅 5/540（0.9%，优秀）。
- **修复**：新接口强制 dc: + v: 消息 + 类型注释（可加到 Code Review 清单 §6.2）；存量按模块渐进补齐；v: 无消息的补 `#中文` 或全局注册中文消息模板。

### 4.8 [ ] Id/ID 缩写命名全库不一致

- **违反**：§1.2「缩写词整词全大写，保持全局一致（userID/apiURL）」
- **证据**：`Id` api/ 198 处 vs `ID` 107 处；internal/ 301 vs 98；同文件混用（`api/tenant/v1/member.go:89/96`）；另有 `Url` 6 处（`api/admin/v1/file.go:86`）。
- **修复**：已成事实惯例，全量改动破坏 JSON 契约（`json:"id"` 不受影响，仅结构体字段名）。建议：新代码统一 `ID`，存量触碰时顺手改。

---

## 5. P2 散点缺陷（按规范章节，随迭代修复）

### §1.4 注释

- [x] TODO 格式 3 处均无负责人：`internal/logic/update/checker.go:240`（2026-06 引入，陈年）、`relay/common/relay_info.go:416/418`。改为 `// TODO(负责人): 说明` 或完成删除。（2026-11-19 已补 `TODO(qian)` 负责人）

### §2.3 控制流

- [x] goto 3 处：`relay/helper/stream_scanner.go:213/215/218`（`goto shutdown`），规范明令禁止，重构为早返回。（2026-11-19 重构为 shutdown 闭包 + 早返回，行为不变）

### §2.4 错误处理

- [x] 字符串判错 2 处：`internal/logic/common/errors.go:71`（PG 唯一冲突用 `strings.Contains(err.Error(), "23505")`，注释自知应改 `pgconn.PgError` + `errors.As` 而未改——PG 版本升级/驱动切换即失效）；`internal/jsplugin/engine.go:311`（"interrupted" 兜底，前面已有 gerror.As，删掉兜底）。（2026-11-19 前者改 pgconn.PgError/lib/pq 类型化断言 + duplicate key 兜底，后者删字符串兜底）
- [x] err.Error() 绕过统一脱敏透给客户端 4 处：`internal/logic/admin/update.go:143`（下载失败透 URL/网络细节）、`:163`（回滚错误拼接）、`internal/logic/tenant/order.go:276/476`（支付渠道配置细节）——后两处还使用裸 `422`（不在 consts 定义），update.go:20 裸 `500`。改 consts 错误码 + 用户可读中文。（2026-11-19 完成：新增 CodeUpdateCheckFailed=10420，order 两处改 CodePaymentInvalidConfig，细节全部进日志）
- [x] 审计/安全日志吞错约 15 处：`internal/logic/tenant/auth.go:321-464`（12 处 `_ = common.RecordLoginHistory`，含失败登录记录——安全审计场景）、`internal/logic/common/session.go:239`（「刷新令牌重放」安全事件被吞）、`internal/logic/tenant/oauth.go:339/368`。（2026-11-19 收敛到 recordLoginHistorySafe，重放事件入库失败 Errorf）
- [x] 批量写 Flush 吞错：`internal/logic/common/batch_writer.go:118/130/168`（有 DroppedCount 缓解，补 Warningf 即可）。
  - ✅ 2026-11-19 复审：间接修复——调用点仍 `_ =`，但 Flush 内部 `batch_writer.go:110` 已 Errorf 记日志，不再静默。
- [x] `internal/plugin/hooks.go:121`：`_, _ = e.Emit(...)` 吞错无注释 + 该 goroutine 无 recover。（2026-11-19 safego.Run 收口 + 补尽力而为语义注释）
- [ ] 裸 `return err` 约 50-80 处：多数属同层快速传递可接受；跨层丢上下文的典型：`internal/logic/relay/provider.go:1039-1047`（InitHealthScore，调用方不知哪个 channelID 失败）、`admin/data_governance.go:235/256`、`admin/model.go:1008/1036/1058`、`monitor/alert_rule.go:159/275`。补 Wrapf 带关键参数。

### §2.5 context

- [x] OAuth 族 HTTP 请求无 ctx 共 9 处：`internal/logic/common/oauth/gemini.go:202`（GeminiRefreshToken 整函数无 ctx，退避 Sleep 不可打断）、`claude.go:240/302/353`、`openai.go:125/188`、`internal/logic/tenant/oauth.go:557/597/663`（均 `http.NewRequest` 无 ctx）。（2026-11-19 全族 ctx 化：NewRequestWithContext，退避感知取消，ClaudeCookieAuth 内存量 Background 一并清除，buildProxyClient/GetHTTPClient 同步）
- [x] 生产 taskchannel 3 处无 ctx：`relay/taskchannel/sora/adaptor.go:152`、`minimax/cancel.go:30`、`midjourney/adaptor.go:224`。（2026-11-19 三处加 ctx + NewRequestWithContext，调用方同步）
- [ ] `internal/logic/tenant/webhook_dispatcher.go:30`：WebhookDispatcher 结构体持有 ctx（任务根设计可接受），补注释说明持有理由与生命周期。

### §2.6 并发

- [ ] `ghttp.Request` 传入响应写出后才跑的 goroutine 共 6 处（请求对象回收竞态风险 + 外层无 recover）：`relay/handler/task.go:91`、`videos.go:92`、`minimax_video.go:91/203`、`ali_video.go:91`、`jsdispatcher.go:243` 的 `go recordTaskSubmitAudit(r, ...)`。改为先提取所需字段（URL/Header 拷贝）再传值。
- [ ] 三个 Pub/Sub 订阅循环无停止通道（仅靠进程退出）：`internal/logic/common/cache.go:349`、`config.go:376`、`content_filter.go:59`（对比 `dispatchadapter/catalog.go:319` 有 stop 的正确实现）。
- [ ] 钱包物化器无 Stop 注册（`billing/sweeper.go:43`，`cmd.go:172` 启动），停机不等待；webhook `sweepLoop/cleanupLoop` 不在 wg 内（`webhook_dispatcher.go:50-51`）。
- [ ] `internal/logic/billing/settlement.go:253/401`、`task_billing.go:324` 的 `go CheckBalanceWarning` 无并发上限（高 QPS 下 goroutine 无界堆积），建议信号量/合并去抖。

### §2.7 时间

- [x] Sleep 做同步 2 处：
  - `internal/handler/public/payment.go:50-53` —— HTTP handler 内 `Sleep(500ms)×8` 轮询 DB 等异步回调入账（占请求 ~4s、不感知 ctx 取消）；改通知机制/前端轮询；
  - `internal/dispatchadapter/maintenance.go:60-62` —— 唯一维护循环 select 分支内 `Sleep(healthRefreshDelay)` 去抖，阻塞期间 ticker 任务与按需重算全部被拖延；改 `time.AfterFunc`/独立 goroutine。
  （2026-11-19：前者改 select 感知取消（通知机制涉及前端改造暂缓）；后者去抖移独立 goroutine，主循环不再被 500ms 阻塞）
- [ ] 裸 int 时长穿越函数边界：`relay/common` 的 `SetGlobalRequestTimeoutSeconds(int)`（`cmd.go:448-449` 调用）、`dispatchadapter/maintenance.go:271` `openDurationMs int64`。改 `time.Duration`。
- [ ] DB 存 UTC 无显式工程约定（未见违规证据，迁移中无 timestamptz 审计）——补一次列类型审计即可闭环。

### §2.1/§2.9 格式与依赖

- [x] 调试残留 2 处：`relay/taskchannel/kling/jwt.go:28`（`fmt.Printf` 错误路径）、`relaykit/types/error.go:406`（错误打印 stdout）。（2026-11-19 改结构化日志/stderr）
- [ ] `interface{}` 未统一 `any`：internal/ 58 处/25 文件（建议级，触碰时顺手改）。

### §3.5 DAO/ORM

- [x] TicketReply TOCTOU：`internal/logic/tenant/ticket.go:192-204`（先单键读取→应用层比对）→ `:224-225`（单键 Update）；同文件 TicketClose `:241-248` 的三键条件更新是正确范本，照改。（2026-11-19 改三键条件更新 + affected 判定，先更状态后写回复）
- [x] admin AI 分析两端点分页无上限：`api/admin/v1/ai_analysis.go:32/88` PageSize 无 v: 规则 + `internal/logic/admin/ai_analysis.go:101/215` 直用 req.PageSize 未过 NormalizePagination（可传超大 size 拖库）。补 v: 规则或统一走 NormalizePagination。（2026-11-19 两处走 NormalizePagination 封顶 100）
- [x] 导入流程 N+1：`internal/logic/admin/model_import_export.go:315-316` 循环内逐模型 Count，改 WhereIn 批量。（2026-11-19 WhereIn 一次往返 + 顺带修复 Count 吞错）
- [ ] 原生 SQL 固定片段拼接约 10 文件：`admin/channel_error.go:88-115`、`channel_debug_log.go:72-103`、`dashboard.go:1436` 等 `"SELECT ... WHERE "+where` 模式——值全部参数化无注入，但违反「禁止拼接 SQL」字面要求，且统计 SQL 游离于 ORM 体系外。可豁免但建议集中到查询构造器。

### §3.7 日志

- [x] 次级泄漏 3 处：`internal/logic/common/oauth/openai.go:144/207`（token 交换失败打上游完整 body）、`relay/handler/task_handler.go:326`（上游错误 body 原文进日志，RedactMessage 只用于入库字段）、`cmd_corpus_extract.go:282`（CLI 已有显式警告，算已告知）。（2026-11-19 前者截断 512B，后者 RedactMessage 脱敏 + 截断 2KB）
- [ ] glog 与 g.Log() 混用约 60 处（plugin/jsplugin/aianalysis，同组件等价，统一即可）。
- [ ] WriteRelayError 三件套用 Background（`chat_handler.go:191` 等，错误写回路径 ctx 常已取消，合理取舍，加注释说明即可）。

### §3.10 定时任务

- [x] cron 任务无统一超时 ctx：`internal/logic/common/cron.go:139` 裸传 `gctx.New()` 无 deadline，对账/聚合类长任务理论上可无限跑（仅 10min 锁 TTL 兜底放行下一轮，本进程不中断）；仅 `channel_auto_test_cron.go:68` 自建 30s。调度器统一包 `WithTimeout`。（2026-11-19 runJobInternal 统一 10min WithTimeout 对齐锁 TTL，CronJob.Timeout 可覆盖）

### §3.11 HTTP 客户端

- [x] `internal/logic/common/pricing_source.go:65/182/327/477` 4 处 `g.Client()` 无 SetTimeout（远程拉 LiteLLM 定价，可能挂住 worker）。（2026-11-19 统一 SetTimeout 30s）
- [ ] `internal/logic/update/update.go:118` 自健康检查无超时（localhost，低危）。

### §5 安全

- [x] relay 文件上传白名单与大小限制默认关闭：`internal/logic/relay/files.go:120-145`（allowedTypes 空=不限制、maxSize 0=不限制，仅全局 32MB 兜底），文件名随机化已优秀（`uploads/{date}/{uuid}{ext}`）。对照标杆：`internal/jsplugin/pluginicon.go`（512KiB + SVG 六连拒 + PNG 魔数）。建议给配置默认值。（2026-11-19 默认类型白名单=常用语料+图片、默认单文件 32MB，配置可显式覆盖）
- [ ] 模型导入读上传文件无扩展名白名单/大小校验（`internal/logic/admin/model_import_export.go:270-295`，admin 权限缓解）。

### §4 测试

- [ ] 表驱动测试占比约 40%（92 个 logic 测试文件中 36-37 个显式 `tests := []struct`），未达「默认形态」要求。
- [ ] 测试 Sleep 等时序 5 处：`internal/dispatchadapter/state_redis_test.go:637`、`internal/logic/aianalysis/aiclient_test.go:220`、`internal/logic/payment/callback_test.go:58/102`、`tests/integration/tenant/promo_manual_complete_test.go:144`。改轮询断言（`testutil` 已有受控重置工具可参考）。
- [ ] 覆盖率 ≥80% 无度量无卡点（归入 4.1 工具链）。

### §6 工程化

- [ ] commit 瑕疵 2 处：`8e083ed0` `improve(channel)` 非标准 type；`fd1eb8f7` 冒号后无空格。（94/100 已符合，仅记录）

---

## 6. 规范与项目既定约定的冲突项（决策清单，建议修订规范而非机械整改）

| # | 冲突点 | 现状 | 建议 |
|---|---|---|---|
| 1 | Redis key 命名（§3.9 要求 `{项目}:{模块}:...` 四段） | 项目自有体系 `wallet:v2:{tenant}`、`dispatch:v1:breaker:{chn}` 等两三段（30+ 前缀，内部大体一致） | 修订规范承认现状；但内部不一致需收敛：并发限制键三套并存（`conc:apikey:`/`conc:tenant:`/`tenant:conc_limit:`，`ratelimit.go:302/355/419` vs `provider.go:1198`）、`channel_breaker:%d` 单段下划线独此一家（`dispatchadapter/config.go:75`） |
| 2 | ctx 传值模型（§3.8 要求 typed struct `model.Context`） | 项目用「常量 key + accessor 函数」（`middleware.GetTenantID` 等，CLAUDE.md 明示为项目约定）；`open_auth.go:38-40` 与 jsplugin 已用真 typed key | 修订规范与项目对齐；`middleware/request_id.go:26` 裸字符串 "RequestId" 是框架 logger 机制所需，加注释 |
| 3 | 定时任务框架（§3.10 写 gcron AddSingleton） | 自研 robfig/cron + running map 互斥 + Redis SET NX EX 分布式锁 + Lua CAS 释放 + runHandlerSafely recover（`internal/logic/common/cron.go`），语义强于 gcron（秒级粒度 gcron 做不到，注释有说明） | 修订规范改述为「同等语义要求」；仅补任务超时（见 §3.10 散点） |
| 4 | relay 对 POST 重试（§3.11 重试仅幂等请求） | AI 推理 POST 也重试（网关领域惯例），`bil_records.request_id` 唯一约束幂等闸门兜底 | 规范加网关场景豁免条款 |

## 7. 符合度良好的方面（保持，不必动）

- **分层零违规**：logic 不 import ghttp/controller（150 文件）；controller 全目录 0 处 SQL/配置/调试输出；`admin/channel.go:138-157` 匿名 struct 投影查询，**Res 零泄漏 entity**（api/ 与 controller/ 引用 entity = 0）。
- **多租户双键校验 >90%**：约 40 处按 ID 资源操作抽查未发现可利用跨租户路径；`tenant/api_key.go` 多处叠加 user_id 三键。
- **g.Meta 注解 100% 齐全**（503 个，四要素缺失 0）；统一响应 + `internal/response` 脱敏出口 + Recovery 中间件 panic→500 结构化中文响应，§3.6 完整落地。
- **错误码集中定义质量高**：`internal/consts/consts.go:84-200+` 按功能域分段 + 编号策略注释；哨兵错误 Err 前缀规范。
- **并发工程化示范级**：`usage_log_writer.go:165` worker 带 panic 自动重启；`sync_image_worker.go:163-199` 三段式优雅停机；`cron.go:288` runHandlerSafely；`cmd.go:362-385` LIFO 优雅退出顺序文档化；channel 均发送方关闭 + 方向限定；循环 time.After 0 处泄漏（全部 NewTicker+Stop）。
- **Redis 强制项全合规**：23 处原生 SET 全带 EX/NX+EX；统一缓存层强制 SETEX 且兜底 24h；钱包 fail-closed + Lua 原子（`billing/wallet.go:319-494`）；cron 锁 Lua CAS。
- **SSRF 防护完整**：`internal/utility/ssrf`（dial 阶段校验防 DNS rebinding、拒云元数据、重定向复检），4 个用户可控 URL 场景全覆盖——2026-10 审计的 Webhook SSRF 项**已修复**。
- **HTTP 客户端超时完备**：relay 统一连接池客户端（300s 默认 + 按渠道覆盖 + 长任务专用）、oauth 30s、webhook 30s + LimitReader；resp 判空 + defer Close 模式一致。
- 事务规范：49 处全部闭包 + ctx 传播式；钱包「Redis Lua 为提交点 + 事务失败补偿逆转」注释完备（`payment/fulfill.go:22-23/132-135`）。
- do 结构体主流（62 处）vs g.Map 仅 12 处显式少列更新，无零值覆盖风险；分页统一入口 NormalizePagination（64 处调用）。
- 常量全驼峰（SCREAMING_SNAKE = 0）、文件名全 snake_case、`context.TODO()` 全仓 0 处、ctx 传 nil 0 处、注释掉的代码基本为零、commit 94% 符合 Conventional Commits、logic 导出函数 godoc 覆盖 99.1%、核心模块测试覆盖广（billing 30/32 文件、relayconvert 59/50、dispatch 9/9 全覆盖）。

## 8. 整改路线建议

| 阶段 | 内容 | 条目 |
|---|---|---|
| 本周 | P0 四项：凭据轮换 + git 历史清除 → 默认密钥黑名单 → 退款吞错加日志与对账确认 → handler_response 脱敏 | §3.1-3.4 |
| 短期 | 落地 .golangci.yml + CI lint/test（工具链是执行不一致的根治手段）；goroutine recover 统一包装；Background→WithoutCancel 替换；计费吞错逐处修复 | §4.1-4.4 |
| 中期 | common 包按职责拆分（专门分支）；v:/dc:/godoc 渐进补齐并加入 Review 清单；kling/relaykit 调试残留清理 | §4.5、§4.7、§5 |
| 决策 | Redis key 前缀、ctx 模型、cron 框架、relay 重试——修订规范与项目对齐 | §6 |
| 随手 | 触碰文件时顺手：Id→ID、interface{}→any、列名字符串→Columns()（若决策改造）、TODO 补负责人 | 童子军军规 |

---

## 9. 复审记录（2026-11-19）

> **复审基线**：feat/coding 分支 HEAD（4915e5ee），距初审约 36 个提交（jsplugin 体系、AI 分析/助手、表达式计费、租户模型价格重构、工具计费等）。
> **复审方式**：P0/P1 逐条核对 + P2 散点 20 项逐条核对 + 11-07 后新增模块（internal/jsplugin、internal/logic/aianalysis、internal/logic/aiassistant、relay/taskchannel/jsbridge、internal/logic/task/artifact、billing/formula、tool_billing、tenant_model 重构）专项扫描。行号为复审时工作区实测。
> **总体结论**：略有好转但幅度有限——仅 §3.1 主体修复、§3.3 小幅改善、P2 批量写 1 项闭环；同时新增代码引入 2 处高优先级违规（常驻 worker 缺 recover）。新增模块自身质量高于存量均值。

### 9.1 已修复 / 好转

| 原条目 | 状态 | 说明 |
|---|---|---|
| §3.1 config.yaml 凭据入 git | ✅ 主体修复（已勾选） | 文件不再被跟踪、已入 .gitignore、git 历史无该文件。遗留：口令轮换需运维侧确认；example encryptionKey 仍为固定值 |
| §3.3 SettleFailed 吞错 | 🔶 小幅好转（未勾选） | 23 处 → 20 处：relay_handler.go:545/997/1107 已检查错误记日志，:790/828/855 改用 WithoutCancel；剩余 relay_handler 13、realtime 4、sync_image 3 处零日志 |
| §5 P2 批量写 Flush 吞错 | ✅ 间接修复（已勾选） | Flush 内部 batch_writer.go:110 已 Errorf 记日志 |
| （非原条目）relay 错误防泄漏 | ✅ 改善 | 提交 4915e5ee 新增 NewInternalError（Cause 仅入服务端日志），修正 sync_image 预扣内部错误透出细节。原条目点名的管理侧 4 处（admin/update.go、tenant/order.go）仍未动 |

### 9.2 未修复（维持原状，行号可能有漂移）

- **P0**：§3.2 默认密钥黑名单；§3.4 handler_response 全量请求体入日志未脱敏。
- **P1**：§4.1 工具链（无 .golangci.yml / Makefile 无 lint·test / CI 无测试 / gofmt 偏差仍 7 文件，均在 relaykit）；§4.2 点名 24 处仅约 1.5 处实质达标（task.go worker 池经 ExecuteTask 内 recover 间接覆盖、common/config.go 回调经 fireSettingsChanged per-hook recover 部分兜底）；§4.3 点名 9 处全部未动（仅同文件 GetLevelPriceMultiplier 补了日志）；§4.4 点名 11 处全部未动（实现侧 provider.go:494/645 的 RecordAudit/UpdateTaskAudit 内部亦自建 Background 丢 ctx，改调用点时需一并处理）；§4.5-4.8 无相关提交。
- **P2 散点 20 项**：1 项修复（批量写）、2 项局部改善（§2.4-err.Error 管理 4 处仍未换文案、§3.7 task_handler.go:381 网络错误已收敛但 :396 上游 body 全文仍入日志）、其余 17 项未动（23505 字符串判错、goto 3 处、OAuth 无 ctx 9 处、taskchannel 无 ctx 3 处、payment.go Sleep×8 轮询、maintenance select 内 Sleep、TicketReply TOCTOU、ai_analysis 分页无上限、model 导出 N+1、oauth 日志泄漏、上传白名单默认关闭、pricing_source 无超时、cron 无统一超时（另 cron.go:213/420 新增同类）、kling/relaykit 调试残留、TODO 无负责人、审计日志吞错 12+ 处）。

### 9.3 新增违规（2026-11-07 后引入，按严重度）

| # | 严重度 | 位置 | 问题 |
|---|---|---|---|
| 1 | 高 | `internal/logic/task/artifact/artifact.go:104`（worker 体 :241-252） | 新增产物转存常驻 worker 无 defer recover，processJob 内含 DB 读写/文件下载/存储上传，panic 即崩整个多租户进程 |
| 2 | 高 | `internal/jsplugin/invalidate.go:80`（订阅体 :94-123） | jsplugin 跨实例失效的 Redis 订阅重连循环无 recover，Receive/rebuildRemote panic 即崩进程（对照正确范本：aianalysis 的 safeExecuteJob） |
| 3 | 中 | `internal/jsplugin/dryrun.go:73/121` | DryRun() 不收 ctx，withDryRunMarker(context.Background())，被管理端试运行 API（logic/admin/jsplugin.go:82）与 AI 助手技能链同步调用（有独立硬超时缓解） |
| 4 | 低 | `internal/handler/relay/jsdispatcher.go:244`、`task.go:116/367/458`、`videos.go:114` | go recordTaskSubmitAudit 外层 goroutine 无 recover（内层 RecordAudit 有，外层组包/captureRequestHeaders 裸奔） |
| 5 | 低 | `internal/handler/admin/ai_assistant_chat.go:234` | `_, _ = redis.Do(EXPIRE)` 吞错：INCR 成功而 EXPIRE 失败时限流计数键永不过期 |
| 6 | 低 | `internal/logic/task/async_polling.go:547` | `json.Marshal` 吞错（snapshot/state 已 json.Valid 预校验，概率低），失败时向上游发 null 字节 |
| 7 | 低 | `internal/logic/billing/pricing.go:1131`、`task_billing.go:570-577` | 估算/展示口径出现 decimal→float64→再取整的链式折返（不进结算，建议收敛口径） |
| 8 | 低 | `internal/jsplugin/registry.go:921/929`、`generation.go:214` | 请求路径可达的无 ctx 日志（checkManifestConflict 在激活/配置保存请求链被调用） |

### 9.4 新增代码符合性评价（保持项）

11-07 后新增模块整体质量高于存量均值：jsplugin 引擎 poisoned 隔离与中断标志清理（engine.go:315-321）；表达式计费/token 成本链/工具附加费全程 decimal，出边界即 NewFromFloat；CodeJSPluginOperation=10419 集中错误码（无裸 422）；admin/tenant_model 双键校验齐全、内容链接 HMAC 域分离 + 常量时间比较；jobrunner CAS 认领防多副本重复 + safeExecuteJob recover；observe 钩子脱敏先抹除后截断；新文件 Tab 缩进与中文 godoc 覆盖良好。

### 9.5 复审建议优先级

1. §9.3 #1/#2 常驻 worker recover——改动最小、风险最高，顺手把 #4 的 recordTaskSubmitAudit 族一起包掉；
2. §3.3 剩余 20 处 SettleFailed 补 Warningf（带 tenantID/requestID/冻结金额）；
3. §3.4 日志脱敏（复用 maskSensitiveFields + 2KB 截断，一处改动）；
4. 确认 §3.1 口令轮换已执行；example encryptionKey 改为明显占位符。

---

## 10. 整改完成记录（2026-11-19 当日）

> 复审当日启动全量整改（feat/coding 分支，批次 B–G 共 6 个提交：`2af6de3f`/`c20f9f07`/`0e31efeb`/`43a711f4`/`23a3e9c3`/文档批次），
> 每批 `go build` + `go vet` + `go test ./internal/... ./relay/...` + relaykit 全绿后提交。

### 10.1 已修复（本轮勾选）

- **§3.3** SettleFailed 吞错 20 处（releasePreDeduct / releaseSyncImagePreDeduct 统一入口）
- **§4.1** 质量三件套（.golangci.yml ×2 + Makefile fmt/lint/test/cov + ci.yml；lint 首期报告模式）
- **§4.2** goroutine recover 全量收口（新增 internal/utility/safego，点名 24 处 + 新增 #1/#2/#4）
- **§4.3** 九处零值吞错补留痕
- **§4.4** 11 处 Background → WithoutCancel + provider 实现侧 + DryRun ctx 化（§9.3 #3）
- **P2 已修**：字符串判错 2、goto 3、err.Error 透出 4、审计吞错 15、hooks.go、OAuth ctx 9、taskchannel ctx 3、
  payment/maintenance Sleep 2、调试残留 2、TODO 负责人 3、TicketReply TOCTOU、ai_analysis 分页、N+1、
  日志泄漏 3、上传默认值、pricing_source 超时、cron 统一超时
- **§9.3 新增违规**：#1/#2（recover）、#3（DryRun ctx）、#4（audit 族）、#5/#6/#8（吞错留痕）已修；#7（decimal 折返）未动
- **规范文档对齐**（§6 决策项落地）：Redis key 两三段体系、ctx 常量 key+accessor、CronScheduler 同等语义、
  relay POST 重试豁免、列名字符串约定（替代 Columns() 强制）、Id→ID 新代码统一——均已写入《Go与GoFrame代码开发规范.md》

### 10.2 仍未开放（后续批次）

| 项 | 状态 | 说明 |
|---|------|------|
| §3.2 默认密钥黑名单 + §3.4 handler_response 脱敏 + example 占位符 | **暂缓** | 用户决策：启动 fail-fast 可能影响线上既有部署，待运维窗口单独实施（§9.5 #3/#4 顺延） |
| §3.1 遗留 | 待确认 | 数据库/Redis 口令轮换属运维动作；config.example.yaml encryptionKey 仍为固定值 |
| §4.5 common 拆包 | 决策不改 | 规范已承认现状；新代码不再往 common 加文件 |
| §4.6/§4.7/§4.8 | 决策不改 | 规范已对齐；童子军军规渐进 |
| §2.4 裸 return err 补 Wrapf | 未动 | 量大（50-80 处），触碰时顺手 |
| §2.5 webhook_dispatcher 持有 ctx 注释 | 未动 | 低优先级 |
| §2.6 ghttp.Request 进 goroutine 6 处（竞态）、Pub/Sub 停止通道 3 处、物化器 Stop 注册、CheckBalanceWarning 并发上限 | 未动 | 结构性改造，需单独设计 |
| §2.7 裸 int 时长、DB timestamptz 审计 | 未动 | 低优先级 |
| §2.1/2.9 interface{}→any 58 处 | 未动 | 触碰时顺手 |
| §3.5 原生 SQL 拼接豁免确认 | 未动 | 值全部参数化，建议集中查询构造器 |
| §3.7 glog 混用、WriteRelayError Background 注释 | 未动 | 低优先级 |
| §3.11 update 自检超时 | 未动 | localhost 低危 |
| §5 模型导入文件校验 | 未动 | admin 权限缓解 |
| §4 表驱动测试占比、测试 Sleep 5 处、覆盖率度量 | 未动 | 工具链已就绪（make cov），随迭代补 |
| lint 存量 457+31 项 | 清理中 | CI 报告模式跟踪，清零后转阻塞门禁 |
| 已知 flaky：middleware TestResourceGuard_DisabledPassThrough（包级偶发）、-count>1 路由重复注册 | 未动 | 测试隔离问题，待专项 |

---

## 附录：审查方法说明

- 机械扫描：fmt.Println / strings.Contains(err.Error()) / time.Sleep / context.Background / TODO / goto / fallthrough / interface{} / SCREAMING_SNAKE 常量 / replace 指令 / golangci 配置存在性 / gofmt 偏差实测。
- 分维度深查（五路并行）：错误处理（§2.4/§3.6）、分层与 DAO（§3.1/§3.4/§3.5）、context·并发·时间（§2.5-§2.7）、缓存·日志·配置·安全（§3.7-§3.11/§5）、API 定义·测试·工程化（§3.2/§3.3/§1.2/§1.4/§4/§6）。
- 行号为 2026-11-07 工作区实测（feat/v0.3 分支），后续提交可能漂移，修复前以当时文件为准。

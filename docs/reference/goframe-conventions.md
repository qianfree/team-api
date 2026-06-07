# GoFrame 框架使用规范

本文档记录项目中 GoFrame v2 的使用规范、正确模式和已修复的框架使用错误。开发时按需查阅，发现新的框架使用 bug 修复后同步更新本文档。

> **提示**：遇到不确定的 GoFrame 用法时，可使用 `/goframe-v2` skill 查询框架最新规范和最佳实践。

## 时间处理

### 规则

- **ORM 写入**：使用 `gtime.Now()`，与 GoFrame ORM 的时间类型自然兼容
- **纯计算/计时**：使用标准库 `time.Now()`（如 `time.Since(start)` 计算耗时、`time.Now().Unix()` 生成时间戳）
- **time → gtime 转换**：使用 `gtime.NewFromTime(t)` 包装后再写入 ORM

### 正确示例

```go
// ORM 写入场景 — 用 gtime
dao.XxxTable.Ctx(ctx).Data(g.Map{
    "updated_at": gtime.Now(),
}).Update()

// 耗时计算 — 用标准 time
start := time.Now()
doSomething()
elapsed := time.Since(start)

// time.Time 转 gtime 写入 ORM
nextRetry := time.Now().Add(delay)
dao.XxxTable.Ctx(ctx).Data(g.Map{
    "next_retry_at": gtime.NewFromTime(nextRetry),
}).Update()
```

### 常见错误

- 在 ORM `.Data()` 中直接传 `time.Now()` — 应使用 `gtime.Now()`
- 在 `time.Since()` 中使用 `gtime.Now()` — 应使用 `time.Now()`

## 错误处理

### 规则

- 业务错误使用 `gerror.NewCode(gcode.New(code, msg, nil), msg)` 创建预定义错误常量
- 预定义错误常量放在 `internal/consts/consts.go`
- 包装底层错误时使用 `gerror.Wrapf(err, "context message")`
- 临时业务错误使用 `gerror.Newf("中文提示 %s", param)`
- Controller 层不处理错误，直接 `return nil, err` 交给中间件统一处理

### 错误码分级

| 码段 | 用途 | 示例 |
|------|------|------|
| 400-499 | 标准 HTTP 客户端错误 | 401 未认证、403 无权限、404 不存在 |
| 500 | 服务器内部错误 | 未预期的异常 |
| >= 10000 | 业务规则错误（HTTP 422） | 10001 余额不足、10002 额度用完 |

### 正确示例

```go
// consts.go — 预定义业务错误
var ErrInsufficientBalance = gerror.NewCode(
    gcode.New(CodeInsufficientBalance, MsgInsufficientBalance, nil),
    MsgInsufficientBalance,
)

// logic 层 — 返回预定义错误
if balance < amount {
    return nil, consts.ErrInsufficientBalance
}

// logic 层 — 包装底层错误
result, err := dao.XxxTable.Ctx(ctx).Insert(data)
if err != nil {
    return nil, gerror.Wrapf(err, "insert xxx")
}

// logic 层 — 临时业务错误
return nil, gerror.Newf("兑换码状态为%s", redemption.Status)
```

### 错误安全规则

`internal/response` 包自动执行错误脱敏：
- 业务错误（>= 10000）和 HTTP 4xx：原始消息透传给客户端
- 其他错误（数据库、网络等）：替换为 `"服务器内部错误"`，原始错误记日志
- 禁止在错误消息中暴露 SQL、堆栈、内部路径等技术细节

## 日志记录

### 规则

- 统一使用 `g.Log()` 全局日志，不要创建自定义 Logger 实例
- **必须传 ctx**：`g.Log().Errorf(ctx, "message: %v", err)`，确保日志携带 request_id
- 日志级别：`Error`（需要修复的异常）、`Warning`（降级处理、已脱敏的错误）、`Info`（关键业务事件）
- 后台 goroutine 中如果没有请求 ctx，使用 `context.TODO()`

### 正确示例

```go
g.Log().Errorf(ctx, "update task %d to failed: %v", taskID, err)
g.Log().Warningf(ctx, "webhook: config %d auto-disabled after %d failures", configID, count)
g.Log().Infof(ctx, "API key %d disabled, cache invalidation for prefix %s", keyID, prefix)
```

### 常见错误

- `g.Log().Error("message")` — 缺少 ctx，日志中不会有 request_id
- 在可恢复的业务场景使用 Error 级别 — 应使用 Warning

## ORM 操作

### DAO 模式（优先）

使用生成的 `dao.Xxx.Ctx(ctx)` 操作数据库，禁止直接使用 `g.DB().Model("table_name")`：

```go
// 查询（指针类型，用 nil 判断无数据）
var user *entity.TntUsers
err := dao.TntUsers.Ctx(ctx).Where("id", id).Where("tenant_id", tenantID).Scan(&user)
if err != nil {
    return nil, err
}
if user == nil {
    return nil, common.NewNotFoundError("用户")
}

// 插入（使用 DO 结构体）
id, err := dao.ChnChannels.Ctx(ctx).InsertAndGetId(do.ChnChannels{
    Name:     req.Name,
    Provider: req.Provider,
})

// 更新（使用 DO 结构体，框架自动维护 updated_at）
_, err := dao.TntUsers.Ctx(ctx).Where("id", id).Data(do.TntUsers{
    Status: "disabled",
}).Update()

// 删除
_, err := dao.ApiKeys.Ctx(ctx).Where("id", id).Where("tenant_id", tenantID).Delete()

// 分页
m := dao.TntUsers.Ctx(ctx).Where("tenant_id", tenantID)
total, _ := m.Count()
var list []entity.TntUsers
err := m.Page(page, pageSize).OrderDesc("id").Scan(&list)
```

### 禁止模式

```go
// 禁止 — 绕过 DAO 直接操作表
g.DB().Model("tnt_users").Ctx(ctx).Where("id", id).Scan(&user)

// 正确 — 使用 DAO 对象
dao.TntUsers.Ctx(ctx).Where("id", id).Scan(&user)
```

DAO 优势：链安全、列名映射、Handler 链（自动注入 tenant_id 等）。仅在以下场景允许 `g.DB().Model()`：
- 动态表名（如泛型 batch writer）
- 表别名 JOIN 查询（如 `Model("mdl_model_groups mg").LeftJoin(...)`）

### 原生 SQL（仅用于复杂查询和批量操作）

```go
// 聚合查询、分析报表等无法用 ORM 表达的场景
// 注意：始终使用 ? 占位符，禁止 fmt.Sprintf 拼接值
result, err := g.DB().Ctx(ctx).Raw(`
    SELECT date_trunc('hour', created_at) AS hour, count(*)
    FROM aud_request_logs WHERE created_at >= ?
`, sinceTime).All()

// 原子更新（如余额操作）
_, err = g.DB().Exec(ctx,
    `UPDATE bil_wallets SET balance = balance - ? WHERE id = ? AND balance >= ?`,
    amount, walletID, amount,
)
```

**SQL 参数化规则**：GoFrame 统一使用 `?` 占位符，驱动层自动转换为 PostgreSQL 的 `$1, $2...`。开发者永远写 `?`，禁止 `fmt.Sprintf` 拼接 SQL 值。

### Scan 指针类型规则

查询单行记录时，**始终使用指针类型**接收 Scan 结果：

```go
// 正确 — nil 指针
var wallet *entity.BilWallets
err := dao.BilWallets.Ctx(ctx).Where("tenant_id", tenantID).Scan(&wallet)
if err != nil {
    return nil, err
}
if wallet == nil {
    return nil, common.NewNotFoundError("钱包")
}

// 错误 — 值类型，无行时返回 sql.ErrNoRows
var wallet entity.BilWallets
err := dao.BilWallets.Ctx(ctx).Where("tenant_id", tenantID).Scan(&wallet)
```

指针类型 Scan 无行时返回 `nil` 错误 + `nil` 指针，用 `if x == nil` 判断。值类型 Scan 无行时返回 `sql.ErrNoRows`，容易漏处理导致暴露技术细节给用户。

### 写操作错误处理

所有数据库写操作（Insert/Update/Delete）的错误**必须处理**：

```go
// 关键操作 — 必须 return err
if _, err := dao.BilTransactions.Ctx(ctx).Insert(do.BilTransactions{...}); err != nil {
    return nil, gerror.Wrapf(err, "record transaction")
}

// 级联操作 — 必须逐一检查
if _, err := dao.MdlPricingTiers.Ctx(ctx).Where("model_id", id).Delete(); err != nil {
    return nil, gerror.Wrapf(err, "delete pricing")
}

// 非关键操作 — 至少记录日志
if _, err := dao.OpnWebhookEvents.Ctx(ctx).Where("id", id).Data(do.OpnWebhookEvents{...}).Update(); err != nil {
    g.Log().Errorf(ctx, "webhook: update event %d failed: %v", id, err)
}

// 禁止 — 静默丢弃错误
_, _ = dao.ApiKeys.Ctx(ctx).Where("id", id).Delete()
```

### 自动时间维护

当表包含 `created_at`、`updated_at` 字段时，GoFrame ORM **自动处理**，禁止手动设置：

```go
// 正确 — 框架自动写入 updated_at
dao.TntUsers.Ctx(ctx).Where("id", id).Data(do.TntUsers{
    Status: "disabled",
}).Update()

// 错误 — 手动设置 updated_at（多余且违反规范）
dao.TntUsers.Ctx(ctx).Where("id", id).Data(do.TntUsers{
    Status:    "disabled",
    UpdatedAt: gtime.Now(),  // 多余！框架自动处理
}).Update()
```

### 事务

```go
err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
    // tx.Model() 替代 dao.Xxx.Ctx()，确保操作在同一事务内
    _, err := tx.Model("tnt_tenants").Ctx(ctx).Data(do.TntTenants{...}).Insert()
    if err != nil {
        return err  // 自动回滚
    }
    _, err = tx.Model("tnt_users").Ctx(ctx).Data(do.TntUsers{...}).Insert()
    return err
})
```

### g.Map vs DO 结构体

- **禁止使用 `g.Map` 做数据库操作**：所有 `Data()` 调用必须使用 `do.XxxTable{}` 结构体
- DO 结构体字段类型为 `interface{}`，未赋值的字段保持 `nil`，ORM 内置 OmitNil 自动跳过
- 已赋值的字段（包括零值 `0`、`""`）会被正常写入

```go
// 正确 — 使用 DO 对象
dao.Users.Ctx(ctx).Data(do.Users{
    Name:     req.Name,
    Password: hash,
}).Where("id", id).Update()

// 正确 — 条件更新，未赋值字段自动跳过
data := do.Users{}
if req.Name != nil { data.Name = *req.Name }
if req.Count != nil { data.Count = *req.Count } // 即使 *req.Count == 0 也会写入
dao.Users.Ctx(ctx).Where("id", id).Data(data).Update()

// 错误 — 禁止使用 g.Map
dao.Users.Ctx(ctx).Data(g.Map{"name": req.Name}).Where("id", id).Update()
```

## 配置读取

```go
// 使用 g.Cfg() 读取 manifest/config/ 下的配置
secret := g.Cfg().MustGet(ctx, "jwt.secret").String()
maxSessions := g.Cfg().MustGet(ctx, "jwt.adminMaxSessions").Int()
hexKey := g.Cfg().MustGet(ctx, "crypto.encryptionKey").String()
```

## 参数校验

API 结构体使用 `v` tag 声明校验规则，GoFrame 自动执行校验：

```go
type CreateAdminUserReq struct {
    g.Meta   `path:"/admin-users" method:"post" tags:"管理员" summary:"创建管理员"`
    Username string `json:"username" v:"required|length:3,50#请输入用户名|用户名长度为3-50位"`
    Password string `json:"password" v:"required|length:8,64#请输入密码|密码长度为8-64位"`
    Status   string `json:"status" v:"required|in:active,disabled#请选择状态|状态值无效"`
}
```

- `#` 后面是自定义中文错误消息，多个规则的消息用 `|` 分隔
- 校验失败自动返回 400 错误，无需在 logic 层重复校验

## 类型转换

使用 `gconv` 包进行安全类型转换：

```go
name := gconv.String(record["name"])
ids := gconv.Int64s(record["user_ids"])
rate := gconv.Float64(record["threshold"])
methods := gconv.Strings(record["notification_methods"])
```

## 缓存

项目使用 `gcache` 作为 L1 内存缓存，封装在 `internal/logic/common/cache.go`：

```go
// 通过 common 包的缓存封装使用，不要直接调用 gcache
common.CacheSet(ctx, "key", value, ttl)
val, err := common.CacheGet(ctx, "key")
common.CacheRemove(ctx, "key")
```

## Context 传递

```go
// 从 Context 读取请求级变量（中间件注入）
requestID := r.GetCtxVar("RequestId").String()
scope := r.GetCtxVar("ApiKeyScope").String()

// 获取当前请求对象
r := g.RequestFromCtx(ctx)
```

---

## 已修复的框架使用错误记录

> 每次修复 GoFrame 框架使用相关的 bug 后，在此处记录修复内容、原因和正确做法，防止同类问题再次出现。

### 2026-04-16：GoFrame gcode 非标准码用作 HTTP 状态码导致 panic

**问题**：`response.Error()` 中直接将 `gerror.Code().Code()` 作为 HTTP 状态码传给 `WriteHeader()`，但 GoFrame 内置的 `gcode`（如 `CodeNotModified=68`）不是合法 HTTP 状态码，导致 `WriteHeader` panic。

**修复**：在 `response.go` 中增加范围检查，非 100-599 的 code 回退为 500。

### 2026-04-16：MiddlewareHandlerResponse 未过滤文件下载响应

**问题**：导出 CSV/Excel 时，控制器直接写入 `r.Response.Writer`，但中间件仍然追加 `{"code":0,...}` JSON 到响应体末尾，导致文件损坏。

**修复**：在 `handler_response.go` 中检查 `Content-Type`，匹配 `text/csv`、`application/vnd.openxmlformats` 等下载类型时跳过 JSON 包装。

### 2026-04-16：isSystemError 日志污染

**问题**：所有错误（包括 400 参数校验、401 认证失败等正常业务流）都以 Warning 级别记录日志，大量无意义日志淹没真正的异常。

**修复**：增加 `isSystemError()` 函数，4xx 和 >= 10000 的业务错误不记日志，只有 5xx 和未知错误才记录。

### 2026-05-14：Scan(&struct) 查询无结果返回 sql: no rows in result set

**问题**：GoFrame v2 中 `dao.Xxx.Ctx(ctx).Where(...).Scan(&structValue)` 当查询无匹配行时，返回 `sql: no rows in result set` 错误（Go 标准库 `sql.ErrNoRows`）。后续通过 `structField == 0` 判断"无数据"的代码永远不会执行，因为 `err != nil` 会先返回。导致 playground chat、定价查询、钱包查询、渠道调度等链路在数据库无对应记录时暴露原始 SQL 错误给用户。

**原因**：GoFrame v2 的 `Scan` 对 struct 值类型和指针类型行为不同：
- `Scan(&structValue)` — 无行时返回 `sql.ErrNoRows`
- `Scan(&pointerValue)` — 无行时返回 `nil`，指针设为 `nil`

**修复**：将所有"期望零或一行"的 `Scan` 调用从值类型改为指针类型，用 `if x == nil` 替代 `if x.Field == 0` 判断。涉及文件：
- `internal/logic/tenant/playground.go`（findActiveApiKey）
- `internal/logic/relay/provider.go`（CheckTenantModelAccess、tryAffinityChannel、GetModelDetail、getChannelKey）
- `internal/logic/billing/pricing.go`（GetModelPrice 的三次 Scan、EstimatePreDeductAmount）
- `internal/logic/billing/wallet.go`（GetWallet、syncWalletToRedis、preDeductDB、preDeductSyncDB、unfreezeDB、recordTransaction）

**正确做法**：查询单行记录时，始终使用指针类型接收 Scan 结果，通过 `nil` 检查判断无数据。

### 2026-05-23：gf gen ctrl withService:true 对新方法生成桩代码（Not Implemented）

**问题**：按正确顺序执行 `logic → gf gen service → gf gen ctrl` 后，`gf gen ctrl` 生成的控制器文件中方法体是 `return nil, gerror.NewCode(gcode.CodeNotImplemented)`，调用 API 返回 501 Not Implemented。

**原因**：`gf gen ctrl` 的 `withService: true` 自动接线机制对新方法不可靠。即使 service 接口文件已包含正确的方法签名，ctrl 生成器仍可能无法匹配并生成桩代码。且一旦首次生成桩代码，后续重跑 `gf gen ctrl` 会跳过已存在的文件，不会覆盖修复。

**修复**：手动将控制器文件改为 `return service.Admin().MethodName(ctx, req)` 调用。

**正确做法**：每次执行 `gf gen ctrl` 后，必须检查生成的控制器文件是否包含 `CodeNotImplemented`：
```bash
grep -r "CodeNotImplemented" internal/controller/
```
如果有匹配，手动将桩代码替换为 `return service.Admin().MethodName(ctx, req)`。或者先删除对应控制器文件再重新执行 `gf gen ctrl`。

### 2026-05-28：Data(g.Map{}) 做部分更新导致更新无效

**问题**：租户等级配置的 `UpdateTenantLevelConfig` 使用 `g.Map{}` 收集待更新字段，再调用 `Data(data).Update()`。接口返回成功但数据库数据不变。

**原因**：GoFrame 规范要求数据库操作必须使用 DO 对象，禁止使用 `g.Map`。DO 结构体字段类型为 `interface{}`（`any`），未赋值的字段保持 `nil`，ORM 内置 OmitNil 行为会自动跳过；而已赋值的字段（包括零值 `0`、`""`）会被正常写入。使用 `g.Map` 则可能遇到：键名与列名不匹配被静默忽略、框架行为不一致等问题。

**修复**：将 `g.Map{}` 替换为 `do.TntTenantLevelConfigs{}`，通过指针 nil 检查决定是否赋值 DO 字段，用 `hasUpdate` 标记是否有更新。

**正确做法**：
```go
// 正确 — 使用 DO 对象做部分更新
data := do.XxxTable{}
hasUpdate := false
if req.Name != nil {
    data.Name = *req.Name
    hasUpdate = true
}
if req.Count != nil {
    data.Count = *req.Count  // 即使 *req.Count == 0 也会写入
    hasUpdate = true
}
if !hasUpdate {
    return res, nil
}
dao.XxxTable.Ctx(ctx).Where("id", req.Id).Data(data).Update()

// 错误 — 使用 g.Map
data := g.Map{}
if req.Name != nil { data["name"] = *req.Name }
dao.XxxTable.Ctx(ctx).Where("id", req.Id).Data(data).Update()
```

### 2026-05-30：全项目数据库查询代码批量修复（130+ 处）

**修复内容**：6 类问题，规则已整合到上方 ORM 操作章节：
1. SQL 注入 — `fmt.Sprintf` 拼接 SQL → `?` 参数化（见"SQL 参数化规则"）
2. `Data(g.Map{})` → `Data(do.Xxx{})`（见"g.Map vs DO 结构体"）
3. 写操作错误静默丢弃 `_, _ =`（见"写操作错误处理"）
4. Scan 值类型 → 指针类型（见"Scan 指针类型规则"）
5. `g.DB().Model("table")` → `dao.Xxx.Ctx(ctx)`（见"DAO 模式"）
6. 冗余 `updated_at` 手动设置（见"自动时间维护"）

额外修复：16 处 Scan 改指针后遗漏 `nil` 检查导致 panic（member.go、organization.go、order.go、notification.go、permission.go、channel.go、help_center.go、feature_flag.go）。

### 2026-05-30：MiddlewareHandlerResponse 追加标准响应到文件导出

**问题描述**：模型导出功能下载的 JSON 文件末尾被追加了 `{"code":0,"message":"ok","data":null}`，导致重新导入时报错 `invalid character '{' after top-level value`。

**原因**：Go 的 nil interface 陷阱。`ExportModelsJson` 返回 `(nil, nil)`，但 Controller 将 `(*ModelExportJsonRes)(nil)` 存入 `interface{}` 后，GoFrame 的 `r.GetHandlerResponse()` 判断 `res != nil` 为 true（typed nil pointer ≠ nil），中间件误调用 `response.Success()` 追加了标准响应体。

**修复方式**：
1. **中间件**（`handler_response.go`）：在检查 `GetHandlerResponse()` 前增加两道防线：
   - 检测 `Content-Disposition` 头（handler 直接写入下载文件时设置）
   - 检测 `BufferLength() > 0 && GetHandlerResponse() == nil`（handler 已写响应体且返回值确实为 nil）
2. **导入端**（`model_import_export.go`）：`json.Unmarshal` 改为 `json.NewDecoder(...).Decode()`，只解析第一个完整 JSON 值，兼容已有的脏导出文件。

### 2026-06-03：DO 插入遗漏 NOT NULL 字段导致 INSERT 必然失败

**问题描述**：幂等中间件（`idempotency.go`）将原生 SQL 重构为 DO 对象插入时，遗漏了 `expires_at` 字段：
```go
dao.SysIdempotencyRecords.Ctx(ctx).Data(do.SysIdempotencyRecords{
    IdempotencyKey: idempotencyKey,
    Status:         "processing",
}).Insert()   // 未设置 ExpiresAt
```
而 `sys_idempotency_records.expires_at` 为 `TIMESTAMPTZ NOT NULL` 且无 `DEFAULT`。每次插入都因 NOT NULL 约束失败，代码又把"插入失败"一律当作"并发重复请求"返回 409，导致所有带 `Idempotency-Key` 的请求恒返回 409。

**原因**：GoFrame 的 OmitNil/OmitEmpty 行为——DO 中未赋值的字段为 `nil`，ORM 自动从 INSERT 语句中剔除该列。对于**有 DEFAULT 的列**这是期望行为；但对**无 DEFAULT 的 NOT NULL 列**，剔除后数据库无值可填，INSERT 直接失败。原生 SQL 版本显式写了 `expires_at = NOW() + INTERVAL '24 hours'`，重构时丢失。

**修复方式**：DO 插入时显式赋值 `ExpiresAt: gtime.Now().Add(idempotencyTTL)`。

**正确做法**：将原生 SQL 改写为 DO 插入时，逐列核对目标表中**所有 NOT NULL 且无 DEFAULT 的列**是否都在 DO 中赋了值——这类列不能依赖框架的 OmitNil 自动跳过，否则 INSERT 必然失败。框架自动维护的 `created_at`/`updated_at` 不在此列。

### 2026-06-07：`length` 校验规则对 `[]string` 校验的是 JSON 字符串长度，不是元素个数

**问题描述**：Webhook 创建接口的 Events 字段使用 `v:"required|length:1,50#请选择事件|事件数量不正确"`，用户选了 3-4 个事件后提交报错"事件数量不正确"。

**原因**：GoFrame v2 的 `length:min,max` 校验规则对 `[]string` 类型不是检查 slice 元素个数，而是先将整个 slice 通过 `json.Marshal` 转成 JSON 字符串，再检查该字符串的字符长度。例如 `["member.created","key.deleted"]` 的 JSON 字符串长度约 33 个字符。用户选了 3-4 个事件后 JSON 字符串就可能超过 50 字符，导致校验失败。

**修复方式**：去掉 `length:1,50`，只保留 `required`（事件从固定列表中选择，无需额外长度校验）。

**正确做法**：
```go
// 错误 — length 对 []string 校验的是 JSON 字符串长度，不是元素个数
Events []string `json:"events" v:"required|length:1,50#请选择事件|事件数量不正确"`

// 正确 — 只用 required 保证至少选了一个事件
Events []string `json:"events" v:"required#请选择事件"`

// 如果需要校验每个元素字符串长度，用 foreach|length
Events []string `json:"events" v:"required|foreach|length:1,100#请选择事件|事件名长度不正确"`
```
**注意**：GoFrame v2 没有内置规则直接校验 slice 的元素个数。如需限制元素数量，需自定义校验规则或在 logic 层手动检查。


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

**统一写法：`g.DB().Transaction` 入口 + 闭包内 `dao.Xxx.Ctx(ctx)` 传播式**（GoFrame 官方推荐）。

GoFrame 的 `Transaction` 会把事务对象**注入闭包的 `ctx`**，闭包内任何 `dao.Xxx.Ctx(ctx)` / `g.DB().Ctx(ctx)` 只要用的是这个 `ctx`，就自动挂到当前事务，无需手动持有 `tx` 句柄。

```go
// 正确 — ctx 传播式：闭包内统一用 dao.Xxx.Ctx(ctx)，不碰 tx 句柄
err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
    _, err := dao.TntTenants.Ctx(ctx).Data(do.TntTenants{...}).Insert()
    if err != nil {
        return err // 返回 error 自动回滚；返回 nil 自动提交；panic 也会回滚
    }
    _, err = dao.TntUsers.Ctx(ctx).Data(do.TntUsers{...}).Insert()
    return err
})
```

**强约束（写事务必须遵守）：**

1. **入口统一用 `g.DB().Transaction(ctx, ...)`**，不要用 `dao.Xxx.Transaction(...)`。后者会把事务"挂在某张表"上产生误导（闭包内往往操作的是别的表），且不利于统一。
2. **闭包内一律用 `dao.Xxx.Ctx(ctx)`**（或原生 `g.DB().Ctx(ctx).Exec(...)`），**不使用 `tx` 句柄**。闭包签名 `func(ctx, tx gdb.TX)` 是框架要求，保留不动，但函数体不引用 `tx`（Go 不会因未使用的参数报错）。
3. **`ctx` 必须逐层传递**——这是本写法的**承重纪律**。闭包内调用其它函数时，必须把闭包的 `ctx` 传下去，被调函数内部也必须用 `dao.Xxx.Ctx(ctx)`，事务才会贯穿。
4. **原生 SQL 例外**：钱包自减/`GREATEST` 等 DO 表达不了的算术更新，保留原生 SQL，入口从 `tx.Ctx(ctx).Exec(...)` 换成 `g.DB().Ctx(ctx).Exec(...)`，SQL 与 PostgreSQL 的 `$1/$2` 占位符原样保留。

**⚠️ 头号陷阱——静默脱离事务**：闭包内若漏写 `.Ctx(ctx)`（如 `g.DB().Model("t").Insert()`）或误传了不带事务的 ctx，该语句会**脱离事务被真正提交、无法回滚**，且**没有任何编译/运行时报错**。验证手段：开 SQL 调试日志，事务内每条语句都应带 `[txid:N]`；不带 `txid` 的就是漏挂了。

**嵌套事务/传播行为**：默认传播类型为 `PropagationNested`（用 SavePoint），闭包内再调一个自己开 `Transaction` 的函数会自动成为父事务的嵌套保存点，跟随父事务回滚。若某操作需要"即使主流程回滚也要独立提交"（如审计留痕），必须显式用 `TransactionWithOptions(ctx, gdb.TxOptions{Propagation: gdb.PropagationRequiresNew}, ...)`。

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

### 2026-06-16：gerror.NewCode 误用为格式化构造，错误消息占位符未替换

**问题描述**：注册时命中禁用词，返回的错误消息未格式化，用户看到 `%s包含禁用词「%s」，请修改后重试, 用户名, test`——`%s` 原样输出，参数被用 `, ` 拼接在末尾。

**原因**：`gerror.NewCode(code gcode.Code, text ...string)` 的第二及后续参数是**字面文本**（`...string`），不是 `format + args`。代码误把含 `%s` 占位符的模板连同 `fieldName`、`word` 一起传入，GoFrame 把它们当作多段 text 用 `, ` 拼接，模板原样保留、占位符不替换。带格式化的对应方法是 `gerror.NewCodef(code, format, args...)`。

```go
// 错误 — NewCode 的参数是字面 text（...string），%s 不会被替换，多余参数被拼到末尾
return gerror.NewCode(gcode.New(...), "%s包含禁用词「%s」，请修改后重试", fieldName, word)
// 输出：%s包含禁用词「%s」，请修改后重试, 用户名, test

// 正确 — 用 NewCodef 做格式化
return gerror.NewCodef(gcode.New(...), "%s包含禁用词「%s」，请修改后重试", fieldName, word)
// 输出：用户名包含禁用词「test」，请修改后重试
```

**修复方式**：`internal/logic/common/validation.go` 中 `ValidateForbiddenWords` 的 `gerror.NewCode` 改为 `gerror.NewCodef`。

**正确做法**：构造错误时区分 `NewXxx`（字面文本）与 `NewXxxf`（格式化）两个系列。凡消息含 `%s`/`%v`/`%d` 等占位符且需填入变量，必须用 `f` 变体（`gerror.Newf`、`gerror.NewCodef`、`gerror.NewWrapf`、`gerror.NewSkipf`）；把格式串当字面文本传入 `NewXxx` 系列，占位符不会替换，多余实参还会被拼接到消息末尾。

### 2026-06-23：Scan 查询单标量值（`*string`/`*int`）报参数类型错误

**问题描述**：`organization.go` 的 `GetOrgInfo` 查询租户等级名称时，`var levelName *string; dao.Xxx.Scan(&levelName)` 触发 WARN：`element of parameter "pointer" for function Scan should type of struct/*struct/[]struct/[]*struct`，等级名查不到（降级为空）。

**原因**：GoFrame 的 `Scan` **仅支持 struct / *struct / []struct / []*struct**，不支持标量类型（string/int/bool 等）的指针。`levelName` 声明为 `*string`，`&levelName` 是 `**string`，不在支持范围内，直接报参数类型错误。注意这和 2026-05-14 记录的「Scan 值类型 vs 指针类型」是两回事——那个讲的是 struct 查询无行时的行为差异，本次是 **Scan 根本不接受标量**。

**修复方式**：查询单个标量值改用 `Value()`，返回 `*gvar.Var`，再用 `.String()`/`.Int64()`/`.Bool()` 取值：
```go
// 错误 — Scan 不支持标量类型指针
var levelName *string
dao.Xxx.Ctx(ctx).Where(...).Fields("name").Scan(&levelName)  // 报参数类型错误

// 正确 — 单标量值用 Value()
v, err := dao.Xxx.Ctx(ctx).Where(...).Fields("name").Value()
if err != nil {
    g.Log().Warningf(ctx, "查询失败: %v", err)
} else {
    levelName = v.String()  // 无记录时返回 ""，不会报错
}
```

**正确做法**：按查询意图选择 API：
- 查询单行记录 → `Scan(&structPtr)`（指针类型，无行返回 nil，见 2026-05-14 记录）
- 查询多行记录 → `Scan(&[]struct{})`
- 查询单个标量值（`COUNT`、`SUM`、单个字段）→ `Value()` 返回 `*gvar.Var`，用 `.String()/.Int64()/.Bool()` 取值，**不要用 `Scan(&scalar)` 或 `Scan(&scalarPtr)`**

### 2026-07-16：CAS 更新用「已改写的目标状态」当 WHERE 谓词导致超时兜底网永久失效

**问题**：异步任务超时兜底 `handleTimedOutTasks`（`internal/logic/task/async_polling.go`）把任务标记失败时，先执行 `t.Status = "FAILURE"`，随后又把 `t.Status` 传给 `UpdateTaskCAS(ctx, t, t.Status)` 作为 CAS 的 oldStatus。`GetTimedOutTasks` 只返回非终态行（`status NOT IN ('SUCCESS','FAILURE')`），于是 CAS 生成的 SQL 是 `WHERE id=? AND status='FAILURE' SET status='FAILURE'`，对任何非终态行匹配 0 行 → `RowsAffected()==0` → 返回 CAS conflict → `continue`。结果：**超时任务从未被真正 FAILURE、预扣永不退款（资损）、active 计数永不递减**，且影响所有异步平台（sora/kling/suno/mj/ali/gemini/sync_image）。

**原因**：GoFrame 的「CAS 更新」惯用法 `Where("id", id).Where("status", oldStatus).Update(...)` + `RowsAffected()` 依赖 oldStatus 是**变更前**的真实值。此处在计算 oldStatus 之前就地改写了同一个 `t.Status` 字段，使谓词退化为「新值 == 旧值」，永不成立。

**修复方式**：把「计算 oldStatus + 改写为 FAILURE」抽成纯函数 `buildTimeoutFailure(t, now) (oldStatus string)`，在覆盖前捕获真实状态返回，供 CAS 使用；并补 DB-free 回归单测 `TestBuildTimeoutFailure_UsesOriginalStatusAsCASPredicate` 锁定「oldStatus 必须是覆盖前状态、绝不为 FAILURE」。

**正确做法**：任何 CAS 式条件更新，**先快照旧值再改写**，用快照做 WHERE 谓词：
```go
// 错误 — 先改写再拿改写后的值当谓词，CAS 永不匹配
t.Status = "FAILURE"
dao.Xxx.Ctx(ctx).Where("id", t.ID).Where("status", t.Status).Update(...) // WHERE status='FAILURE'

// 正确 — 先快照旧状态
oldStatus := t.Status
t.Status = "FAILURE"
dao.Xxx.Ctx(ctx).Where("id", t.ID).Where("status", oldStatus).Update(...) // WHERE status='IN_PROGRESS'
```

### 2026-07-24：DO 更新无法置 NULL，且强类型时间字段连 gdb.Raw 也不可用

**问题**：用户解锁接口返回成功，但数据库 `locked_until` 未被清除（仍是锁定时间），账号依旧登不进。同样的写法潜伏在登录成功重置、锁定过期自愈、重置密码顺带清锁，共 9 处。

**原因（两层）**：

1. **DO 更新对 nil 字段的 omit 行为**：GoFrame DO 对象更新时，值为 nil 的字段会被自动跳过、不进入 SET 子句（"部分更新"设计意图，见 2026-05-28）。`Data(do.SysAdminUsers{..., LockedUntil: nil})` 中 `LockedUntil: nil` 被当作"未设置"而非"写 NULL"，列原值保留。`Update` 不报错、affected rows 可能为 0，接口照常返回成功——极其隐蔽。

2. **本项目 DO 是混合类型**（`internal/model/do/sys_admin_users.go` 实测）：
   - `FailedAttempts any` —— 标准接口字段，赋 `0`（非 nil）正常写入
   - `LockedUntil *gtime.Time` —— **强类型时间指针**（非 any），赋 nil 被 omit

**官方推荐与本项目现实的冲突**：

GoFrame 官方（goframe-v2 skill）对"置 NULL"的推荐做法是 DO + `gdb.Raw("NULL")`：
```go
dao.Instances.Ctx(ctx).Where(cols.Id, id).Data(do.Instance{IdleSince: gdb.Raw("NULL")}).Update()
```
但这要求该 DO 字段是 `any`/`interface{}`（才能接受 `gdb.Raw` 这个字符串类型）。**本项目的 `LockedUntil` 是强类型 `*gtime.Time`**，`gdb.Raw("NULL")` 无法赋值，编译报 `cannot use gdb.Raw("NULL") (of type gdb.Raw) as *gtime.Time`。官方推荐的 `gdb.Raw` 路径在本项目此类强类型字段上**不适用**（已实测编译失败）。

**修复**：本项目对强类型 `*gtime.Time` 字段置 NULL，只能用 `map[string]interface{}`（map 不触发 omit，nil 显式写 NULL）：
```go
// 错误 — DO 的 nil 被 omit，locked_until 不会被置 NULL
dao.Xxx.Ctx(ctx).Where("id", id).Data(do.Xxx{
    FailedAttempts: 0,
    LockedUntil:    nil,   // ← 被 omit，列原值保留
}).Update()

// 错误 — 本项目 LockedUntil 是强类型 *gtime.Time，gdb.Raw 类型不匹配，编译失败
dao.Xxx.Ctx(ctx).Where("id", id).Data(do.Xxx{
    LockedUntil: gdb.Raw("NULL"),  // ← cannot use gdb.Raw as *gtime.Time
}).Update()

// 正确（本项目强类型时间字段置 NULL 的唯一可行途径）— map
dao.Xxx.Ctx(ctx).Where("id", id).Data(map[string]interface{}{
    "failed_attempts": 0,
    "locked_until":    nil,   // ← 写入 NULL
}).Update()
```
> 若 DO 字段是 `any`（如本项目的 `FailedAttempts`），官方 `gdb.Raw("NULL")` 可用、DO 方案更合规。本记录的 map 方案**仅针对强类型字段**（`*gtime.Time` 等）置 NULL 这一本项目特定场景；此处为一致起见，`failed_attempts` 也一并用 map。

**影响范围（9 处，本次全修）**：`admin/admin_user.go`（UnlockUser）、`admin/auth.go`（登录成功重置 + 锁定过期自愈）、`admin/member.go`（ResetMemberPassword + UnlockMember）、`tenant/auth.go`（登录成功重置 + 锁定过期自愈）、`tenant/member.go`（UnlockMember）、`tenant/email.go`（邮箱重置密码）。

**正确做法（按字段类型选择）**：
- 普通部分更新（非空字段）→ 用 DO struct（见 2026-05-28）
- 置 NULL/清空**且字段是 `any`/`interface{}`** → DO + `gdb.Raw("NULL")`（GoFrame 官方推荐）
- 置 NULL/清空**且字段是强类型**（本项目 `*gtime.Time`）→ `map[string]interface{}`（`gdb.Raw` 类型不匹配，map 是唯一途径）

排查信号：接口返回成功但数据库某可空字段没变化，且代码用了 `Data(do.Xxx{Field: nil})`。


### 2026-08-03：补偿路径复用已取消的请求 ctx + 资金多语句未用事务，导致冻结余额永久泄漏

**问题**：高并发下客户端超时断开时出现 `pre-deduct db rollback after track failure: ... context canceled`。预扣的冻结 UPDATE 已提交，但追踪 INSERT（`bil_prededuct_tracks`）因 ctx 取消失败，随后的回滚补偿 `unfreezeDBAmount(ctx)` 复用**同一个已取消的 ctx** 也必然失败。最终 `bil_wallets.frozen_balance` 永久多冻结且无追踪记录——孤儿清理与对账均以 tracks 表为权威，这笔泄漏对整个对账体系不可见，租户可用余额被持续蚕食。

**原因（三层）**：

1. **补偿/回滚操作复用请求 ctx**：补偿正是在出错（常为 ctx 取消）时才执行的，用会随客户端断开取消的 ctx 等于没有补偿。GoFrame 的 `g.DB().Exec(ctx)` 在 ctx 取消后直接返回 `context canceled`。
2. **资金多语句操作未包事务**：冻结 UPDATE 与追踪 INSERT 在 autocommit 下各自成独立事务，中间任意点被打断即状态撕裂，只能依赖脆弱的手动补偿。
3. **ctx canceled 被误判为 Redis 故障**：`PreDeduct` 对 Redis 报错不加区分一律降级到 `preDeductDB`，客户端断开（ctx 取消）也会触发降级去打 DB 热点行。

**修复**（`internal/logic/billing/wallet.go`）：

```go
// 错误 — 两条语句独立提交 + 补偿复用请求 ctx
g.DB().Exec(ctx, "UPDATE bil_wallets SET frozen_balance = frozen_balance + ? ...")  // 已提交
trackPreDeduct(ctx, ...)                    // ctx 取消 → 失败
unfreezeDBAmount(ctx, ...)                  // 同一 ctx → 补偿也失败 → 冻结永久泄漏

// 正确 — 资金临界区：单事务原子化 + WithoutCancel 脱离取消 + 独立兜底超时
dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), preDeductDBTimeout)
defer cancel()
err := g.DB().Transaction(dbCtx, func(txCtx context.Context, tx gdb.TX) error {
    // 注意：事务内的裸 SQL 必须用 g.DB().Ctx(txCtx).Exec(txCtx, ...) 传播事务
    if _, err := g.DB().Ctx(txCtx).Exec(txCtx, "UPDATE bil_wallets ..."); err != nil {
        return err
    }
    return trackPreDeduct(txCtx, ...)  // 任一失败整体回滚，无需手动补偿
})
```

同时在 `preDeductDB` 入口检查 `ctx.Err()`：请求 ctx 已取消时直接返回错误，不再降级打 DB。

**正确做法（通用规则）**：

- **一旦决定发生资金状态变更，写入过程必须不可被客户端取消**：临界区统一用 `context.WithoutCancel(ctx)` + 独立超时（防连接异常无限占用）。已有先例：`rollbackRedisPreDeduct`、`IncrMemberQuotaUsed`。
- **关联的多条资金语句必须包进 `g.DB().Transaction`**，靠事务回滚而非手动补偿保证原子性；事务内裸 SQL 用 `g.DB().Ctx(ctx).Exec(ctx, ...)`（与 `executeSettlementTx` 一致），`g.DB().Exec(ctx, ...)` 不带 `.Ctx()` 不会加入 ctx 携带的事务。
- **降级分支先判 `ctx.Err()`**：`context canceled` 不是基础设施故障，不应触发降级重试。

排查信号：日志同时出现「主操作 context canceled」和「rollback/补偿也 context canceled」，且资金字段与明细表对不上。

### 2026-08-03（二）：进程级全局状态回源使用请求 ctx + 负结果不缓存，客户端断开时刷出 DB 报错风暴

**问题**：高并发客户端断开时出现大量 `[Config] DB error reading option tenant_qps_limit: ... context canceled`。

**原因（三层叠加，`ConfigService.GetOption`）**：

1. **全局配置回源用请求 ctx**：配置缓存是进程级共享状态，回填 DB 查询却随单个请求的 ctx 取消而中断——客户端断开既刷误报，又导致缓存填不上、后续请求继续穿透。
2. **未落库的 key 不缓存（无负缓存）**：`option == nil` 走注册表默认值分支时不写缓存，导致该 key **每次调用都穿透查 `sys_options`**。`LoadRateLimitConfig` 每请求读 5 个 key（`CheckRateLimit`/`AcquireConcurrent` 各调一次），只要这些 key 没在后台配置过，就是每请求最多 10 次 DB 查询的读风暴。
3. **无 singleflight**：缓存过期瞬间所有并发请求同时回源。

**修复**（`internal/logic/common/config.go`）：回源包进 `singleflight.Do(key, ...)`，闭包内用 `context.WithoutCancel(ctx)` 先双重检查缓存再查 DB；DB 有行缓存行值，无行缓存注册表默认值（负缓存）。缓存默认值不影响改配置生效——`SetOption` 写库后 `cache.Delete` + Pub/Sub 广播失效。

**正确做法（通用规则）**：

- **进程级共享状态（配置、字典、全局缓存）的回源查询，一律 `context.WithoutCancel(ctx)`**：回填结果服务于所有请求，不属于发起请求的生命周期。请求私有数据（该请求自己的钱包/Key 查询）才随请求 ctx 取消。
- **回退到默认值的分支也要写缓存（负缓存）**，否则"未配置"成为永久缓存穿透源；前提是写路径有对应的缓存失效。
- **同 key 并发回源加 `singleflight`**（项目已有先例：`walletSyncGroup`）。

排查信号：某张小配置表的 SELECT 出现在高频请求路径日志中（尤其带 context canceled），说明缓存没挡住——检查负结果是否被缓存、回源是否用了请求 ctx。

### 2026-08-03（三）：客户端断开后请求继续空转流水线，ctx 取消被逐级误报为业务错误

**问题**：高并发客户端断开时连环出现三类告警——`api_key_quota: load failed ... context canceled, skipping check`、`member_quota: load failed ... skipping check`、`[RelayHandler] No available channel for model=...`。

**原因**：请求 ctx 取消后流水线不终止，每个阶段各自撞上 canceled ctx 并按本阶段的错误语义误报：额度检查把取消当 DB 故障告警并 fail-open"跳过"；调度器的 Redis 操作（快照/租约/绑定）被取消导致空决策，被误报成"无可用渠道"，还会记失败用量、污染渠道统计。

**修复**：

1. `RelayHandler` 入口 `ctx.Err()` fail-fast——断开的请求直接终止，不进入流水线；
2. 调度空决策（`sess.Next` 返回 nil）与 `CheckTenantModelAccess` 出错处识别 `context.Canceled/DeadlineExceeded`：静默退还预扣（`WithoutCancel`）后返回，不记失败用量、不打无可用渠道告警（对齐既有的 `MaterializeSelection` 断开处理）；
3. `CheckMemberQuota`/`CheckApiKeyQuota` 加载失败时区分取消与真实 DB 故障：取消静默跳过（请求随后在预扣处 fail-fast），仅真实故障才 WARN。

**正确做法（通用规则，与本日（一）（二）合并成完整的 ctx 取消处理决策表）**：

| 操作性质 | ctx 取消时的正确行为 |
|---------|---------------------|
| 请求私有读（本请求的额度/Key/权限查询） | 随请求终止；**不告警**（取消不是故障），必要时 Debug |
| 进程级共享状态回源（配置/字典缓存回填） | `WithoutCancel` 继续完成（见（二）） |
| 资金状态变更（预扣/结算/解冻及其补偿） | `WithoutCancel` + 事务 + 兜底超时，不可撕裂（见（一）） |
| 长流水线编排（relay handler） | 入口与各昂贵阶段前 `ctx.Err()` fail-fast；下游报错先判 `errors.Is(err, context.Canceled)`，勿映射为业务错误（"无可用渠道"/"余额不足"等） |

排查信号：同一 request_id 在日志里留下一串不同模块、不同语义的报错，尾部都是 `context canceled`——说明缺 fail-fast 入口闸门，且各模块把取消误当成了自己领域的故障。

### 2026-08-14（五）：更新接口非指针 int 字段被零值无条件覆写（渠道 weight/priority 被洗成 0）

**问题**：渠道创建后数据库 `chn_channels.weight` 是 0，但创建表单明明提交了 100。前端"仅切换状态"或"仅更新 API Key"的 PUT 请求会把已有渠道的 weight/priority 洗成 0。

**原因**：`ChannelUpdateReq.Weight/Priority` 是非指针 `int`，JSON 里不带该字段时解析为 Go 零值 0，无法区分"未提交"与"显式提交 0"；而 `UpdateChannel` 里 `data.Weight = req.Weight` 无条件写入，`do` 结构体的非 nil 字段（含 0）都会进 UPDATE SET 列表，于是 0 覆盖了库里的 100。前端曾在多处用"提交时带上当前 weight"打补丁，但详情页的状态切换/Key 更新漏带，且编辑表单 `weight || 100` 的 falsy 兜底把数据损坏掩盖成"显示正常"。

**修复**：

1. `ChannelUpdateReq.Weight/Priority` 改为 `*int`（与同结构体 `MaxConcurrency`、`Type` 等"留空不更新"字段风格一致），Logic 里仅在非 nil 时赋值；
2. 删除前端"必须携带当前 priority/weight"的 workaround（ChannelsPage 的状态切换/行内权重/行内层级三处），只提交变更字段；
3. 详情页编辑表单去掉 `detail.weight || 100` 兜底，显示真实库值，避免再次掩盖数据异常。

**正确做法（通用规则）**：

- **更新接口的"可选更新"字段一律用指针类型**（`*int`/`*string`/`*bool`），`nil` = 不更新，非 nil（含零值）= 显式更新；非指针字段意味着"调用方必须每次全量携带"，要在 API 注释里写明并让所有前端调用点遵守。
- **do 结构体字段是 `interface{}`**：nil 才会被 ORM 过滤，非 nil 的 0/""/false 都会写入——"零值是否合法的业务值"必须在 Req 层用指针区分，不能指望 ORM 帮你跳过。
- **前端编辑表单慎用 `x || 默认值` 兜底**：对数值字段，0 会被 falsy 规则吞掉，既掩盖真实数据，又可能在保存时把合法的 0 改写为默认值。

排查信号：某字段在库里莫名变成 0/空串，而创建时明明写过合法值——查该表所有 UPDATE 路径中是否存在非指针字段的无条件赋值。

### 2026-08-16：httptest.NewServer 直接包裹未 Start 的 g.Server，每个请求都在 Session.Close() panic

**问题**：`go test -v` 下凡是用 `httptest.NewServer(g.Server(guid.S()))` 方式测 HTTP 处理逻辑的用例，每个请求都向 stderr 刷一段 `http: panic serving ... nil pointer dereference`（`gsession.(*Session).Close`）。测试仍然 PASS——响应在 panic 前已完整写出、panic 发生在 deferred `handleAfterRequestDone` 中且被 net/http recover——导致问题长期被掩盖（不带 `-v` 时 `go test` 丢弃通过用例的输出）。涉及 `internal/middleware`、`internal/response`、`internal/handler/setup` 三处。

**原因**：gf v2.10.2 只在 `Server.Start()` 中初始化 `sessionManager`（ghttp_server.go），而 `ServeHTTP` 对每个请求无条件执行 `request.Session.Close()`，后者第一行就解引用 `s.manager.storage`。未经 `Start()` 的 server 其 `sessionManager` 为 nil → `Session.manager` 为 nil → 必 panic。无导出 API 可以单独初始化 manager，绕不开。

**修复**：新增共享测试辅助 `internal/testutil.StartGFServer(t, s)`——把 server 绑定到 `127.0.0.1:0`（内核随机分配端口避免冲突，绑定回环地址避免 Windows 防火墙弹窗）走真实 `s.Start()`，返回 `http://127.0.0.1:{port}` 供用例直接请求，`t.Cleanup` 注册 `s.Shutdown()`。`Start()` 返回时 listener 已创建完毕（`startServer` 内 `wg.Wait()` 等待 `CreateListener` 完成后才返回），`GetListenedPort()` 立即可用，无需轮询。三处测试全部改用该辅助。

**正确做法**：测试中需要真实走 ghttp 请求链路（中间件、handler、hook）时，一律用 `testutil.StartGFServer(t, s)` 启动真实 server；**禁止**把 `g.Server` 直接塞进 `httptest.NewServer`——那只会执行 `ServeHTTP` 路径而不执行 `Start()` 的初始化（session manager、graceful listener 等），属于未定义用法。`httptest.NewServer` 仅用于包装自建的普通 `http.Handler`。

排查信号：`go test -v` 输出中出现成段的 `http: panic serving` + `gsession.(*Session).Close` 栈。

### 2026-08-14（五）：更新接口非指针 int 字段被零值无条件覆写（渠道 weight/priority 被洗成 0）

**问题**：渠道创建后数据库 `chn_channels.weight` 是 0，但创建表单明明提交了 100。前端"仅切换状态"或"仅更新 API Key"的 PUT 请求会把已有渠道的 weight/priority 洗成 0。

**原因**：`ChannelUpdateReq.Weight/Priority` 是非指针 `int`，JSON 里不带该字段时解析为 Go 零值 0，无法区分"未提交"与"显式提交 0"；而 `UpdateChannel` 里 `data.Weight = req.Weight` 无条件写入，`do` 结构体的非 nil 字段（含 0）都会进 UPDATE SET 列表，于是 0 覆盖了库里的 100。前端曾在多处用"提交时带上当前 weight"打补丁，但详情页的状态切换/Key 更新漏带，且编辑表单 `weight || 100` 的 falsy 兜底把数据损坏掩盖成"显示正常"。

**修复**：

1. `ChannelUpdateReq.Weight/Priority` 改为 `*int`（与同结构体 `MaxConcurrency`、`Type` 等"留空不更新"字段风格一致），Logic 里仅在非 nil 时赋值；
2. 删除前端"必须携带当前 priority/weight"的 workaround（ChannelsPage 的状态切换/行内权重/行内层级三处），只提交变更字段；
3. 详情页编辑表单去掉 `detail.weight || 100` 兜底，显示真实库值，避免再次掩盖数据异常。

**正确做法（通用规则）**：

- **更新接口的"可选更新"字段一律用指针类型**（`*int`/`*string`/`*bool`），`nil` = 不更新，非 nil（含零值）= 显式更新；非指针字段意味着"调用方必须每次全量携带"，要在 API 注释里写明并让所有前端调用点遵守。
- **do 结构体字段是 `interface{}`**：nil 才会被 ORM 过滤，非 nil 的 0/""/false 都会写入——"零值是否合法的业务值"必须在 Req 层用指针区分，不能指望 ORM 帮你跳过。
- **前端编辑表单慎用 `x || 默认值` 兜底**：对数值字段，0 会被 falsy 规则吞掉，既掩盖真实数据，又可能在保存时把合法的 0 改写为默认值。

排查信号：某字段在库里莫名变成 0/空串，而创建时明明写过合法值——查该表所有 UPDATE 路径中是否存在非指针字段的无条件赋值。

### 2026-08-20（三）：Fields 挂在共用模型上导致 COUNT(col1, col2, ...) 报错

**问题**：`TenantApiKeySelect` 中把 `.Fields("id, name, ...")` 挂在模型链上后调用 `m.Count()`，PostgreSQL 报 `function count(bigint, character varying, ...) does not exist`——gdb 生成了 `SELECT COUNT("id","name",...)` 而不是 `COUNT(*)`。

**原因**：gdb 的 `Count()` 会复用模型上已设置的 `Fields` 作为 COUNT 的参数列表（多字段时逐个传入 count 聚合函数），而 PostgreSQL 的 `count` 只接受单个参数（MySQL 语义下多参 count 也不等于总行数）。这是"链式复用同一 model 做计数 + 查询"模式下的隐蔽陷阱：查列表正常、计数 SQL 被悄悄改写。

**修复**：`.Fields(...)` 只挂在分页查询链上（`m.Fields(...).Page(...).Scan(...)`），`Count()` 用不带 Fields 的基础模型（生成 `COUNT(*)`）。

**正确做法**：凡"先 Count 再分页 Scan"的写法，构建条件模型（Where/Order）后**先 Count**，再把 `Fields/Page/Scan` 链在同一个基础模型上；不要在 Count 之前设置 Fields。若必须提前设置 Fields，用 `m.Clone()` 或为 Count 单独构建条件链。

排查信号：日志出现 `function count(... N 个参数 ...) does not exist` 或 `COUNT("a","b")` 形态的 SQL，且调用栈落在 `*.Count()`。

### 2026-08-20（三）：Model.Scan(&string) 静默吞错，局部更新把未提交的 JSONB 字段重置为默认值

**问题**：渠道调试日志「保存捕捉条件后开关自动关闭」。PUT 只带 `debug_log_tenant_id` 等过滤字段，settings JSONB 里的 `debug_log_enabled:true` 被静默丢弃。查库发现过滤字段写入成功、唯独开关标志消失。

**原因**：读旧 settings 用的是 `_ = dao.ChnChannels...Fields("settings").Scan(&currentSettings)`——gdb 的 `Model.Scan` **只接受 struct/*struct/[]struct/[]*struct**，传入 `*string` 直接返回错误 `element of parameter "pointer" for function Scan should type of struct/...`；`_ =` 把错误吞掉后 `currentSettings` 永远是空串，`ParseChannelSettings("")` 返回默认值，套上本次请求的字段 marshal 回写——**所有未在本次请求中显式提交的 settings 字段全部被重置为默认**。开关单独提交时显式带上了 enabled 所以正常；过滤条件单独提交时没带 enabled，于是被重置为 false。这是个从原 UseProxy 更新块继承来的潜伏 bug：此前每次切换 use_proxy 同样会把 timeout_seconds 覆盖、header_override 等自定义配置静默洗掉，只是显式字段每次都带、无人察觉。同模式的 `GetTenantAuditLevel`（audit.go，只读路径）也中招——租户级审计设置因此**从未生效过**。

**修复**：读单列字符串一律用 `Model.Value()`（返回 `*gvar.Var`，`v.String()` 取值），两处（channel.go 更新块、audit.go）均已改为 Value()。

**正确做法（通用规则）**：

- **读单个标量列用 `.Value()` 或 `.Array()`/`.All()` 后取字段，禁止 `Scan(&string)`/`Scan(&int)`**——Scan 只认 struct 系列，对基础类型指针直接报错。
- **禁止 `_ =` 吞掉 DB 读取错误**，尤其是「读旧值 → 改字段 → 写回」的合并更新模式：读失败意味着回写会把整个 JSONB 洗成默认值，属于数据破坏而非可忽略的小错。
- 「读-改-写」JSONB 配置的合并更新块，读失败时应中止更新并返回错误，而不是带着默认值继续。

排查信号：JSONB 配置列中「本次没提交的字段莫名回到默认值」；`grep 'Scan(&' `命中基础类型指针。

### 2026-08-26（二）：gf redis 驱动 HMGET 缺失字段返回空字符串而非 nil，IsNil() 判缺失失效

**问题**：渠道健康度一段时间后自动掉到个位数。排查发现健康快照（`runMaintenance` → `ReadRuntime` 平均渠道下所有模型的 succ EWMA）把**无健康键的模型读成 succ=0** 平均了进去；健康键 TTL 24h，无流量模型的键过期后即变成 0，渠道健康度随键过期逐渐坍缩到个位数。

**原因**：`g.Redis().Do(ctx, "HMGET", key, "f1", "f2")` 对**不存在的 key/字段**，经 gf redis contrib 驱动转换后返回的是 `[]string{"", ""}`（空字符串），`.Vars()` 后每个元素 **`IsNil()==false`**、`Float64()==0`。`ReadRuntime` 用 `!vals[0].IsNil()` 判「有数据才覆盖乐观默认值 1」，判断永远为真 → 默认值被 0 覆盖。调度打分侧 `healthFactor` 恰好有 `succ<=0 && lat==0 视为满分` 的兜底所以转发不受影响，但快照聚合没有兜底，展示值被拖垮。`readBreaker` 同模式（空串 state 恰好 Int()==0==CLOSED 侥幸正确）。

**修复**：`internal/dispatchadapter/state_redis.go` `ReadRuntime`/`readBreaker` 改为 `vals[i].String() != ""` 判有无数据；空串一律按无数据处理保留默认值。回归测试 `TestMissingHealthKey_ReadsDefault`。

**正确做法（通用规则）**：判断 gf redis `HMGET`/批量读的字段是否存在，**不要依赖 `IsNil()`**（驱动转换后 nil 会变空字符串）；用 `String() != ""` 判断，或改用 Lua 脚本内 `HGET ... or '默认值'` 在 Redis 侧兜底。单 key `HGET` 缺失返回的是真 nil（`IsNil()==true`），两者行为不一致，混用时尤其注意。

### 2026-08-27：Redis HMGET 缺失字段的 gvar 判空失效（`IsNil()` 恒为 false），默认值兜底全线失守

**问题**：渠道探测一切正常（Redis succ_ewma=1.0），管理后台渠道健康度却长期停在 25，其余渠道全是 0。排查发现维护快照读到的几乎所有「从未被真实流量访问过」的渠道×模型健康值都是 `0.000`，而这些模型的 Redis key **根本不存在**——按设计应回落到默认满分 1.0。

**原因**：`RedisState.ReadRuntime` 用如下写法读健康 EWMA：

```go
v, _ := g.Redis().Do(ctx, "HMGET", key, "succ_ewma", "lat_ewma")
vals := v.Vars()
if !vals[0].IsNil() { out.SuccEwma = vals[0].Float64() }  // ← 判空失效
```

HMGET 对不存在的 key/字段返回 Redis nil，但 gredis 把它们包装成**非 nil 的空 gvar**：实测 `IsNil()=false`、`IsEmpty()=true`、`String()=""`、`Float64()=0`。于是判空分支永远进入，`Float64("")` 静默返回 **0**，把「无数据」变成了「健康分 0 分」。影响远超展示：`ReadRuntime` 同时供调度目录构建 `healthFactor`，所有冷模型的路由权重被压成 0。

两个读取方对同一份「无数据」表现还不一致，极大增加了排查难度——管理后台 `batchGetModelHealth` 用 `fmt.Sscanf` 解析，解析失败就不赋值，UI 显示"无数据"；维护循环用 gconv 读成 0。同一个模型，UI 说无数据、健康分算 0。

**修复**：`state_redis.go` 的 `ReadRuntime` 改为显式字符串校验——`TrimSpace(String())` 为空则保持默认值，非空再 `strconv.ParseFloat` 并做值域校验（succ 取 `0 < f <= 1`；EWMA 由正数衰减而来，数学上不会精确到达 0，精确 0 视为异常数据）。回归测试 `TestReadRuntime脏数据防御`。

**正确做法（通用规则）**：

- **禁止用 `gvar.IsNil()` 判断 Redis 读取结果是否存在**——gredis 的空回复是非 nil 空 gvar，`IsNil()` 恒为 false。用 `IsEmpty()`，或取 `String()` 后判空串。
- **禁止把 `gvar.Float64()`/`Int()` 直接用于「缺失应回落默认值」的场景**：gconv 对空串/非法值静默返回零值，会把「无数据」变成「0 分/0 次」这类**具有强业务含义的极端值**。默认值不是 0 的字段（健康分、成功率、权重、配额），必须显式校验后再赋值。
- 同一份状态有多个读取方时（调度用 gconv、管理后台用 Sscanf），**解析方式必须统一**，否则同一数据在不同页面表现不同，故障期误导排查方向。

排查信号：某项指标「本该回落默认值却全是 0」；同一数据 UI 显示"无数据"而后台计算按 0 处理；`grep -n 'IsNil()' ` 命中 Redis 读取路径。

### 2026-08-31：`g.Cfg().MustGet` 读取不存在的配置段静默返回空值，告警邮件长期发送失败

**问题**：监控告警的邮件通知从上线起就没成功发出过，`ntf_send_log` 里堆满 `status=failed` 的记录，错误信息是 SMTP 连接失败——但管理后台「系统设置 → 邮件配置」里 SMTP 配置完全正常，验证码等其他邮件都能正常投递。

**原因**：`internal/logic/monitor/alert_notify.go` 的 `sendAlertEmailToAdmins` 自己拼 `EmailConfig`，读的是配置文件：

```go
sender := common.NewEmailSender(&common.EmailConfig{
    Host: g.Cfg().MustGet(ctx, "email.smtp.host").String(),   // ← 配置文件里根本没有 email 段
    Port: g.Cfg().MustGet(ctx, "email.smtp.port").Int(),
    ...
})
```

`manifest/config/config.yaml` 中并不存在 `email` 段。`MustGet` 的 `Must` 只针对**读取过程出错**（配置文件损坏等）才 panic，**「键不存在」不是错误**：它返回一个空 `*gvar.Var`，`.String()` 得到 `""`、`.Int()` 得到 `0`。于是 Host 为空、Port 为 0，每封告警邮件都要走满 3 次重试才失败，而且失败原因看起来像"SMTP 服务器有问题"，与真正的原因（配置压根没读到）南辕北辙。

**修复**：改为与其余三条发信链路一致，走设置注册表 `common.EmailConfigFromOptions(ctx)`（数据源 `sys_options`），配置缺失时返回明确错误并直接跳过发送。至此邮件配置的唯一来源是数据库，配置文件不再参与。

**正确做法（通用规则）**：

- **`MustGet` 不保证键存在**——它只保证读取过程无错。对必填配置要显式判空并给出可读错误，不要让空值一路流进业务逻辑变成"连不上/额度为 0/超时 0 秒"这类误导性症状。
- **运行期可由管理员调整的配置一律走设置注册表 `common.Config()`（`sys_options`），不要放配置文件**：配置文件只承载启动期基础设施配置（数据库、Redis、监听端口）。两套来源并存时，改了后台却不生效、或某条链路读到空配置，是最难排查的一类问题。
- 同一份配置有多个消费方时，**必须收敛到同一个加载函数**（本例为 `EmailConfigFromOptions`），禁止各处自行拼装配置结构体。

排查信号：某个功能"配置明明填了却不生效"，而同类功能正常；`grep -rn "g.Cfg()" ` 命中的键在 `manifest/config/*.yaml` 中搜不到。

### 2026-09-01：测试里用 `gtest.DataPath()` 当 server 名，Windows 下 `Start()` 静默失败

**问题**：`internal/middleware` 的 `request_id_test.go` 三个用例在 Windows 上全部失败，断言形如 `EXPECT 0 == 10`、`EXPECT 0 == 100`，或客户端报 `EOF`——看起来像中间件没生成 RequestId，实则请求根本没发出去。

**原因**：测试用路径当服务器名：

```go
s := g.Server(gtest.DataPath("request-id-test"))   // 名字是 D:\...\testdata\request-id-test
s.Start()                                          // 返回值被丢弃
```

GoFrame 用**服务器名**拼 session 存储目录：`%TEMP%\gsessions\<server-name>`。名字里带盘符冒号时路径非法，`os.MkdirAll` 失败，`Start()` 返回 error——但测试没接这个返回值，于是服务器没监听，`GetListenedPort()` 返回 `-1`，客户端请求 `http://127.0.0.1:-1` 直接 EOF。Linux 下 `gtest.DataPath` 返回的路径不含冒号，同样的代码能跑通，所以问题只在 Windows 暴露。

同一文件还继承了 `manifest/config/config.yaml` 的 `server.address: :18888`（真实应用端口），即便名字合法也会与本机运行中的服务抢端口。

**修复**：服务器名改用普通标识符，并显式绑定空闲端口：

```go
s := g.Server("mw-request-id-dual")   // 纯名字，不是路径
s.SetAddr("127.0.0.1:0")             // 不继承配置文件端口，让系统分配
s.SetDumpRouterMap(false)
s.Start()
```

**正确做法（通用规则）**：

- **`g.Server(name)` 的参数是「服务器名」不是「路径」**：它会参与 session 目录等文件路径拼接，只能用普通标识符（字母/数字/连字符）。`gtest.DataPath()` 是用来定位测试数据文件的，不要拿来当名字。
- **测试里起 server 必须 `SetAddr("127.0.0.1:0")`**：`g.Server()` 会读取项目配置，默认继承生产监听端口，在本机跑着服务时必然冲突。
- **不要丢弃 `s.Start()` 的 error**：Start 失败后测试仍会继续跑，症状表现为断言数值不对或 EOF，与真正原因（服务器没起来）完全无关，极易误导排查方向。

排查信号：测试断言"期望 N 实际 0"且伴随 `EOF`；`GetListenedPort()` 返回 `-1`；同一测试在 Linux CI 通过但本地 Windows 失败。

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

```go
// 查询
var user entity.TntUsers
err := dao.TntUsers.Ctx(ctx).Where("id", id).Where("tenant_id", tenantID).Scan(&user)

// 插入（使用 DO 结构体）
id, err := dao.ChnChannels.Ctx(ctx).InsertAndGetId(do.ChnChannels{
    Name:     req.Name,
    Provider: req.Provider,
})

// 更新（使用 DO 结构体）
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

### 原生 SQL（仅用于复杂查询和批量操作）

```go
// 聚合查询、分析报表等无法用 ORM 表达的场景
result, err := g.DB().Ctx(ctx).Raw(`
    SELECT date_trunc('hour', created_at) AS hour, count(*)
    FROM aud_request_logs WHERE ...
`, args...).All()

// 原子更新（如余额操作）
_, err = g.DB().Exec(ctx,
    `UPDATE bil_wallets SET balance = balance - $1 WHERE id = $2 AND balance >= $1`,
    amount, walletID,
)
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

- **新增/全量更新**：优先使用 `do.XxxTable{}` 结构体，类型安全
- **部分更新**：可使用 `g.Map{"field": value}`，灵活但需注意字段名拼写
- **混用场景**：changelog 等直接操作表名的场景可用 `g.DB().Model("table").Data(g.Map{...})`

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

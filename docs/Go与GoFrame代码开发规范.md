# Go 与 GoFrame 代码开发规范

> **适用范围**：所有 Go 后端项目（GoFrame v2 框架）。
> **参考标准**：Effective Go、Go Code Review Comments、Google Go Style Guide、Uber Go Style Guide、《代码整洁之道》（Robert C. Martin）、GoFrame v2 官方文档。
> **约定用词**：【必须】强制遵守，违反不允许合入；【建议】应当遵守，例外需说明理由；【禁止】不允许出现。

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-11-07 | 初版 |

---

## 目录

1. [总纲：代码简洁之道的 Go 落地](#1-总纲代码简洁之道的-go-落地)
2. [Go 语言规范](#2-go-语言规范)
3. [GoFrame v2 框架规范](#3-goframe-v2-框架规范)
4. [测试规范](#4-测试规范)
5. [安全规范](#5-安全规范)
6. [工程化与协作](#6-工程化与协作)
7. [附录 A：Do / Don't 速查表](#附录-ado--dont-速查表)
8. [附录 B：golangci-lint 参考配置](#附录-bgolangci-lint-参考配置)

---

## 1. 总纲：代码简洁之道的 Go 落地

### 1.1 核心原则

1. **代码是写给人看的，只是恰好能在机器上运行。** 评审的黄金标准：一个不了解这块业务的新人，能否在 5 分钟内看懂这个函数在做什么。
2. **童子军军规**：每次提交让代码比来时更干净。发现坏味道随手修，不留给"以后重构"。
3. **命名即文档**：名字要说清楚它是什么、做什么。名字取不好，说明你还没理解这个东西。
4. **函数只做一件事，只在一个抽象层级**：要么是业务流程编排，要么是具体步骤实现，不混写。
5. **错误处理是业务逻辑的一部分**，不是补丁。设计函数时先想清楚它的失败模式。
6. **注释解释 Why，不解释 What**：好的代码自己会说话，注释负责记录代码说不出来的东西——决策原因、约束、踩过的坑。
7. **防止过度设计（YAGNI）**：不为"将来可能需要"写代码。同样的重复出现 3 次以内不抽象——**错误的抽象比重复代码更昂贵**。
8. **一致性强于聪明**：整个代码库用同一种方式做同一件事。花哨的技巧不如朴素的规律。

### 1.2 命名规范

#### 1.2.1 基本规则

| 对象 | 规则 | 示例 |
|------|------|------|
| 包名 | 全小写、短、单数、名词，不用下划线/复数 | `encrypt`、`billing` |
| 文件名 | snake_case | `wallet_service.go` |
| 导出标识符 | 大驼峰 PascalCase | `UserService` |
| 非导出标识符 | 小驼峰 camelCase | `maxRetryCount` |
| 缩写词 | 整词全大写，保持全局一致（新代码统一 `ID`/`URL`） | `userID`、`apiURL`、`serveHTTP` |

> 缩写存量说明：存量代码中 `Id`（198+ 处）已成事实惯例，全量改动破坏面大且 JSON 契约不受影响（`json:"id"`）。
> 约定：**新代码一律 `ID`**，触碰存量文件时顺手改（童子军军规），不做一次性全量替换（2026-11 审计 §4.8 决策）。
| 单方法接口 | -er 后缀 | `Reader`、`Validator` |
| 哨兵错误 | `Err` 前缀，包级导出 | `var ErrNotFound` |
| 错误码变量 | `Code` 前缀 | `CodeUserBanned` |
| 常量 | 驼峰（不用 SCREAMING_SNAKE） | `maxIdleConns` |

- 【必须】驼峰命名，禁止下划线（`_test.go` 等语言机制除外）。
- 【必须】**同一概念全代码库只用同一个词**。fetch / get / query / load / retrieve 选一个；manager / handler / controller / service 在各自语义层内用对，不混用。
- 【禁止】无意义的信息噪声命名：`data2`、`strTmp`、`flag1`、`aUser`。
- 【禁止】匈牙利式编码前缀：`sName`、`bActive`、`iCount`。
- 【禁止】"垃圾抽屉"包名：`util`、`common`、`misc`、`helper`、`shared`。包名应当描述具体职责——放加密的就叫 `encrypt`，放幂等的就叫 `idempotency`。发现 util 包膨胀，按职责拆分。
- 【必须】`ctx context.Context` 参数名固定为 `ctx`。
- 【建议】Getter 不加 Get 前缀（`user.Name()`），Setter 用 Set 前缀（`user.SetName()`），与标准库一致。

#### 1.2.2 示例

```go
// ❌ 反例：缩写随意、含义模糊、命名含噪声
func getUsrData2(u *UsrInfo) []byte { ... }

// ✅ 正例：动作明确、对象明确
func fetchUserOrders(ctx context.Context, user *entity.User) ([]*entity.Order, error) { ... }
```

### 1.3 函数规范

- 【必须】单一职责、单一抽象层级。函数名是动词短语；返回布尔的函数用 Is/Has/Can 开头。
- 【建议】函数体不超过 50 行（经验值，硬上限 100 行）。超过时先尝试按业务步骤拆出私有方法，而不是调大阈值。
- 【建议】参数不超过 3 个。更多时聚合为参数结构体；同质可选参数用 Functional Options 模式。
- 【禁止】布尔旗标参数控制行为（`render(true)`——true 是什么意思？）。拆成两个语义明确的函数。
- 【必须】卫语句早返回，消灭箭头型嵌套。嵌套不超过 3 层，超过用提前 return / continue / 提取函数压平。
- 【禁止】输出参数（传入 slice/map 让函数填充后使用）。要输出就作为返回值返回。
- 【必须】函数无隐藏副作用。名字里只写它做的事，"顺便"做的事要么删掉、要么写进名字。

```go
// ❌ 反例：嵌套箭头 + 混杂层级
func (s *sOrder) Pay(ctx context.Context, orderID int64) error {
	order, err := dao.Order.Ctx(ctx).Where("id", orderID).One()
	if err == nil {
		if order != nil {
			if order["status"].Int() == StatusPending {
				// ... 30 行支付逻辑
				return nil
			}
		}
	}
	return err
}

// ✅ 正例：卫语句早返回，一层一职责
func (s *sOrder) Pay(ctx context.Context, orderID int64) error {
	order, err := s.getByID(ctx, orderID)              // 数据访问：下层
	if err != nil {
		return err
	}
	if order.Status != StatusPending {                  // 业务规则：本层
		return gerror.NewCode(CodeOrderNotPayable, "订单当前状态不可支付")
	}
	return s.doPay(ctx, order)                          // 支付执行：下层
}
```

### 1.4 注释规范

- 【必须】导出的类型、函数、变量必须有 godoc 格式注释：以标识符名字开头、完整句子，说明用途。参数、返回值、错误分支里有非显然语义时写明。
- 【必须】注释统一使用中文（团队约定），专有技术名词保留英文原文。
- 【必须】注释记录**决策与约束**：为什么这么做、有什么坑、改这里要注意什么。不翻译代码。
- 【禁止】注释掉的代码（git 有历史，删掉就是）。
- 【禁止】用注释补偿失败的命名——去改名。
- 【必须】TODO 格式统一为 `// TODO(负责人): 说明`，完成即删除，不带尸横遍野的陈年 TODO。
- 【必须】包注释（`package xxx ...`）写在包的主文件或 `doc.go`。

```go
// SaveWallet 更新钱包余额并写入流水。
// 提交点为 Redis Lua 脚本，DB 仅滞后物化，事务回滚时必须调用 Reverse 补偿逆转，
// 否则会出现 Redis 与账本不一致。
func (s *sWallet) SaveWallet(ctx context.Context, in WalletInput) error { ... }
```

#### 1.4.1 结构体字段注释（写不写在字段上，看信息增量）

字段上方的注释不是坏味道——godoc 会把它渲染成 API 文档的一部分，它是**契约文档**而不是代码补丁。要审视的从来不是"写在字段上"，而是"有没有信息增量"。

**判定标准（删除测试）**：删掉这条注释，调用方会不会少知道些什么？会 → 留；不会 → 删。

**【必须】写的场景**（名字和类型说不出来的信息）：

```go
type ChannelItem struct {
	BreakerState   int    `json:"breaker_state"`   // 调度熔断状态：0=正常 1=熔断中 2=半开探活
	MaxConcurrency int    `json:"max_concurrency"` // 0=不限，按上游 429 水位自动估算
	Balance        int64  `json:"balance"`         // 钱包余额，单位 micro-USD
	PluginKey      string `json:"plugin_key"`      // 仅 type=43（JS 任务插件）渠道有效
}
```

- 枚举 / 状态取值域（`0=正常 1=熔断中 2=半开探活`）
- 零值 / 空值语义（`0=不限`、`留空=不更新`）
- 单位（micro-USD、毫秒、分）
- 条件适用（仅某类型渠道有效）
- 坑与约束（为什么这个字段是字符串、什么场景必须传）

**【禁止】**：

- 翻译式注释：`Name string // 渠道名称`——字段名本身就是这句话，注释只会在字段改名后过期变谣言。
- 同一内容在 `dc:` tag 和行注释里写两遍，选一处。
- 版本编年史：`// v0.2.3 新增字段`——字段何时来的是 git log / CHANGELOG 的事实，注释只描述它**现在**是什么。唯一例外：插件 / 开放契约中"字段的可用性本身是调用方必须遵守的约束"（旧运行时不存在、需防御性判空），此时写成行为指令：`// v0.2.3 起提供，兼容旧运行时需判空`。
- 空泛的 why：`// 用于处理 xxx 问题`——把 why 里的 why 抠掉了。要么写到能指导行为（什么问题、什么约束、谁负责），要么删。
- 同一字段两条注释流（上方独立注释 + 行尾注释并存）。短注释走行尾，长注释（超一行）置字段上方独立成行，二选一；伪精度分列（两个分支结果相同还逐项罗列）同样算噪声。

**GoFrame 的分工**：

- 【必须】Req/Res 等对外契约结构体的字段文档统一写在 `dc:` tag——它是 OpenAPI 文档的唯一来源，请求字段应当每个都有 dc。
- 【必须】行注释（`//`）只承载 dc 放不下、或不需要进对外文档的实现语义（枚举、单位、坑）；与 dc 不重复。
- 生成物豁免：`gf gen dao` 生成的 entity/do 字段注释来自数据库列 COMMENT，属自动生成物，不手改、不参与评审。

### 1.5 结构与抽象

- 【必须】**接口在消费者处定义**（accept interfaces, return structs）：谁用谁定义，接口只包含它需要的方法。返回值返回具体类型。
- 【必须】接口要小（1~3 个方法）。出现十几个方法的大接口，先拆分再使用。
- 【建议】组合优于继承。结构体嵌入（embedding）要克制——它是把内部实现暴露成 API，慎用。
- 【禁止】为"将来可能"预留参数、配置项、抽象层（YAGNI）。
- 【建议】第三方 SDK 用适配层（adapter）隔离，不把第三方类型散落进业务代码。换库时只改适配层。

---

## 2. Go 语言规范

### 2.1 格式与工具链

- 【必须】所有代码通过 `gofmt` / `goimports` 格式化（缩进为 tab），CI 强制检查。
- 【必须】import 按三组分块：标准库 / 第三方 / 本项目，组间空行。
- 【必须】`go mod tidy` 后提交；引入新依赖需说明理由。
- 【必须】提交前 `golangci-lint run` 通过；`//nolint` 必须带原因注释。
- 【建议】`go vet`、`staticcheck` 纳入本地工作流。

### 2.2 声明与零值

- 【必须】局部变量优先 `:=`；需要零值即用的类型直接 `var` 声明（`var buf bytes.Buffer`），不用 `new`。
- 【必须】`var` / `const` / `type` 分组声明；相关常量用 const 块 + `iota`，而非散落的魔法数字。
- 【必须】**nil slice 与空 slice 语义区分**：API 响应中的列表字段禁止返回 nil（JSON 序列化成 `null`），统一返回 `[]T{}`。
- 【必须】结构体含 `sync.Mutex` 等同步字段时禁止值拷贝，方法一律用指针接收者（`go vet -copylocks` 会拦截）。
- 【必须】map 遍历无序，禁止依赖遍历顺序；需要有序先对 key 排序。
- 【必须】JSON 序列化的结构体字段带 tag（`json:"user_id"`），显式声明对外契约。
- 【建议】泛型只用于通用容器/工具函数；业务代码不过度泛化，可读性优先。
- 【建议】`interface{}` 统一写 `any`。

### 2.3 控制流

- 【必须】range 的 value 是副本：修改它不影响原集合；需要索引修改时用下标遍历。
- 【必须】注意循环变量捕获：Go 1.22 起每次迭代是新变量，闭包捕获安全；若团队需兼容更低版本，闭包内使用前先 `v := v` 拷贝。
- 【建议】分支多于 2 路时用 `switch` 替代 if-else 链。
- 【禁止】`goto`；【禁止】依赖 `fallthrough`。

### 2.4 错误处理（重点章节）

#### 2.4.1 铁律

- 【必须】**每个 error 都必须处理**：向上返回、记日志后吞掉、或显式忽略（`_ = xxx` 并注释为什么可以忽略）。禁止下划线静默吞错后继续用零值。
- 【必须】**跨层返回必须包装上下文**：`fmt.Errorf("查询用户订单失败 userID=%d: %w", userID, err)` 或 `gerror.Wrapf(err, ...)`。顶层日志看到的错误必须能定位到发生点。
- 【必须】**判断错误类型用 `errors.Is` / `errors.As`**，禁止 `strings.Contains(err.Error(), "xxx")`。
- 【必须】哨兵错误包级导出：`var ErrNotFound = errors.New("not found")`；GoFrame 场景用 `gerror.NewCode` 携带错误码（见 3.6）。
- 【禁止】`panic` 处理可预期的业务/输入错误。panic 仅限"程序性不可恢复错误"（启动配置缺失、不变量被破坏、初始化失败）。
- 【必须】每个自行启动的 goroutine 顶层必须有 `defer recover()`，否则一个 panic 打死整个进程。
- 【必须】并发子任务错误必须收集：`errgroup` 或 channel 汇聚，禁止丢错误只等 Wait。

#### 2.4.2 示例

```go
// ❌ 反例：裸抛丢上下文 / 静默吞错 / 字符串判错
data, _ := json.Marshal(v)                 // 吞错，后续用零值
if err != nil {
	return err                              // 顶层不知道错在哪
}
if strings.Contains(err.Error(), "duplicate") { ... }

// ✅ 正例：包装上下文 + 类型化判断
data, err := json.Marshal(v)
if err != nil {
	return nil, gerror.Wrapf(err, "序列化请求体失败 model=%s", model)
}
if errors.Is(err, sql.ErrNoRows) {
	return nil, gerror.NewCode(CodeOrderNotFound, "订单不存在")
}
```

#### 2.4.3 错误文案分层

- 底层 wrap 的错误文案面向**开发者**（定位问题），简短、含关键参数。
- 用户可见的错误提示必须由 **api 层错误码 → 中文文案映射** 产生，禁止把底层 `err.Error()` 直接透给客户端（见 3.6 / 5）。

### 2.5 context 规范

- 【必须】`ctx` 永远是第一个参数，命名 `ctx`。
- 【必须】ctx 不作为结构体字段长期持有（请求生命周期内的 handler 对象除外，且注释说明生命周期）。
- 【必须】每个 ctx 必须有取消路径：超时 `context.WithTimeout`、显式取消 `WithCancel`；HTTP 场景用请求自带的 ctx（GoFrame 中 `r.GetCtx()`）。
- 【必须】只有任务根 / 测试才 `context.Background()` / `context.TODO()`，禁止在调用链中段新开 Background 掐断取消信号。
- 【禁止】ctx 传 nil 占位。
- 【必须】`context.Value` 只放请求级元数据（trace_id、登录身份、租户 ID），禁止携带业务参数。
- 【必须】io / RPC / DB / 缓存操作都要接受并传递 ctx，让超时与取消能贯穿。

### 2.6 并发规范

- 【必须】**每个 goroutine 必须有明确退出路径**（ctx Done / 关闭 channel / 计数归零），创建者负责它的生命周期。写 `go func` 前先回答：它什么时候退出？
- 【必须】程序退出时等待后台任务收尾（WaitGroup / errgroup），不留悬空写操作。
- 【必须】channel 规范：**由发送方关闭**，接收方不关闭；参数按需限定方向（`chan<- T` / `<-chan T`）；无缓冲为默认，加缓冲必须有明确理由。
- 【必须】map 并发读写会 panic：并发场景用 `sync.Mutex` 保护或 `sync.Map`（仅读多写少、key 稳定的场景）。
- 【必须】循环里的 `time.After` 造成 timer 泄漏，长循环用 `time.NewTimer` + `Reset`。
- 【建议】channel 表达"数据流动"，mutex 表达"状态保护"，两者都合法，按语义选，勿教条站队。
- 【建议】服务优雅退出：监听信号 → 停止接新请求 → 处理存量 → 关闭连接（GoFrame `s.Shutdown()`）。

```go
// ✅ 后台消费 goroutine 的标准形态：ctx 控制退出 + 顶层 recover
go func() {
	defer func() {
		if r := recover(); r != nil {
			g.Log().Errorf(ctx, "消费协程 panic: %v\n%s", r, gdebug.Stack())
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			handle(ctx, msg)
		}
	}
}()
```

### 2.7 时间与时长

- 【必须】时长统一 `time.Duration`，禁止裸 `int` 秒/毫秒穿越函数边界（`timeout := 30 * time.Second`）。
- 【必须】数据库存 UTC，时区转换只在展示层做。
- 【必须】时刻比较用 `Before/After/Equal`，禁止 Format 成字符串再比较。
- 【禁止】`time.Sleep` 做同步（等待另一 goroutine 完成应该用 channel / WaitGroup；测试用轮询断言）。

### 2.8 性能要点

按需采用，**优化必须有 benchmark 或 pprof 数据支撑**，禁止凭感觉优化：

- 循环内字符串拼接用 `strings.Builder`。
- 已知长度的 slice / map 用 `make([]T, 0, n)` 预分配。
- 热路径上的大对象缓冲用 `sync.Pool` 复用。
- range 大结构体时取值是整份拷贝，遍历下标或存 `*T`。
- 避免 `string([]byte)` / `[]byte(string)` 在热路径反复互转。
- 高 QPS 接口注意锁粒度与临界区大小；统计类计数用原子操作。

### 2.9 依赖管理

- 引入新库前评审：维护活跃度、License、传递依赖体积、是否标准事实方案。能标准库解决的不引第三方。
- 【禁止】长期悬挂的 `replace` 指向个人分支。
- 【禁止】把第三方源码复制进 `internal/` 伪装成自己的代码。

---

## 3. GoFrame v2 框架规范

### 3.1 分层架构与职责边界

```
cmd（路由注册/服务启动）
  → api（Req/Res 结构体定义 + 路由注解）
    → controller（薄壳：参数绑定 → 调 service → 返回）
      → service（接口定义，gf gen service 生成）
        → logic（业务逻辑实现，核心代码所在）
          → dao（ORM 数据访问，gf gen dao 生成）
```

| 层 | 允许做 | 禁止做 |
|----|--------|--------|
| controller | 参数绑定、调 service、组装返回 | 业务逻辑、SQL、读全局配置、fmt.Println 调试 |
| service | 接口声明 | 任何实现 |
| logic | 业务规则、事务编排、调 dao / 其他 service | import ghttp / controller；直接操作 `*ghttp.Request` |
| dao | 数据访问 | 业务判断 |

- 【必须】**请求级数据（登录身份、租户 ID）从 ctx 取**，通过中间件注入、统一存取模型传递；logic 层禁止反向触碰 HTTP 层对象。
- 【必须】代码生成顺序：**编写 api + logic → `gf gen service` → `gf gen ctrl`**。生成的 `controller/` `service/` `dao/` `model/` 目录禁止手改。
- 【必须】公共业务逻辑下沉到 service 层复用，禁止 controller 之间复制粘贴。

### 3.2 API 定义规范（api/）

- 【必须】请求/响应统一 `XxxReq` / `XxxRes` 后缀，同名方法成对定义在同一文件。
- 【必须】`g.Meta` 注解完整：`path` / `method` / `tags` / `summary` 必填——它们就是 OpenAPI 文档与路由注册的真相源。
- 【必须】参数校验规则写在 Req 的 `v:` tag 上，入口统一校验（见 3.3）。
- 【必须】每个字段写 `dc:` 描述，生成可读的接口文档。
- 【必须】**Res 只声明调用方需要的字段**，禁止直接返回 ORM entity（会把内部字段、敏感字段泄漏成 API 契约）；entity → Res 的转换收敛到独立函数。
- 【必须】统一响应结构由响应中间件封装（`{code, message, data, request_id}` 风格），业务代码不手写 HTTP 状态码与响应包装。
- 流式转发 / 文件下载 / 第三方回调等特殊端点走独立 handler 直写 `http.ResponseWriter`，不走统一 JSON 响应，并注释说明原因。

```go
type CreateUserReq struct {
	g.Meta `path:"/user" method:"post" tags:"用户管理" summary:"创建用户"`
	Name   string `json:"name" dc:"用户名" v:"required|length:2,32#请输入用户名|用户名长度 2-32 位"`
	Email  string `json:"email" dc:"邮箱" v:"required|email#请输入邮箱|邮箱格式不正确"`
}

type CreateUserRes struct {
	Id int64 `json:"id" dc:"用户 ID"`
}
```

### 3.3 参数校验（gvalid）

- 【必须】入口统一校验：Req 层用内置规则（`required` / `min` / `max` / `length` / `in:..` / `email` 等）挡掉格式问题，logic 层只做业务规则（余额是否足够、状态是否允许）。
- 【必须】错误消息中文化：`v:"required#请输入用户名"` 用 `#` 定制，多条规则用 `|` 分隔、消息一一对应。
- 跨字段 / 需要查库的校验：注册自定义规则或放 logic 层，禁止在 tag 里塞复杂表达式。
- 自定义规则全局注册一次，命名 `小写-短横` 风格（如 `phone-cn`）。

### 3.4 Controller / Service / Logic

```go
// ✅ controller：只做接线，一眼看完
func (c *ControllerV1) CreateUser(ctx context.Context, req *v1.CreateUserReq) (res *v1.CreateUserRes, err error) {
	return service.User().CreateUser(ctx, req)
}

// logic：业务实现
func (u *sUser) CreateUser(ctx context.Context, req *v1.CreateUserReq) (*v1.CreateUserRes, error) {
	if exists, err := u.emailExists(ctx, req.Email); err != nil {
		return nil, err
	} else if exists {
		return nil, gerror.NewCode(CodeEmailTaken, "邮箱已被使用")
	}
	id, err := dao.User.Ctx(ctx).Data(do.User{
		Name:  req.Name,
		Email: req.Email,
	}).InsertAndGetId()
	if err != nil {
		return nil, gerror.Wrap(err, "写入用户失败")
	}
	return &v1.CreateUserRes{Id: id}, nil
}
```

- 【必须】service 方法签名 `ctx` 为第一参数。
- 【建议】一个 logic 文件对应一个业务聚合（user.go / order.go / wallet.go）；文件超过 600 行考虑按子域拆分。
- 【建议】logic 依赖（dao、下游 service）通过构造函数注入，便于测试替换；可用包级注册（`service.RegisterUser(New())`）保持 gf 生成风格。

### 3.5 DAO / ORM 规范

#### 3.5.1 查询

- 【必须】**禁止字符串拼接 SQL**，一律参数化写法；必须写原生 SQL 时用占位符 + 参数（`db.GetAll(ctx, "SELECT ... WHERE id=?", id)`）。
- 【必须】列名用**小写蛇形字符串常量 + 参数化占位符**（`Where("tenant_id", tenantID)`），禁止拼接 SQL 字符串。

```go
// ❌ 反例：拼接 SQL
dao.Order.Ctx(ctx).Where("tenant_id = "+tid).Where("status", 1).One()

// ✅ 正例：字符串列名 + 参数化（项目现行约定）
dao.Order.Ctx(ctx).
	Where("tenant_id", tenantID).
	Where("status", StatusPaid).
	One()
```

> 列名约定说明：规范原要求 `dao.Xxx.Columns().Field` 引用列名，但存量代码为字符串列名主流（440+ 处，Columns() 采用率 0），
> 全量改造代价大。经 2026-11 审计 §4.6 决策：**承认字符串列名现状**，列改名时的一致性由 code review 把关
> （新增/修改列时同步搜索全部引用点）；新代码同样沿用字符串列名，保持风格统一。

- 【必须】**按 ID 取对象必须带归属条件**（`WHERE id = ? AND tenant_id = ?` 双键校验），防止越权访问他人资源（IDOR）。
- 【必须】列表分页必须限制上限：`Page(page, size)` 前对 size 做钳制（如 ≤ 100），默认值兜底。
- 【建议】只查需要的列（`.Fields(...)`），大表禁止默认全列查询；`Scan`（列表）/ `Struct`（单条）按语义选用。
- 【建议】列表 + 总数一次完成用 `AllAndCount()`（gf v2.4+），旧版本 `Count()` + `Page().All()` 两次。
- 【必须】禁止在循环里逐条查询（N+1），改为 `WhereIn` 批量取回后在内存组装。

#### 3.5.2 写入

- 【必须】**部分更新用 `do` 结构体**（nil 字段不参与 UPDATE）；全量覆盖才用 entity / map。
- 【必须】`Insert` / `Update` / `Delete` 都要检查返回的 error；写操作记业务日志（含关键 ID）。
- 软删除由 `deleted_at` 自动过滤；确需物理删除时 `Unscoped()` 并注释原因。

#### 3.5.3 事务

```go
err := dao.Order.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
	// 闭包内所有 dao 调用必须用入参 ctx（tx 已注入其中，自动加入事务）
	if _, err := dao.Order.Ctx(ctx).Data(do.Order{...}).Insert(); err != nil {
		return gerror.Wrap(err, "创建订单失败")   // 返回 error 即自动回滚
	}
	if err := s.deductStock(ctx, skuID, qty); err != nil {
		return err
	}
	return nil // 返回 nil 自动提交
})
```

- 【必须】事务用 `Transaction` 闭包，闭包内**必须使用入参 ctx** 让后续 dao 自动加入事务；任何一步失败直接 return error。
- 【禁止】事务闭包里做 RPC 调用、发消息、耗时的外部 IO——事务只包数据库写操作，长事务拖垮连接池。
- 事务外的补偿动作（发通知等）放在事务成功之后执行。

### 3.6 错误处理与错误码

- 【必须】错误码集中在 consts 包统一定义、分模块分段（如 10000-10999 用户、11000-11999 订单），禁止散落魔法数字。

```go
var (
	CodeUserNotFound  = gcode.New(10101, "用户不存在", nil)
	CodeEmailTaken    = gcode.New(10102, "邮箱已被使用", nil)
	CodeOrderNotPayable = gcode.New(11101, "订单当前状态不可支付", nil)
)

// 创建带码错误
return nil, gerror.NewCode(CodeUserNotFound, "用户不存在")
// 包装底层错误并携带码（保留堆栈）
return nil, gerror.WrapCode(CodeDBWrite, err, "写入订单失败")
```

- 【必须】对外 message 为用户可读中文，不暴露技术细节（SQL、内部地址、堆栈）。
- 【必须】错误响应结构由统一响应中间件输出；`request_id` 必带，方便用户报障时定位日志。
- 【建议】优先复用框架内置 `gcode.CodeValidationFailed`、`gcode.CodeNotAuthorized` 等语义码。
- 【必须】api 层入口处（中间件）统一兜底 `recover`，任何 panic 转为 500 结构化响应，不带崩进程、不泄漏堆栈。

### 3.7 日志规范

- 【必须】统一 `g.Log()`，带 ctx（自动携带 trace / request_id）；禁止 `fmt.Println`、标准库 `log` 混用与调试残留。
- 【必须】分级语义：

| 级别 | 场景 |
|------|------|
| Debug | 开发排查细节，生产默认关闭 |
| Info | 关键业务动作（创建/支付/删除、状态流转） |
| Warning | 可恢复异常：上游超时已重试、降级触发、参数容错 |
| Error | 需要人介入的失败：落库失败、一致性破坏、panic 恢复 |

- 【必须】日志内容含定位要素：关键 ID（订单号 / 用户 ID / 模型名），格式统一 `动作 + 结果 + 要素`。
- 【必须】**密码、API Key、Token、身份证等敏感字段必须脱敏**后再入日志。
- 【建议】高频路径避免每请求多条 Info 日志，采样或聚合，防止日志洪水。
- 结构化键值对写法：`g.Log().Info(ctx, "订单支付成功", "orderID", orderID, "amount", amount)`。

### 3.8 配置与中间件

- 【必须】配置统一走 `g.Cfg()`；读取时给默认值，配置缺失不得 panic（除非启动必需项，启动即 fail-fast）。
- 【必须】敏感配置（数据库密码、密钥）优先环境变量注入，禁止明文提交进仓库。
- 【必须】横切关注点全部走中间件：统一响应、鉴权、CORS、限流、日志；禁止散落在 controller。
- 【必须】中间件注册集中在路由定义处，执行顺序显式可见。
- 【必须】ctx 传值使用统一的封装：本项目约定为「包内私有常量 key + accessor 函数」（如 `middleware.GetTenantID(ctx)` / `middleware.GetUserID(ctx)`），封装强度等价于 typed struct 模型，禁止跨包裸字符串字面量 key 直接 `context.WithValue`。
  （例外：`middleware/request_id.go` 的 `"RequestId"` 字符串 key 是 GoFrame logger 机制所需，文件内已注释说明——2026-11 审计 §6 决策项 2。）

### 3.9 缓存与 Redis

- 【必须】缓存/状态 key 统一命名：`{域}:{业务}:{标识}` 两三段式（如 `wallet:v2:{tenant_id}`、`dispatch:v1:breaker:{channel_id}`、`cache:invalidate`），同一域内前缀保持一致、全库唯一、可读可排查；**禁止同一业务多套前缀并存**（并发限制等场景应收敛为单一前缀）。（2026-11 审计 §6 决策项 1：承认项目自有两三段体系，替代原四段式要求。）
- 【必须】**每条缓存必须设置 TTL**；确实要长持有的显式给过期时间，禁止裸 Set 不带 TTL。
- 【必须】缓存故障（Redis 不可用）按语义分级：纯加速缓存降级直查 DB；作为权威状态（如余额）则 fail-closed，禁止降级放行。
- 【建议】Cache-Aside 模式：先查缓存 → miss 查库回填；更新走"更新 DB → 删缓存"。
- 【建议】防击穿用 singleflight / 逻辑过期；防雪崩 TTL 加随机抖动。
- 【必须】资金 / 计数类原子操作用 Redis Lua 脚本，禁止 Go 层"读-改-写"三步走。
- 多实例部署下，进程内缓存只放只读数据，可变状态一律进 Redis。

### 3.10 定时任务与后台任务

- 【必须】定时任务防重叠：本项目采用自研 `internal/logic/common.CronScheduler`（robfig/cron + 进程内 running map 互斥 + Redis SET NX EX 分布式锁 + Lua CAS 释放 + `runHandlerSafely` panic 兜底 + 统一 10min 执行超时，CronJob.Timeout 可覆盖），语义强于 gcron `AddSingleton`（秒级粒度 gcron 做不到），按「同等语义要求」执行；新引入定时任务一律走 CronScheduler.Register，直接使用 gcron 时必须 `AddSingleton`。（2026-11 审计 §6 决策项 3。）
- 【必须】任务函数带超时 ctx、顶层 recover、失败打 Error 日志（含任务名），关键失败要有告警出口。
- 【必须】多实例部署时同一定时任务加分布式锁（Redis），防止重复执行。
- 【建议】任务入口与执行逻辑分离：cron 只做触发，逻辑放 service 可单独测试。

### 3.11 HTTP 客户端（gclient）

```go
resp, err := g.Client().
	SetTimeout(15 * time.Second).
	Retry(2, 500*time.Millisecond).
	Post(ctx, url, g.Map{"k": "v"})
if err != nil {
	return nil, gerror.Wrapf(err, "调用下游失败 url=%s", url)
}
defer resp.Close()
```

- 【必须】所有出站请求设置总超时；重试只用于幂等请求且有上限、有退避。
  （例外：AI 网关对推理 POST 的跨渠道重试是领域惯例——上游瞬断换渠道重发，幂等性由 `bil_records.request_id` 唯一约束闸门兜底，不计入违规。2026-11 审计 §6 决策项 4。）
- 【必须】`resp` 判空后 `defer resp.Close()`；状态码非 2xx 按下游错误处理，不算 Go 层 error。
- 【必须】URL 拼参数必须 encode；用户提供 URL 的场景做 SSRF 校验（禁内网地址、禁非常规协议）。
- 【必须】下游错误分类：网络错误可重试，4xx 不重试（立刻失败），5xx 视语义——分类逻辑收敛到一处。

---

## 4. 测试规范

- 【必须】`go test ./...` CI 全绿；核心逻辑（计费、权限、调度、金额运算）单元测试覆盖率 ≥ 80%。
- 【必须】**表驱动测试**为默认形态，子测试名描述业务场景：

```go
func TestSplitBill(t *testing.T) {
	tests := []struct {
		name    string
		total   decimal.Decimal
		parts   int
		wantErr bool
	}{
		{"三人均分整额", decimal.NewFromInt(90), 3, false},
		{"除不尽时余数给首人", decimal.NewFromFloat(10.1), 3, false},
		{"份数为零拒绝", decimal.NewFromInt(10), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitBill(tt.total, tt.parts)
			if tt.wantErr {
				gtest.AssertNE(err, nil)
				return
			}
			gtest.AssertNil(err)
			gtest.Assert(got.Sum().Equal(tt.total), "分账之和必须等于总额")
		})
	}
}
```

- 【必须】测试之间不共享可变全局状态；每个测试自建数据、`t.Cleanup` 自清理。
- 【建议】mock 最小化：接口在消费者处定义天然可替换，优先依赖注入而非重 mock 框架。
- 【必须】时间相关的逻辑把"当前时间"作为参数或注入函数传入，测试可冻结时间；禁止在测试里 Sleep 等时序。
- 【建议】关键链路（登录 → 调用 → 计费）保留端到端集成测试，跑在独立环境与数据上。
- 【必须】性能改动附 `benchmark` 前后对比数据，"感觉变快了"不算结论。

---

## 5. 安全规范

- 【必须】SQL 一律参数化（见 3.5），杜绝拼接注入。
- 【必须】所有外部输入走 Req 层白名单校验（类型、长度、范围、枚举），不信任任何客户端数据，包括自己前端。
- 【必须】资源访问做归属校验：按 ID 操作必须带归属条件（多租户即双键 `id + tenant_id`），防 IDOR 越权。
- 【必须】密码只存 bcrypt/argon2 哈希；API Key / Secret 哈希或加密存储，明文只在创建时展示一次。
- 【必须】日志、错误响应、导出文件全部脱敏；错误响应不带堆栈、SQL、内部 IP（见 3.6 / 3.7）。
- 【必须】文件上传：类型白名单 + 大小限制 + 存储文件名随机化，禁止保留用户原始文件名直接落盘。
- 【必须】用户提供 URL 发起出站请求的场景做 SSRF 防护（见 3.11）。
- 【建议】定期 `govulncheck` / `gosec` 扫描依赖与代码漏洞。

---

## 6. 工程化与协作

### 6.1 提交规范

- 【必须】Conventional Commits 格式：`type(scope): 中文摘要`，type ∈ `feat` / `fix` / `refactor` / `perf` / `docs` / `test` / `chore` / `ci`。
- 【必须】一个提交一个意图：功能、重构、格式化不混在一个 commit 里。
- 【必须】提交前三件套通过：格式化（gofmt/goimports）、静态检查（golangci-lint）、测试（go test）。

### 6.2 Code Review 清单

评审人按序过一遍，任何一条不过即打回：

- [ ] 命名是否表意一致；有无垃圾抽屉包、魔法数字、注释掉的代码
- [ ] 函数是否单一职责、无箭头嵌套、无布尔旗标参数
- [ ] 每个 error 都被处理且跨层包装了上下文；无 panic 做业务错误
- [ ] goroutine 有退出路径、有 recover；ctx 传递未被掐断
- [ ] 多租户 / 归属过滤条件逐条核对（漏写即越权）
- [ ] SQL 参数化、事务边界最小、无循环内查询
- [ ] Res 未泄漏内部字段；日志与响应已脱敏
- [ ] 错误码走统一定义，message 用户可读中文
- [ ] 新逻辑有表驱动测试覆盖关键分支与边界

### 6.3 分支与发布

- 【必须】功能开发走 feature 分支，主干保持随时可发布。
- 【必须】合并前 CI（lint + test + build）全绿。

---

## 附录 A：Do / Don't 速查表

| 场景 | ✅ 应该 | ❌ 避免 |
|------|--------|--------|
| 错误返回 | `gerror.Wrapf(err, "写入订单失败 orderID=%d", id)` | 裸 `return err` / `_ =` 吞错 |
| 判断错误 | `errors.Is(err, ErrNotFound)` | `err.Error()` 字符串匹配 |
| 查询条件 | `Where(dao.Order.Columns().TenantId, tid)` | `Where("tenant_id="+tid)` |
| 更新字段 | `do` 结构体（nil 不更新） | entity 误置零值覆盖 |
| 响应列表 | 空时返回 `[]T{}` | 返回 nil（JSON 变 null） |
| 时长 | `30 * time.Second` | 裸 `int` 毫秒满天飞 |
| 日志 | `g.Log().Infof(ctx, ...)` 带关键 ID | `fmt.Println` 调试残留 |
| 并发等待 | errgroup 收集错误 | 裸 `go` 无退出路径 |
| 缓存 | key 带模块前缀 + 必设 TTL | 裸 Set 永不过期 |
| 注释 | 解释决策原因与坑 | 逐行翻译代码 |
| 抽象 | 重复第 3 次再抽 | 一次性代码先上接口 |
| 包名 | `encrypt` / `billing`（具体职责） | `util` / `common` / `misc` |
| 事务 | 只包 DB 写操作 | 事务里做 RPC / 发消息 |
| 布尔参数 | 拆成两个函数 | `render(isAsync bool)` |

## 附录 B：golangci-lint 参考配置

```yaml
run:
  timeout: 5m

linters:
  enable:
    - gofmt          # 格式化
    - goimports      # import 分组排序
    - govet          # 官方静态检查（含 copylocks）
    - staticcheck    # 强化 vet
    - errcheck       # 未检查的 error
    - unused         # 未使用符号
    - gosimple       # 可简化写法
    - ineffassign    # 无效赋值
    - gocyclo        # 圈复杂度
    - misspell       # 拼写
    - bodyclose      # HTTP body 关闭
    - rowserrcheck   # sql.Rows.Err 检查
    - unconvert      # 多余类型转换
    - copyloopvar    # 循环变量捕获
    - gosec          # 安全检查
  disable:
    - lll            # 行宽不强制

linters-settings:
  gocyclo:
    min-complexity: 15

issues:
  exclude-rules:
    - path: _test\.go
      linters: [gosec, errcheck]
```

## 附录 C：参考资料

- [Effective Go](https://go.dev/doc/effective_go)（官方）
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)（官方）
- [Google Go Style Guide](https://google.github.io/styleguide/go/)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [GoFrame v2 官方文档](https://goframe.org/)
- 《代码整洁之道》Robert C. Martin

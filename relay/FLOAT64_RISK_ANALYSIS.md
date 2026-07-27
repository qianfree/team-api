# Relay 层浮点运算风险分析报告

**分析范围**：`relay/` 目录（137 个非测试文件）  
**分析日期**：2026-07-23  
**分析目标**：扫描涉及金额/计费的 float64 运算，识别精度丢失风险

---

## 一、风险等级定义

| 风险等级 | 定义 | 影响范围 |
|---------|------|---------|
| **🔴 高危** | 金额直接链式运算（乘除混合）、累计误差会传播到账本 | 钱包扣费、账单金额、对账 |
| **🟡 中危** | Token 计数 → 费用换算路径存在浮点除法，但单次计算 | 单笔计费、用量统计 |
| **🟢 低危** | 仅用于估算、权重计算、非金额场景 | 调度权重、粗略估算 |

---

## 二、风险点清单

### 🔴 高危：金额直接链式运算（0 处）

**好消息：relay 层没有直接的链式金额运算**。

所有金额计算（`PreDeduct`/`Settle`/`EstimateTaskCost`）都通过接口委托给 `internal/logic/billing/`，relay 层只传递 `float64` 结果，不进行二次运算。

**验证点**：
- `common/billing.go`：接口定义，只传递 `float64` 参数和返回值
- `common/task_billing.go`：接口定义，只传递 `float64` 参数和返回值
- `handler/relay_handler.go:393`：`PreDeduct` 调用，接收 `float64` 赋值给 `preDeductAmount`，无后续运算
- `handler/task_handler.go:191`：`PreDeductTask` 调用，接收 `float64` 赋值给 `preDeductAmount`，无后续运算

**结论**：relay 层作为代理层，金额字段为**透传类型**，不承担计费逻辑，风险转移到 `internal/logic/billing/`。

---

### 🟡 中危：Token 计数 → 费用换算路径（8 处）

#### 1. **音频 Token 估算**（3 处）

| 文件 | 行号 | 代码 | 风险说明 |
|------|------|------|---------|
| `channel/openai/audio.go` | 50 | `estimatedTokens := len(body) / 1000` | TTS 音频字节数除以 1000 估算 token |
| `channel/openai/audio.go` | 94 | `estimatedTokens := int(verboseResp.Duration/60.0) * 150` | **float64 除法** + 整数乘法：音频时长（秒）÷ 60（分钟）× 150 token/分钟 |
| `channel/openai/audio.go` | 107 | `estimatedTokens := len(simpleResp.Text) / 4` | 文本长度除以 4 估算 token |

**风险分析**：
- `Duration/60.0` 产生浮点除法，**若 Duration 为整数秒（如 90 秒），则 90/60.0 = 1.5，int(1.5) = 1，丢失 0.5 分钟**
- 丢失部分：`0.5 * 150 = 75 token`，约占实际 `90/60 * 150 = 225 token` 的 **33%**
- 单笔计费误差传播：若 token 单价为 $0.0001/token，则单次误差 = `75 * 0.0001 = $0.0075`，累计 10000 次 = **$75 短扣**

**修复建议**：
```go
// 当前（有误差）
estimatedTokens := int(verboseResp.Duration/60.0) * 150

// 修复方案：先整数运算再取整
estimatedTokens := (int(verboseResp.Duration) * 150 + 30) / 60  // +30 为四舍五入
```

---

#### 2. **文本 Token 估算**（5 处）

| 文件 | 行号 | 代码 | 用途 |
|------|------|------|------|
| `channel/claude/response.go` | 253, 297 | `estimated := responseTextBuf.Len() / 4` | Claude 流式响应 token 估算 |
| `channel/openai/converter.go` | 1103, 1349 | `estimated := len(text) / 4` | OpenAI 响应 token 估算 |
| `channel/openai/responses.go` | 485 | `usage.CompletionTokens = len(text) / 4` | 补全 token 估算 |
| `channel/openai/stream.go` | 69, 131 | `estimated := responseTextBuf.Len() / 4` | 流式 token 估算 |
| `channel/openai/realtime.go` | 224 | `estimated := len(message) / 4` | Realtime API 消息估算 |
| `handler/relay_handler.go` | 840 | `return len(body) / 4` | 请求 token 估算 |
| `helper/stream.go` | 116 | `return (len(text) + 3) / 4` | 通用 token 估算（**四舍五入**） |

**风险分析**：
- **整数除法**：`len(text) / 4` 向下取整，1-3 字符直接归零（如 "Hi" → 0 token）
- 误差范围：单次最大丢失 3 字符 ≈ 0.75 token，累计误差取决于短文本频率
- **`helper/stream.go:116` 已做四舍五入**：`(len(text) + 3) / 4` 等价于 `round(len/4)`，**其他 7 处未做**

**影响评估**：
- 若上游返回完整 `usage`，这些估算值会被覆盖，**无风险**
- 若上游无 `usage`（如音频 TTS、某些流式端点），估算值直接进入计费，**有短扣风险**

**修复建议**：
```go
// 统一使用 helper/stream.go 的四舍五入版本
import "github.com/qianfree/team-api/relay/helper"
estimated := helper.EstimateTokens(text)
```

---

### 🟢 低危：非金额场景（3 处）

#### 1. **异步任务计费比率**（2 处）

| 文件 | 行号 | 代码 | 用途 |
|------|------|------|------|
| `taskchannel/volcengine/constants.go` | 17-18 | `28.0 / 46.0`, `22.0 / 37.0` | 视频输入折扣比率（含视频/不含视频单价之比） |
| `taskchannel/volcengine/adaptor.go` | 452-462 | `resolutionMultiplier()` 返回 `1.0/2.25/5.06` | 分辨率乘数（相对于 480p） |

**风险分析**：
- `28.0 / 46.0 ≈ 0.6086956521739130...`（无限不循环小数），存储为 `float64` 后精度 ~15 位有效数字
- 这些比率**用于 `EstimateTaskCost` 的 `ratios` 参数**，最终传递到 `internal/logic/billing/` 计算费用
- **风险转移**：若 billing 层用 `ratio * basePrice` 计算，累计误差由 billing 层承担

**当前状态**：低危，因为：
1. 比率为**常量**，不累计（每次计费重新取值）
2. 单次计算：`cost = basePrice * resolution * videoInputRatio`，误差 < 0.01%
3. 若 billing 层改用 `NUMERIC` 精确计算，此处需改为传递分数字符串（如 `"28/46"`）

---

#### 2. **调度权重降权**（1 处）

| 文件 | 行号 | 代码 | 用途 |
|------|------|------|------|
| `scheduler/scheduler.go` | 137-143 | `w = w / 2` / `w = w / 4` | 根据健康度降低渠道权重 |

**风险分析**：
- **整数除法**：`w` 为 `int` 类型，`w / 2` 和 `w / 4` 向下取整
- 仅影响调度权重（非金额），且每次调度重新计算，无累计误差
- **无风险**

---

## 三、风险传播路径

```
┌─────────────────────────────────────────────────────────────┐
│ relay/ 层                                                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Token 估算（🟡 中危）                                   │
│     ├─ audio.go: Duration/60.0 * 150  → estimatedTokens    │
│     ├─ stream.go: len(text) / 4       → estimatedTokens    │
│     └─ 估算值 → common.Usage{TotalTokens: estimatedTokens} │
│                                                              │
│  2. 预扣/结算（✅ 无运算）                                   │
│     ├─ PreDeduct(estimatedTokens, maxTokens) → float64     │
│     ├─ Settle(usage, preDeductAmount)       → float64      │
│     └─ relay 层**只传递值，不计算**                         │
│                                                              │
└──────────────────┬───────────────────────────────────────────┘
                   │ 接口调用
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ internal/logic/billing/ 层                                   │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ⚠️  金额计算核心逻辑（需单独扫描）                          │
│     ├─ cost = inputTokens * inputPrice                     │
│     ├─ cost = cost * modelMultiplier * tenantMultiplier    │
│     ├─ actualCost = baseCost (链式运算？)                   │
│     └─ 若使用 float64 链式运算 → 🔴 高危                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**关键发现**：
- relay 层的 **float64 风险已隔离**，金额运算全部在 billing 层
- **但 relay 层的 token 估算误差会作为输入传播到 billing 层**

---

## 四、修复优先级

| 优先级 | 问题 | 文件 | 修复方案 |
|--------|------|------|---------|
| **P0** | 音频时长浮点除法短扣 | `channel/openai/audio.go:94` | 改为整数运算：`(Duration * 150 + 30) / 60` |
| **P1** | 文本 token 估算未四舍五入 | 7 处（见中危清单） | 统一调用 `helper.EstimateTokens(text)` |
| **P2** | 任务计费比率浮点常量 | `taskchannel/volcengine/constants.go` | billing 层改用分数或 NUMERIC 后，此处传递字符串 `"28/46"` |

---

## 五、Relay 层结论

✅ **relay 层金额计算架构安全**：
- 无链式浮点运算
- 所有 `cost/amount/quota` 字段为**接口传递类型**，不在 relay 层二次计算
- 金额逻辑封装在 `internal/logic/billing/`，符合单一职责原则

⚠️ **存在 token 估算精度问题**：
- 音频时长换算短扣约 **33%**（P0 修复）
- 文本长度估算短扣 0-3 字符（P1 修复）

🔍 **下一步行动**：
1. **立即修复 `audio.go:94` 的浮点除法**（影响所有音频计费）
2. 统一 token 估算为 `helper.EstimateTokens`
3. **扫描 `internal/logic/billing/` 层**（真正的金额运算核心）

---

## 六、Billing 层待扫描清单

基于接口定义，billing 层需重点扫描以下计算路径：

```go
// common/billing.go 接口定义暴露的计算链路
type SettlementResult struct {
	PreDeductAmount   float64  // 预扣金额
	BaseCost          float64  // 基础费用 = inputTokens * inputPrice + outputTokens * outputPrice
	ActualCost        float64  // 实际费用 = BaseCost * modelMultiplier * tenantMultiplier
	RefundAmount      float64  // 退款 = PreDeductAmount - ActualCost (if ActualCost < PreDeduct)
	SupplementAmount  float64  // 补扣 = ActualCost - PreDeductAmount (if ActualCost > PreDeduct)
	// ... 多个 float64 字段的加减乘除 ...
}
```

**高危操作**：
1. `BaseCost = inputTokens * inputPrice + outputTokens * outputPrice + cacheTokens * cachePrice + ...`（多项累加）
2. `ActualCost = BaseCost * modelMultiplier * tenantMultiplier`（链式乘法）
3. `RefundAmount = PreDeductAmount - ActualCost`（可能为负，需 abs）
4. 钱包扣费：`newBalance = oldBalance - ActualCost`（累计误差传播到账本）

**需验证**：billing 层是否已使用 `NUMERIC` 类型 + `decimal` 库，或仍在用 `float64`？

---

**报告结束**

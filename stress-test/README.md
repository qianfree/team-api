# Team-API 压力测试工具

Go 原生压测工具，支持 Chat 对话（SSE 流式）、图片生成、视频生成（异步提交+轮询）和混合场景。

内置本地 Mock AI 服务器，**可以不请求远端渠道，只测自己系统的吞吐**。

---

## 两种压测模式

| 模式 | 说明 | 费用 | 适用场景 |
|------|------|------|----------|
| **Mock 模式** 🎯 | 上游指向本地 Mock 服务器 | **零费用** | 测系统自身吞吐上限、找瓶颈 |
| **真实模式** | 上游指向 OpenAI/Claude 等 | 产生实际费用 | 验证端到端链路、测真实延迟 |

---

## 模式一：Mock 模式（推荐，零费用）

### 步骤 1：启动 Mock AI 服务器

```bash
# 终端 1：启动 Mock 服务器（默认端口 19000）
cd stress-test/mock
go run . -port 19000 -latency 500ms -speed 40 -v

# 参数说明：
#   -port       监听端口（默认 19000）
#   -latency    模拟 AI 推理延迟（默认 500ms）
#   -speed      流式输出速度 tokens/秒（默认 40）
#   -max-tokens 每次响应最大 token 数（默认 256）
#   -error-rate 模拟错误率 0.0~1.0（默认 0，压测稳定性时可以设 0.05）
#   -timeout-rate 模拟超时率 0.0~1.0（默认 0）
#   -video-time 视频生成模拟耗时（默认 30s）
#   -v          详细日志
```

### 步骤 2：在 team-api 管理后台配置渠道

在管理后台添加一个新渠道：

| 字段 | 值 |
|------|----|
| 渠道名称 | `Mock-Test` |
| 渠道类型 | `OpenAI` |
| Base URL | `http://localhost:19000` |
| API Key | 任意值（Mock 不校验，填 `sk-mock` 即可） |
| 模型 | 勾选你要压测的模型（如 `gpt-4o`、`dall-e-3`、`kling-video`） |
| 优先级 | 设为最高（确保请求优先路由到 Mock 渠道） |

### 步骤 3：运行压测

```bash
# 终端 2：运行压测（指向 team-api 服务，不是 Mock 服务器）
cd stress-test
go run . -url http://localhost:18888 -key "sk-your-api-key" -c 100 -d 3m
```

### Mock 模式下请求链路

```
压测客户端 → team-api (认证 → 限流 → 预扣 → 渠道选择)
  → Mock 服务器 (localhost:19000，模拟 SSE 流式响应)
    → team-api (结算 → 响应)
      → 压测客户端
```

整条内部链路（Redis Lua × 5-10 次、DB 查询 × 3-5 次、SSE 流式转发、计费结算）全部真实执行，但上游请求只到本地 Mock，**零外部流量、零费用**。

---

## 模式二：真实模式

直接压测，请求会到上游 AI 供应商，**产生实际费用**：

```bash
cd stress-test
go run . -key "sk-your-api-key" -c 10 -d 60s
```

---

## 压测工具参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-url` | `http://localhost:18888` | team-api 服务地址 |
| `-key` | （必填） | API Key（Bearer Token） |
| `-scene` | `chat` | 场景: `chat` / `image` / `video` / `mixed` |
| `-model` | `gpt-4o` | 模型名称（需与渠道中配置的一致） |
| `-c` | `10` | 并发客户端数 |
| `-d` | `60s` | 压测总时长 |
| `-n` | `0` | 总请求数（0=按时间停止） |
| `-ramp` | `0` | 并发递增时间（逐步拉满，如 `-ramp 2m`） |
| `-stream` | `true` | 流式响应（仅 chat） |
| `-timeout` | `120s` | 单请求超时 |
| `-v` | `false` | 详细日志 |
| `-output` | `./stress-test/reports` | 报告输出目录 |

## 渐进式压测策略

### 阶段一：基准测试（确认链路可用）

```bash
# 1 并发，3 个请求，验证认证、计费、流式转发正常
go run . -key "sk-xxx" -c 1 -n 3 -v
```

### 阶段二：线性爬坡（找到拐点）

```bash
# 逐步增加到 100 并发，持续 5 分钟
go run . -key "sk-xxx" -c 100 -ramp 2m -d 5m
```

### 阶段三：极限测试（确认天花板）

```bash
go run . -key "sk-xxx" -c 200 -d 3m
go run . -key "sk-xxx" -c 500 -d 3m
go run . -key "sk-xxx" -c 1000 -d 3m
```

### 阶段四：混合场景（模拟真实流量）

```bash
# chat 60% + image 30% + video 10%
go run . -key "sk-xxx" -scene mixed -c 100 -d 10m
```

### 阶段五：容错测试（模拟上游故障）

```bash
# 终端 1：启动带 10% 错误率的 Mock
cd mock && go run . -error-rate 0.1 -v

# 终端 2：压测，观察重试行为和结算正确性
cd stress-test && go run . -key "sk-xxx" -c 50 -d 3m -v
```

## 输出指标说明

| 指标 | 含义 |
|------|------|
| **QPS** | 每秒完成的请求数 |
| **成功率** | 成功请求 / 总请求 × 100% |
| **TTFB (P50/P90/P95)** | 首 Token 延迟，反映用户体感等待时间 |
| **延迟 (P50/P90/P99)** | 端到端请求完成耗时 |
| **Tokens/s** | Token 吞吐率 |
| **实时 QPS** | 滑动窗口（10s）计算的瞬时 QPS |

## 瓶颈排查方向

压测时关注以下瓶颈点，按优先级排序：

| 优先级 | 瓶颈 | 表现 | 排查方式 |
|--------|------|------|----------|
| 🔴 | **Redis Lua 脚本延迟** | P99 飙升但 P50 正常 | `redis-cli --latency-history` |
| 🔴 | **DB 连接池耗尽** | 请求排队超时 | PostgreSQL `pg_stat_activity` |
| 🟡 | **平台限流触发** | 特定租户/Key 返回 429 | 查看错误信息中的 rate limit 标识 |
| 🟡 | **并发控制** | 并发上不去 | 检查 Redis `conc:*` 计数器 |
| 🟡 | **Goroutine 泄漏** | 内存持续增长 | `pprof` goroutine profile |
| 🟢 | **渠道健康退化** | 重试率升高 | 查看渠道健康分数 |
| 🟢 | **上游限流（真实模式）** | 429 状态码激增 | 查看错误分布中 429 占比 |

## 文件结构

```
stress-test/
├── main.go          # 压测主入口（并发调度、梯度上升、实时显示）
├── config.go        # 压测参数配置
├── chat.go          # Chat 对话压测（SSE 流式 + 非流式）
├── image.go         # 图片生成压测
├── video.go         # 视频生成压测（异步提交 + 轮询）
├── reporter.go      # 指标收集、报告生成、JSON 导出
├── README.md        # 本文档
├── mock/            # 本地 Mock AI 服务器
│   ├── main.go      # Mock 服务器入口（路由注册、启动）
│   ├── config.go    # Mock 服务器配置
│   └── handlers.go  # Mock 响应处理器（SSE 流式、图片、视频、错误模拟）
└── reports/         # 压测报告输出目录（JSON 格式）
```

## 注意事项

1. **Mock 模式零费用**：Mock 服务器在本地运行，不会请求远端 AI 供应商，但 team-api 的计费流程仍会执行（预扣→结算→退款），需要有足够余额或使用测试租户
2. **真实模式产生费用**：请求到真实上游会产生 API 调用费用
3. **API Key 限流**：默认单 Key 60 QPS / 5 并发，高并发测试需要调大限流配置
4. **Ctrl+C 安全退出**：支持优雅中断，会打印已有的测试报告
5. **生产环境慎重**：对生产环境压测会影响真实用户

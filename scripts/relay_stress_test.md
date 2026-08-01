# Relay 层压力测试说明

对 `/v1/chat/completions`（OpenAI 兼容代理端点）进行压力测试，验证 relaykit 转换层在高并发下的稳定性与性能，并与旧 relay 代码路径对照。

> 适用阶段 6.4。本测试需要**可访问的运行中服务 + 可用的上游渠道/Key**，在本地开发环境无法真正执行；以下为执行手册。

## 前置

- 服务已启动（`gf run` 或部署实例），监听如 `http://localhost:8080`
- 一个有效的租户 API Key（`RELAY_API_KEY`）
- 目标模型对应的上游渠道已配置且健康
- 已安装 [wrk](https://github.com/wg/wrk)

## 基础压测

```bash
# 压 OpenAI 兼容模型（OpenAI 透传路径，无转换）
RELAY_API_KEY=sk-xxx RELAY_MODEL=gpt-4 \
  wrk -t4 -c100 -d30s --latency -s scripts/wrk_relay.lua http://localhost:8080

# 压 Claude（OpenAI↔Claude 转换路径）
RELAY_API_KEY=sk-xxx RELAY_MODEL=claude-3-5-sonnet \
  wrk -t4 -c100 -d30s --latency -s scripts/wrk_relay.lua http://localhost:8080

# 压流式
RELAY_API_KEY=sk-xxx RELAY_MODEL=gemini-2.0-flash RELAY_STREAM=1 \
  wrk -t4 -c100 -d30s --latency -s scripts/wrk_relay.lua http://localhost:8080
```

参数：`-t4` 4 线程，`-c100` 100 并发连接，`-d30s` 持续 30 秒，`--latency` 输出延迟分布。

## 新旧路径对照（核心目的）

relaykit 默认关闭（`relaykit.enabled: false`），所有请求走旧 relay 代码。对照步骤：

1. **旧路径基线**：保持 `relaykit.enabled: false`，重启服务，压测并记录指标。
2. **relaykit 路径**：在 `manifest/config/config.yaml` 设置 `relaykit.enabled: true` 并把目标供应商加入 `providers`（如 `["claude"]`），重启服务，相同参数再压一次。
3. **对比**：两次的 QPS、P95/P99 延迟、错误率、错误日志。

## 验收指标（阶段 6.4）

| 指标 | 目标 |
|------|------|
| QPS | 不低于旧路径的 95% |
| P95 延迟增加 | < 10ms（转换开销） |
| P99 延迟增加 | < 20ms |
| 错误率 | < 0.1% |
| 转换成功率（relaykit 路径） | ≥ 99.5%（见 `GetRelaykitConverterMetrics`） |

> 转换层本身开销见 Benchmark（`relaykit/relayconvert/internal/*/benchmark_test.go`）：单次请求转换 < 1µs 级、< 1KB 分配，远低于上游模型推理耗时，因此对端到端 QPS/延迟的影响应可忽略。

## 观测

- `monitor.GetRelaykitConverterMetrics()`：转换器累计成功/失败/平均耗时/错误率（`relaykit.enabled=true` 时才有数据）
- 服务日志 `[relaykit]` 前缀：转换失败回退旧路径的告警
- 错误率 / 延迟超阈值 → 参见 `docs/relaykit-gray-release-runbook.md` 回滚。

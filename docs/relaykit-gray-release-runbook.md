# Relaykit 灰度发布运维手册

> 阶段 6.6 交付物。relaykit 协议转换层从「特性开关默认关闭、与旧 relay 代码双轨并存」逐步放量到全量。本文档定义灰度策略、告警阈值、回滚步骤与监控 checklist。

## 背景与当前机制

relaykit 是 relay 层协议转换的模块化改造（OpenAI↔Claude/Gemini/Coze/Dify/Ollama）。阶段 4/5 已完成全部转换器迁移并接入宿主桥接，**所有新路径默认关闭**，请求走旧 relay 代码。

**特性开关（`manifest/config/config.yaml`）**：

```yaml
relaykit:
  enabled: false        # 全局开关
  providers: []         # 供应商白名单；为空 = 全部走旧路径
```

- `enabled: false`（默认）→ **零运行时影响**，所有请求走旧路径。
- `enabled: true` 且 `providers` 含某供应商 key（`claude`/`gemini`/`openai`/`coze`/`dify`/`ollama`）→ 该供应商的请求先尝试 relaykit 转换，**任何失败（无匹配转换器 / 解析失败 / 转换失败）自动回退旧路径**，请求不会因 relaykit 中断。

> ⚠️ **当前为二元开关（按供应商全量切换），不支持按流量百分比放量**（迁移计划文档中的 `traffic_percentage` 字段当前未实现）。灰度粒度 = 供应商维度。按供应商逐个放量即等价于渐进灰度。

## 灰度策略（按供应商逐步放量）

每个供应商独立灰度，互不影响。建议顺序（按流量/重要性）：

| 阶段 | providers 配置 | 持续 | 放行条件 |
|------|----------------|------|----------|
| 0 基线 | `enabled: false` | — | 采集旧路径 QPS/延迟/错误率基线 |
| 1 Claude | `["claude"]` | ≥ 24h | 转换成功率 ≥ 99.5%、P95 +< 10ms、错误率 < 0.1% |
| 2 +Gemini | `["claude","gemini"]` | ≥ 24h | 同上 |
| 3 +Coze/Dify/Ollama | `["claude","gemini","coze","dify","ollama"]` | ≥ 24h | 同上 |
| 4 全量 | 所有原生格式供应商 | 长期 | 稳定运行 ≥ 1 周 |

每阶段配置示例（编辑 `manifest/config/config.yaml` 后重启服务）：

```yaml
# 阶段 1：仅 Claude
relaykit:
  enabled: true
  providers:
    - claude

# 阶段 3：全部原生格式供应商
# relaykit:
#   enabled: true
#   providers:
#     - claude
#     - gemini
#     - coze
#     - dify
#     - ollama
```

> OpenAI 及其兼容透传供应商（DeepSeek/Azure/Qwen/… 约 22 个）`inbound==upstream`，无转换路径，relaykit 对它们无工作 —— 无需也无需加入 `providers`。

## 告警阈值与回滚

灰度期间持续观察 `monitor.GetRelaykitConverterMetrics()`（每分钟聚合落库 `ops_system_metrics`，`metric_type=relaykit_converter`）。

| 指标 | 阈值 | 动作 |
|------|------|------|
| 转换成功率 < 99.5% | 立即 | **回滚**（移除该供应商或关 enabled） |
| 错误率 > 0.5% | 立即 | **回滚** |
| P95 延迟增加 > 50ms | 告警 | 人工审查，评估是否回滚 |
| 出现 `[relaykit]` 转换失败回退日志突增 | 告警 | 排查转换器 bug |

**回滚步骤（< 5 分钟）**：

1. 编辑 `manifest/config/config.yaml`：把异常供应商从 `providers` 移除，或直接 `enabled: false`。
2. 重启服务（`systemctl restart team-api` 或滚动重启）。
3. 确认 `GetRelaykitConverterMetrics` 受影响供应商计数停止增长，错误率回落。
4. 服务日志确认相关请求恢复走旧 relay 路径（不再出现 `[relaykit]` 前缀）。

回滚无数据丢失、用户无感知（请求自动回退旧路径）。修复后可重新加入灰度。

## 监控 Checklist

灰度放量前 / 每阶段持续观察：

- [ ] `GetRelaykitConverterMetrics`：目标供应商的 `success`/`failed`/`error_rate`/`avg_duration_ms` 在预期范围
- [ ] 服务整体错误率、P95/P99 延迟与基线对比无明显回退
- [ ] `[relaykit] convert ... failed ... fallback to legacy` 日志频率（应接近 0；偶发可接受，因会回退）
- [ ] 计费用量（`bil_usage_logs`）与旧路径一致（token 数无异常偏差）
- [ ] 各端点功能手动抽测（流式 / 非流式 / 工具调用 / 多模态）

## 已知可接受偏差（灰度前需对齐，非阻断）

以下为特性开关默认 OFF 下记录的偏差，全量前应逐项确认或修正：

- **响应 ID/Created**：部分转换器用固定/实时时间戳，与旧路径略有差异（不影响功能与计费）。
- **Coze `user` 字段**：relaykit Meta 未暴露 tenant/user 上下文，用通用占位 `relay-user`，丢失 Coze 侧 per-用户归因。
- **Ollama generate（completions）/ embedding 未迁移**：仅 chat 路径走 relaykit，其余自动回退旧 adaptor。

## 阶段 7 前置条件

全部供应商灰度稳定 ≥ 2 周后，方可进入阶段 7（删除旧 relay 转换代码、移除特性开关、清理重复 DTO/辅助函数）。详见 `docs/relaykit-migration-plan.md` 阶段 7。

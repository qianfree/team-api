<script setup lang="ts">
import { ref } from 'vue'
import { IconExclamationCircle } from '@arco-design/web-vue/es/icon'
import { useFormValues } from './useSettings'
const values = useFormValues()
const showPolicyGuide = ref(false)
</script>

<template>
	<div class="tab-content">
		<!-- 自动探测 -->
		<div class="section">
			<div class="section-title">自动探测</div>
			<div class="switch-row">
				<span class="switch-label">渠道自动探测</span>
				<span class="switch-desc">定期向活跃渠道发送测试请求，检测连通性并更新健康度（会消耗少量 Token）。禁用渠道不自动探测，由管理员手动测试确认后再启用</span>
				<ASwitch
					:model-value="!!values['channel_auto_test_enabled']"
					@change="(v: string | number | boolean) => values['channel_auto_test_enabled'] = v"
				/>
			</div>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="探测间隔（分钟）">
					<AInputNumber
						:model-value="values['channel_auto_test_interval_minutes'] as number"
						@change="(v: number | undefined) => values['channel_auto_test_interval_minutes'] = v ?? 5"
						:min="1" :max="1440" style="width: 100%"
					/>
				</AFormItem>
			</div>
			<div class="section-desc">
				保存后即时生效，无需重启。探测是串行的（每渠道最多 30 秒），渠道多或上游超时多时单轮耗时会变长；
				上一轮未结束时下一轮自动跳过，因此间隔不宜小于「渠道数 × 30 秒」。
			</div>
		</div>

		<!-- 自动禁用 -->
		<div class="section">
			<div class="section-title">自动禁用</div>
			<div class="switch-row">
				<span class="switch-label">渠道自动禁用</span>
				<span class="switch-desc">渠道熔断持续超过路由策略 breaker.autoDisableAfterSeconds（默认 10 分钟）未恢复时自动禁用</span>
				<ASwitch
					:model-value="!!values['channel_auto_disable_enabled']"
					@change="(v: string | number | boolean) => values['channel_auto_disable_enabled'] = v"
				/>
			</div>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="健康快照保留天数">
					<AInputNumber
						:model-value="values['health_snapshot_retention_days'] as number"
						@change="(v: number | undefined) => values['health_snapshot_retention_days'] = v ?? 7"
						:min="1" :max="90" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- 路由策略 -->
		<div class="section">
			<div class="section-title section-title-with-help">
				路由策略
				<IconExclamationCircle class="help-icon" :size="14" @click="showPolicyGuide = true" />
			</div>
			<AFormItem label="路由策略覆盖（JSON）">
				<ATextarea
					:model-value="values['channel_routing_policy'] as string"
					@input="(v: string) => values['channel_routing_policy'] = v"
					:auto-size="{ minRows: 4, maxRows: 12 }"
					placeholder='{"tierFactors":{"secondary":0.3},"breaker":{"failThreshold":5}}'
					allow-clear
				/>
			</AFormItem>
			<div class="section-desc">
				渠道调度引擎策略，部分字段覆盖内置默认值，保存后 30 秒内热生效；留空使用全部默认。
				保存时会校验 JSON 合法性与取值范围。<b>此为高级功能，非必要不要修改</b>；
				点击标题旁的感叹号图标可查看字段说明与配置示例。
			</div>
		</div>

		<!-- 代理设置 -->
		<div class="section">
			<div class="section-title">代理设置</div>
			<AFormItem label="代理地址">
				<AInput
					:model-value="values['channel_proxy_url'] as string"
					@input="(v: string) => values['channel_proxy_url'] = v"
					placeholder="http://127.0.0.1:7890"
					allow-clear
				/>
			</AFormItem>
			<div class="section-desc">
				全局代理 URL，支持 http:// 和 socks5:// 协议。在渠道编辑中开启"使用代理"即可生效，保存后即时生效，无需重启。
			</div>
		</div>

		<!-- 同步图片异步化 -->
		<div class="section">
			<div class="section-title">同步图片异步化</div>
			<div class="switch-row">
				<span class="switch-label">同步图片厂商异步化</span>
				<span class="switch-desc">同步阻塞返回的图片厂商（OpenAI/DALL·E 等）走 /v1/images/generations/async 时改由后台 worker 池异步处理，客户端提交即拿 task_id 后轮询取图；关闭则该端点对同步厂商返回不支持</span>
				<ASwitch
					:model-value="!!values['sync_image_async_enabled']"
					@change="(v: string | number | boolean) => values['sync_image_async_enabled'] = v"
				/>
			</div>
			<div class="switch-row" style="margin-top: 12px">
				<span class="switch-label">图片 URL 转存对象存储</span>
				<span class="switch-desc">上游返回图片 URL 时下载并转存对象存储（返回 24h 稳定链接，需已配置存储）；关闭则直接透传上游 URL（部分厂商约 1h 过期）。b64_json 始终转存</span>
				<ASwitch
					:model-value="!!values['sync_image_rehost_url']"
					@change="(v: string | number | boolean) => values['sync_image_rehost_url'] = v"
				/>
			</div>
		</div>
		<!-- 路由策略功能说明弹窗 -->
		<AModal v-model:visible="showPolicyGuide" title="路由策略覆盖说明" :width="720" :footer="false">
			<div class="policy-guide">
				<div class="guide-warning">
					<IconExclamationCircle :size="16" class="warning-icon" />
					<div>
						<b>高级功能，非必要不要修改。</b>
						此处参数直接影响线上渠道调度行为，内置默认值已适用于绝大多数场景，调整前请先理解各参数含义。
					</div>
				</div>

				<p class="guide-intro">
					这是渠道调度引擎的总参数配置：调度器如何挑选渠道、失败如何重试、何时熔断，均由这份 JSON 控制。
					留空 = 全部使用内置默认值；填写的字段会浅覆盖到默认值之上（只写要改的字段）。
					保存时按 Schema 校验，非法配置直接拒绝；保存后最长 30 秒内热生效，无需重启。
				</p>

				<div class="guide-section">
					<div class="guide-title"><span class="guide-index">1</span>调度打分公式</div>
					<p>调度器为每个请求给候选渠道打分，按权重加权随机选择：</p>
					<p><code>W(c) = baseWeight × tierFactor × healthFactor × headroom^γ × costFactor × rampFactor × protoFactor</code></p>
				</div>

				<div class="guide-section">
					<div class="guide-title"><span class="guide-index">2</span>可覆盖字段与默认值</div>
					<div class="policy-table-wrap">
						<table class="policy-table">
							<thead>
								<tr>
									<th>字段组</th>
									<th>关键字段（默认值）</th>
									<th>作用</th>
								</tr>
							</thead>
							<tbody>
								<tr>
									<td>tierFactors</td>
									<td>primary 1.0 / secondary 0.15 / reserve 0.02</td>
									<td>渠道层级流量偏置：备用层级平时几乎不接流量，主级全挂时才扩组兜底</td>
								</tr>
								<tr>
									<td>health</td>
									<td>alpha 2 / latRefMs 3000</td>
									<td>成功率指数（越大越惩罚低成功率渠道）；延迟基准（超过开始降权）</td>
								</tr>
								<tr>
									<td>load</td>
									<td>gamma 2 / leaseSeconds 90</td>
									<td>余量指数（越满的渠道越少接新请求）；并发租约时长</td>
								</tr>
								<tr>
									<td>cost</td>
									<td>beta 0.5 / min 0.5 / max 2.0</td>
									<td>成本偏好（越大越偏向便宜渠道）及影响幅度上下限</td>
								</tr>
								<tr>
									<td>ramp</td>
									<td>windowSeconds 120 / floor 0.05</td>
									<td>新渠道、熔断恢复渠道的小流量爬坡</td>
								</tr>
								<tr>
									<td>binding</td>
									<td>ttlSeconds 1800 等</td>
									<td>会话→渠道亲和绑定的有效期与守卫阈值</td>
								</tr>
								<tr>
									<td>retry</td>
									<td>原地 2 / 换渠道 2 / 凭证轮换 1 次；总时限 30s</td>
									<td>失败重试预算、退避曲线（100~1000ms）、429 等待上限（2000ms）</td>
								</tr>
								<tr>
									<td>breaker</td>
									<td>渠道级 8 / 模型级 4 次；冷却 30~300s；autoDisableAfterSeconds 600</td>
									<td>熔断阈值与冷却；autoDisableAfterSeconds 即本页「渠道自动禁用」的持续时长阈值</td>
								</tr>
								<tr>
									<td>session</td>
									<td>headerName X-Session-Id 等</td>
									<td>会话亲和的解析来源（Claude metadata.user_id、Responses previous_response_id）</td>
								</tr>
								<tr>
									<td>replay</td>
									<td>unsafeModes / safeModes</td>
									<td>禁止盲目重放的端点（默认图片/视频生成，防止重复扣费）</td>
								</tr>
								<tr>
									<td>proto</td>
									<td>responsesMismatch 0.25 / chatBridgeMismatch 0.75</td>
									<td>入站协议与渠道能力不匹配时的降权因子</td>
								</tr>
							</tbody>
						</table>
					</div>
				</div>

				<div class="guide-section">
					<div class="guide-title"><span class="guide-index">3</span>书写规则</div>
					<ol class="guide-rules">
						<li><b>浅覆盖</b>：嵌套对象只覆盖写出的字段，如 <code>{ "breaker": { "failThreshold": 5 } }</code> 只改这一项，同组其余保持默认</li>
						<li><code>tierFactors</code> 按键合并：只写 <code>secondary</code> 不影响 <code>primary</code> / <code>reserve</code></li>
						<li>⚠️ 数组字段（<code>replay.unsafeModes</code> / <code>safeModes</code>）是<b>整体替换</b>：要改必须写全完整列表，漏一项等于把它移出保护</li>
						<li>字段名使用 camelCase（如 <code>latRefMs</code>，不是 <code>lat_ref_ms</code>）</li>
						<li>清空文本框 = 恢复全部内置默认（空串合法）</li>
					</ol>
				</div>

				<div class="guide-section">
					<div class="guide-title"><span class="guide-index">4</span>校验规则（保存时拦截）</div>
					<p>
						常见拒绝原因：<code>tierFactors.primary</code> ≤ 0 或层级名不合法（仅 primary / secondary / reserve）；
						<code>health.alpha</code> ≤ 0；<code>cost.min</code> &gt; <code>cost.max</code>；重试预算为负；
						<code>backoffMaxMs</code> &lt; <code>backoffBaseMs</code>；<code>cooldownMaxSeconds</code> &lt; <code>cooldownSeconds</code>；proto 因子不在 (0,1]。
					</p>
				</div>

				<div class="guide-example">
					<div class="guide-example-title">配置示例</div>
					<pre><code>{
  "version": 1,
  "tierFactors": { "secondary": 0.3 },
  "breaker": { "failThreshold": 5, "autoDisableAfterSeconds": 300 },
  "retry": { "totalDeadlineSeconds": 45, "failoverBudget": 3 },
  "binding": { "ttlSeconds": 900 }
}</code></pre>
					<p>
						含义：备用渠道流量偏置翻倍；60 秒窗口 5 次失败即熔断；熔断 5 分钟不恢复自动禁用；
						重试总时限放宽到 45 秒、最多换 3 个渠道；会话亲和绑定缩短到 15 分钟。
					</p>
					<p>再如 <code>{ "health": { "latRefMs": 1500 }, "cost": { "beta": 1.0 } }</code>：延迟超过 1.5 秒即开始降权、更偏向便宜渠道。</p>
				</div>
			</div>
		</AModal>
	</div>
</template>

<style scoped>
@import './common.css';
.section-desc {
	color: var(--color-text-3);
	font-size: 13px;
	line-height: 1.6;
	margin-top: -8px;
}
.section-title-with-help {
	display: flex;
	align-items: center;
	gap: 6px;
}
.help-icon {
	color: var(--color-text-3);
	cursor: pointer;
	transition: color 0.2s;
	flex: 0 0 auto;
}
.help-icon:hover {
	color: rgb(var(--arcoblue-6));
}

/* 路由策略说明弹窗（样式惯例对齐 ChannelsPage 调度说明弹窗） */
.policy-guide {
	color: var(--color-text-1);
	font-size: 13px;
	line-height: 1.7;
	max-height: 65vh;
	overflow-y: auto;
	padding-right: 6px;
}
.policy-guide .guide-intro {
	margin: 0 0 14px;
	color: var(--color-text-2);
}
.policy-guide .guide-section {
	margin-bottom: 16px;
}
.policy-guide .guide-title {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-bottom: 6px;
	font-weight: 600;
}
.policy-guide .guide-index {
	display: inline-flex;
	width: 22px;
	height: 22px;
	align-items: center;
	justify-content: center;
	flex: 0 0 22px;
	border-radius: 50%;
	color: rgb(var(--arcoblue-6));
	background: var(--color-primary-light-1);
	font-size: 12px;
}
.policy-guide p {
	margin: 0;
}
.policy-guide p + p {
	margin-top: 6px;
}
.policy-guide code {
	padding: 1px 4px;
	border-radius: 3px;
	color: rgb(var(--arcoblue-6));
	background: var(--color-primary-light-1);
}
.guide-warning {
	display: flex;
	gap: 8px;
	padding: 10px 12px;
	margin-bottom: 14px;
	border-radius: 6px;
	background: var(--color-warning-light-1);
}
.guide-warning .warning-icon {
	color: rgb(var(--warning-6));
	flex: 0 0 auto;
	margin-top: 3px;
}
.guide-rules {
	margin: 0;
	padding-left: 20px;
}
.guide-rules li {
	margin-bottom: 4px;
}
.policy-table-wrap {
	overflow-x: auto;
}
.policy-table {
	width: 100%;
	border-collapse: collapse;
	font-size: 12.5px;
}
.policy-table th,
.policy-table td {
	padding: 8px 10px;
	border-bottom: 1px solid var(--color-fill-2);
	text-align: left;
	vertical-align: top;
}
.policy-table th {
	font-weight: 600;
	white-space: nowrap;
	background: var(--color-fill-1);
}
.policy-table td:first-child {
	white-space: nowrap;
	font-weight: 500;
	color: var(--color-text-1);
}
.policy-guide .guide-example {
	padding: 14px 16px;
	border-radius: 6px;
	background: var(--color-fill-2);
}
.policy-guide .guide-example-title {
	margin-bottom: 6px;
	font-weight: 600;
}
.policy-guide .guide-example p {
	margin: 8px 0 0;
}
.policy-guide pre {
	margin: 8px 0 0;
	padding: 12px 14px;
	border-radius: 6px;
	background: var(--color-fill-1);
	overflow-x: auto;
	font-size: 12.5px;
}
.policy-guide pre code {
	padding: 0;
	background: none;
	color: inherit;
}
</style>

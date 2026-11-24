package jsplugin

import (
	"encoding/json"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
	relaycommon "github.com/qianfree/team-api/relay/common"
)

// 宿主固定用量词汇 → 计费信号映射（预扣 ratios / 结算 TaskInfo），桥与分发器
// （immediate 终态）共用同一口径。插件只报「用了多少」，计费模式（per_second 矩阵、
// 公式 u()、素材方案）消费哪些信号由宿主模型定价决定：
//   - output_seconds      → spec.duration      / MaterialUsage.OutputSeconds
//   - resolution          → spec.resolution    / MaterialUsage.Resolution
//   - output_count        → count              / （冻结 count）
//   - input_image_count   → spec.input_image_count   / MaterialUsage.InputImageCount
//   - input_video_seconds → spec.input_video_seconds / MaterialUsage.InputVideoSeconds
//   - input_audio_seconds → spec.input_audio_seconds / MaterialUsage.InputAudioSeconds
//   - input/output_tokens → TaskInfo.PromptTokens/CompletionTokens/TotalTokens（结算）
//   - generate_audio      → spec.generate_audio（bool）
// 全部数值键 + generate_audio(1/0) 同时进公式 u() 通道（formula_facts / MaterialUsage.Facts）。

// RatioKeyFormulaFacts 预扣 ratios 中公式 u() 事实集的键（计费层 mergeTaskFacts 读取）。
const RatioKeyFormulaFacts = "formula_facts"

// ParseHostUsage 解析并校验用量 JSON（空/null → nil,nil）。
func ParseHostUsage(raw json.RawMessage) (*contract.HostUsage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	obj, err := contract.ParseJSONObject(raw)
	if err != nil {
		return nil, err
	}
	return contract.ValidateHostUsage(obj)
}

// ApplyEstimateUsage 把预扣估算用量写入 ratios。
func ApplyEstimateUsage(ratios map[string]any, u *contract.HostUsage) {
	if u == nil || u.IsEmpty() {
		return
	}
	facts := u.Facts()
	if len(facts) > 0 {
		// map[string]any：与 billing_context JSONB 回读形态一致，预扣/结算两次求值同形
		m := make(map[string]any, len(facts))
		for k, v := range facts {
			m[k] = v
		}
		ratios[RatioKeyFormulaFacts] = m
	}
	if v, ok := u.Number(contract.UsageOutputSeconds); ok && v > 0 {
		ratios["spec.duration"] = v
	}
	if u.Resolution != "" {
		ratios["spec.resolution"] = u.Resolution
	}
	if v, ok := u.Number(contract.UsageOutputCount); ok && v > 0 {
		ratios["count"] = v
	}
	if v, ok := u.Number(contract.UsageInputImageCount); ok {
		ratios["spec.input_image_count"] = v
	}
	if v, ok := u.Number(contract.UsageInputVideoSeconds); ok {
		ratios["spec.input_video_seconds"] = v
	}
	if v, ok := u.Number(contract.UsageInputAudioSeconds); ok {
		ratios["spec.input_audio_seconds"] = v
	}
	if u.GenerateAudio != nil {
		ratios["spec.generate_audio"] = *u.GenerateAudio
	}
}

// ApplySettleUsage 把终态官方用量写入 TaskInfo（MaterialUsage + token 计数）。
func ApplySettleUsage(info *relaycommon.TaskInfo, u *contract.HostUsage) {
	if info == nil || u == nil || u.IsEmpty() {
		return
	}
	mu := &relaycommon.TaskMaterialUsage{Facts: u.Facts(), Resolution: u.Resolution}
	if v, ok := u.Number(contract.UsageOutputSeconds); ok {
		mu.OutputSeconds = v
	}
	if v, ok := u.Number(contract.UsageInputImageCount); ok {
		mu.InputImageCount = int(v)
	}
	if v, ok := u.Number(contract.UsageInputVideoSeconds); ok {
		mu.InputVideoSeconds = v
	}
	if v, ok := u.Number(contract.UsageInputAudioSeconds); ok {
		mu.InputAudioSeconds = v
	}
	info.MaterialUsage = mu
	in, hasIn := u.Number(contract.UsageInputTokens)
	out, hasOut := u.Number(contract.UsageOutputTokens)
	if hasIn || hasOut {
		info.PromptTokens = int(in)
		info.CompletionTokens = int(out)
		info.TotalTokens = int(in) + int(out)
	}
}

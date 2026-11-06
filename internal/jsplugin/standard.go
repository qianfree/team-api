package jsplugin

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// 标准任务端点提交链的解码计划合成：JS 插件模型经 /v1/videos、/v1/video/generations
// 提交时，任务管线（relay/handler.HandleTaskSubmit）在此完成「通用任务体 → 原生
// requestBody」的解码并合成 SubmitPlan——等价于 /pjs 分发器的 decodeRequest 环节，
// jsbridge 提交链由此无感复用（ValidateRequest 起的全链只认计划不认入口）。
// 插件按需经 meta.standardVideos 声明解码钩子；未声明 = 仅原生路由入口（调用方明确拒绝）。

// SynthesizeStandardPlan 调用插件 standardVideos 声明的解码钩子，合成标准端点的
// 解码计划。pluginKey 由调用方解析（渠道显式配置优先，缺省按模型解析——与
// HandleTaskSubmit 的平台判定同源，避免二次解析产生口径分歧）。
//   - (nil, nil)：插件未声明 standardVideos，不支持标准端点（调用方按不支持处理）；
//   - 错误：插件未激活/钩子执行失败/返回不合规（调用方 400 透出——预扣尚未发生，无资金副作用）。
//
// 与 /pjs 分发器的 decode 调用同口径：无渠道上下文（解码钩子内 utils.fetch 仅允许
// manifest.allowedHosts，且不可用相对路径），渠道相关信息（apiKey/baseURL）由
// buildSubmitRequest 阶段注入。
func SynthesizeStandardPlan(ctx context.Context, pluginKey, surface, path string, body []byte, model string) (*SubmitPlan, error) {
	plugin, ok := CurrentGeneration().Plugin(pluginKey)
	if !ok {
		return nil, gerror.Newf("js plugin %q is not active", pluginKey)
	}
	hook := plugin.Manifest.StandardVideos
	if hook == "" {
		return nil, nil
	}

	args, err := json.Marshal(contract.StandardEntryContext{
		Surface: surface,
		Path:    path,
		Body:    body,
		Model:   model,
	})
	if err != nil {
		return nil, err
	}
	out, err := plugin.Pool.Call(ctx, hook, args)
	if err != nil {
		return nil, err
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return nil, gerror.Wrapf(err, "%s: ", hook)
	}
	// 与分发器解码同校验：模型须在 meta.models 声明、kind=submit（标准端点无查询语义）
	decoded, err := contract.ValidateDecoded(raw, plugin.Manifest, false)
	if err != nil {
		return nil, gerror.Wrapf(err, "%s: ", hook)
	}
	return &SubmitPlan{
		Plugin:     plugin,
		NativePath: path,
		Decoded:    decoded,
	}, nil
}

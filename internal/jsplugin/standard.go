package jsplugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// 标准任务端点提交链的解码计划合成：JS 插件模型经 /v1/videos、/v1/video/generations
// 提交时，任务管线（relay/handler.HandleTaskSubmit）在此完成「通用任务体 → 原生
// requestBody」的解码并合成 SubmitPlan——等价于 /pjs 分发器的 decodeRequest 环节，
// jsbridge 提交链由此无感复用（ValidateRequest 起的全链只认计划不认入口）。
// 插件按需经 meta.standardVideos 声明解码钩子；未声明 = 仅原生路由入口（调用方明确拒绝）。

// ReferenceRoleError 插件未声明承接的参考素材角色（调用方映射为 reference_not_supported）。
type ReferenceRoleError struct {
	PluginKey string
	Role      string
}

func (e *ReferenceRoleError) Error() string {
	return fmt.Sprintf("reference role %q is not supported by plugin %q", e.Role, e.PluginKey)
}

// checkReferenceRoles 标准端点参考素材的插件能力拦截：通用任务体 references[] 中出现插件
// 未在 meta.referenceRoles 声明的角色即拒绝——未升级的插件只读兼容字段（首帧），放行会把
// 参考图/视频/音频静默丢弃。非 JSON / 无 references 时不拦截。
func checkReferenceRoles(m *contract.Manifest, pluginKey string, body []byte) error {
	var probe struct {
		References []struct {
			Role string `json:"role"`
		} `json:"references"`
	}
	if json.Unmarshal(body, &probe) != nil {
		return nil
	}
	for _, ref := range probe.References {
		if !m.SupportsReferenceRole(ref.Role) {
			return &ReferenceRoleError{PluginKey: pluginKey, Role: ref.Role}
		}
	}
	return nil
}

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
	if err := checkReferenceRoles(plugin.Manifest, pluginKey, body); err != nil {
		return nil, err
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

package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// 任务请求扩展字段 extra：客户端在通用任务体顶层携带 extra 对象，网关在适配器构建出上游
// 请求体后将其解包、平铺合并进上游 JSON 顶层（对象值递归深合并，数组/标量整体替换，
// null 删除该键——同 RFC 7386 merge-patch 语义），用于透传厂商私有控制参数。
//
// 计费安全：预扣按网关解析的标准字段估算，extra 若能改写时长/分辨率/档位/张数等计费
// 规格即可「付 5 秒的钱生成 15 秒」。故在预扣前做保护键校验：全局保护键 + 适配器经
// ExtraProtectedPathsProvider 声明的计费敏感路径，命中即 400（不发上游、无资金副作用）。

// ExtraFieldName 通用任务体中扩展字段的键名
const ExtraFieldName = "extra"

// 扩展字段错误码
const (
	ErrCodeInvalidExtra        = "invalid_extra"
	ErrCodeExtraFieldProtected = "extra_field_protected"
)

// globalExtraProtectedPaths 全部渠道共用的保护键：模型名（绕过模型路由与计价），
// 回调地址（上游回调携带上游 task_id，绕过网关轮询结算）
var globalExtraProtectedPaths = []string{"model", "callback_url"}

// ExtraProtectedPathsProvider 适配器可选能力：声明 extra 禁止改写的上游请求体路径
// （点分路径，如 "parameters.duration"），一般为影响计费规格的字段。
type ExtraProtectedPathsProvider interface {
	ExtraProtectedPaths() []string
}

// ParseTaskExtra 从通用任务体顶层取 extra 并校验为 JSON 对象。
// 未传 / null / 空对象返回 nil；数字以 json.Number 保真（避免大整数 seed 经 float64 失精）。
func ParseTaskExtra(raw json.RawMessage) (map[string]any, *TaskError) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	if trimmed[0] != '{' {
		return nil, &TaskError{StatusCode: http.StatusBadRequest, Message: "invalid extra: expected a JSON object", ErrCode: ErrCodeInvalidExtra}
	}
	extra, err := decodeJSONObject(trimmed)
	if err != nil {
		return nil, &TaskError{StatusCode: http.StatusBadRequest, Message: "invalid extra: " + err.Error(), ErrCode: ErrCodeInvalidExtra}
	}
	if len(extra) == 0 {
		return nil, nil
	}
	return extra, nil
}

// ValidateTaskExtra 校验 extra 未触碰保护路径（全局 + 适配器声明）。
func ValidateTaskExtra(extra map[string]any, adaptorProtected []string) *TaskError {
	if len(extra) == 0 {
		return nil
	}
	for _, list := range [][]string{globalExtraProtectedPaths, adaptorProtected} {
		for _, path := range list {
			if extraTouchesPath(extra, strings.Split(path, ".")) {
				return &TaskError{
					StatusCode: http.StatusBadRequest,
					Message:    fmt.Sprintf("extra must not set %q; use the standard request field instead", path),
					ErrCode:    ErrCodeExtraFieldProtected,
				}
			}
		}
	}
	return nil
}

// extraTouchesPath 判断合并 extra 是否会改写 segs 指向的路径：
// 精确命中，或在某一前缀处给出非对象值（整体替换会连带改写其下的保护键）。
func extraTouchesPath(extra map[string]any, segs []string) bool {
	cur := extra
	for i, seg := range segs {
		v, ok := cur[seg]
		if !ok {
			return false
		}
		if i == len(segs)-1 {
			return true
		}
		next, isObj := v.(map[string]any)
		if !isObj {
			return true
		}
		cur = next
	}
	return false
}

// MergeTaskExtra 把 extra 平铺深合并进适配器构建的上游请求体（须为 JSON 对象），返回新请求体。
func MergeTaskExtra(upstream io.Reader, extra map[string]any) (io.Reader, error) {
	raw, err := io.ReadAll(upstream)
	if err != nil {
		return nil, fmt.Errorf("read upstream body: %w", err)
	}
	if len(extra) == 0 {
		return bytes.NewReader(raw), nil
	}
	body, err := decodeJSONObject(bytes.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("upstream request body is not a JSON object")
	}
	deepMergeExtra(body, extra)
	out, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal merged body: %w", err)
	}
	return bytes.NewReader(out), nil
}

// deepMergeExtra 合并规则：双方均为对象 → 递归；extra 值为 null → 删除；其余 → 覆盖
func deepMergeExtra(dst, src map[string]any) {
	for k, v := range src {
		if v == nil {
			delete(dst, k)
			continue
		}
		if srcObj, ok := v.(map[string]any); ok {
			if dstObj, ok := dst[k].(map[string]any); ok {
				deepMergeExtra(dstObj, srcObj)
				continue
			}
		}
		dst[k] = v
	}
}

func decodeJSONObject(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}
	return obj, nil
}

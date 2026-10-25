// Package relay —— chat 请求中 /v1/files 引用的中转重写。
//
// 客户端在 image_url.url 里传 file-xxx（OpenAI 协议允许以文件 ID 引用
// purpose=vision 的上传文件）；网关侧 ID 上游不认识，转发前在线替换为
// 对象存储签名 URL。URL / data URI 引用一律原样透传（含站内签名 URL——
// 签名即鉴权，透传给上游拉取正是其用途）。
package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
)

// relayFileRefPattern file ID 引用形态：file- + 8~64 位字母数字。
// 不匹配 http(s)/data URI——它们本就是上游可消费的形态。
var relayFileRefPattern = regexp.MustCompile(`^file-[A-Za-z0-9]{8,64}$`)

// RewriteImageFileRefsInBody 扫描 chat 类请求体，把 messages 中
// image_url.url 的 file-xxx 引用替换为签名 URL（批量解析，重复引用只签一次）。
//
// 返回 (新 body, 是否有替换, 错误)。非 JSON / 无 messages / 无引用的请求体
// 原样返回（changed=false），保证绝大多数请求零开销且透传路径字节不变。
// 引用无法解析（不存在/跨租户）返回 ErrRelayFileRefUnknown——放行会把
// file-xxx 原样送上游，换来更难排查的上游报错。
func RewriteImageFileRefsInBody(ctx context.Context, tenantID int64, body []byte) ([]byte, bool, error) {
	var refs []string
	seen := make(map[string]struct{}, 2)
	walkImageFileURLs(body, func(url string) (string, bool) {
		if relayFileRefPattern.MatchString(url) {
			if _, ok := seen[url]; !ok {
				seen[url] = struct{}{}
				refs = append(refs, url)
			}
		}
		return url, false
	})
	if len(refs) == 0 {
		return body, false, nil
	}

	urls, err := ResolveRelayFileURLs(ctx, tenantID, refs)
	if err != nil {
		return body, false, err
	}

	newBody, changed := walkImageFileURLs(body, func(url string) (string, bool) {
		if target, ok := urls[url]; ok {
			return target, true
		}
		return url, false
	})
	return newBody, changed, nil
}

// walkImageFileURLs 单遍遍历 messages 分段内容中的 image_url.url，
// 以 transform 决定是否改写；任一处改写则重组 body 返回（changed=true），
// 否则原样返回入参字节。解析失败（非 JSON 对象 / 无 messages 数组 /
// multipart 等）等价于零命中，静默跳过——发现与改写共用同一判断口径。
func walkImageFileURLs(body []byte, transform func(url string) (string, bool)) ([]byte, bool) {
	// 快速通道：正文不含 "file- 字面量时必然零命中（file 引用只出现在字符串值里），
	// 跳过全量解析——透传模式保持零解析开销。普通对话文本偶含该子串时多付一次
	// 解析但结果不变。
	if !bytes.Contains(body, []byte(`"file-`)) {
		return body, false
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return body, false
	}
	messagesRaw, ok := root["messages"]
	if !ok {
		return body, false
	}

	var messages []json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return body, false
	}

	changed := false
	for i, msgRaw := range messages {
		var msg map[string]json.RawMessage
		if err := json.Unmarshal(msgRaw, &msg); err != nil {
			continue
		}
		contentRaw, ok := msg["content"]
		if !ok {
			continue
		}

		// content 可能是字符串或分段数组，只有数组形态能携带 image_url
		var parts []map[string]json.RawMessage
		if err := json.Unmarshal(contentRaw, &parts); err != nil {
			continue
		}

		partChanged := false
		for j, part := range parts {
			if string(part["type"]) != `"image_url"` {
				continue
			}
			imgURLRaw, ok := part["image_url"]
			if !ok {
				continue
			}
			var imgURL map[string]json.RawMessage
			if err := json.Unmarshal(imgURLRaw, &imgURL); err != nil {
				continue
			}
			urlRaw, ok := imgURL["url"]
			if !ok {
				continue
			}
			var url string
			if err := json.Unmarshal(urlRaw, &url); err != nil {
				continue
			}
			if target, rewritten := transform(url); rewritten {
				imgURL["url"], _ = json.Marshal(target)
				part["image_url"], _ = json.Marshal(imgURL)
				parts[j] = part
				partChanged = true
			}
		}

		if partChanged {
			msg["content"], _ = json.Marshal(parts)
			messages[i], _ = json.Marshal(msg)
			changed = true
		}
	}

	if !changed {
		return body, false
	}

	root["messages"], _ = json.Marshal(messages)
	result, err := json.Marshal(root)
	if err != nil {
		return body, false
	}
	return result, true
}

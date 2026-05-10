package zhipu

import (
	"encoding/json"
	"strings"
)

// stripImageURLPrefixes 遍历 messages 中的 image_url，去除 "data:image/...;base64," 前缀
// GLM 视觉模型要求纯 base64 数据，不接受 data URI 格式
func stripImageURLPrefixes(messagesRaw json.RawMessage) (json.RawMessage, bool) {
	var messages []json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return messagesRaw, false
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

		// content 可能是字符串或数组，只处理数组形式
		var contentParts []map[string]json.RawMessage
		if err := json.Unmarshal(contentRaw, &contentParts); err != nil {
			continue
		}

		partChanged := false
		for j, part := range contentParts {
			typeRaw, ok := part["type"]
			if !ok {
				continue
			}
			var partType string
			if err := json.Unmarshal(typeRaw, &partType); err != nil || partType != "image_url" {
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

			if strings.HasPrefix(url, "data:image/") {
				if idx := strings.Index(url, ","); idx != -1 {
					url = url[idx+1:]
					imgURL["url"], _ = json.Marshal(url)
					part["image_url"], _ = json.Marshal(imgURL)
					contentParts[j] = part
					partChanged = true
				}
			}
		}

		if partChanged {
			msg["content"], _ = json.Marshal(contentParts)
			messages[i], _ = json.Marshal(msg)
			changed = true
		}
	}

	if !changed {
		return messagesRaw, false
	}

	result, err := json.Marshal(messages)
	if err != nil {
		return messagesRaw, false
	}
	return result, true
}

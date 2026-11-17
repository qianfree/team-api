package volcengine

import "encoding/json"

// Ark 图像生成的参考图字段归一。
//
// 系统入站遵循网关 OpenAI 协议约定（relaykit/dto ImageRequest）：单张参考图放 image
// （string），多张放 images（string[]）。Ark /api/v3/images/generations 只认自己的
// image 字段——单值 string 或 [{type:"image_url",url}] 对象数组，images 字段会被
// 当未知参数忽略，多图垫图将静默丢失。此处在透传前把 images[] 归一为 Ark 形态。

// arkImageRef Ark 多图参考项（与官方文档 5.3 节数组格式一致）
type arkImageRef struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// normalizeArkReferenceImages 把 images[]（string 数组）转为 Ark 的 image 对象数组。
// image 已存在（单图约定/Ark 原生形态）时以 image 为准、丢弃 images；形态异常时
// 原体返回，交由上游校验报错。
func normalizeArkReferenceImages(body []byte) []byte {
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}
	imgsRaw, ok := req["images"]
	if !ok {
		return body
	}
	var imgs []string
	if err := json.Unmarshal(imgsRaw, &imgs); err != nil {
		return body
	}
	delete(req, "images")

	_, hasImage := req["image"]
	if !hasImage {
		refs := make([]arkImageRef, 0, len(imgs))
		for _, u := range imgs {
			if u == "" {
				continue
			}
			refs = append(refs, arkImageRef{Type: "image_url", URL: u})
		}
		if len(refs) > 0 {
			arr, err := json.Marshal(refs)
			if err != nil {
				return body
			}
			req["image"] = arr
		}
	}

	out, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return out
}

package jsbridge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/qianfree/team-api/internal/jsplugin"
)

// FileReference 内联（new-api __fileRef 对齐）：buildSubmitRequest 的 body 里可含
// 占位符对象 {__fileRef, encoding, mimeType?, maxBytes?}（任意深度），任务桥在
// 发送前用 SubmitPlan.Files 的字节替换——编码为 base64 或 dataUrl。
// 字节永不进入 JS；占位符字段白名单严格校验。

// inlineFileCaps 文件内联的绝对上限（对齐 new-api MAX_FILE_DOWNLOAD_MB；单文件
// maxBytes 占位声明可更严，不可更宽）。
const inlineFileCap = 64 << 20

// placeholderKeys 占位符对象允许的键（严格白名单）。
var placeholderKeys = map[string]struct{}{
	"__fileRef": {}, "encoding": {}, "mimeType": {}, "maxBytes": {},
}

// inlineFileRefs 遍历 body JSON 树替换全部占位符；无占位符时原样返回。
func inlineFileRefs(body []byte, files map[string]jsplugin.FilePayload) ([]byte, error) {
	if len(files) == 0 || !bytes.Contains(body, []byte("__fileRef")) {
		return body, nil
	}
	var tree any
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, gerror.Wrap(err, "请求体不是合法 JSON")
	}
	totalInlined := 0
	replaced, err := replacePlaceholders(tree, files, &totalInlined)
	if err != nil {
		return nil, err
	}
	return json.Marshal(replaced)
}

// replacePlaceholders 递归替换 map/数组深处的占位符对象。
func replacePlaceholders(node any, files map[string]jsplugin.FilePayload, totalInlined *int) (any, error) {
	switch typed := node.(type) {
	case map[string]any:
		// 占位符对象判定：含 __fileRef 键
		if _, isPlaceholder := typed["__fileRef"]; isPlaceholder {
			return encodePlaceholder(typed, files, totalInlined)
		}
		for k, v := range typed {
			replaced, err := replacePlaceholders(v, files, totalInlined)
			if err != nil {
				return nil, err
			}
			typed[k] = replaced
		}
		return typed, nil
	case []any:
		for i, v := range typed {
			replaced, err := replacePlaceholders(v, files, totalInlined)
			if err != nil {
				return nil, err
			}
			typed[i] = replaced
		}
		return typed, nil
	default:
		return node, nil
	}
}

// encodePlaceholder 单个占位符 → 编码字符串（base64 / dataUrl）。
func encodePlaceholder(placeholder map[string]any, files map[string]jsplugin.FilePayload, totalInlined *int) (string, error) {
	for k := range placeholder {
		if _, ok := placeholderKeys[k]; !ok {
			return "", gerror.Newf("占位符含未知字段 %q（仅允许 __fileRef/encoding/mimeType/maxBytes）", k)
		}
	}
	ref, _ := placeholder["__fileRef"].(string)
	if ref == "" {
		return "", gerror.New("占位符 __fileRef 为空")
	}
	payload, ok := files[ref]
	if !ok {
		return "", gerror.Newf("文件引用 %q 不存在（multipart 未上传该字段）", ref)
	}
	encoding, _ := placeholder["encoding"].(string)
	if encoding == "" {
		encoding = "base64"
	}
	if encoding != "base64" && encoding != "dataUrl" {
		return "", gerror.Newf("占位符 encoding %q 不合法（base64 或 dataUrl）", encoding)
	}
	if maxBytes, exists := placeholder["maxBytes"]; exists {
		if limit, ok := maxBytes.(float64); ok && limit > 0 && int64(len(payload.Data)) > int64(limit) {
			return "", gerror.Newf("文件 %s（%d 字节）超过占位符上限 %d", ref, len(payload.Data), int64(limit))
		}
	}
	if len(payload.Data) > inlineFileCap {
		return "", gerror.Newf("文件 %s 超过单文件 %dMB 上限", ref, inlineFileCap>>20)
	}
	*totalInlined += len(payload.Data)
	if *totalInlined > inlineFileCap {
		return "", gerror.Newf("内联总量超过 %dMB 上限", inlineFileCap>>20)
	}
	mimeType := payload.MimeType
	if declared, _ := placeholder["mimeType"].(string); declared != "" {
		mimeType = declared // 声明覆盖探测（如厂商要求特型 dataUrl）
	}
	switch encoding {
	case "base64":
		return base64.StdEncoding.EncodeToString(payload.Data), nil
	default: // dataUrl
		return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(payload.Data)), nil
	}
}

// hasFilePlaceholders 快速探测（观测/防御用）。
func hasFilePlaceholders(body []byte) bool {
	return bytes.Contains(body, []byte("__fileRef"))
}

var _ = strings.TrimSpace // 保持 strings 导入（encodePlaceholder 错误信息扩展时使用）

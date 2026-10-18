package helper

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"

	"github.com/qianfree/team-api/relay/common"
)

// RewriteAudioMultipartModel 在音频 multipart 请求体（/v1/audio/transcriptions、
// /v1/audio/translations 的 Whisper 风格表单）中把 model 标量字段替换为渠道映射后的
// 上游模型名。JSON 版模型映射（replaceModelIfNeeded / mapModelIfNeeded）对 multipart
// 体反序列化失败会静默原样返回，导致音频渠道的模型映射悄悄失效（客户端模型名直达
// 上游，配置了别名/接入点的渠道直接 404 或调错模型），故音频渠道须走本函数。
//
// 复用客户端原 boundary 重写，保证转发头里的 Content-Type（含 boundary）与重写后的
// 体保持匹配；未配置映射或任何解析/写出入错均回退原体（fail-open，仅损失映射、
// 不阻断请求）。文件 part（file 等）与其余字段字节原样复制。
func RewriteAudioMultipartModel(requestBody []byte, info *common.RelayInfo) []byte {
	if info == nil || info.ChannelMeta == nil || !info.ChannelMeta.IsModelMapped {
		return requestBody
	}

	var contentType string
	if info.RequestHeaders != nil {
		contentType = info.RequestHeaders.Get("Content-Type")
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		return requestBody
	}
	boundary := params["boundary"]

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.SetBoundary(boundary); err != nil {
		return requestBody
	}

	mr := multipart.NewReader(bytes.NewReader(requestBody), boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return requestBody
		}

		// model 标量字段：替换为上游模型名；文件 part 与其余字段原样复制
		if part.FormName() == "model" && part.FileName() == "" {
			if err := w.WriteField("model", info.ChannelMeta.UpstreamModelName); err != nil {
				return requestBody
			}
			continue
		}
		fw, err := w.CreatePart(part.Header)
		if err != nil {
			return requestBody
		}
		if _, err := io.Copy(fw, part); err != nil {
			return requestBody
		}
	}
	if err := w.Close(); err != nil {
		return requestBody
	}
	return buf.Bytes()
}

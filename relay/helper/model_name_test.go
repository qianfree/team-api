package helper

import (
	"encoding/json"
	"testing"
)

func TestReplaceModelName(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		modelName string
		want      string
	}{
		{
			name:      "Claude message_start 事件（模型映射回写客户端模型名）",
			body:      `{"type":"message_start","message":{"id":"msg_1","model":"deepseek-chat","usage":{"input_tokens":10}}}`,
			modelName: "deepseek-v4-flash",
			want:      `{"type":"message_start","message":{"id":"msg_1","model":"deepseek-v4-flash","usage":{"input_tokens":10}}}`,
		},
		{
			name:      "OpenAI 非流式响应",
			body:      `{"id":"chatcmpl-1","model":"gpt-4o-2024-08-06","choices":[]}`,
			modelName: "gpt-4o",
			want:      `{"id":"chatcmpl-1","model":"gpt-4o","choices":[]}`,
		},
		{
			name:      "多处 model 字段全部替换",
			body:      `{"model":"a","nested":{"model":"b"}}`,
			modelName: "c",
			want:      `{"model":"c","nested":{"model":"c"}}`,
		},
		{
			name:      "无 model 字段原样返回",
			body:      `{"id":"1","choices":[]}`,
			modelName: "x",
			want:      `{"id":"1","choices":[]}`,
		},
		{
			name:      "model 字段值缺少闭合引号（截断数据）原样保留剩余部分",
			body:      `{"model":"trunc`,
			modelName: "x",
			want:      `{"model":"trunc`,
		},
		{
			name:      "空输入",
			body:      ``,
			modelName: "x",
			want:      ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(ReplaceModelName([]byte(tt.body), tt.modelName))
			if got != tt.want {
				t.Errorf("ReplaceModelName() = %s, want %s", got, tt.want)
			}
			// 合法 JSON 输入替换后必须仍是合法 JSON（防止字节级替换破坏结构，
			// 客户端会对每个 SSE data 载荷做 JSON 解析）
			if json.Valid([]byte(tt.body)) && !json.Valid([]byte(got)) {
				t.Errorf("ReplaceModelName() 输出不是合法 JSON: %s", got)
			}
		})
	}
}

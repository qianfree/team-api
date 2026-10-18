package handler

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relay/constant"
)

// rawFrom 构造校验函数入参用的 rawRequest map
func rawFrom(t *testing.T, body string) map[string]json.RawMessage {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("测试体不是合法 JSON: %v", err)
	}
	return raw
}

// TestValidateEmbeddingsRequestParams embeddings 参数本地校验口径：
// input 形态、dimensions 正整数、encoding_format 枚举。
func TestValidateEmbeddingsRequestParams(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"input 字符串", `{"model":"e","input":"hello"}`, false},
		{"input 字符串数组", `{"model":"e","input":["a","b"]}`, false},
		{"input token 数组", `{"model":"e","input":[1,2,3]}`, false},
		{"input 二维 token 数组", `{"model":"e","input":[[1,2],[3]]}`, false},
		{"缺 input", `{"model":"e"}`, true},
		{"input 数字形态", `{"model":"e","input":123}`, true},
		{"input 对象形态", `{"model":"e","input":{"a":1}}`, true},
		{"dimensions 正数", `{"model":"e","input":"x","dimensions":256}`, false},
		{"dimensions 零", `{"model":"e","input":"x","dimensions":0}`, true},
		{"dimensions 负数", `{"model":"e","input":"x","dimensions":-1}`, true},
		{"encoding_format float", `{"model":"e","input":"x","encoding_format":"float"}`, false},
		{"encoding_format base64", `{"model":"e","input":"x","encoding_format":"base64"}`, false},
		{"encoding_format 非法值", `{"model":"e","input":"x","encoding_format":"hex"}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmbeddingsRequestParams(rawFrom(t, tt.body))
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEmbeddingsRequestParams() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateRerankRequestParams rerank 参数本地校验口径：
// query 非空、documents 非空数组且元素形态合法、数值字段下界。
func TestValidateRerankRequestParams(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"字符串文档", `{"model":"r","query":"q","documents":["a","b"]}`, false},
		{"对象文档", `{"model":"r","query":"q","documents":[{"text":"a"}]}`, false},
		{"带合法数值参数", `{"model":"r","query":"q","documents":["a"],"top_n":3,"max_chunks_per_doc":2,"overlap_tokens":10}`, false},
		{"缺 query", `{"model":"r","documents":["a"]}`, true},
		{"query 空串", `{"model":"r","query":"","documents":["a"]}`, true},
		{"缺 documents", `{"model":"r","query":"q"}`, true},
		{"documents 空数组", `{"model":"r","query":"q","documents":[]}`, true},
		{"documents 非数组", `{"model":"r","query":"q","documents":"a"}`, true},
		{"documents 元素数字形态", `{"model":"r","query":"q","documents":[1,2]}`, true},
		{"top_n 负数", `{"model":"r","query":"q","documents":["a"],"top_n":-1}`, true},
		{"max_chunks_per_doc 零", `{"model":"r","query":"q","documents":["a"],"max_chunks_per_doc":0}`, true},
		{"overlap_tokens 负数", `{"model":"r","query":"q","documents":["a"],"overlap_tokens":-5}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRerankRequestParams(rawFrom(t, tt.body))
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRerankRequestParams() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCheckModelCategoryMatchesMode 模型分类与端点匹配预校验：
// 仅 embeddings/rerank 入口生效，category 为空放行，匹配放行。
func TestCheckModelCategoryMatchesMode(t *testing.T) {
	tests := []struct {
		name     string
		category string
		mode     constant.RelayMode
		wantErr  bool
	}{
		{"embedding 模型调 embeddings", "embedding", constant.RelayModeEmbeddings, false},
		{"rerank 模型调 rerank", "rerank", constant.RelayModeRerank, false},
		{"chat 模型调 embeddings 拒绝", "chat", constant.RelayModeEmbeddings, true},
		{"chat 模型调 rerank 拒绝", "chat", constant.RelayModeRerank, true},
		{"embedding 模型调 rerank 拒绝", "embedding", constant.RelayModeRerank, true},
		{"category 为空放行", "", constant.RelayModeEmbeddings, false},
		{"chat 入口不校验", "embedding", constant.RelayModeChatCompletions, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkModelCategoryMatchesMode("test-model", tt.category, tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkModelCategoryMatchesMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

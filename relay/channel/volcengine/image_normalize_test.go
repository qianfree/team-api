package volcengine

import (
	"encoding/json"
	"testing"
)

// normalizeArkReferenceImages 把系统 images[]（string 数组）归一为 Ark image 对象数组
func TestNormalizeArkReferenceImages(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "images数组转为image对象数组",
			in:   `{"model":"seedream","prompt":"p","images":["https://a.com/1.png","data:image/png;base64,xxx"]}`,
			want: `{"image":[{"type":"image_url","url":"https://a.com/1.png"},{"type":"image_url","url":"data:image/png;base64,xxx"}],"model":"seedream","prompt":"p"}`,
		},
		{
			name: "image与images并存时以image为准",
			in:   `{"model":"m","image":"https://a.com/1.png","images":["https://a.com/2.png"]}`,
			want: `{"image":"https://a.com/1.png","model":"m"}`,
		},
		{
			name: "无images时原样返回",
			in:   `{"model":"m","prompt":"p","image":"https://a.com/1.png"}`,
			want: `{"model":"m","prompt":"p","image":"https://a.com/1.png"}`,
		},
		{
			name: "images非字符串数组时原样返回",
			in:   `{"model":"m","images":{"bad":true}}`,
			want: `{"model":"m","images":{"bad":true}}`,
		},
		{
			name: "空images数组被移除且不新增image",
			in:   `{"model":"m","prompt":"p","images":[]}`,
			want: `{"model":"m","prompt":"p"}`,
		},
		{
			name: "数组内空串项被跳过",
			in:   `{"model":"m","images":["","https://a.com/1.png"]}`,
			want: `{"image":[{"type":"image_url","url":"https://a.com/1.png"}],"model":"m"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(normalizeArkReferenceImages([]byte(tc.in)))
			var gotMap, wantMap map[string]any
			if err := json.Unmarshal([]byte(got), &gotMap); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.want), &wantMap); err != nil {
				t.Fatalf("want is not valid JSON: %v", err)
			}
			gotBytes, _ := json.Marshal(gotMap)
			wantBytes, _ := json.Marshal(wantMap)
			if string(gotBytes) != string(wantBytes) {
				t.Errorf("got %s, want %s", gotBytes, wantBytes)
			}
		})
	}
}

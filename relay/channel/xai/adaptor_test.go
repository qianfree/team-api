package xai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// pngMagic / jpegMagic 最小魔数字节，用于文件 part 的 MIME 嗅探测试。
var (
	pngMagic  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	jpegMagic = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
)

// xaiInfo 构造指定模式与 Content-Type 的 xAI 渠道 RelayInfo。
func xaiInfo(mode constant.RelayMode, contentType string, modelMapped bool) *common.RelayInfo {
	info := &common.RelayInfo{
		RelayMode:       int(mode),
		InboundFormat:   constant.RelayFormatOpenAI,
		OriginModelName: "grok-imagine-image-2.0",
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderXAI),
			BaseURL:           "https://api.x.ai",
			UpstreamModelName: "grok-imagine-image-2.0",
			IsModelMapped:     modelMapped,
			Settings:          common.ChannelSettings{},
		},
	}
	if contentType != "" {
		info.RequestHeaders = http.Header{"Content-Type": []string{contentType}}
	}
	if modelMapped {
		info.ChannelMeta.UpstreamModelName = "grok-imagine-image-2.0-upstream"
	}
	return info
}

// wantRequestError 断言 err 是 400 请求错误。
func wantRequestError(t *testing.T, err error) {
	t.Helper()
	var reqErr *constant.RelayError
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected *constant.RelayError, got %T: %v", err, err)
	}
	if reqErr.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", reqErr.StatusCode)
	}
}

// decodeConverted 读取 ConvertRequest 结果并反序列化为 map。
func decodeConverted(t *testing.T, r io.Reader) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("converted body is not JSON: %v\nbody: %s", err, raw)
	}
	return m
}

// multipartFile multipart 文件 part 描述。
type multipartFile struct {
	field      string
	filename   string
	contentTyp string
	data       []byte
}

// buildMultipart 构造 multipart 表单体，返回 body 与完整 Content-Type（含 boundary）。
func buildMultipart(t *testing.T, fields map[string]string, files []multipartFile) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for name, val := range fields {
		if err := w.WriteField(name, val); err != nil {
			t.Fatalf("write field %s failed: %v", name, err)
		}
	}
	for _, f := range files {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition",
			`form-data; name="`+f.field+`"; filename="`+f.filename+`"`)
		if f.contentTyp != "" {
			h.Set("Content-Type", f.contentTyp)
		}
		fw, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("create file part %s failed: %v", f.field, err)
		}
		if _, err := fw.Write(f.data); err != nil {
			t.Fatalf("write file part %s failed: %v", f.field, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func TestGetRequestURL_ImageModes(t *testing.T) {
	cases := []struct {
		name string
		mode constant.RelayMode
		want string
	}{
		{"images_generations", constant.RelayModeImagesGenerations, "https://api.x.ai/v1/images/generations"},
		{"images_edits", constant.RelayModeImagesEdits, "https://api.x.ai/v1/images/edits"},
		{"chat_fallback", constant.RelayModeChatCompletions, "https://api.x.ai/v1/chat/completions"},
	}
	for _, c := range cases {
		a := &Adaptor{}
		info := xaiInfo(c.mode, "", false)
		got, err := a.GetRequestURL(info)
		if err != nil {
			t.Fatalf("[%s] GetRequestURL error: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("[%s] GetRequestURL = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestConvertRequest_JSONEdits(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		modelMapped bool
		check       func(t *testing.T, m map[string]any)
		wantErr     bool
	}{
		{
			name: "image_url透传",
			body: `{"model":"grok-imagine-image-2.0","prompt":"a hat","image_url":"https://example.com/cat.jpg"}`,
			check: func(t *testing.T, m map[string]any) {
				if m["image_url"] != "https://example.com/cat.jpg" {
					t.Errorf("image_url = %v, want 原样", m["image_url"])
				}
			},
		},
		{
			name: "image字段URL转image_url",
			body: `{"model":"grok-imagine-image-2.0","prompt":"a hat","image":"https://example.com/cat.jpg"}`,
			check: func(t *testing.T, m map[string]any) {
				if m["image_url"] != "https://example.com/cat.jpg" {
					t.Errorf("image_url = %v, want URL 原样", m["image_url"])
				}
				if _, has := m["image"]; has {
					t.Error("image 字段应被删除")
				}
			},
		},
		{
			name:        "裸base64包装png",
			body:        `{"model":"grok-imagine-image-2.0","image":"iVBORw0KGgoAAAANSUhEUg"}`,
			modelMapped: false,
			check: func(t *testing.T, m map[string]any) {
				want := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg"
				if m["image_url"] != want {
					t.Errorf("image_url = %v, want %s", m["image_url"], want)
				}
			},
		},
		{
			name: "数组形态保持",
			body: `{"model":"grok-imagine-image-2.0","image":["https://a.com/1.jpg","data:image/png;base64,iVBOR"]}`,
			check: func(t *testing.T, m map[string]any) {
				arr, ok := m["image_url"].([]any)
				if !ok || len(arr) != 2 {
					t.Fatalf("image_url 应为 2 元素数组, got %#v", m["image_url"])
				}
				if arr[0] != "https://a.com/1.jpg" || arr[1] != "data:image/png;base64,iVBOR" {
					t.Errorf("数组元素错误: %#v", arr)
				}
			},
		},
		{
			name: "OpenAI专有字段剥离",
			body: `{"model":"grok-imagine-image-2.0","image":"https://a.com/1.jpg","size":"1024x1024","style":"vivid","user":"u1","n":2}`,
			check: func(t *testing.T, m map[string]any) {
				for _, f := range []string{"size", "style", "user"} {
					if _, has := m[f]; has {
						t.Errorf("字段 %s 应被剥离", f)
					}
				}
				if m["n"] != float64(2) {
					t.Errorf("n = %v, want 2", m["n"])
				}
			},
		},
		{
			name:        "模型映射叠加",
			body:        `{"model":"grok-imagine-image-2.0","image":"https://a.com/1.jpg"}`,
			modelMapped: true,
			check: func(t *testing.T, m map[string]any) {
				if m["model"] != "grok-imagine-image-2.0-upstream" {
					t.Errorf("model = %v, want 映射后的上游模型名", m["model"])
				}
			},
		},
		{
			name:    "缺参考图",
			body:    `{"model":"grok-imagine-image-2.0","prompt":"a hat"}`,
			wantErr: true,
		},
		{
			name:    "mask拒绝",
			body:    `{"model":"grok-imagine-image-2.0","image":"https://a.com/1.jpg","mask":"https://a.com/m.png"}`,
			wantErr: true,
		},
		{
			name:    "超过4张",
			body:    `{"image":["https://a.com/1.jpg","https://a.com/2.jpg","https://a.com/3.jpg","https://a.com/4.jpg","https://a.com/5.jpg"]}`,
			wantErr: true,
		},
		{
			name:    "非法JSON",
			body:    `{invalid`,
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &Adaptor{}
			r, err := a.ConvertRequest(nil, xaiInfo(constant.RelayModeImagesEdits, "application/json", c.modelMapped), []byte(c.body))
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				wantRequestError(t, err)
				return
			}
			if err != nil {
				t.Fatalf("ConvertRequest error: %v", err)
			}
			c.check(t, decodeConverted(t, r))
		})
	}
}

func TestConvertRequest_MultipartEdits(t *testing.T) {
	pngURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngMagic)
	jpegURI := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpegMagic)

	t.Run("文件part转dataURI并嗅探MIME", func(t *testing.T) {
		// CreateFormFile 形态：Content-Type 为 application/octet-stream，靠字节魔数嗅探
		body, ct := buildMultipart(t,
			map[string]string{"model": "grok-imagine-image-2.0", "prompt": "add a hat", "n": "2", "size": "1024x1024"},
			[]multipartFile{{field: "image", filename: "cat.png", data: pngMagic}},
		)
		a := &Adaptor{}
		r, err := a.ConvertRequest(nil, xaiInfo(constant.RelayModeImagesEdits, ct, false), body)
		if err != nil {
			t.Fatalf("ConvertRequest error: %v", err)
		}
		m := decodeConverted(t, r)
		if m["image_url"] != pngURI {
			t.Errorf("image_url = %v, want %s", m["image_url"], pngURI)
		}
		if m["n"] != float64(2) {
			t.Errorf("n = %v(%T), want 数字 2", m["n"], m["n"])
		}
		if _, has := m["size"]; has {
			t.Error("size 应被剥离")
		}
		if m["prompt"] != "add a hat" {
			t.Errorf("prompt = %v", m["prompt"])
		}
	})

	t.Run("声明Content-Type优先于魔数嗅探", func(t *testing.T) {
		// 字节是 jpeg 魔数但 part 声明 image/jpeg；用 png 魔数配声明 image/jpeg 验证声明优先
		body, ct := buildMultipart(t,
			map[string]string{"prompt": "x"},
			[]multipartFile{{field: "image[]", filename: "a.jpg", contentTyp: "image/jpeg", data: jpegMagic}},
		)
		a := &Adaptor{}
		r, err := a.ConvertRequest(nil, xaiInfo(constant.RelayModeImagesEdits, ct, false), body)
		if err != nil {
			t.Fatalf("ConvertRequest error: %v", err)
		}
		m := decodeConverted(t, r)
		if m["image_url"] != jpegURI {
			t.Errorf("image_url = %v, want %s", m["image_url"], jpegURI)
		}
	})

	t.Run("多文件part汇成数组", func(t *testing.T) {
		body, ct := buildMultipart(t,
			map[string]string{"prompt": "merge"},
			[]multipartFile{
				{field: "image[]", filename: "a.png", contentTyp: "image/png", data: pngMagic},
				{field: "image[]", filename: "b.jpg", contentTyp: "image/jpeg", data: jpegMagic},
			},
		)
		a := &Adaptor{}
		r, err := a.ConvertRequest(nil, xaiInfo(constant.RelayModeImagesEdits, ct, false), body)
		if err != nil {
			t.Fatalf("ConvertRequest error: %v", err)
		}
		m := decodeConverted(t, r)
		arr, ok := m["image_url"].([]any)
		if !ok || len(arr) != 2 {
			t.Fatalf("image_url 应为 2 元素数组, got %#v", m["image_url"])
		}
		if arr[0] != pngURI || arr[1] != jpegURI {
			t.Errorf("数组元素错误: %#v", arr)
		}
	})

	t.Run("字符串image字段传URL", func(t *testing.T) {
		body, ct := buildMultipart(t,
			map[string]string{"prompt": "x", "image": "https://example.com/cat.jpg"},
			nil,
		)
		a := &Adaptor{}
		r, err := a.ConvertRequest(nil, xaiInfo(constant.RelayModeImagesEdits, ct, false), body)
		if err != nil {
			t.Fatalf("ConvertRequest error: %v", err)
		}
		m := decodeConverted(t, r)
		if m["image_url"] != "https://example.com/cat.jpg" {
			t.Errorf("image_url = %v", m["image_url"])
		}
	})

	t.Run("字符串image_url字段归一化裸base64", func(t *testing.T) {
		body, ct := buildMultipart(t,
			map[string]string{"prompt": "x", "image_url": "iVBORw0KGgo"},
			nil,
		)
		a := &Adaptor{}
		r, err := a.ConvertRequest(nil, xaiInfo(constant.RelayModeImagesEdits, ct, false), body)
		if err != nil {
			t.Fatalf("ConvertRequest error: %v", err)
		}
		m := decodeConverted(t, r)
		if m["image_url"] != "data:image/png;base64,iVBORw0KGgo" {
			t.Errorf("image_url = %v", m["image_url"])
		}
	})

	t.Run("mask拒绝", func(t *testing.T) {
		body, ct := buildMultipart(t,
			map[string]string{"prompt": "x", "image": "https://a.com/1.jpg"},
			[]multipartFile{{field: "mask", filename: "m.png", contentTyp: "image/png", data: pngMagic}},
		)
		a := &Adaptor{}
		_, err := a.ConvertRequest(nil, xaiInfo(constant.RelayModeImagesEdits, ct, false), body)
		if err == nil {
			t.Fatal("expected error for mask, got nil")
		}
		wantRequestError(t, err)
	})

	t.Run("缺参考图拒绝", func(t *testing.T) {
		body, ct := buildMultipart(t, map[string]string{"prompt": "x"}, nil)
		a := &Adaptor{}
		_, err := a.ConvertRequest(nil, xaiInfo(constant.RelayModeImagesEdits, ct, false), body)
		if err == nil {
			t.Fatal("expected error for missing image, got nil")
		}
		wantRequestError(t, err)
	})

	t.Run("模型映射叠加", func(t *testing.T) {
		body, ct := buildMultipart(t,
			map[string]string{"model": "grok-imagine-image-2.0", "prompt": "x", "image": "https://a.com/1.jpg"},
			nil,
		)
		a := &Adaptor{}
		r, err := a.ConvertRequest(nil, xaiInfo(constant.RelayModeImagesEdits, ct, true), body)
		if err != nil {
			t.Fatalf("ConvertRequest error: %v", err)
		}
		m := decodeConverted(t, r)
		if m["model"] != "grok-imagine-image-2.0-upstream" {
			t.Errorf("model = %v, want 映射后的上游模型名", m["model"])
		}
	})
}

// TestConvertRequest_GenerationsNotAffected generations 模式保持原行为：
// 仅模型映射，不进 edits 归一化。
func TestConvertRequest_GenerationsNotAffected(t *testing.T) {
	a := &Adaptor{}
	body := `{"model":"grok-imagine-image-2.0","prompt":"a cat","n":1}`
	r, err := a.ConvertRequest(nil, xaiInfo(constant.RelayModeImagesGenerations, "application/json", false), []byte(body))
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	raw, _ := io.ReadAll(r)
	if string(raw) != body {
		t.Errorf("generations 体应原样返回, got %s", raw)
	}
}

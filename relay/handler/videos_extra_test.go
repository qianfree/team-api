package handler

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

func TestParseVideosCreateRequest_ExtraJSON(t *testing.T) {
	body := `{"model":"m","prompt":"p","extra":{"mode":"pro","vendor_big_id":9007199254740993}}`
	req, terr := ParseVideosCreateRequest([]byte(body), "application/json")
	if terr != nil {
		t.Fatalf("unexpected error: %+v", terr)
	}
	taskBody, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(taskBody, &out); err != nil {
		t.Fatal(err)
	}
	if got := string(out["extra"]); got != `{"vendor_big_id":9007199254740993}` {
		t.Fatalf("未知键须原样进 extra（已知键已提升）: %s", got)
	}
	var meta struct {
		Mode string `json:"mode"`
	}
	_ = json.Unmarshal(out["metadata"], &meta)
	if meta.Mode != "pro" {
		t.Fatalf("已知键 mode 应提升进 metadata: %s", out["metadata"])
	}
}

// extra 以 JSON 字符串传入（multipart 形态 / SDK extra_body 经 json.dumps）
func TestParseVideosCreateRequest_ExtraString(t *testing.T) {
	jsonBody := `{"model":"m","prompt":"p","extra":"{\"watermark\":false}"}`
	req, terr := ParseVideosCreateRequest([]byte(jsonBody), "application/json")
	if terr != nil || string(req.Extra) != `{"watermark":false}` {
		t.Fatalf("JSON 字符串形态: extra=%s terr=%+v", req.Extra, terr)
	}

	body, ct := buildMultipartVideosBody(t,
		map[string]string{"model": "m", "prompt": "p", "extra": `{"watermark":false}`}, "", "", nil)
	req, terr = ParseVideosCreateRequest(body, ct)
	if terr != nil || string(req.Extra) != `{"watermark":false}` {
		t.Fatalf("multipart 形态: extra=%s terr=%+v", req.Extra, terr)
	}
}

func TestParseVideosCreateRequest_ExtraEmptyOmitted(t *testing.T) {
	for _, body := range []string{
		`{"model":"m","prompt":"p","extra":{}}`,
		`{"model":"m","prompt":"p","extra":null}`,
		`{"model":"m","prompt":"p","extra":""}`,
	} {
		req, terr := ParseVideosCreateRequest([]byte(body), "application/json")
		if terr != nil {
			t.Fatalf("%s: %+v", body, terr)
		}
		taskBody, _ := BuildVideosTaskBody(req)
		var out map[string]json.RawMessage
		_ = json.Unmarshal(taskBody, &out)
		if _, ok := out["extra"]; ok {
			t.Fatalf("%s: 空 extra 不应写入任务体", body)
		}
	}
}

func TestParseVideosCreateRequest_ExtraInvalid(t *testing.T) {
	for _, body := range []string{
		`{"model":"m","prompt":"p","extra":[1]}`,
		`{"model":"m","prompt":"p","extra":1}`,
		`{"model":"m","prompt":"p","extra":"not json"}`,
		`{"model":"m","prompt":"p","extra":"[1]"}`,
	} {
		_, terr := ParseVideosCreateRequest([]byte(body), "application/json")
		if terr == nil || terr.StatusCode != 400 || terr.ErrCode != common.ErrCodeInvalidExtra {
			t.Fatalf("%s: 应 400 invalid_extra, got %+v", body, terr)
		}
	}
}

// /v1/videos 的 extra 已知键同样提升：mode 进任务体 metadata（可灵计费联动），
// 从 extra 剔除——修复此前 extra.mode 被保护名单拦截、可灵 pro 档无法设置的问题
func TestBuildVideosTaskBody_ExtraKnownKeyPromotion(t *testing.T) {
	body := `{"model":"kling-v2-master","prompt":"p","extra":{"mode":"pro","x_param":true}}`
	req, terr := ParseVideosCreateRequest([]byte(body), "application/json")
	if terr != nil {
		t.Fatal(terr)
	}
	taskBody, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Metadata struct {
			Mode string `json:"mode"`
		} `json:"metadata"`
		Extra map[string]json.RawMessage `json:"extra"`
	}
	if err := json.Unmarshal(taskBody, &out); err != nil {
		t.Fatal(err)
	}
	if out.Metadata.Mode != "pro" {
		t.Fatalf("mode 应提升进 metadata: %s", taskBody)
	}
	if _, still := out.Extra["mode"]; still {
		t.Fatalf("mode 应从 extra 剔除: %s", out.Extra)
	}
	if xp := out.Extra["x_param"]; string(xp) != `true` {
		t.Fatalf("未知键应留 extra: %s", out.Extra)
	}
}

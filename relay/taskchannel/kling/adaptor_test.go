package kling

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

// 档位计费只认 metadata.mode（与 BuildRequestBody 同源）：
// 顶层 mode 已随 /v1/video/generations 参数收敛废弃，不得再影响计费
func TestEstimateBilling_ModeFromMetadata(t *testing.T) {
	pro := (&KlingAdaptor{}).EstimateBilling(context.Background(), &common.RelayInfo{},
		[]byte(`{"model":"kling-v2-master","prompt":"p","seconds":"5","metadata":{"mode":"pro"}}`))
	if pro["quality"] != 3.5 {
		t.Fatalf("metadata.mode=pro 应加价 3.5x: %+v", pro)
	}

	std := (&KlingAdaptor{}).EstimateBilling(context.Background(), &common.RelayInfo{},
		[]byte(`{"model":"kling-v2-master","prompt":"p","seconds":"5","metadata":{"mode":"std"}}`))
	if _, ok := std["quality"]; ok {
		t.Fatalf("std 不应加价: %+v", std)
	}

	// 顶层 mode 不参与计费（防「按 pro 计费按 std 生成」的历史错位回归）
	ghost := (&KlingAdaptor{}).EstimateBilling(context.Background(), &common.RelayInfo{},
		[]byte(`{"model":"kling-v2-master","prompt":"p","seconds":"5","mode":"pro"}`))
	if _, ok := ghost["quality"]; ok {
		t.Fatalf("顶层 mode 不应影响计费: %+v", ghost)
	}
}

// 时长加价：metadata.duration 或顶层 seconds ≥10s 均 2x
func TestEstimateBilling_DurationMultiplier(t *testing.T) {
	long := (&KlingAdaptor{}).EstimateBilling(context.Background(), &common.RelayInfo{},
		[]byte(`{"model":"m","prompt":"p","seconds":"10"}`))
	if long["duration_multiplier"] != 2.0 {
		t.Fatalf("seconds=10 应 2x: %+v", long)
	}
	short := (&KlingAdaptor{}).EstimateBilling(context.Background(), &common.RelayInfo{},
		[]byte(`{"model":"m","prompt":"p","seconds":"5"}`))
	if _, ok := short["duration_multiplier"]; ok {
		t.Fatalf("5s 不应加价: %+v", short)
	}
}

// 尾帧：通用词汇 metadata.last_frame（references[].role=last_frame 双写产物）兜底进 image_tail，
// 可灵原生 image_tail 优先
func TestBuildRequestBody_LastFrameFallback(t *testing.T) {
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	body := []byte(`{"model":"kling-v2-master","prompt":"p","metadata":{"last_frame":"https://example.com/tail.png"}}`)
	r, err := (&KlingAdaptor{}).BuildRequestBody(context.Background(), info, body)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(r)
	var req klingRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if req.ImageTail != "https://example.com/tail.png" {
		t.Fatalf("last_frame 应兜底进 image_tail: %+v", req)
	}

	body = []byte(`{"model":"kling-v2-master","prompt":"p","metadata":{"image_tail":"https://a.png","last_frame":"https://b.png"}}`)
	r, _ = (&KlingAdaptor{}).BuildRequestBody(context.Background(), info, body)
	raw, _ = io.ReadAll(r)
	_ = json.Unmarshal(raw, &req)
	if req.ImageTail != "https://a.png" {
		t.Fatalf("原生 image_tail 应优先: %+v", req)
	}
}

// 水印开关：通用收敛词汇 metadata.watermark → 可灵原生 watermark_info.enabled
func TestBuildRequestBody_Watermark(t *testing.T) {
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	body := []byte(`{"model":"kling-v2-master","prompt":"p","metadata":{"watermark":true}}`)
	r, err := (&KlingAdaptor{}).BuildRequestBody(context.Background(), info, body)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(r)
	var req klingRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if req.WatermarkInfo == nil || !req.WatermarkInfo.Enabled {
		t.Fatalf("watermark_info.enabled 应为 true: %+v", req.WatermarkInfo)
	}

	// 未传时不输出 watermark_info
	body = []byte(`{"model":"kling-v2-master","prompt":"p"}`)
	r, _ = (&KlingAdaptor{}).BuildRequestBody(context.Background(), info, body)
	raw, _ = io.ReadAll(r)
	var reqNoWM klingRequest
	_ = json.Unmarshal(raw, &reqNoWM)
	if reqNoWM.WatermarkInfo != nil {
		t.Fatalf("未传 watermark 不应输出 watermark_info: %+v", reqNoWM.WatermarkInfo)
	}
}

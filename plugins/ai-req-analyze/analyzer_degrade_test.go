package aireqanalyze

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 降级分析测试：模拟内容安全审查拦截——请求体含特定标记文本时端点返回 400。

func newDegradeServer(t *testing.T, blockMarker string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), blockMarker) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"系统检测到输入或生成内容可能包含不安全或敏感内容"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"findings\":[{\"index\":1,\"dimension\":\"compliance\",\"severity\":\"low\",\"category\":\"other\",\"evidence\":\"正常内容证据\",\"explanation\":\"测试\"}],\"batch_summary\":\"单元正常\"}"}}],"usage":{"total_tokens":10}}`))
	}))
}

func makeUnit(kind string, anchor int64, text string) materializedUnit {
	return materializedUnit{
		su:   &SampledUnit{Kind: kind, AnchorLogID: anchor},
		text: text,
	}
}

func TestAnalyzeBatchDegradeOnReject(t *testing.T) {
	srv := newDegradeServer(t, "BLOCKME")
	defer srv.Close()

	cfg := testClientConfig(srv.URL)
	client := NewAIClient(cfg)

	// 两个单元：第一个正常，第二个含被拦截标记
	mats := []materializedUnit{
		makeUnit(UnitTypeSession, 101, "正常会话内容"),
		makeUnit(UnitTypeStandalone, 202, "BLOCKME 违规内容"),
	}

	res, err := analyzeBatch(context.Background(), client, cfg, mats)
	if err != nil {
		t.Fatalf("降级分析不应整体失败: %v", err)
	}

	// 正常单元的发现保留，index 重映射回原批序号（1 → 单元 1）
	if len(res.Findings) != 1 || res.Findings[0].Index != 1 {
		t.Fatalf("正常单元发现应保留且序号重映射: %+v", res.Findings)
	}
	// 被拒单元的失败明细记入 notes，含锚点定位
	if len(res.notes) != 1 || !strings.Contains(res.notes[0], "日志 202") {
		t.Fatalf("被拒单元应有含锚点的明细: %v", res.notes)
	}
	if !strings.Contains(res.BatchSummary, "逐单元降级") {
		t.Fatalf("批摘要应说明降级: %q", res.BatchSummary)
	}
}

func TestAnalyzeBatchDegradeAllRejected(t *testing.T) {
	// 全部单元被拒：不返回错误（降级吸收），notes 记录全部，摘要给出人工复核指引
	srv := newDegradeServer(t, "BLOCKME")
	defer srv.Close()

	cfg := testClientConfig(srv.URL)
	client := NewAIClient(cfg)

	mats := []materializedUnit{
		makeUnit(UnitTypeSession, 1, "BLOCKME a"),
		makeUnit(UnitTypeSession, 2, "BLOCKME b"),
	}

	res, err := analyzeBatch(context.Background(), client, cfg, mats)
	if err != nil {
		t.Fatalf("全部被拒也不应返回错误: %v", err)
	}
	if len(res.notes) != 2 || len(res.Findings) != 0 {
		t.Fatalf("应记录全部被拒单元且无发现: notes=%d findings=%d", len(res.notes), len(res.Findings))
	}
	if !strings.Contains(res.BatchSummary, "人工复核") {
		t.Fatalf("全拒摘要应引导人工复核: %q", res.BatchSummary)
	}
}

func TestAnalyzeBatchNoDegradeOnServerError(t *testing.T) {
	// 5xx（服务端故障）不降级：避免放大对故障端点的调用
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`bad gateway`))
	}))
	defer srv.Close()

	cfg := testClientConfig(srv.URL)
	client := NewAIClient(cfg)

	mats := []materializedUnit{
		makeUnit(UnitTypeSession, 1, "a"),
		makeUnit(UnitTypeSession, 2, "b"),
	}
	if _, err := analyzeBatch(context.Background(), client, cfg, mats); err == nil {
		t.Fatal("5xx 应返回错误而非降级")
	}
}

package admin

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
	relaycommon "github.com/qianfree/team-api/relay/common"
)

func TestNormalizeOpsStatsDays(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   int
		want int
	}{
		{in: 0, want: 30},
		{in: -5, want: 30},
		{in: 366, want: 30},
		{in: 1, want: 1},
		{in: 30, want: 30},
		{in: 365, want: 365},
	}
	for _, c := range cases {
		if got := normalizeOpsStatsDays(c.in); got != c.want {
			t.Errorf("normalizeOpsStatsDays(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestAggregateJsOpsStatsTasks(t *testing.T) {
	t.Parallel()

	t1 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 5, 12, 30, 0, 0, time.UTC)
	t3 := time.Date(2026, 11, 7, 8, 0, 0, 0, time.UTC)

	// 覆盖：多插件多状态、非 js 前缀行、空 key 行、LastTaskAt 跨组取最大
	rows := []jsOpsStatsTaskRow{
		{Platform: "js:veo", Status: string(relaycommon.TaskStatusSuccess), Count: 7, LastAt: t1},
		{Platform: "js:veo", Status: string(relaycommon.TaskStatusFailure), Count: 3, LastAt: t2},
		{Platform: "js:veo", Status: string(relaycommon.TaskStatusInProgress), Count: 2, LastAt: t3},
		{Platform: "js:veo", Status: string(relaycommon.TaskStatusSubmitted), Count: 1, LastAt: t1},
		{Platform: "js:kling-pro", Status: string(relaycommon.TaskStatusSuccess), Count: 1, LastAt: t1},
		{Platform: "sora", Status: string(relaycommon.TaskStatusSuccess), Count: 99, LastAt: t3}, // 非 js 前缀跳过
		{Platform: "js:", Status: string(relaycommon.TaskStatusSuccess), Count: 50, LastAt: t3},  // 空 key 跳过
	}
	out := aggregateJsOpsStatsTasks(rows)

	if len(out) != 2 {
		t.Fatalf("聚合后插件数 = %d, want 2（非 js 前缀与空 key 行须跳过）: %v", len(out), out)
	}
	veo := out["veo"]
	if veo == nil {
		t.Fatal("缺少 veo 聚合行")
	}
	if veo.Total != 13 || veo.Success != 7 || veo.Failed != 3 || veo.Processing != 3 {
		t.Errorf("veo 计数错误: total=%d success=%d failed=%d processing=%d, want 13/7/3/3",
			veo.Total, veo.Success, veo.Failed, veo.Processing)
	}
	if !veo.LastTaskAt.Equal(t3) {
		t.Errorf("veo LastTaskAt = %v, want 各组最大 %v", veo.LastTaskAt, t3)
	}
	kp := out["kling-pro"]
	if kp == nil || kp.Total != 1 || kp.Success != 1 || kp.Processing != 0 {
		t.Errorf("kling-pro 聚合错误: %+v", kp)
	}
}

func TestBuildJsOpsStatsModelMap(t *testing.T) {
	t.Parallel()

	manifests := []*contract.Manifest{
		{Key: "veo", Name: "Veo", Models: []string{"veo-3.0", "Veo-2.0"}},
		{Key: "", Name: "坏行", Models: []string{"bad-model"}}, // 空 key 跳过
		nil,
	}
	out := buildJsOpsStatsModelMap(manifests)

	if len(out) != 2 {
		t.Fatalf("模型映射数 = %d, want 2: %v", len(out), out)
	}
	if out["veo-3.0"] != "veo" || out["veo-2.0"] != "veo" {
		t.Errorf("模型映射应统一小写键: %v", out)
	}
}

func TestMergeJsOpsStatsRevenue(t *testing.T) {
	t.Parallel()

	modelPlugin := map[string]string{
		"veo-3.0": "veo",
		"veo-2.0": "veo",
		"abc-v1":  "abc",
	}
	rows := []jsOpsStatsRevenueRow{
		{ModelName: "veo-3.0", Cost: decimal.NewFromFloat(1.25)},
		{ModelName: "VEO-2.0", Cost: decimal.NewFromFloat(0.75)},    // 大小写漂移仍命中映射
		{ModelName: "unknown-model", Cost: decimal.NewFromFloat(9)}, // 未命中映射跳过
	}
	out := mergeJsOpsStatsRevenue(modelPlugin, rows)

	if len(out) != 1 {
		t.Fatalf("有营收的插件数 = %d, want 1: %v", len(out), out)
	}
	want := decimal.NewFromFloat(2)
	if !out["veo"].Equal(want) {
		t.Errorf("veo 营收 = %s, want 2", out["veo"].String())
	}
}

func TestJsOpsStatsSuccessRate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		success int64
		failed  int64
		want    float64
	}{
		{name: "常规成功率", success: 7, failed: 3, want: 0.7},
		{name: "全部成功", success: 5, failed: 0, want: 1},
		{name: "全部失败", success: 0, failed: 5, want: 0},
		{name: "分母 0（零任务）", success: 0, failed: 0, want: 0},
		{name: "分母 0（仅有进行中任务）", success: 0, failed: 0, want: 0},
	}
	for _, c := range cases {
		if got := jsOpsStatsSuccessRate(c.success, c.failed); got != c.want {
			t.Errorf("%s: jsOpsStatsSuccessRate(%d, %d) = %v, want %v",
				c.name, c.success, c.failed, got, c.want)
		}
	}
}

func TestBuildJsOpsStatsItems(t *testing.T) {
	t.Parallel()

	lastAt := time.Date(2026, 11, 6, 9, 15, 0, 0, time.UTC)
	manifests := []*contract.Manifest{
		{Key: "veo", Name: "Veo 视频", Models: []string{"veo-3.0"}},
		{Key: "idle", Name: "零任务插件", Models: []string{"idle-v1"}}, // 无任务也须列出
		{Key: "noname", Name: "", Models: []string{"noname-v1"}},  // name 空回落 key
	}
	inService := map[string]bool{"veo": true} // 仅 veo 在服务
	tasks := map[string]*jsOpsStatsTaskAgg{
		"veo":    {Total: 10, Success: 8, Failed: 2, Processing: 0, LastTaskAt: lastAt},
		"ghost":  {Total: 4, Success: 1, Failed: 1, Processing: 2, LastTaskAt: lastAt}, // 孤儿 platform
		"noname": {Total: 3, Success: 0, Failed: 0, Processing: 3},                     // 无终态：成功率 0
	}
	revenue := map[string]decimal.Decimal{
		"veo":   decimal.NewFromFloat(3.5),
		"ghost": decimal.NewFromFloat(1), // 实际不可能出现（孤儿无模型映射），验证字段透传
	}

	items := buildJsOpsStatsItems(jsOpsStatsAssembly{
		Manifests: manifests,
		InService: inService,
		Tasks:     tasks,
		Revenue:   revenue,
	})

	if len(items) != 4 {
		t.Fatalf("结果行数 = %d, want 4（manifest 全集 + 孤儿 platform）", len(items))
	}
	byKey := make(map[string]v1.JSPluginOpsStatsItem, len(items))
	for _, it := range items {
		byKey[it.PluginKey] = it
	}

	veo := byKey["veo"]
	if !veo.Enabled || veo.Orphan || veo.Name != "Veo 视频" {
		t.Errorf("veo 基础字段错误: %+v", veo)
	}
	if veo.TasksTotal != 10 || veo.TasksSuccess != 8 || veo.TasksFailed != 2 || veo.TasksProcessing != 0 {
		t.Errorf("veo 任务计数错误: %+v", veo)
	}
	if veo.SuccessRate != 0.8 {
		t.Errorf("veo success_rate = %v, want 0.8", veo.SuccessRate)
	}
	if veo.Revenue != 3.5 {
		t.Errorf("veo revenue = %v, want 3.5", veo.Revenue)
	}
	if veo.LastTaskTime != lastAt.Format(jsOpsStatsTimeLayout) {
		t.Errorf("veo last_task_time = %q, want %q", veo.LastTaskTime, lastAt.Format(jsOpsStatsTimeLayout))
	}

	idle := byKey["idle"]
	if idle.TasksTotal != 0 || idle.SuccessRate != 0 || idle.Revenue != 0 || idle.LastTaskTime != "" {
		t.Errorf("零任务插件应为全零行: %+v", idle)
	}
	if idle.Enabled || idle.Orphan {
		t.Errorf("零任务插件 enabled/orphan 应为 false: %+v", idle)
	}

	noname := byKey["noname"]
	if noname.Name != "noname" {
		t.Errorf("name 为空的插件应回落 key: %+v", noname)
	}
	if noname.TasksProcessing != 3 || noname.SuccessRate != 0 || noname.TasksTotal != 3 {
		t.Errorf("noname 仅有进行中任务：成功率应为 0: %+v", noname)
	}

	ghost := byKey["ghost"]
	if !ghost.Orphan || ghost.Name != "ghost" || ghost.Enabled {
		t.Errorf("孤儿 platform 行：name 应置 key、orphan=true、enabled=false: %+v", ghost)
	}
	if ghost.TasksTotal != 4 || ghost.TasksProcessing != 2 {
		t.Errorf("孤儿行任务计数错误: %+v", ghost)
	}

	// 排序：营收降序（veo 3.5 > ghost 1 > 0），同营收（idle/noname 均 0）按 key 升序
	wantOrder := []string{"veo", "ghost", "idle", "noname"}
	for i, key := range wantOrder {
		if items[i].PluginKey != key {
			t.Fatalf("排序错误：第 %d 位 = %s, want %s（全序: %v）", i, items[i].PluginKey, key, wantOrder)
		}
	}
}

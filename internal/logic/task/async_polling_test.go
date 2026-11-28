package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/relay/common"
)

// TestBuildTimeoutFailure_UsesOriginalStatusAsCASPredicate 锁定超时兜底网的回归 bug：
// UpdateTaskCAS 的 oldStatus 必须是覆盖前的真实状态，而不是已改写的 "FAILURE"。
func TestBuildTimeoutFailure_UsesOriginalStatusAsCASPredicate(t *testing.T) {
	for _, orig := range []string{"IN_PROGRESS", "QUEUED", "SUBMITTED", "NOT_START"} {
		task := &common.AsyncTask{PublicTaskID: "task_x", Status: orig}
		now := time.Now()

		oldStatus := buildTimeoutFailure(task, now)

		if oldStatus != orig {
			t.Fatalf("oldStatus = %q, want %q (must be the pre-mutation status, never \"FAILURE\")", oldStatus, orig)
		}
		if oldStatus == "FAILURE" {
			t.Fatal("regression: oldStatus must not be the mutated FAILURE value")
		}
		if task.Status != "FAILURE" {
			t.Fatalf("task.Status = %q, want FAILURE", task.Status)
		}
		if task.FailReason != "task timed out" {
			t.Fatalf("task.FailReason = %q, want 'task timed out'", task.FailReason)
		}
		if task.FinishTime == nil || !task.FinishTime.Equal(now) {
			t.Fatal("task.FinishTime must be set to now")
		}
	}
}

// TestPollStateChanged 状态变化判定（轮询调试记录的节流依据）：
// 状态或进度任一相对任务行当前持久化值变化才算变化，其余忽略
func TestPollStateChanged(t *testing.T) {
	task := &common.AsyncTask{Status: "IN_PROGRESS", Progress: "50%"}

	// 完全一致 → 未变化
	if pollStateChanged(&common.TaskInfo{Status: "IN_PROGRESS", Progress: "50%"}, task) {
		t.Error("状态与进度均一致时不应视为变化")
	}
	// 状态变化（终态转移）
	if !pollStateChanged(&common.TaskInfo{Status: "SUCCESS", Progress: "50%"}, task) {
		t.Error("状态变化应视为变化")
	}
	// 仅进度变化
	if !pollStateChanged(&common.TaskInfo{Status: "IN_PROGRESS", Progress: "60%"}, task) {
		t.Error("进度变化应视为变化")
	}
	// 空进度 → 有进度也算变化（提交后首次轮询）
	if !pollStateChanged(&common.TaskInfo{Status: "IN_PROGRESS", Progress: "10%"},
		&common.AsyncTask{Status: "SUBMITTED", Progress: ""}) {
		t.Error("空进度到有进度应视为变化")
	}
}

// TestBackfillPollProgress 非终态未上报进度时的回填：
// 空值直落会让查询端点的 progress 字段随轮询消失；从未上报的任务在
// IN_PROGRESS 期给非零兜底（恒 0% 会被用户误读为卡死）
func TestBackfillPollProgress(t *testing.T) {
	// 插件本轮未上报：保持任务行上一轮值（曾上报后停报，不让进度回跳）
	info := &common.TaskInfo{Status: common.TaskStatusInProgress}
	backfillPollProgress(info, &common.AsyncTask{Progress: "42%"})
	if info.Progress != "42%" {
		t.Errorf("未上报时应保持上一轮值 42%%，得到 %q", info.Progress)
	}
	// 从未上报（提交落库的 "0%" 与历史空值行）：IN_PROGRESS 给非零兜底
	for _, persisted := range []string{"", "0%"} {
		info = &common.TaskInfo{Status: common.TaskStatusInProgress}
		backfillPollProgress(info, &common.AsyncTask{Progress: persisted})
		if info.Progress != noReportInProgressProgress {
			t.Errorf("从未上报的 IN_PROGRESS 应回落兜底 %q（落库 %q），得到 %q",
				noReportInProgressProgress, persisted, info.Progress)
		}
	}
	// 插件已上报：原样保留，不被任务行旧值覆盖
	info = &common.TaskInfo{Status: common.TaskStatusInProgress, Progress: "37%"}
	backfillPollProgress(info, &common.AsyncTask{Progress: "42%"})
	if info.Progress != "37%" {
		t.Errorf("已上报进度不应被覆盖，得到 %q", info.Progress)
	}
	// 早期状态（SUBMITTED）：保持 0%，不提前跳进度
	info = &common.TaskInfo{Status: common.TaskStatusSubmitted}
	backfillPollProgress(info, &common.AsyncTask{Progress: "0%"})
	if info.Progress != "0%" {
		t.Errorf("SUBMITTED 应保持 0%%，得到 %q", info.Progress)
	}
	// 终态不回填：失败任务保持空进度语义（对齐原生适配器）
	info = &common.TaskInfo{Status: common.TaskStatusFailure}
	backfillPollProgress(info, &common.AsyncTask{Progress: "42%"})
	if info.Progress != "" {
		t.Errorf("终态不应回填进度，得到 %q", info.Progress)
	}
}

// TestPollDebugLogEnabled 轮询调试开关判定：JSONB 解析 + 目标过滤 AND 组合
func TestPollDebugLogEnabled(t *testing.T) {
	task := &common.AsyncTask{TenantID: 1, UserID: 2, ApiKeyID: 3}

	// 空/非法 settings 视为未开启
	if pollDebugLogEnabled(nil, task) || pollDebugLogEnabled([]byte("not-json"), task) {
		t.Error("空或非法 settings 应视为未开启")
	}
	// 未开启
	off, _ := json.Marshal(common.ChannelSettings{DebugLogEnabled: false})
	if pollDebugLogEnabled(off, task) {
		t.Error("DebugLogEnabled=false 应不开启")
	}
	// 开启且无过滤 → 匹配
	on, _ := json.Marshal(common.ChannelSettings{DebugLogEnabled: true})
	if !pollDebugLogEnabled(on, task) {
		t.Error("开启且无过滤应匹配")
	}
	// 开启但租户不匹配 → 拒绝
	filtered, _ := json.Marshal(common.ChannelSettings{DebugLogEnabled: true, DebugLogTenantID: 99})
	if pollDebugLogEnabled(filtered, task) {
		t.Error("租户不匹配应拒绝")
	}
}

// stubRecalculator resolveSuccessCost 的重算桩：记录调用次数，按预设返回。
type stubRecalculator struct {
	materialCost  decimal.Decimal
	materialOK    bool
	materialErr   error
	tokenCost     decimal.Decimal
	materialCalls int
	tokenCalls    int
}

func (s *stubRecalculator) RecalculateByMaterials(_ context.Context, _ int64, _ string, _ map[string]any, _ *common.TaskMaterialUsage, _ time.Time) (decimal.Decimal, bool, error) {
	s.materialCalls++
	return s.materialCost, s.materialOK, s.materialErr
}

func (s *stubRecalculator) RecalculateByTokens(_ context.Context, _ int64, _ string, _ int, _ map[string]any, _ time.Time) (decimal.Decimal, error) {
	s.tokenCalls++
	return s.tokenCost, nil
}

// TestResolveSuccessCost 成功终态取费的优先级：上游直接报价 → 素材重算 → token 重算 → 预扣额。
// 素材重算不适用或出错时不短路，继续尝试 token 重算——JS 任务插件只要上报了用量就带
// MaterialUsage（哪怕只报 token 计量），此前「有 MaterialUsage 即短路」让 token 计价模型
// 的 token 重算从未执行、按预扣额成交。
func TestResolveSuccessCost(t *testing.T) {
	preDeduct := decimal.NewFromFloat(0.1)
	newTask := func() *common.AsyncTask {
		return &common.AsyncTask{
			PublicTaskID: "task_x", ModelName: "m", TenantID: 1,
			PreDeductAmount: preDeduct, CreatedAt: time.Now(),
		}
	}
	ratios := map[string]any{}
	// JS 插件只报 token 计量：MaterialUsage 必然非 nil（Facts 携带 token 数）
	tokenUsage := &common.TaskInfo{
		Status:        common.TaskStatusSuccess,
		MaterialUsage: &common.TaskMaterialUsage{Facts: map[string]float64{"output_tokens": 200000}},
		TotalTokens:   200000,
	}

	t.Run("只报 token 计量：素材重算不适用后走 token 重算", func(t *testing.T) {
		stub := &stubRecalculator{tokenCost: decimal.NewFromFloat(3)}
		cost := resolveSuccessCost(context.Background(), stub, newTask(), ratios, tokenUsage)
		if !cost.Equal(decimal.NewFromFloat(3)) {
			t.Fatalf("应取 token 重算额 3，got %s", cost)
		}
		if stub.materialCalls != 1 || stub.tokenCalls != 1 {
			t.Fatalf("素材、token 重算各应调用一次，got %d/%d", stub.materialCalls, stub.tokenCalls)
		}
	})

	t.Run("素材重算适用即采用，不再走 token 重算", func(t *testing.T) {
		stub := &stubRecalculator{
			materialCost: decimal.NewFromFloat(5), materialOK: true, tokenCost: decimal.NewFromFloat(3),
		}
		cost := resolveSuccessCost(context.Background(), stub, newTask(), ratios, tokenUsage)
		if !cost.Equal(decimal.NewFromFloat(5)) {
			t.Fatalf("应取素材重算额 5，got %s", cost)
		}
		if stub.tokenCalls != 0 {
			t.Fatalf("素材重算适用时不应再走 token 重算，got %d 次", stub.tokenCalls)
		}
	})

	t.Run("素材重算出错仍尝试 token 重算", func(t *testing.T) {
		stub := &stubRecalculator{materialErr: errors.New("price lookup failed"), tokenCost: decimal.NewFromFloat(3)}
		cost := resolveSuccessCost(context.Background(), stub, newTask(), ratios, tokenUsage)
		if !cost.Equal(decimal.NewFromFloat(3)) {
			t.Fatalf("素材重算出错后应取 token 重算额 3，got %s", cost)
		}
	})

	t.Run("都未命中保持预扣额", func(t *testing.T) {
		stub := &stubRecalculator{}
		cost := resolveSuccessCost(context.Background(), stub, newTask(), nil,
			&common.TaskInfo{Status: common.TaskStatusSuccess, MaterialUsage: &common.TaskMaterialUsage{}})
		if !cost.Equal(preDeduct) {
			t.Fatalf("应保持预扣额 %s，got %s", preDeduct, cost)
		}
		if stub.materialCalls != 1 || stub.tokenCalls != 0 {
			t.Fatalf("无 token 计量不应走 token 重算，got %d/%d", stub.materialCalls, stub.tokenCalls)
		}
	})

	t.Run("上游直接报价优先", func(t *testing.T) {
		stub := &stubRecalculator{materialCost: decimal.NewFromFloat(5), materialOK: true, tokenCost: decimal.NewFromFloat(3)}
		cost := resolveSuccessCost(context.Background(), stub, newTask(), ratios, &common.TaskInfo{
			Status: common.TaskStatusSuccess, ActualCost: 7, MaterialUsage: &common.TaskMaterialUsage{}, TotalTokens: 100,
		})
		if !cost.Equal(billing.NewFromFloat(7)) {
			t.Fatalf("应取上游直接报价 7，got %s", cost)
		}
		if stub.materialCalls != 0 || stub.tokenCalls != 0 {
			t.Fatalf("上游直接报价时不应重算，got %d/%d", stub.materialCalls, stub.tokenCalls)
		}
	})
}

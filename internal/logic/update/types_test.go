package update

import (
	"testing"
	"time"
)

// 使用独立实例而非全局单例，避免测试间状态污染
func newTestManager() *UpdateManager {
	return &UpdateManager{}
}

func TestUpdateManager_默认状态(t *testing.T) {
	m := newTestManager()

	status := m.GetStatus()
	if status.Updating {
		t.Error("初始状态不应处于更新中")
	}
	if status.Progress != nil {
		t.Errorf("初始状态不应有进度信息，实际 %+v", status.Progress)
	}
	if m.IsUpdating() {
		t.Error("初始 updating 锁应为 false")
	}
	if m.GetCheckResult() != nil {
		t.Error("初始检查结果应为空")
	}
}

func TestUpdateManager_进度状态机(t *testing.T) {
	m := newTestManager()

	m.setProgress(PhaseDownloading, "正在下载...", 50)
	status := m.GetStatus()
	if !status.Updating {
		t.Error("设置进度后应处于更新中")
	}
	if status.Progress == nil || status.Progress.Phase != PhaseDownloading || status.Progress.Percentage != 50 {
		t.Errorf("进度未正确记录: %+v", status.Progress)
	}

	// 多次 setProgress 应覆盖同一份状态快照
	m.setProgress(PhaseVerifying, "正在校验...", 80)
	if p := m.GetStatus().Progress; p.Phase != PhaseVerifying || p.Percentage != 80 {
		t.Errorf("进度覆盖失败: %+v", p)
	}
}

func TestUpdateManager_失败收尾(t *testing.T) {
	m := newTestManager()

	m.setProgress(PhaseDownloading, "正在下载...", 50)
	m.setProgressError("文件校验失败，更新包可能已破损", "SHA256 校验不一致")

	status := m.GetStatus()
	if status.Updating {
		t.Error("失败后 Updating 应复位为 false")
	}
	if status.Progress == nil || status.Progress.Phase != PhaseFailed {
		t.Fatalf("失败后进度应标记 PhaseFailed: %+v", status.Progress)
	}
	if status.Progress.Message != "文件校验失败，更新包可能已破损" {
		t.Errorf("失败提示未记录: %q", status.Progress.Message)
	}
	if status.Progress.Error == "" {
		t.Error("失败详情未记录")
	}
	if m.IsUpdating() {
		t.Error("失败后 updating 锁应释放，允许再次发起更新")
	}
}

func TestUpdateManager_更新锁互斥(t *testing.T) {
	m := newTestManager()

	if !m.updating.CompareAndSwap(false, true) {
		t.Fatal("首次抢锁应成功")
	}
	if m.updating.CompareAndSwap(false, true) {
		t.Fatal("更新进行中再次抢锁应失败")
	}
	if !m.IsUpdating() {
		t.Error("抢锁后 IsUpdating 应为 true")
	}

	m.updating.Store(false)
	if m.IsUpdating() {
		t.Error("释放后 IsUpdating 应为 false")
	}
}

func TestGetStatus_终态进度过期自动失效(t *testing.T) {
	m := newTestManager()

	// 上次升级留下的过期 complete 快照：不应再下发
	stale := time.Now().Add(-(progressExpire + time.Minute))
	m.status.Store(&Status{
		CurrentVersion: "0.2.7",
		Updating:       false,
		Progress:       &Progress{Phase: PhaseComplete, Message: "更新完成", Percentage: 100, FinishedAt: &stale},
	})

	status := m.GetStatus()
	if status.Progress != nil {
		t.Errorf("过期的终态进度不应返回: %+v", status.Progress)
	}
	if status.CurrentVersion != "0.2.7" {
		t.Errorf("剔除过期进度不应影响其余状态字段: %+v", status)
	}

	// 过期的 failed 同样失效
	m.status.Store(&Status{
		Updating: false,
		Progress: &Progress{Phase: PhaseFailed, Message: "失败", FinishedAt: &stale},
	})
	if got := m.GetStatus().Progress; got != nil {
		t.Errorf("过期的 failed 进度不应返回: %+v", got)
	}
}

func TestGetStatus_终态进度窗口内保留(t *testing.T) {
	m := newTestManager()

	// 刚完成的 complete：管理员仍在轮询确认窗口内，应正常返回
	fresh := time.Now().Add(-1 * time.Minute)
	m.status.Store(&Status{
		Updating: false,
		Progress: &Progress{Phase: PhaseComplete, Message: "更新完成", Percentage: 100, FinishedAt: &fresh},
	})

	status := m.GetStatus()
	if status.Progress == nil || status.Progress.Phase != PhaseComplete {
		t.Errorf("窗口内的终态进度应保留: %+v", status.Progress)
	}
}

func TestGetStatus_非终态进度不过期(t *testing.T) {
	m := newTestManager()

	// 进行中的进度（无 FinishedAt）不受过期机制影响
	m.status.Store(&Status{
		Updating: true,
		Progress: &Progress{Phase: PhaseDownloading, Message: "正在下载...", Percentage: 50},
	})

	status := m.GetStatus()
	if status.Progress == nil || status.Progress.Phase != PhaseDownloading {
		t.Errorf("进行中的进度不应被剔除: %+v", status.Progress)
	}

	// setProgressError 写入的 failed 带时间戳，窗口内可读
	m.setProgressError("失败", "boom")
	if got := m.GetStatus().Progress; got == nil || got.Phase != PhaseFailed || got.FinishedAt == nil {
		t.Errorf("setProgressError 应写入带时间戳的 failed 进度: %+v", got)
	}
}

package update

import (
	"context"
	"testing"
)

// TestSetAndConsumeRestartBinary 校验待重启二进制路径的写入/读取语义：
// 读取即失效，保证换壳只发生一次（防止 update 与 rollback、超时兜底与正常退出重复换壳）。
func TestSetAndConsumeRestartBinary(t *testing.T) {
	m := &UpdateManager{}

	m.SetRestartBinary("/opt/team-api/team-api")
	if got := m.ConsumeRestartBinary(); got != "/opt/team-api/team-api" {
		t.Fatalf("ConsumeRestartBinary() = %q, want %q", got, "/opt/team-api/team-api")
	}

	// 二次读取应为空：换壳只允许发生一次
	if got := m.ConsumeRestartBinary(); got != "" {
		t.Fatalf("second ConsumeRestartBinary() = %q, want empty", got)
	}
}

// TestMaybeExecRestartNoop 无待重启版本时 MaybeExecRestart 必须是空操作
// （不 exec、不退出），保证普通关闭路径不被在线更新逻辑干扰。
func TestMaybeExecRestartNoop(t *testing.T) {
	// 清空单例可能残留的待重启状态，确保进入空操作分支
	if manager.ConsumeRestartBinary() != "" {
		t.Fatal("manager 单例残留了待重启状态，测试前置清理失败")
	}

	// 不应 panic、不应触发 exec/退出
	MaybeExecRestart(context.Background())
}

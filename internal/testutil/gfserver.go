// Package testutil 提供跨包共享的测试辅助工具。
package testutil

import (
	"fmt"
	"testing"

	"github.com/gogf/gf/v2/net/ghttp"
)

// StartGFServer 在 127.0.0.1 的随机端口上真实启动 ghttp.Server，
// 返回可访问的基础 URL，并通过 t.Cleanup 在测试结束后关闭服务。
//
// 必须走真实 Start()，禁止把 g.Server 直接塞进 httptest.NewServer：
// gf 只在 Start() 中初始化 sessionManager（ghttp_server.go），未经 Start 的
// server 处理请求时会在 handleAfterRequestDone 的 Session.Close() 中解引用
// nil manager 而 panic——响应虽已写出、panic 被 net/http recover，测试侥幸
// 通过，但每个请求都会向 stderr 刷一段 panic 栈。
//
// 绑定 127.0.0.1 避免触发 Windows 防火墙弹窗；端口 0 由内核分配避免冲突。
func StartGFServer(t *testing.T, s *ghttp.Server) string {
	t.Helper()
	s.SetAddr("127.0.0.1:0")
	s.SetDumpRouterMap(false)
	if err := s.Start(); err != nil {
		t.Fatalf("启动测试 server 失败: %v", err)
	}
	port := s.GetListenedPort()
	if port <= 0 {
		t.Fatalf("测试 server 未监听端口: %d", port)
	}
	t.Cleanup(func() {
		if err := s.Shutdown(); err != nil {
			t.Logf("关闭测试 server 失败: %v", err)
		}
	})
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

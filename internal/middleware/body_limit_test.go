package middleware

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/gclient"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/test/gtest"
)

// startBodyLimitServer 起一个只挂 RelayBodyLimit 中间件的测试服务。
// 通过 once 钩子把阈值压到 1KB，方便用普通请求体构造超限场景，
// 真实生效路径（Content-Length 比较 + 原生格式错误响应）与生产完全一致。
func startBodyLimitServer(t *gtest.T, name string) (*ghttp.Server, *gclient.Client) {
	relayBodyLimitOnce.Do(func() { relayBodyLimitBytes = 1024 })
	s := g.Server(name)
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(RelayBodyLimit)
		group.POST("/echo", func(r *ghttp.Request) {
			r.Response.Write("ok")
		})
		group.POST("/v1/messages", func(r *ghttp.Request) {
			r.Response.Write("ok")
		})
	})
	s.SetAddr("127.0.0.1:0")
	s.SetDumpRouterMap(false)
	s.Start()

	client := g.Client()
	client.SetPrefix(fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort()))
	return s, client
}

// TestRelayBodyLimit_PassThrough 未超限请求必须原样到达业务处理器
func TestRelayBodyLimit_PassThrough(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		s, client := startBodyLimitServer(t, "relay-body-limit-pass")
		defer s.Shutdown()

		resp, err := client.Post(context.Background(), "/echo", strings.Repeat("a", 64))
		t.AssertNil(err)
		defer resp.Close()

		t.Assert(resp.StatusCode, 200)
	})
}

// TestRelayBodyLimit_RejectOpenAIFormat 超限请求返回 413 + OpenAI 格式错误
func TestRelayBodyLimit_RejectOpenAIFormat(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		s, client := startBodyLimitServer(t, "relay-body-limit-openai")
		defer s.Shutdown()

		resp, err := client.Post(context.Background(), "/echo", strings.Repeat("a", 4096))
		t.AssertNil(err)
		defer resp.Close()

		t.Assert(resp.StatusCode, 413)
		body := resp.ReadAllString()
		t.Assert(strings.Contains(body, `"invalid_request_error"`), true)
		t.Assert(strings.Contains(body, "请求体过大"), true)
	})
}

// TestRelayBodyLimit_RejectClaudeFormat /v1/messages 超限请求返回 413 + Claude 格式错误
func TestRelayBodyLimit_RejectClaudeFormat(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		s, client := startBodyLimitServer(t, "relay-body-limit-claude")
		defer s.Shutdown()

		resp, err := client.Post(context.Background(), "/v1/messages", strings.Repeat("a", 4096))
		t.AssertNil(err)
		defer resp.Close()

		t.Assert(resp.StatusCode, 413)
		body := resp.ReadAllString()
		t.Assert(strings.Contains(body, `"request_too_large"`), true)
	})
}

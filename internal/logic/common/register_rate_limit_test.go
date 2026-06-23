package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/consts"
)

func requireRegisterRateLimitRedis(t *testing.T, ctx context.Context) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("Redis adapter is not configured: %v", r)
		}
	}()
	if _, err := g.Redis().Do(ctx, "PING"); err != nil {
		t.Skipf("Redis is not available: %v", err)
	}
}

func setRegisterRateLimitConfig(ctx context.Context, t *testing.T, key string, value string) {
	t.Helper()
	Config().cache.Set(ctx, key, value)
	t.Cleanup(func() {
		Config().cache.Delete(ctx, key)
	})
}

// TestCheckRegisterRateLimit 测试注册速率限制
func TestCheckRegisterRateLimit(t *testing.T) {
	ctx := context.Background()
	requireRegisterRateLimitRedis(t, ctx)

	// 测试IP级别限流
	t.Run("IP hourly limit", func(t *testing.T) {
		testIP := "192.168.1.100"

		// 清理测试数据
		hourlyKey := "register:ip:hourly:" + testIP
		g.Redis().Do(ctx, "DEL", hourlyKey)

		// 设置配置为每小时最多3次
		setRegisterRateLimitConfig(ctx, t, "register_ip_limit_per_hour", "3")

		// 前3次应该成功
		for i := 0; i < 3; i++ {
			err := CheckRegisterRateLimit(ctx, testIP)
			if err != nil {
				t.Errorf("第 %d 次注册应该成功，但得到错误: %v", i+1, err)
			}
		}

		// 第4次应该失败
		err := CheckRegisterRateLimit(ctx, testIP)
		if err == nil {
			t.Error("第4次注册应该被限流，但成功了")
		} else if err.Error() != consts.MsgIpRateLimitExceeded {
			t.Errorf("第4次注册应该返回IP限流错误，但得到: %v", err)
		}

		// 清理测试数据
		g.Redis().Do(ctx, "DEL", hourlyKey)
	})

	t.Run("IP daily limit", func(t *testing.T) {
		testIP := "192.168.1.101"

		// 清理测试数据
		dailyKey := "register:ip:daily:" + testIP
		g.Redis().Do(ctx, "DEL", dailyKey)

		// 设置配置为每天最多5次，并禁用每小时限制
		setRegisterRateLimitConfig(ctx, t, "register_ip_limit_per_day", "5")
		setRegisterRateLimitConfig(ctx, t, "register_ip_limit_per_hour", "0")

		// 前5次应该成功
		for i := 0; i < 5; i++ {
			err := CheckRegisterRateLimit(ctx, testIP)
			if err != nil {
				t.Errorf("第 %d 次注册应该成功，但得到错误: %v", i+1, err)
			}
		}

		// 第6次应该失败
		err := CheckRegisterRateLimit(ctx, testIP)
		if err == nil {
			t.Error("第6次注册应该被限流，但成功了")
		} else if err.Error() != consts.MsgIpRateLimitExceeded {
			t.Errorf("第6次注册应该返回IP限流错误，但得到: %v", err)
		}

		// 清理测试数据
		g.Redis().Do(ctx, "DEL", dailyKey)
	})
}

// TestGlobalRateLimit 测试全局限流
func TestGlobalRateLimit(t *testing.T) {
	ctx := context.Background()
	requireRegisterRateLimitRedis(t, ctx)

	// 清理测试数据
	now := time.Now()
	minuteKey := fmt.Sprintf("register:global:%d", now.Unix()/60)
	g.Redis().Do(ctx, "DEL", minuteKey)

	// 设置配置为每分钟最多2次，并禁用IP限制
	setRegisterRateLimitConfig(ctx, t, "register_global_limit_per_minute", "2")
	setRegisterRateLimitConfig(ctx, t, "register_ip_limit_per_hour", "0")
	setRegisterRateLimitConfig(ctx, t, "register_ip_limit_per_day", "0")

	// 前2次应该成功（使用不同IP）
	for i := 0; i < 2; i++ {
		testIP := fmt.Sprintf("192.168.1.%d", i+100)
		err := CheckRegisterRateLimit(ctx, testIP)
		if err != nil {
			t.Errorf("第 %d 次注册应该成功，但得到错误: %v", i+1, err)
		}
	}

	// 第3次应该失败（无论使用什么IP）
	err := CheckRegisterRateLimit(ctx, "192.168.1.200")
	if err == nil {
		t.Error("第3次注册应该被全局限流，但成功了")
	} else if err.Error() != consts.MsgGlobalRateLimitExceeded {
		t.Errorf("第3次注册应该返回全局限流错误，但得到: %v", err)
	}

	// 清理测试数据
	g.Redis().Do(ctx, "DEL", minuteKey)
}

// TestGetRegisterRateLimitStatus 测试获取限流状态
func TestGetRegisterRateLimitStatus(t *testing.T) {
	ctx := context.Background()
	requireRegisterRateLimitRedis(t, ctx)
	testIP := "192.168.1.102"

	// 清理测试数据
	hourlyKey := "register:ip:hourly:" + testIP
	dailyKey := "register:ip:daily:" + testIP
	g.Redis().Do(ctx, "DEL", hourlyKey)
	g.Redis().Do(ctx, "DEL", dailyKey)

	// 设置配置
	setRegisterRateLimitConfig(ctx, t, "register_ip_limit_per_hour", "5")
	setRegisterRateLimitConfig(ctx, t, "register_ip_limit_per_day", "20")

	// 注册2次
	g.Redis().Do(ctx, "SETEX", hourlyKey, 3600, "2")
	g.Redis().Do(ctx, "SETEX", dailyKey, 86400, "2")

	// 获取状态
	status := GetRegisterRateLimitStatus(ctx, testIP)

	// 验证状态
	if hourlyLimit, ok := status["hourly_limit"].(int); !ok || hourlyLimit != 5 {
		t.Errorf("期望每小时限制为5，得到: %v", status["hourly_limit"])
	}
	if hourlyRemaining, ok := status["hourly_remaining"].(int); !ok || hourlyRemaining != 3 {
		t.Errorf("期望每小时剩余3次，得到: %v", status["hourly_remaining"])
	}
	if dailyLimit, ok := status["daily_limit"].(int); !ok || dailyLimit != 20 {
		t.Errorf("期望每天限制为20，得到: %v", status["daily_limit"])
	}
	if dailyRemaining, ok := status["daily_remaining"].(int); !ok || dailyRemaining != 18 {
		t.Errorf("期望每天剩余18次，得到: %v", status["daily_remaining"])
	}

	// 清理测试数据
	g.Redis().Do(ctx, "DEL", hourlyKey)
	g.Redis().Do(ctx, "DEL", dailyKey)
}

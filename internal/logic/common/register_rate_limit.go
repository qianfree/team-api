package common

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/consts"
)

// CheckRegisterRateLimit 检查注册速率限制
// 1. IP级别限流（每小时/每天）
// 2. 全局限流（每分钟）
func CheckRegisterRateLimit(ctx context.Context, ipAddress string) error {
	// 1. 检查IP级别限流
	if err := checkIPRateLimit(ctx, ipAddress); err != nil {
		return err
	}

	// 2. 检查全局限流
	if err := checkGlobalRateLimit(ctx); err != nil {
		return err
	}

	return nil
}

// checkIPRateLimit 检查IP级别的注册限流
func checkIPRateLimit(ctx context.Context, ipAddress string) error {
	if ipAddress == "" {
		return nil // 无法获取IP时跳过检查
	}

	// 每小时限制
	hourlyLimit := Config().GetInt(ctx, "register_ip_limit_per_hour")
	if hourlyLimit > 0 {
		hourlyKey := fmt.Sprintf("register:ip:hourly:%s", ipAddress)
		count, err := incrementWithExpire(ctx, hourlyKey, 3600) // 1小时过期
		if err != nil {
			g.Log().Warningf(ctx, "IP注册限流检查失败: %v", err)
			return nil // Redis失败时不阻塞注册
		}
		if count > int64(hourlyLimit) {
			return NewBusinessError(consts.CodeIpRateLimitExceeded, consts.MsgIpRateLimitExceeded)
		}
	}

	// 每天限制
	dailyLimit := Config().GetInt(ctx, "register_ip_limit_per_day")
	if dailyLimit > 0 {
		dailyKey := fmt.Sprintf("register:ip:daily:%s", ipAddress)
		count, err := incrementWithExpire(ctx, dailyKey, 86400) // 24小时过期
		if err != nil {
			g.Log().Warningf(ctx, "IP注册限流检查失败: %v", err)
			return nil // Redis失败时不阻塞注册
		}
		if count > int64(dailyLimit) {
			return NewBusinessError(consts.CodeIpRateLimitExceeded, consts.MsgIpRateLimitExceeded)
		}
	}

	return nil
}

// checkGlobalRateLimit 检查全局注册速率限制
func checkGlobalRateLimit(ctx context.Context) error {
	globalLimit := Config().GetInt(ctx, "register_global_limit_per_minute")
	if globalLimit <= 0 {
		return nil // 未启用全局限流
	}

	// 使用当前分钟的Unix时间戳作为key，确保每分钟重置
	now := time.Now()
	minuteKey := fmt.Sprintf("register:global:%d", now.Unix()/60) // 每分钟一个key

	count, err := incrementWithExpire(ctx, minuteKey, 120) // 2分钟过期（确保当前分钟和下一分钟都能计数）
	if err != nil {
		g.Log().Warningf(ctx, "全局注册限流检查失败: %v", err)
		return nil // Redis失败时不阻塞注册
	}

	if count > int64(globalLimit) {
		return NewBusinessError(consts.CodeGlobalRateLimitExceeded, consts.MsgGlobalRateLimitExceeded)
	}

	return nil
}

// incrementWithExpire 原子递增计数器，首次递增时设置过期时间
// 使用Lua脚本确保原子性：INCR + EXPIRE（仅在第一次）
func incrementWithExpire(ctx context.Context, key string, expireSeconds int) (int64, error) {
	// Lua脚本：先递增，如果结果为1则设置过期时间
	luaScript := `
		local count = redis.call("INCR", KEYS[1])
		if count == 1 then
			redis.call("EXPIRE", KEYS[1], ARGV[1])
		end
		return count
	`

	result, err := g.Redis().Do(ctx, "EVAL", luaScript, 1, key, expireSeconds)
	if err != nil {
		return 0, err
	}

	return result.Int64(), nil
}

// GetRegisterRateLimitStatus 获取当前IP的注册限流状态（用于前端提示）
func GetRegisterRateLimitStatus(ctx context.Context, ipAddress string) map[string]any {
	status := make(map[string]any)

	if ipAddress == "" {
		return status
	}

	// 查询IP每小时限制
	hourlyLimit := Config().GetInt(ctx, "register_ip_limit_per_hour")
	if hourlyLimit > 0 {
		hourlyKey := fmt.Sprintf("register:ip:hourly:%s", ipAddress)
		hourlyCount, _ := g.Redis().Do(ctx, "GET", hourlyKey)
		if !hourlyCount.IsNil() {
			remaining := hourlyLimit - hourlyCount.Int()
			status["hourly_remaining"] = remaining
			status["hourly_limit"] = hourlyLimit
			if ttl, err := g.Redis().Do(ctx, "TTL", hourlyKey); err == nil {
				status["hourly_reset_seconds"] = ttl.Int()
			}
		} else {
			status["hourly_remaining"] = hourlyLimit
			status["hourly_limit"] = hourlyLimit
		}
	}

	// 查询IP每天限制
	dailyLimit := Config().GetInt(ctx, "register_ip_limit_per_day")
	if dailyLimit > 0 {
		dailyKey := fmt.Sprintf("register:ip:daily:%s", ipAddress)
		dailyCount, _ := g.Redis().Do(ctx, "GET", dailyKey)
		if !dailyCount.IsNil() {
			remaining := dailyLimit - dailyCount.Int()
			status["daily_remaining"] = remaining
			status["daily_limit"] = dailyLimit
			if ttl, err := g.Redis().Do(ctx, "TTL", dailyKey); err == nil {
				status["daily_reset_seconds"] = ttl.Int()
			}
		} else {
			status["daily_remaining"] = dailyLimit
			status["daily_limit"] = dailyLimit
		}
	}

	return status
}

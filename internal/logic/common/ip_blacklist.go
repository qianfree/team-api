package common

import (
	"context"
	"encoding/json"
	"net"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// IP 黑名单：配置存 sys_options（安全分类，复用通用设置读写链路，无独立表），
// 拦截计数存 Redis hash（field=IP，value=累计拦截次数）。计数仅作运营观测，
// 不落库，随 Redis 数据清空而丢失——这是有意为之的简化（需求明确缓存即可）。
const (
	OptionKeyIpBlacklistEnabled = "ip_blacklist_enabled"
	OptionKeyIpBlacklistList    = "ip_blacklist_list"

	ipBlacklistBlockedRedisKey = "ip_blacklist:blocked"
)

// IpBlacklistEntry 单条黑名单条目：精确 IP 或 CIDR 网段。
type IpBlacklistEntry struct {
	Raw  string
	ip   net.IP     // 精确 IP（CIDR 条目为 nil）
	cidr *net.IPNet // CIDR 网段（精确 IP 条目为 nil）
}

// Contains 判断客户端 IP 是否命中该条目。
func (e *IpBlacklistEntry) Contains(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if e.ip != nil {
		return e.ip.Equal(ip)
	}
	if e.cidr != nil {
		return e.cidr.Contains(ip)
	}
	return false
}

// splitIpBlacklistRaw 拆分黑名单配置值为候选条目：主体格式为 JSON 字符串数组
// （设置页保存），容错支持逗号分隔的裸文本（如手工改库后的脏格式）。
func splitIpBlacklistRaw(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err != nil {
			return nil, gerror.New("IP 黑名单必须是 JSON 字符串数组")
		}
		return arr, nil
	}
	return strings.Split(raw, ","), nil
}

// ParseIpBlacklistEntries 解析黑名单配置值为条目列表（去重、去空、跳过非法条目）。
// 非法条目静默跳过：保存入口已做格式校验，此处兜底脏数据，保证中间件永不 panic。
func ParseIpBlacklistEntries(raw string) []IpBlacklistEntry {
	parts, err := splitIpBlacklistRaw(raw)
	if err != nil {
		return nil
	}

	entries := make([]IpBlacklistEntry, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		if ip := net.ParseIP(p); ip != nil {
			seen[p] = struct{}{}
			entries = append(entries, IpBlacklistEntry{Raw: p, ip: ip})
			continue
		}
		if _, cidr, cidrErr := net.ParseCIDR(p); cidrErr == nil {
			seen[p] = struct{}{}
			entries = append(entries, IpBlacklistEntry{Raw: p, cidr: cidr})
		}
	}
	return entries
}

// ValidateIpBlacklistRaw 校验黑名单配置值：所有条目必须是合法 IP 或 CIDR。
// 坏条目入库后中间件会静默跳过，运营侧无从感知，必须在保存入口拦下。
func ValidateIpBlacklistRaw(raw string) error {
	parts, err := splitIpBlacklistRaw(raw)
	if err != nil {
		return err
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if net.ParseIP(p) == nil {
			if _, _, cidrErr := net.ParseCIDR(p); cidrErr != nil {
				return gerror.Newf("非法的 IP 或网段：%s", p)
			}
		}
	}
	return nil
}

// IncrIpBlacklistBlocked 黑名单 IP 被拦截时累加计数（best-effort）：
// 拦截决策已定，统计失败静默丢弃——不重试、不刷告警日志，避免 Redis 抖动放大成日志风暴。
func IncrIpBlacklistBlocked(ctx context.Context, ip string) {
	_, _ = g.Redis().Do(ctx, "HINCRBY", ipBlacklistBlockedRedisKey, ip, 1)
}

// GetIpBlacklistBlockStats 读取拦截统计：返回 {ip: 累计拦截次数}。
// Redis 异常时返回错误，由调用方降级（看板仍展示其余统计数据）。
func GetIpBlacklistBlockStats(ctx context.Context) (map[string]int64, error) {
	result, err := g.Redis().Do(ctx, "HGETALL", ipBlacklistBlockedRedisKey)
	if err != nil {
		return nil, err
	}
	if result.IsNil() || result.IsEmpty() {
		return map[string]int64{}, nil
	}

	counts := make(map[string]int64)
	for ip, v := range result.MapStrVar() {
		counts[ip] = v.Int64()
	}
	return counts, nil
}

package tenant

import (
	"os"
	"regexp"
	"sort"
	"testing"
)

// checkOpenPermissionCall 匹配 open 层 handler 里的 middleware.CheckOpenPermission(ctx, "xxx:yyy")。
var checkOpenPermissionCall = regexp.MustCompile(`CheckOpenPermission\(ctx,\s*"([^"]+)"\)`)

// TestOpenAppPermissionWhitelistCoversHandlers 保证「可授予的权限点」覆盖「实际被校验的权限点」。
//
// 两者分处两个包：白名单在 tenant（配置侧），校验在 open（执行侧）。若有人新增一个
// CheckOpenPermission("reports:read") 却忘了加进白名单，租户将永远无法把该权限授予应用 ——
// 接口对所有应用恒 403，且因为 sanitizeOpenAppPermissions 会把它判为「无效的权限点」，
// 连手工配置这条退路也没有。这类漂移不会有编译错误，只能靠测试兜住。
func TestOpenAppPermissionWhitelistCoversHandlers(t *testing.T) {
	src, err := os.ReadFile("../open/open_api.go")
	if err != nil {
		t.Fatalf("读取 open 层源码失败: %v", err)
	}

	matches := checkOpenPermissionCall.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("未在 open 层源码中匹配到任何 CheckOpenPermission 调用，正则可能已失效")
	}

	seen := make(map[string]bool)
	var missing []string
	for _, m := range matches {
		perm := m[1]
		if seen[perm] {
			continue
		}
		seen[perm] = true
		if !openAppPermissionPoints[perm] {
			missing = append(missing, perm)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("以下权限点被 handler 校验但不在 openAppPermissionPoints 白名单中，租户无法授予：%v", missing)
	}
}

func TestSanitizeOpenAppPermissions(t *testing.T) {
	t.Run("去重去空并保留合法权限点", func(t *testing.T) {
		got, err := sanitizeOpenAppPermissions([]string{"members:read", " ", "members:read", "usage:read"})
		if err != nil {
			t.Fatalf("预期通过，实际报错: %v", err)
		}
		want := []string{"members:read", "usage:read"}
		if len(got) != len(want) {
			t.Fatalf("预期 %v，实际 %v", want, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("预期 %v，实际 %v", want, got)
			}
		}
	})

	t.Run("拒绝未定义的权限点", func(t *testing.T) {
		// 拼错一个字母不会有编译错误，但会让该应用在对应接口上永久失效
		if _, err := sanitizeOpenAppPermissions([]string{"members:wirte"}); err == nil {
			t.Fatal("预期拒绝拼写错误的权限点，实际通过")
		}
	})

	t.Run("拒绝空集合", func(t *testing.T) {
		// 零权限的应用调用任何接口都是 403，配置时就该报错而不是留一个静默失效的应用
		if _, err := sanitizeOpenAppPermissions([]string{}); err == nil {
			t.Fatal("预期拒绝空权限集合，实际通过")
		}
	})
}

package update

import "testing"

func TestContainsDockerIndicator(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"docker cgroup 路径", "0::/docker/abc123", true},
		{"docker- 前缀", "12:pids:/docker-abc", true},
		{"包含 docker 字样", "1:name=systemd/docker.slice", true},
		{"k8s 环境", "0::/kubepods/pod123", false},
		{"普通 cgroup", "0::/user.slice/user-1000.slice", false},
		{"空内容", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := containsDockerIndicator(c.content); got != c.want {
				t.Errorf("containsDockerIndicator(%q) = %v, want %v", c.content, got, c.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	cases := []struct {
		s, sub string
		want   bool
	}{
		{"hello docker world", "docker", true},
		{"hello docker world", "world", true},
		{"hello", "hello", true}, // 与自身相等
		{"abc", "abcd", false},   // 子串比源串长
		{"", "", true},           // 双空串
		{"abc", "", true},        // 空子串视为匹配（containsDockerIndicator 已另行过滤空内容）
	}
	for _, c := range cases {
		if got := contains(c.s, c.sub); got != c.want {
			t.Errorf("contains(%q, %q) = %v, want %v", c.s, c.sub, got, c.want)
		}
	}
}

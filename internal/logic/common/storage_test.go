package common

import "testing"

// TestProviderFullKeyAppliesPrefixOnce 守护 storage_path 前缀不变量：每个 provider 恰好
// 为 key 加一次配置的路径前缀，且对已带前缀的 key 幂等。新行持久化 raw key（相对前缀），
// 旧行持久化的是 Upload 返回的已带前缀 key。两者都必须解析到同一个「只加一次前缀」的对象，
// 从而 Download/Delete/PresignedURL 无需数据迁移即可正常工作。
func TestProviderFullKeyAppliesPrefixOnce(t *testing.T) {
	const prefix = "team-api"
	const rawKey = "42/1700000000/task_abc.png"
	const want = prefix + "/" + rawKey

	cases := []struct {
		name    string
		fullKey func(string) string
	}{
		{"s3", (&S3StorageProvider{prefix: prefix}).fullKey},
		{"oss", (&OSSStorageProvider{prefix: prefix}).fullKey},
		{"cos", (&COSStorageProvider{prefix: prefix}).fullKey},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 新行（raw key）-> 只加一次前缀。
			if got := c.fullKey(rawKey); got != want {
				t.Fatalf("fullKey(raw) = %q, want %q", got, want)
			}
			// 旧行（已带前缀的 key）-> 原样返回（幂等），不再二次加前缀。
			// 这正是让旧 fil_files 行仍能正确解析的关键。
			if got := c.fullKey(want); got != want {
				t.Fatalf("fullKey(prefixed) = %q, want %q (must be idempotent for legacy rows)", got, want)
			}
		})
	}
}

// TestProviderFullKeyNoPrefix 验证前缀为空时 key 原样返回，即未配置前缀时 key 往返一致。
func TestProviderFullKeyNoPrefix(t *testing.T) {
	const rawKey = "exports/tenant_7/20240101_000000.json"
	providers := map[string]func(string) string{
		"s3":  (&S3StorageProvider{}).fullKey,
		"oss": (&OSSStorageProvider{}).fullKey,
		"cos": (&COSStorageProvider{}).fullKey,
	}
	for name, fullKey := range providers {
		if got := fullKey(rawKey); got != rawKey {
			t.Fatalf("%s: fullKey(%q) with empty prefix = %q, want unchanged", name, rawKey, got)
		}
	}
}

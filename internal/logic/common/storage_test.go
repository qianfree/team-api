package common

import "testing"

// TestProviderFullKeyAppliesPrefixOnce guards the invariant behind the
// storage_path fix: every provider prepends the configured path prefix exactly
// once to whatever key it is given. FileService therefore MUST persist the RAW
// key (relative to the prefix) — if it stored the already-prefixed key returned
// by Upload, Download/Delete/PresignedURL would prefix it a second time and
// target a non-existent object (e.g. "team-api/team-api/…").
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
			// Raw key -> single prefix (the correct, fixed behavior).
			if got := c.fullKey(rawKey); got != want {
				t.Fatalf("fullKey(raw) = %q, want %q", got, want)
			}
			// An already-prefixed key would be double-prefixed — this is exactly
			// the bug we avoid by storing the raw key in fil_files.storage_path.
			if got := c.fullKey(want); got != prefix+"/"+want {
				t.Fatalf("fullKey(prefixed) = %q, want double-prefixed %q (documents why raw key must be stored)", got, prefix+"/"+want)
			}
		})
	}
}

// TestProviderFullKeyNoPrefix verifies that with an empty prefix the key is
// returned unchanged, so keys round-trip identically when no prefix is set.
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

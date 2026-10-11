package plugin

import (
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	uc "github.com/qianfree/team-api/internal/utility/crypto"
)

var testEncKey = func() []byte {
	key, err := uc.GetEncryptionKey(strings.Repeat("ab", 32))
	if err != nil {
		panic(err)
	}
	return key
}()

func TestMergeSecretFields(t *testing.T) {
	secretKeys := map[string]bool{"ai_api_key": true}
	current := g.Map{"ai_base_url": "http://x/v1", "ai_api_key": "v1:oldcipher"}

	// 明文 → 加密存储
	out, err := mergeSecretFields(secretKeys, current, g.Map{
		"ai_base_url": "http://y/v1", "ai_api_key": "sk-newkey",
	}, testEncKey)
	if err != nil {
		t.Fatalf("merge 失败: %v", err)
	}
	if out["ai_base_url"] != "http://y/v1" {
		t.Fatalf("非敏感字段应透传: %v", out["ai_base_url"])
	}
	stored, _ := out["ai_api_key"].(string)
	if !looksEncrypted(stored) {
		t.Fatalf("敏感字段应加密存储: %q", stored)
	}
	plain, err := uc.DecryptString(testEncKey, stored)
	if err != nil || plain != "sk-newkey" {
		t.Fatalf("密文应可解回原文: %q %v", plain, err)
	}

	// 空值 → 沿用现值
	out, err = mergeSecretFields(secretKeys, current, g.Map{"ai_api_key": ""}, testEncKey)
	if err != nil {
		t.Fatalf("merge 失败: %v", err)
	}
	if out["ai_api_key"] != "v1:oldcipher" {
		t.Fatalf("空值应沿用现值: %v", out["ai_api_key"])
	}

	// 掩码值 → 沿用现值（界面回显不清掉真实密钥）
	out, err = mergeSecretFields(secretKeys, current, g.Map{"ai_api_key": SecretMask}, testEncKey)
	if err != nil {
		t.Fatalf("merge 失败: %v", err)
	}
	if out["ai_api_key"] != "v1:oldcipher" {
		t.Fatalf("掩码应沿用现值: %v", out["ai_api_key"])
	}

	// 已是密文 → 幂等保留（重复提交不二次加密）
	out, err = mergeSecretFields(secretKeys, current, g.Map{"ai_api_key": "v1:already"}, testEncKey)
	if err != nil {
		t.Fatalf("merge 失败: %v", err)
	}
	if out["ai_api_key"] != "v1:already" {
		t.Fatalf("密文应幂等保留: %v", out["ai_api_key"])
	}
}

func TestMaskSecretFields(t *testing.T) {
	secretKeys := map[string]bool{"ai_api_key": true}
	masked := maskSecretFields(secretKeys, g.Map{
		"ai_base_url": "http://x/v1",
		"ai_api_key":  "v1:somecipher",
	})
	if masked["ai_api_key"] != SecretMask {
		t.Fatalf("已配置的敏感字段应显示掩码: %v", masked["ai_api_key"])
	}
	if masked["ai_base_url"] != "http://x/v1" {
		t.Fatalf("非敏感字段不应被掩码: %v", masked["ai_base_url"])
	}

	masked = maskSecretFields(secretKeys, g.Map{"ai_api_key": ""})
	if masked["ai_api_key"] != "" {
		t.Fatalf("未配置的敏感字段应显示空串: %v", masked["ai_api_key"])
	}
}

func TestDecryptSecretFields(t *testing.T) {
	secretKeys := map[string]bool{"ai_api_key": true}
	enc, _ := uc.EncryptString(testEncKey, "sk-real")

	out, err := decryptSecretFields(secretKeys, g.Map{"ai_api_key": enc, "model": "m"}, testEncKey)
	if err != nil {
		t.Fatalf("decrypt 失败: %v", err)
	}
	if out["ai_api_key"] != "sk-real" || out["model"] != "m" {
		t.Fatalf("解密结果错误: %v", out)
	}

	// 历史明文兼容：不带 v1 前缀的值原样返回
	out, _ = decryptSecretFields(secretKeys, g.Map{"ai_api_key": "plaintext-legacy"}, testEncKey)
	if out["ai_api_key"] != "plaintext-legacy" {
		t.Fatalf("历史明文应原样返回: %v", out["ai_api_key"])
	}

	// 坏密文：不中断读取，返回空串
	out, _ = decryptSecretFields(secretKeys, g.Map{"ai_api_key": "v1:garbage!!"}, testEncKey)
	if out["ai_api_key"] != "" {
		t.Fatalf("坏密文应返回空串: %v", out["ai_api_key"])
	}
}

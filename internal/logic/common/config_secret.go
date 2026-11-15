package common

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	uc "github.com/qianfree/team-api/internal/utility/crypto"
)

// 设置项敏感值加密（AES-256-GCM，密文 v1: 前缀）：
// 注册表中 Encrypted: true 的键，写入前加密、读取时解密，密钥与渠道 Key/
// 插件配置共用 crypto.encryptionKey——同一密钥体系下各读取方密文互通。
// 界面掩码（"******"）语义由 UpdateCategory 统一处理（掩码=保持现值，不走到加密）。

// optionsEncryptionKey 读取设置项加密密钥。
func optionsEncryptionKey(ctx context.Context) ([]byte, error) {
	hexKey := g.Cfg().MustGet(ctx, "crypto.encryptionKey").String()
	if hexKey == "" {
		return nil, gerror.New("crypto.encryptionKey 未配置，无法处理加密设置项")
	}
	return uc.GetEncryptionKey(hexKey)
}

// encryptOptionValue 加密设置值（已是 v1: 密文则原样返回，支持重复提交幂等）。
func encryptOptionValue(ctx context.Context, value string) (string, error) {
	if value == "" || optionValueEncrypted(value) {
		return value, nil
	}
	key, err := optionsEncryptionKey(ctx)
	if err != nil {
		return "", err
	}
	return uc.EncryptString(key, value)
}

// optionValueEncrypted 判断值是否已是 v1 密文形态（与 utility/crypto 的版本前缀约定一致）。
func optionValueEncrypted(s string) bool {
	return strings.HasPrefix(s, "v1:")
}

// GetSecret 读取加密设置项并解密（业务内部使用，如分析 AI 密钥）。
// 解密失败（密钥轮换后的历史数据）不中断读取，返回空串，调用方可给出明确错误；
// 未加密的历史明文值原样返回，与插件配置解密的兼容语义一致。
func (s *ConfigService) GetSecret(ctx context.Context, key string) string {
	val := s.GetOption(ctx, key)
	if val == "" || !optionValueEncrypted(val) {
		return val
	}
	cipherKey, err := optionsEncryptionKey(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[Config] 读取加密设置项 %s 失败（加密密钥不可用）: %v", key, err)
		return ""
	}
	plain, err := uc.DecryptString(cipherKey, val)
	if err != nil {
		g.Log().Warningf(ctx, "[Config] 解密设置项 %s 失败（密钥可能已轮换）", key)
		return ""
	}
	return plain
}

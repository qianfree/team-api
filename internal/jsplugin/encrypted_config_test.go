package jsplugin

import (
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// TestValidateEncryptedConfigTypes 加密字段类型限制：string/text 放行，
// 其余类型（select/int/json）拒绝——加密写路径只覆盖 string/text，
// 其他类型声明 encrypted 会明文落库而读取侧却走解密。
func TestValidateEncryptedConfigTypes(t *testing.T) {
	cases := []struct {
		name    string
		config  []contract.ConfigField
		wantErr bool
	}{
		{"无配置", nil, false},
		{"加密 string 放行", []contract.ConfigField{{Key: "ak", Label: "AK", Type: "string", Encrypted: true}}, false},
		{"加密 text 放行", []contract.ConfigField{{Key: "ak", Label: "AK", Type: "text", Encrypted: true}}, false},
		{"非加密任意类型放行", []contract.ConfigField{{Key: "mode", Label: "模式", Type: "select", Options: []string{"a"}}}, false},
		{"加密 json 拒绝", []contract.ConfigField{{Key: "cred", Label: "凭据", Type: "json", Encrypted: true}}, true},
		{"加密 select 拒绝", []contract.ConfigField{{Key: "env", Label: "环境", Type: "select", Options: []string{"prod"}, Encrypted: true}}, true},
		{"加密 int 拒绝", []contract.ConfigField{{Key: "n", Label: "N", Type: "int", Encrypted: true}}, true},
	}
	for _, c := range cases {
		err := validateEncryptedConfigTypes(&contract.Manifest{Config: c.config})
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v, wantErr=%v", c.name, err, c.wantErr)
		}
	}
	// golden 内置插件声明 encrypted+string，不受影响（防回归）
	if err := validateEncryptedConfigTypes(mustCompile(t, goldenSource(t)).Manifest); err != nil {
		t.Fatalf("golden 内置插件应通过校验: %v", err)
	}
}

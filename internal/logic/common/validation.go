package common

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/qianfree/team-api/internal/consts"
)

// ValidateUsername 校验用户名格式：仅允许英文字母和数字，不能为纯数字，长度3-50
func ValidateUsername(username string) error {
	if len(username) < 3 || len(username) > 50 {
		return gerror.New("用户名长度为3-50位")
	}
	allDigit := true
	for _, c := range username {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			allDigit = false
		} else if c >= '0' && c <= '9' {
			// digits are ok
		} else {
			return gerror.New("用户名仅支持英文字母和数字，不能包含特殊字符或中文")
		}
	}
	if allDigit {
		return gerror.New("用户名不能为纯数字")
	}
	return nil
}

// ValidatePassword 校验密码强度：至少8位，且同时包含字母（不区分大小写）和数字
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return gerror.New("密码长度不能少于8位")
	}
	var hasLetter, hasDigit bool
	for _, c := range password {
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
			hasLetter = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return gerror.New("密码必须同时包含字母和数字")
	}
	return nil
}

// ValidateTenantRole 校验租户角色是否合法
func ValidateTenantRole(role string) error {
	if role != "admin" && role != "member" {
		return gerror.New("角色无效")
	}
	return nil
}

// ValidateAdminRole 校验管理后台角色是否合法
func ValidateAdminRole(role string) error {
	if role != "admin" && role != "super_admin" {
		return gerror.New("角色无效")
	}
	return nil
}

// ValidateDateParam 校验日期参数格式是否为 YYYY-MM-DD
func ValidateDateParam(date string, fieldName string) error {
	if date == "" {
		return nil
	}
	_, err := time.Parse("2006-01-02", date)
	if err != nil {
		return gerror.NewCode(gcode.New(consts.CodeBadRequest, "", nil), "%s格式无效，应为 YYYY-MM-DD", fieldName)
	}
	return nil
}

// ValidateForbiddenWords 校验名称是否包含系统禁用词
// 从系统配置 register_forbidden_words 读取逗号分隔的禁用词列表，对 value 做大小写不敏感的包含检查
func ValidateForbiddenWords(ctx context.Context, value, fieldName string) error {
	raw := Config().GetString(ctx, "register_forbidden_words")
	if raw == "" {
		return nil
	}
	lowerVal := strings.ToLower(value)
	for _, word := range strings.Split(raw, ",") {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		if strings.Contains(lowerVal, strings.ToLower(word)) {
			return gerror.NewCodef(gcode.New(consts.CodeForbiddenWord, consts.MsgForbiddenWord, nil),
				"%s包含禁用词「%s」，请修改后重试", fieldName, word)
		}
	}
	return nil
}

// TenantNameMaxDisplayWidth 组织名称最大显示宽度。
// 采用“显示宽度”模型：1 个汉字算 2、其余字符算 1，总宽度上限 16。
// 等价于“汉字最多 8 个（8×2=16）”或“字母最多 16 个（16×1=16）”，中英文混排按比例折算。
const TenantNameMaxDisplayWidth = 16

// displayWidth 计算“显示宽度”：CJK 汉字（unicode.Han）每个算 2，其余字符每个算 1。
// 用于组织名称等中英文混排场景的长度衡量，使 8 个汉字与 16 个字母视觉等宽。
func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

// ValidateTenantName 校验组织名称：显示宽度需在 2~TenantNameMaxDisplayWidth 之间。
// 配合 API 层使用，规则为“汉字最多 8 个、字母最多 16 个”。
func ValidateTenantName(name string) error {
	w := displayWidth(name)
	if w < 2 {
		return gerror.New("组织名称长度不能少于 2 个字符")
	}
	if w > TenantNameMaxDisplayWidth {
		return gerror.New(consts.MsgInvalidTenantName)
	}
	return nil
}

// TruncateToDisplayWidth 按显示宽度截断字符串，使结果宽度不超过 max。
// 用于自动生成的组织名称兜底（如“xxx 的组织”），避免用户名过长导致存入超长数据。
func TruncateToDisplayWidth(s string, max int) string {
	width := 0
	out := make([]rune, 0, len(s))
	for _, r := range s {
		rw := 1
		if unicode.Is(unicode.Han, r) {
			rw = 2
		}
		if width+rw > max {
			break
		}
		width += rw
		out = append(out, r)
	}
	return string(out)
}

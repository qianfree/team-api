package common

import (
	"time"
	"unicode"

	"github.com/gogf/gf/v2/errors/gerror"
)

// ValidatePassword 校验密码强度：至少8位，包含大写、小写、数字
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return gerror.New("密码长度不能少于8位")
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return gerror.New("密码必须包含大写字母、小写字母和数字")
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
		return gerror.Newf("%s格式无效，应为 YYYY-MM-DD", fieldName)
	}
	return nil
}

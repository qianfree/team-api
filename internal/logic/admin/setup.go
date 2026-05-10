package admin

import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)

// AdminExists checks whether any admin user exists in the database.
func AdminExists(ctx context.Context) (bool, error) {
	count, err := dao.SysAdminUsers.Ctx(ctx).Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ValidateSetupUsername validates the username format.
func ValidateSetupUsername(username string) error {
	if !usernameRegex.MatchString(username) {
		return common.NewBusinessError(consts.CodeSetupInvalidUsername, consts.MsgSetupInvalidUsername)
	}
	return nil
}

// CreateAdmin creates a new super admin account.
func CreateAdmin(ctx context.Context, username, password, displayName string) error {
	exists, err := AdminExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return common.NewBusinessError(consts.CodeSetupCompleted, consts.MsgSetupCompleted)
	}

	if err := ValidateSetupUsername(username); err != nil {
		return err
	}
	if err := common.ValidatePassword(password); err != nil {
		return common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	if displayName == "" {
		displayName = username
	}

	passwordHash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}

	_, err = dao.SysAdminUsers.Ctx(ctx).Data(do.SysAdminUsers{
		Username:     strings.TrimSpace(username),
		PasswordHash: passwordHash,
		DisplayName:  strings.TrimSpace(displayName),
		Role:         "super_admin",
		Status:       "active",
	}).Insert()
	if err != nil {
		if isDuplicateKeyError(err) {
			return common.NewBusinessError(consts.CodeSetupCompleted, consts.MsgSetupCompleted)
		}
		return err
	}

	g.Log().Infof(ctx, "管理员账号创建成功: username=%s", username)
	return nil
}

// AutoInitAdmin checks INIT_ADMIN_USERNAME/INIT_ADMIN_PASSWORD env vars.
func AutoInitAdmin(ctx context.Context) (bool, error) {
	username := os.Getenv("INIT_ADMIN_USERNAME")
	password := os.Getenv("INIT_ADMIN_PASSWORD")
	if username == "" || password == "" {
		return false, nil
	}

	exists, err := AdminExists(ctx)
	if err != nil {
		return false, err
	}
	if exists {
		g.Log().Info(ctx, "管理员已存在，跳过自动初始化")
		return true, nil
	}

	if err := CreateAdmin(ctx, username, password, username); err != nil {
		return false, err
	}
	g.Log().Info(ctx, "通过环境变量自动初始化管理员完成")
	return true, nil
}

func isDuplicateKeyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

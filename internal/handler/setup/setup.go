package setup

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"unicode/utf8"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/logic/admin"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/packed"
	"github.com/qianfree/team-api/internal/response"
)

var setupMode atomic.Bool

// SetSetupMode sets whether the system is in setup mode.
func SetSetupMode(enabled bool) {
	setupMode.Store(enabled)
}

// IsSetupMode returns whether the system is currently in setup mode.
func IsSetupMode() bool {
	return setupMode.Load()
}

// SetSetupComplete marks setup as complete.
func SetSetupComplete() {
	setupMode.Store(false)
}

// HandleSetupPage serves the embedded setup HTML page.
func HandleSetupPage(r *ghttp.Request) {
	r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.Response.Write(packed.SetupHTML)
}

// HandleSetupStatus returns system initialization status.
func HandleSetupStatus(r *ghttp.Request) {
	exists, err := admin.AdminExists(r.Context())
	if err != nil {
		response.Error(r, err)
		return
	}
	response.Success(r, g.Map{"initialized": exists})
}

// HandleSetupInitialize creates the admin account.
func HandleSetupInitialize(r *ghttp.Request) {
	var req struct {
		Username        string `json:"username"`
		DisplayName     string `json:"displayName"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirmPassword"`
		Currency        string `json:"currency"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		response.ErrorMsg(r, consts.CodeBadRequest, "请求格式错误")
		return
	}

	// 本位币：USD/CNY，未选择时默认 USD；初始化写入后不可再更改
	currency := req.Currency
	if currency == "" {
		currency = consts.BillingCurrencyUSD
	}
	if currency != consts.BillingCurrencyUSD && currency != consts.BillingCurrencyCNY {
		response.ErrorMsg(r, consts.CodeBadRequest, "本位币仅支持 USD 或 CNY")
		return
	}

	// Validate required fields
	if req.Username == "" || req.Password == "" {
		response.ErrorMsg(r, consts.CodeBadRequest, "用户名和密码不能为空")
		return
	}

	// Validate username format
	if utf8.RuneCountInString(req.Username) < 3 || utf8.RuneCountInString(req.Username) > 20 {
		response.ErrorWithCode(r, http.StatusUnprocessableEntity, consts.CodeSetupInvalidUsername, consts.MsgSetupInvalidUsername)
		return
	}

	// Validate password strength
	if err := common.ValidatePassword(req.Password); err != nil {
		response.ErrorWithCode(r, http.StatusUnprocessableEntity, consts.CodePasswordTooWeak, err.Error())
		return
	}

	// Validate password confirmation
	if req.Password != req.ConfirmPassword {
		response.ErrorWithCode(r, http.StatusUnprocessableEntity, consts.CodeSetupPasswordMismatch, consts.MsgSetupPasswordMismatch)
		return
	}

	// Truncate display name
	if utf8.RuneCountInString(req.DisplayName) > 50 {
		req.DisplayName = string([]rune(req.DisplayName)[:50])
	}

	if err := admin.CreateAdmin(r.Context(), req.Username, req.Password, req.DisplayName); err != nil {
		response.Error(r, err)
		return
	}

	// 本位币随初始化写入（首次落库，此后 SetOption 只读保护拒绝修改）
	if err := common.Config().SetOption(r.Context(), consts.OptionKeyBillingCurrency, currency); err != nil {
		response.Error(r, err)
		return
	}
	SetSetupComplete()
	response.Success(r, nil)
}

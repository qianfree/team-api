package v1

import "github.com/gogf/gf/v2/frame/g"

// CaptchaGenerateReq 生成滑块验证码请求
type CaptchaGenerateReq struct {
	g.Meta `path:"/" method:"get" mime:"json" tags:"公共-验证码" summary:"生成滑块验证码" group:"public" middleware:"-"`
}

type CaptchaGenerateRes struct {
	CaptchaKey  string `json:"captcha_key"`
	MasterImage string `json:"master_image"`
	TileImage   string `json:"tile_image"`
	TileY       int    `json:"tile_y"`
}

// CaptchaVerifyReq 验证滑块验证码请求
type CaptchaVerifyReq struct {
	g.Meta     `path:"/verify" method:"post" mime:"json" tags:"公共-验证码" summary:"验证滑块验证码" group:"public" middleware:"-"`
	CaptchaKey string `json:"captcha_key" v:"required#请提供验证码key" dc:"验证码key"`
	CaptchaX   int    `json:"captcha_x" v:"required|min:0#请提供滑块X坐标|X坐标不能为负数" dc:"滑块X坐标"`
}

type CaptchaVerifyRes struct {
	Verified bool `json:"verified"`
}

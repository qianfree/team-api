package public

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/payment"
)

// HandlePaymentCallback 处理支付渠道的异步回调通知（支持 GET/POST）。
// channel 从 URL 路径获取（如 "epay"）。
func HandlePaymentCallback(r *ghttp.Request) {
	channel := r.Get("channel").String()
	if channel == "" {
		r.Response.WriteStatus(400)
		r.Response.Write("fail")
		return
	}

	err := payment.ProcessCallback(r.Context(), r.Request, channel)
	if err != nil {
		r.Response.WriteStatus(200)
		r.Response.Write("fail")
		return
	}

	r.Response.WriteStatus(200)
	r.Response.Write("success")
}

// HandlePaymentEpayReturn 处理 EPay 支付完成后的浏览器同步跳转（支持 GET/POST）。
// 验签成功后尝试完成订单，然后重定向到前端页面。
func HandlePaymentEpayReturn(r *ghttp.Request) {
	// 先验签，确保跳转来源可信
	err := payment.VerifyEpayReturn(r.Context(), r.Request)
	if err != nil {
		r.Response.WriteStatus(302)
		r.Response.Header().Set("Location", "/console/wallet?pay=fail")
		return
	}

	// 验签通过，尝试完成订单（幂等，已处理过的不会重复处理）
	_ = payment.ProcessCallback(r.Context(), r.Request, "epay")

	r.Response.WriteStatus(302)
	r.Response.Header().Set("Location", "/console/wallet?pay=success")
}

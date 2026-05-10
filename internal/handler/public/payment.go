package public

import (
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gconv"

	"github.com/qianfree/team-api/internal/logic/payment"
)

// handlePaymentCallback 处理支付渠道的异步回调通知（Epay 等支持 GET/POST 两种方式）。
func HandlePaymentCallback(r *ghttp.Request) {
	channelID := gconv.Int64(r.Get("channel_id").String())
	if channelID <= 0 {
		r.Response.WriteStatus(400)
		r.Response.Write("fail")
		return
	}

	err := payment.ProcessCallback(r.Context(), r.Request, channelID)
	if err != nil {
		r.Response.WriteStatus(200)
		r.Response.Write("fail")
		return
	}

	r.Response.WriteStatus(200)
	r.Response.Write("success")
}

// handlePaymentEpayReturn 处理 Epay 支付完成后的浏览器同步跳转。
func HandlePaymentEpayReturn(r *ghttp.Request) {
	channelID := gconv.Int64(r.Get("channel_id").String())
	if channelID <= 0 {
		r.Response.WriteStatus(302)
		r.Response.Header().Set("Location", "/console/topup?pay=fail")
		return
	}

	err := payment.ProcessCallback(r.Context(), r.Request, channelID)
	if err != nil {
		r.Response.WriteStatus(302)
		r.Response.Header().Set("Location", "/console/topup?pay=fail")
		return
	}
	r.Response.WriteStatus(302)
	r.Response.Header().Set("Location", "/console/topup?pay=success")
}

// handlePaymentStripeWebhook 处理 Stripe Webhook 回调。
func HandlePaymentStripeWebhook(r *ghttp.Request) {
	channelID := gconv.Int64(r.Get("channel_id").String())
	if channelID <= 0 {
		r.Response.WriteStatus(400)
		return
	}

	err := payment.ProcessCallback(r.Context(), r.Request, channelID)
	if err != nil {
		r.Response.WriteStatus(400)
		return
	}

	r.Response.WriteStatus(200)
}

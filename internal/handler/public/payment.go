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

// HandlePaymentEpayReturn 处理 Epay 支付完成后的浏览器同步跳转。
// 仅查询订单状态决定跳转目标，履约由异步回调完成。
func HandlePaymentEpayReturn(r *ghttp.Request) {
	orderNo := r.GetQuery("out_trade_no").String()
	paid := payment.QueryOrderPaid(r.Context(), orderNo)
	if paid {
		r.Response.RedirectTo("/console/wallet?pay=success")
		return
	}
	r.Response.RedirectTo("/console/wallet?pay=fail")
}

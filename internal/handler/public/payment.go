package public

import (
	"time"

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
	if orderNo == "" {
		r.Response.RedirectTo("/console/wallet?pay=fail")
		return
	}

	ctx := r.Context()
	// 异步 notify 可能晚于浏览器回跳到达：最多轮询约 4 秒等待回调入账。
	// 仍未到账不一定是失败（回调可能仍在途），引导到"处理中"页由前端继续刷新，
	// 避免给用户展示误导性的"失败"。
	paid := payment.QueryOrderPaid(ctx, orderNo)
	for i := 0; !paid && i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		paid = payment.QueryOrderPaid(ctx, orderNo)
	}

	if paid {
		r.Response.RedirectTo("/console/wallet?pay=success")
		return
	}
	r.Response.RedirectTo("/console/wallet?pay=processing")
}

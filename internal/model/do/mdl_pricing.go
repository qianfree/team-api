// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlPricing is the golang structure of table mdl_pricing for DAO operations like Where/Data.
type MdlPricing struct {
	g.Meta          `orm:"table:mdl_pricing, do:true"`
	Id              any         //
	ModelId         any         // 关联模型ID
	BillingMode     any         // 计费模式：token（按量）/ per_request（按次）/ tiered（阶梯按量）
	CreatedAt       *gtime.Time //
	UpdatedAt       *gtime.Time //
	PriceNote       any         // 价格说明（仅管理后台可见，调价背景等内部备注），仅 min_tokens=0 锚点行使用，NULL=无
	DiscountLabel   any         // 折扣标签（对外展示，如"7折起"、"限时5折"），仅 min_tokens=0 锚点行使用，NULL/空=不展示
	PriceChangeNote any         // 价格调整说明（对外展示，提示用户价格有变动，如"9月1日起输入价下调"），仅 min_tokens=0 锚点行使用，NULL/空=不展示
	Pricing         any         // 计费详情（JSONB，按 billing_mode 单模式存储）：token={input_price,output_price,cache_read_price,cache_creation_price}；tiered={tiers:[{min_tokens,max_tokens,input_price,output_price}],cache_read_price,cache_creation_price}；per_request={price}；per_second={unit:"second",prices:{分辨率规格:每秒单价,"*":兜底价}}；顶层可含 time_segments（时段定价）
}

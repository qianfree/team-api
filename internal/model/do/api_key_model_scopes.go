// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ApiKeyModelScopes is the golang structure of table api_key_model_scopes for DAO operations like Where/Data.
type ApiKeyModelScopes struct {
	g.Meta    `orm:"table:api_key_model_scopes, do:true"`
	Id        any         // 主键ID
	ApiKeyId  any         // 关联 API Key ID
	ModelName any         // 允许调用的模型名
	CreatedAt *gtime.Time // 创建时间
	UpdatedAt *gtime.Time // 更新时间
}

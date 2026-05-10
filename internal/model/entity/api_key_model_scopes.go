// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ApiKeyModelScopes is the golang structure for table api_key_model_scopes.
type ApiKeyModelScopes struct {
	Id        int64       `json:"id"         orm:"id"         description:"主键ID"`          // 主键ID
	ApiKeyId  int64       `json:"api_key_id" orm:"api_key_id" description:"关联 API Key ID"` // 关联 API Key ID
	ModelName string      `json:"model_name" orm:"model_name" description:"允许调用的模型名"`      // 允许调用的模型名
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:"创建时间"`          // 创建时间
	UpdatedAt *gtime.Time `json:"updated_at" orm:"updated_at" description:"更新时间"`          // 更新时间
}

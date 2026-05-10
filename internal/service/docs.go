// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	"encoding/json"

	v1 "github.com/qianfree/team-api/api/docs/v1"
)

type (
	IDocs interface {
		OpenAPISpec(ctx context.Context, _ *v1.OpenAPISpecReq) (json.RawMessage, error)
	}
)

var (
	localDocs IDocs
)

func Docs() IDocs {
	if localDocs == nil {
		panic("implement not found for interface IDocs, forgot register?")
	}
	return localDocs
}

func RegisterDocs(i IDocs) {
	localDocs = i
}
